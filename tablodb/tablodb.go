package tablodb

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"tablo-manager/tabloapi"
	"tablo-manager/utils"

	_ "github.com/mattn/go-sqlite3"
)

type TabloDB struct {
	database *sql.DB
	log      *log.Logger
}

type QueueRecord struct {
	QueueID int
	Action  string
	Details string
}

func New(ipAddress string, name string, serverID string, directory string) (TabloDB, error) {
	var tabloDB TabloDB
	databaseFile := directory + string(os.PathSeparator) + utils.SanitizeFileString(serverID) + ".cache"
	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		return tabloDB, fmt.Errorf("unable to create database %s: %s", databaseFile, err)
	}

	tabloDB.database = db

	err = tabloDB.initialSetup()
	if err != nil {
		return tabloDB, fmt.Errorf("unable to initialize database: %s", err)
	}

	qryUpsertSystemInfo := fmt.Sprintf(templates["upsertSystemInfo"], sanitizeSqlString(serverID), sanitizeSqlString(name), sanitizeSqlString(ipAddress), dbVer)
	_, err = tabloDB.database.Exec(qryUpsertSystemInfo)
	if err != nil {
		return tabloDB, fmt.Errorf("unable to insert system info: %s", err)
	}
	return tabloDB, nil
}

func (db *TabloDB) Close() {
	defer db.database.Close()
}

func (db *TabloDB) initialSetup() error {
	_, err := db.database.Exec(queries["createDatabase"])
	if err != nil {
		return err
	}
	return nil
}

func (db *TabloDB) GetGuideLastUpdated() (time.Time, error) {
	result, err := db.database.Query(queries["getGuideLastUpdated"])
	if err != nil {
		return time.Unix(0, 0), err
	}
	var lastUpdatedRaw int64
	result.Scan(&lastUpdatedRaw)
	return int64ToTime(lastUpdatedRaw), nil
}

func (db *TabloDB) GetScheduledLastUpdated() (time.Time, error) {
	result, err := db.database.Query(queries["getScheduledLastUpdated"])
	if err != nil {
		return time.Unix(0, 0), err
	}
	var lastUpdatedRaw int64
	result.Scan(&lastUpdatedRaw)
	return int64ToTime(lastUpdatedRaw), nil
}

func (db *TabloDB) Enqueue(action string, details string) {
	var qry string
	if details == "" {
		qry = fmt.Sprintf(templates["insertQueuePriority"], action, details)
	} else {
		qry = fmt.Sprintf(templates["insertQueue"], action, details)
	}
	db.database.Exec(qry)
}

func (db *TabloDB) GetQueue() ([]QueueRecord, error) {
	rows, err := db.database.Query(queries["selectQueue"])
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var queue []QueueRecord
	for rows.Next() {
		var rec QueueRecord
		rows.Scan(&rec.QueueID, &rec.Action, &rec.Details)
		queue = append(queue, rec)
	}

	return queue, nil
}

func (db *TabloDB) InsertChannels(channels map[string]tabloapi.Channel) {
	for _, v := range channels {
		qry := fmt.Sprintf(templates["upsertChannel"], v.Object_id, sanitizeSqlString(v.Channel.Call_sign), v.Channel.Major, v.Channel.Minor, sanitizeSqlString(v.Channel.Network))
		db.database.Exec(qry)
	}
}

func (db *TabloDB) InsertShows(shows map[string]tabloapi.Show) {
	for k, v := range shows {
		showType := utils.Substring(k, 7, 6)
		var genres []string
		var cast []string
		var awards []tabloapi.Award
		var directors []string
		var qry string
		var channel int
		if v.Schedule.Channel_path != "" {
			channel, _ = strconv.Atoi(utils.Substring(v.Schedule.Channel_path, 16, 8))
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
			fmt.Println(showType)
			continue
		}
		db.database.Exec(qry)
		for _, g := range genres {
			qry = fmt.Sprintf(templates["insertShowGenre"], v.Object_id, sanitizeSqlString(g))
			db.database.Exec(qry)
		}
		for _, c := range cast {
			qry = fmt.Sprintf(templates["insertShowCastMember"], v.Object_id, sanitizeSqlString(c))
			db.database.Exec(qry)
		}
		for _, a := range awards {
			won := 0
			if a.Won {
				won = 1
			}
			qry = fmt.Sprintf(templates["upsertShowAward"], v.Object_id, won, sanitizeSqlString(a.Name), sanitizeSqlString(a.Category), a.Year, sanitizeSqlString(a.Nominee))
			db.database.Exec(qry)
		}
		for _, d := range directors {
			qry = fmt.Sprintf(templates["insertShowDirector"], v.Object_id, sanitizeSqlString(d))
			db.database.Exec(qry)
		}
	}
}

func (db *TabloDB) DeleteQueueRecord(i int) {
	db.database.Exec(templates["deleteQueueRecord"], i)
}

func int64ToTime(i int64) time.Time {
	return time.Unix(i, 0)
}

func dateStringToInt64(s string) int64 {
	switch len(s) {
	case 10:
		year, _ := strconv.Atoi(utils.Substring(s, 0, 4))
		month, _ := strconv.Atoi(utils.Substring(s, 5, 2))
		day, _ := strconv.Atoi(utils.Substring(s, 8, 2))
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
