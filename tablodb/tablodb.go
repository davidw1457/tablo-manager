package tablodb

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
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
		tabloDB.log.Println(qryUpsertSystemInfo)
		tabloDB.log.Println(err)
		return tabloDB, err
	}

	tabloDB.log.Println("tabloDB created")
	return tabloDB, nil
}

func Open(serverID string, ipAddress string, name string, directory string) (TabloDB, error) {
	var tabloDB TabloDB
	logFile, err := os.OpenFile(directory+string(os.PathSeparator)+"main.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, userRWX)
	if err != nil {
		return tabloDB, err
	}
	tabloDB.log = log.New(io.MultiWriter(logFile, os.Stdout), "tablodb "+serverID+": ", log.LstdFlags)

	tabloDB.log.Println("opening tabloDB")
	databaseFile := directory + string(os.PathSeparator) + utils.SanitizeFileString(serverID) + ".cache"
	tabloDB.log.Printf("opening %s\n", databaseFile)
	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		tabloDB.log.Println(err)
		return tabloDB, err
	}

	tabloDB.database = db

	tabloDB.log.Println("verifying database version")
	currentDBVer, err := tabloDB.getVersion()
	if err != nil {
		tabloDB.log.Println(err)
		return tabloDB, err
	}
	if currentDBVer < dbVer {
		err := tabloDB.updateVer(currentDBVer)
		if err != nil {
			tabloDB.log.Println(err)
			return tabloDB, err
		}
	}

	tabloDB.log.Println("updating systemInfo")
	qryUpdateSystemInfo := fmt.Sprintf(templates["updateSystemInfo"], sanitizeSqlString(name), sanitizeSqlString(ipAddress))
	_, err = tabloDB.database.Exec(qryUpdateSystemInfo)
	if err != nil {
		tabloDB.log.Println(qryUpdateSystemInfo)
		tabloDB.log.Println(err)
		return tabloDB, err
	}

	tabloDB.log.Println("tabloDB opened")
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
		db.log.Println(queries["createDatabase"])
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
		db.log.Println(qry)
		db.log.Println(err)
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

func (db *TabloDB) UpsertChannels(channels map[string]tabloapi.Channel) error {
	db.log.Printf("preparing %d channels to insert\n", len(channels))
	var channelValues []string
	for k, v := range channels {
		if v.Object_ID == 0 {
			continue
		}

		updateType := strings.Split(k, "/")[1]
		network := "null"
		if v.Channel.Network != nil {
			network = "'" + sanitizeSqlString(*v.Channel.Network) + "'"
		}
		var channelValue strings.Builder
		channelValue.WriteRune('(')
		if updateType == "recordings" {
			channelValue.WriteString("-" + strconv.Itoa(v.Object_ID))
		} else {
			channelValue.WriteString(strconv.Itoa(v.Object_ID))
		}
		channelValue.WriteString(",'")
		channelValue.WriteString(sanitizeSqlString(v.Channel.Call_Sign))
		channelValue.WriteString("',")
		channelValue.WriteString(strconv.Itoa(v.Channel.Major))
		channelValue.WriteRune(',')
		channelValue.WriteString(strconv.Itoa(v.Channel.Minor))
		channelValue.WriteString(",")
		channelValue.WriteString(network) // Previously sanitized
		channelValue.WriteRune(')')
		channelValues = append(channelValues, channelValue.String())
	}

	if len(channelValues) == 0 {
		err := fmt.Errorf("No channels in insert values")
		db.log.Println(err)
		return err
	}

	db.log.Printf("upserting %d channels\n", len(channelValues))
	qry := fmt.Sprintf(templates["upsertChannel"], strings.Join(channelValues, ","))
	_, err := db.database.Exec(qry)
	if err != nil {
		db.log.Println(qry)
		db.log.Println(err)
		return err
	}

	db.log.Println("channels inserted")
	return nil
}

func (db *TabloDB) UpsertShows(shows map[string]tabloapi.Show) error {
	db.log.Printf("preparing %d shows to insert\n", len(shows))
	var showGenreValues []string
	var showCastMemberValues []string
	var showAwardValues []string
	var showDirectorValues []string
	var showValues []string
	for k, s := range shows {
		if s.Object_ID == 0 {
			continue
		}

		updateType := strings.Split(k, "/")[1]

		showID := strconv.Itoa(s.Object_ID)
		if updateType == "recordings" {
			showID = "-" + showID
		}

		parentShowID := "null"
		if s.Guide_Path != "" {
			parentShowID = strings.Split(s.Guide_Path, "/")[3]
		}

		rule := "null"
		if s.Schedule.Rule != "" {
			rule = "'" + sanitizeSqlString(s.Schedule.Rule) + "'"
		}

		channelID := "null"
		if s.Schedule.Channel_Path != "" {
			channelID = strings.Split(s.Schedule.Channel_Path, "/")[3]
			if updateType == "recordings" {
				channelID = "-" + channelID
			}
		}

		count := "null"
		if s.Keep.Count != nil {
			count = strconv.Itoa(*s.Keep.Count)
		}

		showType := strings.Split(s.Path, "/")[2]

		var genres []string
		var cast []string
		var awards []tabloapi.Award
		var directors []string

		var showValue strings.Builder
		showValue.WriteRune('(')
		showValue.WriteString(showID) // Int value, no sanitizaion needed
		showValue.WriteRune(',')
		showValue.WriteString(sanitizeSqlString(parentShowID))
		showValue.WriteString(",'")
		showValue.WriteString(sanitizeSqlString(showType))
		showValue.WriteString("',")
		showValue.WriteString(rule) // Previously sanitized
		showValue.WriteRune(',')
		showValue.WriteString(sanitizeSqlString(channelID))
		showValue.WriteString(",'")
		showValue.WriteString(sanitizeSqlString(s.Keep.Rule))
		showValue.WriteString("',")
		showValue.WriteString(sanitizeSqlString(count))
		showValue.WriteString(",'")
		switch showType {
		case "series":
			description := "null"
			if s.Series.Description != nil {
				description = "'" + sanitizeSqlString(*s.Series.Description) + "'"
			}

			airdate := "null"
			if s.Series.Orig_Air_Date != nil {
				airdateInt, err := dateStringToInt(*s.Series.Orig_Air_Date)
				if err != nil {
					db.log.Println(err)
					return err
				}
				airdate = strconv.Itoa(airdateInt)
			}

			rating := "null"
			if s.Series.Series_Rating != nil {
				rating = "'" + sanitizeSqlString(*s.Series.Series_Rating) + "'"
			}

			showValue.WriteString(sanitizeSqlString(s.Series.Title))
			showValue.WriteString("',")
			showValue.WriteString(description) // Previously sanitized
			showValue.WriteRune(',')
			showValue.WriteString(airdate) // Int value, no sanitization needed
			showValue.WriteRune(',')
			showValue.WriteString(strconv.Itoa(s.Series.Episode_Runtime))
			showValue.WriteRune(',')
			showValue.WriteString(rating) // Previously sanitized
			showValue.WriteString(",null)")

			genres = s.Series.Genres
			cast = s.Series.Cast
			awards = s.Series.Awards
		case "movies":
			description := "null"
			if s.Movie.Plot != nil {
				description = "'" + sanitizeSqlString(*s.Movie.Plot) + "'"
			}

			airdate := "null"
			if s.Movie.Release_Year != nil {
				airdate = strconv.Itoa(dateYearToInt(*s.Movie.Release_Year))
			}

			filmRating := "null"
			if s.Movie.Film_Rating != nil {
				filmRating = "'" + sanitizeSqlString(*s.Movie.Film_Rating) + "'"
			}

			qualityRating := "null"
			if s.Movie.Quality_Rating != nil {
				qualityRating = strconv.Itoa(*s.Movie.Quality_Rating)
			}

			showValue.WriteString(sanitizeSqlString(s.Movie.Title))
			showValue.WriteString("',")
			showValue.WriteString(description) // Previosly sanitized
			showValue.WriteRune(',')
			showValue.WriteString(airdate) // Int value, no sanitization needed
			showValue.WriteRune(',')
			showValue.WriteString(strconv.Itoa(s.Movie.Original_Runtime))
			showValue.WriteRune(',')
			showValue.WriteString(filmRating) // Previously sanitized
			showValue.WriteRune(',')
			showValue.WriteString(qualityRating) // Int value, no sanitization needed
			showValue.WriteRune(')')

			genres = s.Movie.Genres
			cast = s.Movie.Cast
			awards = s.Movie.Awards
			directors = s.Movie.Directors
		case "sports":
			showValue.WriteString(sanitizeSqlString(s.Sport.Title))
			showValue.WriteString("','")
			showValue.WriteString(sanitizeSqlString(s.Sport.Description))
			showValue.WriteString("',null,null,null,null)")

			genres = s.Sport.Genres
		default:
			err := fmt.Errorf("Invalid showtype: %s", showType)
			db.log.Println(err)
			return err
		}
		showValues = append(showValues, showValue.String())

		for _, g := range genres {
			var genreValue strings.Builder
			genreValue.WriteRune('(')
			genreValue.WriteString(showID) // Int value, no sanitization needed
			genreValue.WriteString(",'")
			genreValue.WriteString(sanitizeSqlString(g))
			genreValue.WriteString("')")
			showGenreValues = append(showGenreValues, genreValue.String())
		}

		for _, c := range cast {
			var castValue strings.Builder
			castValue.WriteRune('(')
			castValue.WriteString(showID) // Int value, no sanitization needed
			castValue.WriteString(",'")
			castValue.WriteString(sanitizeSqlString(c))
			castValue.WriteString("')")
			showCastMemberValues = append(showCastMemberValues, castValue.String())
		}

		for _, d := range directors {
			var directorValue strings.Builder
			directorValue.WriteRune('(')
			directorValue.WriteString(showID) // Int value, no sanitization needed
			directorValue.WriteString(",'")
			directorValue.WriteString(sanitizeSqlString(d))
			directorValue.WriteString("')")
			showDirectorValues = append(showDirectorValues, directorValue.String())
		}

		for _, a := range awards {
			won := "0"
			if a.Won {
				won = "1"
			}

			nominee := "null"
			if a.Nominee != "" {
				nominee = "'" + sanitizeSqlString(a.Nominee) + "'"
			}

			var awardValue strings.Builder
			awardValue.WriteRune('(')
			awardValue.WriteString(showID) // Int value, no sanitization needed
			awardValue.WriteRune(',')
			awardValue.WriteString(won) // Bool value, no sanitization needed
			awardValue.WriteString(",'")
			awardValue.WriteString(sanitizeSqlString(a.Name))
			awardValue.WriteString("','")
			awardValue.WriteString(sanitizeSqlString(a.Category))
			awardValue.WriteString("',")
			awardValue.WriteString(strconv.Itoa(a.Year))
			awardValue.WriteRune(',')
			awardValue.WriteString(nominee) // Previously sanitized
			awardValue.WriteRune(')')

			showAwardValues = append(showAwardValues, awardValue.String())
		}
	}

	if len(showValues) == 0 {
		err := fmt.Errorf("no shows in upsert values")
		db.log.Println(err)
		return err
	}

	db.log.Printf("Upserting %d shows\n", len(showValues))
	qryUpsertShow := fmt.Sprintf(templates["upsertShow"], strings.Join(showValues, ","))
	_, err := db.database.Exec(qryUpsertShow)
	if err != nil {
		db.log.Println(qryUpsertShow)
		db.log.Println(err)
		return err
	}

	if len(showGenreValues) > 0 {
		db.log.Printf("Inserting %d genres\n", len(showGenreValues))
		qryInsertShowGenre := fmt.Sprintf(templates["insertShowGenre"], strings.Join(showGenreValues, ","))
		_, err = db.database.Exec(qryInsertShowGenre)
		if err != nil {
			db.log.Println(qryInsertShowGenre)
			db.log.Println(err)
			return err
		}
	}

	if len(showCastMemberValues) > 0 {
		db.log.Printf("Inserting %d cast members\n", len(showCastMemberValues))
		qryInsertShowCastMember := fmt.Sprintf(templates["insertShowCastMember"], strings.Join(showCastMemberValues, ","))
		_, err = db.database.Exec(qryInsertShowCastMember)
		if err != nil {
			db.log.Println(qryInsertShowCastMember)
			db.log.Println(err)
			return err
		}
	}

	if len(showAwardValues) > 0 {
		db.log.Printf("Upserting %d awards\n", len(showAwardValues))
		qryUpsertShowAward := fmt.Sprintf(templates["upsertShowAward"], strings.Join(showAwardValues, ","))
		_, err = db.database.Exec(qryUpsertShowAward)
		if err != nil {
			db.log.Println(qryUpsertShowAward)
			db.log.Println(err)
			return err
		}
	}

	if len(showDirectorValues) > 0 {
		db.log.Printf("Inserting %d directors\n", len(showDirectorValues))
		qryInsertShowDirector := fmt.Sprintf(templates["insertShowDirector"], strings.Join(showDirectorValues, ","))
		_, err = db.database.Exec(qryInsertShowDirector)
		if err != nil {
			db.log.Println(qryInsertShowDirector)
			db.log.Println(err)
			return err
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
		db.log.Println(qry)
		db.log.Println(err)
		return err
	}
	db.log.Println("queue record deleted")
	return err
}

func (db *TabloDB) UpsertAirings(airings map[string]tabloapi.Airing) error {
	db.log.Printf("preparing %d airings to insert\n", len(airings))

	var airingValues []string
	var teamValues []string
	var episodeValues []string
	var episodeTeamValues []string

	for _, a := range airings {
		if a.Object_ID == 0 {
			continue
		}

		var showID string
		var episodeID string
		airdate, err := dateStringToInt(a.Airing_Details.Datetime)
		if err != nil {
			db.log.Println(err)
			return err
		}

		if a.Series_Path != "" {
			showID = strings.Split(a.Series_Path, "/")[3]

			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, strconv.Itoa(a.Episode.Season_Number), a.Episode.Number, airdate)) + "'"

			title := "null"
			if a.Episode.Title != "" {
				title = "'" + sanitizeSqlString(a.Episode.Title) + "'"
			}

			description := "null"
			if a.Episode.Description != "" {
				description = "'" + sanitizeSqlString(a.Episode.Description) + "'"
			}

			originalAirDate := "null"
			if a.Episode.Orig_Air_Date != nil {
				dateInt, err := dateStringToInt(*a.Episode.Orig_Air_Date)
				if err != nil {
					db.log.Println(err)
					return err
				}
				originalAirDate = strconv.Itoa(dateInt)
			}

			var episodeValue strings.Builder
			episodeValue.WriteRune('(')
			episodeValue.WriteString(episodeID) // Previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(sanitizeSqlString(showID))
			episodeValue.WriteRune(',')
			episodeValue.WriteString(title) // Previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(description) // Previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(strconv.Itoa(a.Episode.Number))
			episodeValue.WriteString(",'")
			episodeValue.WriteString(strconv.Itoa(a.Episode.Season_Number))
			episodeValue.WriteString("',null,")
			episodeValue.WriteString(originalAirDate) // Int value, no sanitization needed
			episodeValue.WriteString(",null)")
			episodeValues = append(episodeValues, episodeValue.String())
		} else if a.Movie_Path != "" {
			showID = strings.Split(a.Movie_Path, "/")[3]
			episodeID = "null"
		} else if a.Sport_Path != "" {
			showID = strings.Split(a.Sport_Path, "/")[3]
			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, a.Event.Season, 0, airdate)) + "'"

			for _, t := range a.Event.Teams {
				var teamValue strings.Builder
				teamValue.WriteRune('(')
				teamValue.WriteString(strconv.Itoa(t.Team_ID))
				teamValue.WriteString(",'")
				teamValue.WriteString(sanitizeSqlString(t.Name))
				teamValue.WriteString("')")
				teamValues = append(teamValues, teamValue.String())

				var episodeTeamValue strings.Builder
				episodeTeamValue.WriteRune('(')
				episodeTeamValue.WriteString(episodeID)
				episodeTeamValue.WriteRune(',')
				episodeTeamValue.WriteString(strconv.Itoa(t.Team_ID))
				episodeTeamValue.WriteRune(')')
				episodeTeamValues = append(episodeTeamValues, episodeTeamValue.String())
			}

			season := "null"
			if a.Event.Season != "" {
				season = "'" + sanitizeSqlString(a.Event.Season) + "'"
			}

			seasonType := "null"
			if a.Event.Season_Type != "" {
				seasonType = "'" + sanitizeSqlString(a.Event.Season_Type) + "'"
			}

			homeTeamID := "null"
			if a.Event.Home_Team_ID != nil {
				homeTeamID = strconv.Itoa(*a.Event.Home_Team_ID)
			}

			var episodeValue strings.Builder
			episodeValue.WriteRune('(')
			episodeValue.WriteString(episodeID)
			episodeValue.WriteRune(',')
			episodeValue.WriteString(sanitizeSqlString(showID))
			episodeValue.WriteString(",'")
			episodeValue.WriteString(sanitizeSqlString(a.Event.Title))
			episodeValue.WriteString("','")
			episodeValue.WriteString(sanitizeSqlString(a.Event.Description))
			episodeValue.WriteString("',null,")
			episodeValue.WriteString(season) // previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(seasonType) // previously sanitized
			episodeValue.WriteString(",null,")
			episodeValue.WriteString(homeTeamID) // Int value, no sanitization required
			episodeValue.WriteRune(')')
			episodeValues = append(episodeValues, episodeValue.String())
		} else {
			err := fmt.Errorf("No show path for %d", a.Object_ID)
			db.log.Println(err)
			return err
		}

		var airingValue strings.Builder
		airingValue.WriteRune('(')
		airingValue.WriteString(strconv.Itoa(a.Object_ID))
		airingValue.WriteRune(',')
		airingValue.WriteString(sanitizeSqlString(showID))
		airingValue.WriteRune(',')
		airingValue.WriteString(strconv.Itoa(airdate))
		airingValue.WriteRune(',')
		airingValue.WriteString(strconv.Itoa(a.Airing_Details.Duration))
		airingValue.WriteRune(',')
		airingValue.WriteString(strconv.Itoa(a.Airing_Details.Channel.Object_ID))
		airingValue.WriteString(",'")
		airingValue.WriteString(sanitizeSqlString(a.Schedule.State))
		airingValue.WriteString("',")
		airingValue.WriteString(episodeID) // previously sanitized
		airingValue.WriteRune(')')
		airingValues = append(airingValues, airingValue.String())
	}

	if len(airingValues) == 0 {
		err := fmt.Errorf("no airings in upsert values")
		fmt.Println(err)
		return err
	}

	if len(teamValues) > 0 {
		db.log.Printf("inserting %d teams\n", len(teamValues))
		qryUpsertTeam := fmt.Sprintf(templates["upsertTeam"], strings.Join(teamValues, ","))
		_, err := db.database.Exec(qryUpsertTeam)
		if err != nil {
			db.log.Println(qryUpsertTeam)
			db.log.Println(err)
			return err
		}
	}

	if len(episodeValues) > 0 {
		db.log.Printf("inserting %d episodes\n", len(episodeValues))
		qryUpsertEpisode := fmt.Sprintf(templates["upsertEpisode"], strings.Join(episodeValues, ","))
		_, err := db.database.Exec(qryUpsertEpisode)
		if err != nil {
			db.log.Println(qryUpsertEpisode)
			db.log.Println(err)
			return err
		}
	}

	if len(episodeTeamValues) > 0 {
		db.log.Printf("inserting %d episode teams\n", len(episodeTeamValues))
		qryInsertEpisodeTeam := fmt.Sprintf(templates["insertEpisodeTeam"], strings.Join(episodeTeamValues, ","))
		_, err := db.database.Exec(qryInsertEpisodeTeam)
		if err != nil {
			db.log.Println(qryInsertEpisodeTeam)
			db.log.Println(err)
			return err
		}
	}

	db.log.Printf("inserting %d airings\n", len(airingValues))
	qryUpsertAiring := fmt.Sprintf(templates["upsertAiring"], strings.Join(airingValues, ","))
	_, err := db.database.Exec(qryUpsertAiring)
	if err != nil {
		db.log.Println(qryUpsertAiring)
		db.log.Println(err)
		return err
	}

	db.log.Println("airings inserted")
	return nil
}

func (db *TabloDB) UpdateGuideLastUpdated(guideLastUpdated time.Time) error {
	dateInt := int(guideLastUpdated.Unix())
	qryUpdateGuideLastUpdated := fmt.Sprintf(templates["updateGuideLastUpdated"], dateInt)
	_, err := db.database.Exec(qryUpdateGuideLastUpdated)
	if err != nil {
		db.log.Println(qryUpdateGuideLastUpdated)
		db.log.Println(err)
		return err
	}
	return nil
}

func (db *TabloDB) UpdateScheduledLastUpdated(scheduledLastUpdated time.Time) error {
	dateInt := int(scheduledLastUpdated.Unix())
	qryUpdateScheduledLastUpdated := fmt.Sprintf(templates["updateScheduledLastUpdated"], dateInt)
	_, err := db.database.Exec(qryUpdateScheduledLastUpdated)
	if err != nil {
		db.log.Println(qryUpdateScheduledLastUpdated)
		db.log.Println(err)
		return err
	}
	return nil
}

func (db *TabloDB) UpdateRecordingsLastUpdated(recordingsLastUpdated time.Time) error {
	dateInt := int(recordingsLastUpdated.Unix())
	qryUpdateRecordingsLastUpdated := fmt.Sprintf(templates["updateRecordingsLastUpdated"], dateInt)
	_, err := db.database.Exec(qryUpdateRecordingsLastUpdated)
	if err != nil {
		db.log.Println(qryUpdateRecordingsLastUpdated)
		db.log.Println(err)
		return err
	}
	return nil
}

func (db *TabloDB) UpsertRecordings(recordings map[string]tabloapi.Recording) error {
	db.log.Printf("preparing %d recording airings to insert\n", len(recordings))

	var recordingValues []string
	var teamValues []string
	var episodeValues []string
	var episodeTeamValues []string
	var errorValues []string

	for _, r := range recordings {
		if r.Object_ID == 0 {
			continue
		}

		var showID string
		var episodeID string
		airdate, err := dateStringToInt(r.Airing_Details.Datetime)
		if err != nil {
			db.log.Println(err)
			return err
		}

		if r.Series_Path != "" {
			showID = "-" + strings.Split(r.Series_Path, "/")[3]

			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, strconv.Itoa(r.Episode.Season_Number), r.Episode.Number, airdate)) + "'"

			title := "null"
			if r.Episode.Title != "" {
				title = "'" + sanitizeSqlString(r.Episode.Title) + "'"
			}

			description := "null"
			if r.Episode.Description != "" {
				description = "'" + sanitizeSqlString(r.Episode.Description) + "'"
			}

			originalAirDate := "null"
			if r.Episode.Orig_Air_Date != nil {
				dateInt, err := dateStringToInt(*r.Episode.Orig_Air_Date)
				if err != nil {
					db.log.Println(err)
					return err
				}
				originalAirDate = strconv.Itoa(dateInt)
			}

			var episodeValue strings.Builder
			episodeValue.WriteRune('(')
			episodeValue.WriteString(episodeID) // Previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(sanitizeSqlString(showID))
			episodeValue.WriteRune(',')
			episodeValue.WriteString(title) // Previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(description) // Previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(strconv.Itoa(r.Episode.Number))
			episodeValue.WriteString(",'")
			episodeValue.WriteString(strconv.Itoa(r.Episode.Season_Number))
			episodeValue.WriteString("',null,")
			episodeValue.WriteString(originalAirDate) // Int value, no sanitization needed
			episodeValue.WriteString(",null)")
			episodeValues = append(episodeValues, episodeValue.String())
		} else if r.Movie_Path != "" {
			showID = "-" + strings.Split(r.Movie_Path, "/")[3]
			episodeID = "null"
		} else if r.Sport_Path != "" {
			showID = "-" + strings.Split(r.Sport_Path, "/")[3]
			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, r.Event.Season, 0, airdate)) + "'"

			for _, t := range r.Event.Teams {
				var teamValue strings.Builder
				teamValue.WriteRune('(')
				teamValue.WriteString(strconv.Itoa(t.Team_ID))
				teamValue.WriteString(",'")
				teamValue.WriteString(sanitizeSqlString(t.Name))
				teamValue.WriteString("')")
				teamValues = append(teamValues, teamValue.String())

				var episodeTeamValue strings.Builder
				episodeTeamValue.WriteRune('(')
				episodeTeamValue.WriteString(episodeID)
				episodeTeamValue.WriteRune(',')
				episodeTeamValue.WriteString(strconv.Itoa(t.Team_ID))
				episodeTeamValue.WriteRune(')')
				episodeTeamValues = append(episodeTeamValues, episodeTeamValue.String())
			}

			season := "null"
			if r.Event.Season != "" {
				season = "'" + sanitizeSqlString(r.Event.Season) + "'"
			}

			seasonType := "null"
			if r.Event.Season_Type != "" {
				seasonType = "'" + sanitizeSqlString(r.Event.Season_Type) + "'"
			}

			homeTeamID := "null"
			if r.Event.Home_Team_ID != nil {
				homeTeamID = strconv.Itoa(*r.Event.Home_Team_ID)
			}

			var episodeValue strings.Builder
			episodeValue.WriteRune('(')
			episodeValue.WriteString(episodeID)
			episodeValue.WriteRune(',')
			episodeValue.WriteString(sanitizeSqlString(showID))
			episodeValue.WriteString(",'")
			episodeValue.WriteString(sanitizeSqlString(r.Event.Title))
			episodeValue.WriteString("','")
			episodeValue.WriteString(sanitizeSqlString(r.Event.Description))
			episodeValue.WriteString("',null,")
			episodeValue.WriteString(season) // previously sanitized
			episodeValue.WriteRune(',')
			episodeValue.WriteString(seasonType) // previously sanitized
			episodeValue.WriteString(",null,")
			episodeValue.WriteString(homeTeamID) // Int value, no sanitization required
			episodeValue.WriteRune(')')
			episodeValues = append(episodeValues, episodeValue.String())
		} else {
			err := fmt.Errorf("No show path for %d", r.Object_ID)
			db.log.Println(err)
			return err
		}

		clean := "0"
		if r.Video_Details.Clean {
			clean = "1"
		}

		var recordingValue strings.Builder
		recordingValue.WriteRune('(')
		recordingValue.WriteString(strconv.Itoa(r.Object_ID))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(sanitizeSqlString(showID))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(airdate))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(r.Airing_Details.Duration))
		recordingValue.WriteRune(',')
		recordingValue.WriteString("-" + strconv.Itoa(r.Airing_Details.Channel.Object_ID))
		recordingValue.WriteString(",'")
		recordingValue.WriteString(sanitizeSqlString(r.Video_Details.State))
		recordingValue.WriteString("',")
		recordingValue.WriteString(clean) // Bool value, no sanitization needed
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(r.Video_Details.Duration))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(r.Video_Details.Size))
		recordingValue.WriteString(",'")
		recordingValue.WriteString(sanitizeSqlString(r.Video_Details.ComSkip.State))
		recordingValue.WriteString("',")
		recordingValue.WriteString(episodeID) // previously sanitized
		recordingValue.WriteRune(')')
		recordingValues = append(recordingValues, recordingValue.String())

		if r.Video_Details.State == "failed" || !r.Video_Details.Clean || r.Video_Details.ComSkip.State != "none" {
			var comSkipError = "null"
			if r.Video_Details.ComSkip.Error != nil {
				comSkipError = "'" + sanitizeSqlString(*r.Video_Details.ComSkip.Error) + "'"
			}

			var errorCode = "null"
			if r.Video_Details.Error.Code != nil {
				comSkipError = "'" + sanitizeSqlString(*r.Video_Details.Error.Code) + "'"
			}

			var errorDetails = "null"
			if r.Video_Details.Error.Details != nil {
				comSkipError = "'" + sanitizeSqlString(*r.Video_Details.Error.Details) + "'"
			}

			var errorDescription = "null"
			if r.Video_Details.Error.Description != nil {
				comSkipError = "'" + sanitizeSqlString(*r.Video_Details.Error.Description) + "'"
			}

			var errorValue strings.Builder
			errorValue.WriteRune('(')
			errorValue.WriteString(strconv.Itoa(r.Object_ID))
			errorValue.WriteRune(',')
			errorValue.WriteString(sanitizeSqlString(showID))
			errorValue.WriteRune(',')
			errorValue.WriteString(episodeID) // previously sanitized
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.Airing_Details.Channel.Object_ID))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(airdate))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.Airing_Details.Duration))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.Video_Details.Duration))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.Video_Details.Size))
			errorValue.WriteString(",'")
			errorValue.WriteString(sanitizeSqlString(r.Video_Details.State))
			errorValue.WriteString("',")
			errorValue.WriteString(clean) // Bool value. No sanitization needed
			errorValue.WriteString(",'")
			errorValue.WriteString(sanitizeSqlString(r.Video_Details.ComSkip.State))
			errorValue.WriteString("',")
			errorValue.WriteString(comSkipError) // previously sanitized
			errorValue.WriteRune(',')
			errorValue.WriteString(errorCode) // previously sanitized
			errorValue.WriteRune(',')
			errorValue.WriteString(errorDetails) // previously sanitized
			errorValue.WriteRune(',')
			errorValue.WriteString(errorDescription) // previously sanitized
			errorValue.WriteRune(')')
			errorValues = append(errorValues, errorValue.String())
		}
	}

	if len(recordingValues) == 0 {
		err := fmt.Errorf("no recording airings in upsert values")
		fmt.Println(err)
		return err
	}

	if len(teamValues) > 0 {
		db.log.Printf("inserting %d teams\n", len(teamValues))
		qryUpsertTeam := fmt.Sprintf(templates["upsertTeam"], strings.Join(teamValues, ","))
		_, err := db.database.Exec(qryUpsertTeam)
		if err != nil {
			db.log.Println(qryUpsertTeam)
			db.log.Println(err)
			return err
		}
	}

	if len(episodeValues) > 0 {
		db.log.Printf("inserting %d episodes\n", len(episodeValues))
		qryUpsertEpisode := fmt.Sprintf(templates["upsertEpisode"], strings.Join(episodeValues, ","))
		_, err := db.database.Exec(qryUpsertEpisode)
		if err != nil {
			db.log.Println(qryUpsertEpisode)
			db.log.Println(err)
			return err
		}
	}

	if len(episodeTeamValues) > 0 {
		db.log.Printf("inserting %d episode teams\n", len(episodeTeamValues))
		qryInsertEpisodeTeam := fmt.Sprintf(templates["insertEpisodeTeam"], strings.Join(episodeTeamValues, ","))
		_, err := db.database.Exec(qryInsertEpisodeTeam)
		if err != nil {
			db.log.Println(qryInsertEpisodeTeam)
			db.log.Println(err)
			return err
		}
	}

	if len(errorValues) > 0 {
		db.log.Printf("inserting %d errors\n", len(errorValues))
		qryInsertErrors := fmt.Sprintf(templates["insertError"], strings.Join(errorValues, ","))
		_, err := db.database.Exec(qryInsertErrors)
		if err != nil {
			db.log.Println(qryInsertErrors)
			db.log.Println(err)
			return err
		}
	}

	db.log.Printf("inserting %d recording airings\n", len(recordingValues))
	qryUpsertRecording := fmt.Sprintf(templates["upsertRecording"], strings.Join(recordingValues, ","))
	_, err := db.database.Exec(qryUpsertRecording)
	if err != nil {
		db.log.Println(qryUpsertRecording)
		db.log.Println(err)
		return err
	}

	db.log.Println("recording airings inserted")
	return nil
}

func (db *TabloDB) GetLastUpdated() (time.Time, time.Time, time.Time, error) {
	row := db.database.QueryRow(queries["getLastUpdated"])

	var guideLastUpdated, scheduledLastUpdated, recordingsLastUpdated int64
	err := row.Scan(&guideLastUpdated, &scheduledLastUpdated, &recordingsLastUpdated)
	if err != nil {
		db.log.Println(queries["getLastUpdated"])
		db.log.Println(err)
		return time.Unix(0, 0), time.Unix(0, 0), time.Unix(0, 0), err
	}

	return time.Unix(guideLastUpdated, 0), time.Unix(scheduledLastUpdated, 0), time.Unix(recordingsLastUpdated, 0), nil
}

func (db *TabloDB) GetDefaultExportPath() (string, error) {
	row := db.database.QueryRow(queries["getDefaultExportPath"])

	var defaultExportPath string
	err := row.Scan(&defaultExportPath)
	if err != nil {
		db.log.Println(queries["getDefaultExportPath"])
		db.log.Println(err)
		return "", err
	}

	return defaultExportPath, nil
}

func (db *TabloDB) getVersion() (int, error) {
	row := db.database.QueryRow(queries["getDBVer"])

	var currentDBVer int
	err := row.Scan(&currentDBVer)
	if err != nil {
		db.log.Println(queries["getDefaultExportPath"])
		db.log.Println(err)
		return 0, err
	}

	return currentDBVer, nil
}

func (db *TabloDB) updateVer(currentVer int) error {
	// TODO: Implement version upgrading if the database structure is changed
	db.log.Printf("currentVer: %d\n", currentVer)
	db.log.Println("not yet implemented")
	return nil
}

func getEpisodeID(showID, season string, episode, airdate int) string {
	var episodeID string
	if season == "" {
		season = "0"
	}

	if episode == 0 {
		episodeID = showID + "." + season + "." + strconv.Itoa(airdate)
	} else {
		episodeID = showID + "." + season + "." + strconv.Itoa(episode)
	}
	return episodeID
}

func int64ToTime(i int64) time.Time {
	return time.Unix(i, 0)
}

func dateStringToInt(s string) (int, error) {
	switch len(s) {
	case 10:
		year, err := strconv.Atoi(utils.Substring(s, 0, 4))
		if err != nil {
			return 0, err
		}
		month, err := strconv.Atoi(utils.Substring(s, 5, 2))
		if err != nil {
			return 0, err
		}
		day, err := strconv.Atoi(utils.Substring(s, -2, 2))
		if err != nil {
			return 0, err
		}
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
		return int(date.Unix()), nil
	case 17:
		date, err := time.Parse("2006-01-02T15:04Z", s)
		if err != nil {
			return 0, err
		}
		return int(date.Unix()), nil
	default:
		err := fmt.Errorf("unsupported Datetime format: %s", s)
		return 0, err
	}
}

func dateYearToInt(y int) int {
	date := time.Date(y, time.Month(1), 1, 0, 0, 0, 0, time.Local)
	return int(date.Unix())
}

func sanitizeSqlString(s string) string {
	var out []rune
	for _, s := range s {
		switch s {
		case '\'':
			out = append(out, '\'', '\'')
		default:
			out = append(out, s)
		}
	}
	return string(out)
}
