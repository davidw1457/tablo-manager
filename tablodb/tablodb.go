package tablodb

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"tablo-manager/tabloapi"
	"tablo-manager/utils"

	_ "github.com/mattn/go-sqlite3"
)

const userRWX = 0700 // unix-style octal permission

type TabloDB struct {
	database *sql.DB
	log      *log.Logger
}

type QueueRecord struct {
	QueueID    int
	Action     string
	Details    string
	ExportPath string
}

func New(ipAddress string, name string, serverID string, directory string) (TabloDB, error) {
	var tabloDB TabloDB
	logFile, err := os.OpenFile(directory+string(os.PathSeparator)+"main.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, userRWX)
	if err != nil {
		return tabloDB, err
	}
	tabloDB.log = log.New(io.MultiWriter(logFile, os.Stdout), "tablodb "+serverID+": ", log.LstdFlags)

	databaseFile := directory + string(os.PathSeparator) + utils.SanitizeFileString(serverID) + ".cache"
	tabloDB.log.Printf("creating %s\n", databaseFile)
	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		tabloDB.log.Println(err)
		return tabloDB, err
	}

	tabloDB.database = db

	tabloDB.log.Println("performing initial setup")
	err = tabloDB.initialSetup()
	if err != nil {
		tabloDB.log.Println(err)
		return tabloDB, err
	}

	tabloDB.log.Println("upserting systemInfo")
	qryUpsertSystemInfo := fmt.Sprintf(templates["upsertSystemInfo"], sanitizeSqlString(serverID), sanitizeSqlString(name), sanitizeSqlString(ipAddress), dbVer)
	_, err = tabloDB.database.Exec(qryUpsertSystemInfo)
	if err != nil {
		tabloDB.log.Println(err)
		return tabloDB, err
	}

	tabloDB.log.Println("creating dummy keys")
	_, err = tabloDB.database.Exec("INSERT INTO channel (channelID, callSign, major, minor) VALUES (0, '', 0, 0);")
	if err != nil {
		tabloDB.log.Println(err)
		return tabloDB, err
	}
	tabloDB.log.Println("tabloDB created")
	return tabloDB, nil
}

func (db *TabloDB) Close() {
	db.log.Println("closing database")
	defer db.database.Close()
}

func (db *TabloDB) initialSetup() error {
	db.log.Println("creating database tables")
	_, err := db.database.Exec(queries["createDatabase"])
	if err != nil {
		db.log.Println(err)
		return err
	}
	db.log.Println("database tables successfully created")
	return nil
}

func (db *TabloDB) GetGuideLastUpdated() (time.Time, error) {
	db.log.Println("getting guideLastUpdated from systemInfo")
	result, err := db.database.Query(queries["getGuideLastUpdated"])
	if err != nil {
		db.log.Println(err)
		return time.Unix(0, 0), err
	}
	var lastUpdatedRaw int64
	err = result.Scan(&lastUpdatedRaw)
	if err != nil {
		db.log.Println()
		return time.Unix(0, 0), err
	}
	db.log.Printf("raw date %d\n", lastUpdatedRaw)
	return int64ToTime(lastUpdatedRaw), nil
}

func (db *TabloDB) GetScheduledLastUpdated() (time.Time, error) {
	db.log.Println("getting scheduledLastUpdated from systeminfo")
	result, err := db.database.Query(queries["getScheduledLastUpdated"])
	if err != nil {
		db.log.Println(err)
		return time.Unix(0, 0), err
	}
	var lastUpdatedRaw int64
	err = result.Scan(&lastUpdatedRaw)
	if err != nil {
		db.log.Println(err)
		return time.Unix(0, 0), err
	}

	db.log.Printf("scheduled lastUpdatedRaw %d\n", lastUpdatedRaw)
	return int64ToTime(lastUpdatedRaw), nil
}

func (db *TabloDB) Enqueue(action string, details string, exportPath string) error {
	db.log.Printf("enqueueing '%s' '%s' '%s'\n", action, details, exportPath)
	var qry string
	if details == "" {
		qry = fmt.Sprintf(templates["insertQueuePriority"], action, details, exportPath)
	} else {
		qry = fmt.Sprintf(templates["insertQueue"], action, details, exportPath)
	}
	_, err := db.database.Exec(qry)
	if err != nil {
		return err
	}
	db.log.Println("enqueued successfully")
	return nil
}

func (db *TabloDB) GetQueue() ([]QueueRecord, error) {
	db.log.Println("getting queue")
	rows, err := db.database.Query(queries["selectQueue"])
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var queue []QueueRecord
	for rows.Next() {
		var rec QueueRecord
		err = rows.Scan(&rec.QueueID, &rec.Action, &rec.Details, &rec.ExportPath)
		if err != nil {
			return nil, err
		}
		queue = append(queue, rec)
	}
	db.log.Printf("%d queue records retrieved\n", len(queue))
	return queue, nil
}

func (db *TabloDB) InsertChannels(channels map[string]tabloapi.Channel) error {
	db.log.Printf("inserting %d channels\n", len(channels))
	for _, v := range channels {
		qry := fmt.Sprintf(templates["upsertChannel"], v.Object_id, sanitizeSqlString(v.Channel.Call_sign), v.Channel.Major, v.Channel.Minor, sanitizeSqlString(v.Channel.Network))
		_, err := db.database.Exec(qry)
		if err != nil {
			return err
		}
	}
	db.log.Println("channels inserted")
	return nil
}

func (db *TabloDB) InsertShows(shows map[string]tabloapi.Show) error {
	db.log.Printf("inserting %d shows\n", len(shows))
	for k, v := range shows {
		showType := utils.Substring(k, 7, 6)
		var genres []string
		var cast []string
		var awards []tabloapi.Award
		var directors []string
		var qry string
		var channel int
		if v.Schedule.Channel_path != "" {
			var err error
			channel, err = strconv.Atoi(utils.Substring(v.Schedule.Channel_path, 16, 8))
			if err != nil {
				db.log.Println(err)
				return err
			}
		} else {
			channel = 0
		}
		switch showType {
		case "series":
			airdate := dateStringToInt64(v.Series.Orig_air_date)
			qry = fmt.Sprintf(templates["upsertShow"], v.Object_id, "series", sanitizeSqlString(v.Schedule.Rule), channel, sanitizeSqlString(v.Keep.Rule), v.Keep.Count, sanitizeSqlString(v.Series.Title), sanitizeSqlString(v.Series.Description), airdate, v.Series.Episode_runtime, sanitizeSqlString(v.Series.Series_rating), 0)
			genres = v.Series.Genres
			cast = v.Series.Cast
			awards = v.Series.Awards
		case "movies":
			airdate := dateYearToInt64(v.Movie.Release_year)
			qry = fmt.Sprintf(templates["upsertShow"], v.Object_id, "movies", sanitizeSqlString(v.Schedule.Rule), channel, sanitizeSqlString(v.Keep.Rule), v.Keep.Count, sanitizeSqlString(v.Movie.Title), sanitizeSqlString(v.Movie.Plot), airdate, v.Movie.Original_runtime, sanitizeSqlString(v.Movie.Film_rating), v.Movie.Quality_rating)
			genres = v.Movie.Genres
			cast = v.Movie.Cast
			awards = v.Movie.Awards
			directors = v.Movie.Directors
		case "sports":
			qry = fmt.Sprintf(templates["upsertShow"], v.Object_id, "sports", sanitizeSqlString(v.Schedule.Rule), channel, sanitizeSqlString(v.Keep.Rule), v.Keep.Count, sanitizeSqlString(v.Sport.Title), sanitizeSqlString(v.Sport.Description), 0, 0, "", 0)
			genres = v.Sport.Genres
		default:
			db.log.Printf("unrecognized showtype %s\n", showType)
			continue
		}
		_, err := db.database.Exec(qry)
		if err != nil {
			db.log.Printf("query: %s", qry)
			db.log.Println(err)
			return err
		}
		for _, g := range genres {
			qry = fmt.Sprintf(templates["insertShowGenre"], v.Object_id, sanitizeSqlString(g))
			_, err = db.database.Exec(qry)
			if err != nil {
				db.log.Printf("query: %s", qry)
				db.log.Println(err)
				return err
			}
		}
		for _, c := range cast {
			qry = fmt.Sprintf(templates["insertShowCastMember"], v.Object_id, sanitizeSqlString(c))
			_, err = db.database.Exec(qry)
			if err != nil {
				db.log.Printf("query: %s", qry)
				db.log.Println(err)
				return err
			}
		}
		for _, a := range awards {
			won := 0
			if a.Won {
				won = 1
			}
			qry = fmt.Sprintf(templates["upsertShowAward"], v.Object_id, won, sanitizeSqlString(a.Name), sanitizeSqlString(a.Category), a.Year, sanitizeSqlString(a.Nominee))
			_, err = db.database.Exec(qry)
			if err != nil {
				db.log.Printf("query: %s", qry)
				db.log.Println(err)
				return err
			}
		}
		for _, d := range directors {
			qry = fmt.Sprintf(templates["insertShowDirector"], v.Object_id, sanitizeSqlString(d))
			_, err = db.database.Exec(qry)
			if err != nil {
				db.log.Printf("query: %s", qry)
				db.log.Println(err)
				return err
			}
		}
	}
	db.log.Println("shows inserted")
	return nil
}

func (db *TabloDB) DeleteQueueRecord(i int) error {
	db.log.Printf("deleting queueid %d\n", i)
	qry := fmt.Sprintf(templates["deleteQueueRecord"], i)
	_, err := db.database.Exec(qry)
	if err != nil {
		db.log.Println(err)
		return err
	}
	db.log.Println("queue record deleted")
	return err
}

func int64ToTime(i int64) time.Time {
	return time.Unix(i, 0)
}

func dateStringToInt64(s string) int64 {
	switch len(s) {
	case 10:
		year, _ := strconv.Atoi(utils.Substring(s, 0, 4))
		month, _ := strconv.Atoi(utils.Substring(s, 5, 2))
		day, _ := strconv.Atoi(utils.Substring(s, -2, 2))
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		return date.Unix()
	default:
		return 0
	}
}

func dateYearToInt64(y int) int64 {
	date := time.Date(y, time.Month(1), 1, 0, 0, 0, 0, time.Local)
	return date.Unix()
}

func sanitizeSqlString(s string) string {
	var out []rune
	for _, v := range s {
		switch v {
		case '\'':
			out = append(out, '\'', '\'')
		default:
			out = append(out, v)
		}
	}
	return string(out)
}
