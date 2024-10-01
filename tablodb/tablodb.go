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

type ScheduledAiringRecord struct {
	AiringID     int
	AirDate      int
	ShowType     string
	ShowTitle    string
	Season       string
	Episode      int
	EpisodeTitle string
	ReleaseYear  int
}

type PrioritizedConflictRecord struct {
	AiringID int
	ShowType string
	AirDate  int
	EndDate  int
	Priority int
}

type conflictRecord struct {
	airingID int
	showID   int
	airDate  int
	duration int
	endDate  int
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
		qrySelectQueueRecordByAction := fmt.Sprintf(templates["selectQueueRecordByAction"], sanitizeSqlString(action))
		var count int
		err := db.database.QueryRow(qrySelectQueueRecordByAction).Scan(&count)
		if err != nil {
			db.log.Println(qry)
			db.log.Println(err)
			return err
		} else if count == 0 {
			qry = fmt.Sprintf(templates["insertQueuePriority"], sanitizeSqlString(action), sanitizeSqlString(details), sanitizeSqlString(exportPath))
		} else {
			return nil
		}
	} else {
		qry = fmt.Sprintf(templates["insertQueue"], sanitizeSqlString(action), sanitizeSqlString(details), sanitizeSqlString(exportPath))
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
		if v.ObjectID == 0 {
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
			channelValue.WriteString("-" + strconv.Itoa(v.ObjectID))
		} else {
			channelValue.WriteString(strconv.Itoa(v.ObjectID))
		}
		channelValue.WriteString(",'")
		channelValue.WriteString(sanitizeSqlString(v.Channel.CallSign))
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
		if s.ObjectID == 0 {
			continue
		}

		updateType := strings.Split(k, "/")[1]

		showID := strconv.Itoa(s.ObjectID)
		if updateType == "recordings" {
			showID = "-" + showID
		}

		parentShowID := "null"
		if s.GuidePath != "" {
			parentShowID = strings.Split(s.GuidePath, "/")[3]
		}

		rule := "null"
		if s.Schedule.Rule != "" {
			rule = "'" + sanitizeSqlString(s.Schedule.Rule) + "'"
		}

		channelID := "null"
		if s.Schedule.ChannelPath != "" {
			channelID = strings.Split(s.Schedule.ChannelPath, "/")[3]
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
			if s.Series.OrigAirDate != nil {
				airdateInt, err := dateStringToInt(*s.Series.OrigAirDate)
				if err != nil {
					db.log.Println(err)
					return err
				}
				airdate = strconv.Itoa(airdateInt)
			}

			rating := "null"
			if s.Series.SeriesRating != nil {
				rating = "'" + sanitizeSqlString(*s.Series.SeriesRating) + "'"
			}

			showValue.WriteString(sanitizeSqlString(s.Series.Title))
			showValue.WriteString("',")
			showValue.WriteString(description) // Previously sanitized
			showValue.WriteRune(',')
			showValue.WriteString(airdate) // Int value, no sanitization needed
			showValue.WriteRune(',')
			showValue.WriteString(strconv.Itoa(s.Series.EpisodeRuntime))
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
			if s.Movie.ReleaseYear != nil {
				airdate = strconv.Itoa(dateYearToInt(*s.Movie.ReleaseYear))
			}

			filmRating := "null"
			if s.Movie.FilmRating != nil {
				filmRating = "'" + sanitizeSqlString(*s.Movie.FilmRating) + "'"
			}

			qualityRating := "null"
			if s.Movie.QualityRating != nil {
				qualityRating = strconv.Itoa(*s.Movie.QualityRating)
			}

			showValue.WriteString(sanitizeSqlString(s.Movie.Title))
			showValue.WriteString("',")
			showValue.WriteString(description) // Previosly sanitized
			showValue.WriteRune(',')
			showValue.WriteString(airdate) // Int value, no sanitization needed
			showValue.WriteRune(',')
			showValue.WriteString(strconv.Itoa(s.Movie.OriginalRuntime))
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
		if a.ObjectID == 0 {
			continue
		}

		var showID string
		var episodeID string
		airdate, err := dateStringToInt(a.AiringDetails.Datetime)
		if err != nil {
			db.log.Println(err)
			return err
		}

		if a.SeriesPath != "" {
			showID = strings.Split(a.SeriesPath, "/")[3]

			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, strconv.Itoa(a.Episode.SeasonNumber), a.Episode.Number, airdate)) + "'"

			title := "null"
			if a.Episode.Title != "" {
				title = "'" + sanitizeSqlString(a.Episode.Title) + "'"
			}

			description := "null"
			if a.Episode.Description != "" {
				description = "'" + sanitizeSqlString(a.Episode.Description) + "'"
			}

			originalAirDate := "null"
			if a.Episode.OrigAirDate != nil {
				dateInt, err := dateStringToInt(*a.Episode.OrigAirDate)
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
			episodeValue.WriteString(strconv.Itoa(a.Episode.SeasonNumber))
			episodeValue.WriteString("',null,")
			episodeValue.WriteString(originalAirDate) // Int value, no sanitization needed
			episodeValue.WriteString(",null)")
			episodeValues = append(episodeValues, episodeValue.String())
		} else if a.MoviePath != "" {
			showID = strings.Split(a.MoviePath, "/")[3]
			episodeID = "null"
		} else if a.SportPath != "" {
			showID = strings.Split(a.SportPath, "/")[3]
			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, a.Event.Season, 0, airdate)) + "'"

			for _, t := range a.Event.Teams {
				var teamValue strings.Builder
				teamValue.WriteRune('(')
				teamValue.WriteString(strconv.Itoa(t.TeamID))
				teamValue.WriteString(",'")
				teamValue.WriteString(sanitizeSqlString(t.Name))
				teamValue.WriteString("')")
				teamValues = append(teamValues, teamValue.String())

				var episodeTeamValue strings.Builder
				episodeTeamValue.WriteRune('(')
				episodeTeamValue.WriteString(episodeID)
				episodeTeamValue.WriteRune(',')
				episodeTeamValue.WriteString(strconv.Itoa(t.TeamID))
				episodeTeamValue.WriteRune(')')
				episodeTeamValues = append(episodeTeamValues, episodeTeamValue.String())
			}

			season := "null"
			if a.Event.Season != "" {
				season = "'" + sanitizeSqlString(a.Event.Season) + "'"
			}

			seasonType := "null"
			if a.Event.SeasonType != "" {
				seasonType = "'" + sanitizeSqlString(a.Event.SeasonType) + "'"
			}

			homeTeamID := "null"
			if a.Event.HomeTeamID != nil {
				homeTeamID = strconv.Itoa(*a.Event.HomeTeamID)
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
			err := fmt.Errorf("No show path for %d", a.ObjectID)
			db.log.Println(err)
			return err
		}

		var airingValue strings.Builder
		airingValue.WriteRune('(')
		airingValue.WriteString(strconv.Itoa(a.ObjectID))
		airingValue.WriteRune(',')
		airingValue.WriteString(sanitizeSqlString(showID))
		airingValue.WriteRune(',')
		airingValue.WriteString(strconv.Itoa(airdate))
		airingValue.WriteRune(',')
		airingValue.WriteString(strconv.Itoa(a.AiringDetails.Duration))
		airingValue.WriteRune(',')
		airingValue.WriteString(strconv.Itoa(a.AiringDetails.Channel.ObjectID))
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
	var recordingIDs []string

	for _, r := range recordings {
		if r.ObjectID == 0 {
			continue
		}

		var showID string
		var episodeID string
		airdate, err := dateStringToInt(r.AiringDetails.Datetime)
		if err != nil {
			db.log.Println(err)
			return err
		}

		if r.SeriesPath != "" {
			showID = "-" + strings.Split(r.SeriesPath, "/")[3]

			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, strconv.Itoa(r.Episode.SeasonNumber), r.Episode.Number, airdate)) + "'"

			title := "null"
			if r.Episode.Title != "" {
				title = "'" + sanitizeSqlString(r.Episode.Title) + "'"
			}

			description := "null"
			if r.Episode.Description != "" {
				description = "'" + sanitizeSqlString(r.Episode.Description) + "'"
			}

			originalAirDate := "null"
			if r.Episode.OrigAirDate != nil {
				dateInt, err := dateStringToInt(*r.Episode.OrigAirDate)
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
			episodeValue.WriteString(strconv.Itoa(r.Episode.SeasonNumber))
			episodeValue.WriteString("',null,")
			episodeValue.WriteString(originalAirDate) // Int value, no sanitization needed
			episodeValue.WriteString(",null)")
			episodeValues = append(episodeValues, episodeValue.String())
		} else if r.MoviePath != "" {
			showID = "-" + strings.Split(r.MoviePath, "/")[3]
			episodeID = "null"
		} else if r.SportPath != "" {
			showID = "-" + strings.Split(r.SportPath, "/")[3]
			episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, r.Event.Season, 0, airdate)) + "'"

			for _, t := range r.Event.Teams {
				var teamValue strings.Builder
				teamValue.WriteRune('(')
				teamValue.WriteString(strconv.Itoa(t.TeamID))
				teamValue.WriteString(",'")
				teamValue.WriteString(sanitizeSqlString(t.Name))
				teamValue.WriteString("')")
				teamValues = append(teamValues, teamValue.String())

				var episodeTeamValue strings.Builder
				episodeTeamValue.WriteRune('(')
				episodeTeamValue.WriteString(episodeID)
				episodeTeamValue.WriteRune(',')
				episodeTeamValue.WriteString(strconv.Itoa(t.TeamID))
				episodeTeamValue.WriteRune(')')
				episodeTeamValues = append(episodeTeamValues, episodeTeamValue.String())
			}

			season := "null"
			if r.Event.Season != "" {
				season = "'" + sanitizeSqlString(r.Event.Season) + "'"
			}

			seasonType := "null"
			if r.Event.SeasonType != "" {
				seasonType = "'" + sanitizeSqlString(r.Event.SeasonType) + "'"
			}

			homeTeamID := "null"
			if r.Event.HomeTeamID != nil {
				homeTeamID = strconv.Itoa(*r.Event.HomeTeamID)
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
			err := fmt.Errorf("No show path for %d", r.ObjectID)
			db.log.Println(err)
			return err
		}

		clean := "0"
		if r.VideoDetails.Clean {
			clean = "1"
		}

		var recordingValue strings.Builder
		recordingValue.WriteRune('(')
		recordingValue.WriteString(strconv.Itoa(r.ObjectID))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(sanitizeSqlString(showID))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(airdate))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(r.AiringDetails.Duration))
		recordingValue.WriteRune(',')
		recordingValue.WriteString("-" + strconv.Itoa(r.AiringDetails.Channel.ObjectID))
		recordingValue.WriteString(",'")
		recordingValue.WriteString(sanitizeSqlString(r.VideoDetails.State))
		recordingValue.WriteString("',")
		recordingValue.WriteString(clean) // Bool value, no sanitization needed
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(r.VideoDetails.Duration))
		recordingValue.WriteRune(',')
		recordingValue.WriteString(strconv.Itoa(r.VideoDetails.Size))
		recordingValue.WriteString(",'")
		recordingValue.WriteString(sanitizeSqlString(r.VideoDetails.ComSkip.State))
		recordingValue.WriteString("',")
		recordingValue.WriteString(episodeID) // previously sanitized
		recordingValue.WriteRune(')')
		recordingValues = append(recordingValues, recordingValue.String())

		recordingIDs = append(recordingIDs, strconv.Itoa(r.ObjectID))

		if r.VideoDetails.State == "failed" || !r.VideoDetails.Clean || r.VideoDetails.ComSkip.State != "none" {
			var comSkipError = "null"
			if r.VideoDetails.ComSkip.Error != nil {
				comSkipError = "'" + sanitizeSqlString(*r.VideoDetails.ComSkip.Error) + "'"
			}

			var errorCode = "null"
			if r.VideoDetails.Error.Code != nil {
				comSkipError = "'" + sanitizeSqlString(*r.VideoDetails.Error.Code) + "'"
			}

			var errorDetails = "null"
			if r.VideoDetails.Error.Details != nil {
				comSkipError = "'" + sanitizeSqlString(*r.VideoDetails.Error.Details) + "'"
			}

			var errorDescription = "null"
			if r.VideoDetails.Error.Description != nil {
				comSkipError = "'" + sanitizeSqlString(*r.VideoDetails.Error.Description) + "'"
			}

			var errorValue strings.Builder
			errorValue.WriteRune('(')
			errorValue.WriteString(strconv.Itoa(r.ObjectID))
			errorValue.WriteRune(',')
			errorValue.WriteString(sanitizeSqlString(showID))
			errorValue.WriteRune(',')
			errorValue.WriteString(episodeID) // previously sanitized
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.AiringDetails.Channel.ObjectID))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(airdate))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.AiringDetails.Duration))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.VideoDetails.Duration))
			errorValue.WriteRune(',')
			errorValue.WriteString(strconv.Itoa(r.VideoDetails.Size))
			errorValue.WriteString(",'")
			errorValue.WriteString(sanitizeSqlString(r.VideoDetails.State))
			errorValue.WriteString("',")
			errorValue.WriteString(clean) // Bool value. No sanitization needed
			errorValue.WriteString(",'")
			errorValue.WriteString(sanitizeSqlString(r.VideoDetails.ComSkip.State))
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

	db.log.Println("purging deleted recordings")
	qryDeleteRemovedRecordings := fmt.Sprintf(templates["deleteRemovedRecordings"], strings.Join(recordingIDs, "),("))
	_, err = db.database.Exec(qryDeleteRemovedRecordings)
	if err != nil {
		db.log.Println(qryDeleteRemovedRecordings)
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

func (db *TabloDB) UpdateSpace(total int64, free int64) error {
	qryUpdateSpace := fmt.Sprintf(templates["updateSpace"], total, free)
	_, err := db.database.Exec(qryUpdateSpace)
	if err != nil {
		db.log.Println(qryUpdateSpace)
		db.log.Println(err)
		return err
	}

	return nil
}

func (db *TabloDB) UpdateConflicts() error {
	_, err := db.database.Exec(queries["deleteConflicts"])
	if err != nil {
		fmt.Println(queries["deleteConflicts"])
		fmt.Println(err)
		return err
	}

	rows, err := db.database.Query(queries["selectConflicts"])
	if err != nil {
		db.log.Println(queries["selectConflicts"])
		db.log.Println(err)
		return err
	}

	var conflicts []conflictRecord
	for rows.Next() {
		var conflict conflictRecord
		err = rows.Scan(&conflict.airingID, &conflict.showID, &conflict.airDate, &conflict.duration)
		if err != nil {
			db.log.Println(err)
			return err
		}
		conflict.endDate = conflict.airDate + conflict.duration
		conflicts = append(conflicts, conflict)
	}

	rows.Close()

	if len(conflicts) == 0 {
		db.log.Println("No conflicts")
		return nil
	}

	rows, err = db.database.Query(queries["selectScheduled"])
	if err != nil {
		db.log.Println(queries["selectScheduled"])
		db.log.Println(err)
		return err
	}

	var scheduled []*conflictRecord

	for rows.Next() {
		schedule := new(conflictRecord)
		err = rows.Scan(&schedule.airingID, &schedule.showID, &schedule.airDate, &schedule.duration)
		if err != nil {
			db.log.Println(err)
			return err
		}
		schedule.endDate = schedule.airDate + schedule.duration
		scheduled = append(scheduled, schedule)
	}

	rows.Close()

	var conflictValues []string
	for _, c := range conflicts {
		conflictValue := createConflictValue(c)
		conflictValues = append(conflictValues, conflictValue)
		for i, s := range scheduled {
			if s == nil {
				continue
			}

			if isOverlapping(c, *s) {
				conflictValue := createConflictValue(*s)
				conflictValues = append(conflictValues, conflictValue)
				scheduled[i] = nil
			}
		}
	}

	if len(conflictValues) == 0 {
		err = fmt.Errorf("no conflicts in insert values")
		fmt.Println(err)
		return err
	}

	qryInsertConflicts := fmt.Sprintf(templates["insertConflicts"], strings.Join(conflictValues, ","))
	_, err = db.database.Exec(qryInsertConflicts)
	if err != nil {
		fmt.Println(qryInsertConflicts)
		fmt.Println(err)
		return err
	}

	return nil
}

func (db *TabloDB) GetExported() ([]string, error) {
	db.log.Println("selecting exported values")

	rows, err := db.database.Query(queries["selectExported"])
	if err != nil {
		db.log.Println(queries["selectExported"])
		db.log.Println(err)
		return nil, err
	}

	var exported []string
	for rows.Next() {
		var export string
		err = rows.Scan(&export)
		if err != nil {
			db.log.Println(err)
			return nil, err
		}

		exported = append(exported, export)
	}

	db.log.Printf("%d exported values selected\n", len(exported))
	return exported, nil
}

func (db *TabloDB) DeleteExported(toDelete []string) error {
	db.log.Printf("removing %d missing exports\n", len(toDelete))

	var sanitizedToDelete []string
	for _, v := range toDelete {
		sanitizedToDelete = append(sanitizedToDelete, sanitizeSqlString(v))
	}

	qryDeleteExported := fmt.Sprintf(templates["deleteExported"], strings.Join(sanitizedToDelete, "','"))
	_, err := db.database.Exec(qryDeleteExported)
	if err != nil {
		db.log.Println(qryDeleteExported)
		db.log.Println(err)
		return err
	}

	db.log.Println("missing exports removed")
	return nil
}

func (db *TabloDB) InsertExported(toInsert []string) error {
	db.log.Printf("inserting %d exported values\n", len(toInsert))

	var sanitizedToInsert []string
	for _, v := range toInsert {
		sanitizedToInsert = append(sanitizedToInsert, sanitizeSqlString(v))
	}

	qryInsertExported := fmt.Sprintf(templates["insertExported"], strings.Join(sanitizedToInsert, "'),('"))
	_, err := db.database.Exec(qryInsertExported)
	if err != nil {
		db.log.Println(qryInsertExported)
		db.log.Println(err)
		return err
	}

	db.log.Println("exported values inserted")
	return nil
}

func (db *TabloDB) GetScheduledAirings() ([]ScheduledAiringRecord, error) {
	db.log.Println("getting all scheduled airings")

	rows, err := db.database.Query(queries["selectScheduledAirings"])
	if err != nil {
		db.log.Println(err)
		return nil, err
	}

	var airings []ScheduledAiringRecord
	for rows.Next() {
		var airing ScheduledAiringRecord
		err = rows.Scan(&airing.AiringID, &airing.ShowType, &airing.ShowTitle, &airing.Season, &airing.Episode, &airing.AirDate, &airing.EpisodeTitle, &airing.ReleaseYear)
		if err != nil {
			db.log.Println(err)
			return nil, err
		}

		releaseDate := time.Unix(int64(airing.ReleaseYear), 0)
		airing.ReleaseYear = releaseDate.Year()
		airings = append(airings, airing)
	}

	db.log.Printf("%d scheduled airings found\n", len(airings))
	return airings, nil
}

func (db *TabloDB) PurgeExpiredAirings() error {
	db.log.Println("Deleting expired airings")
	now := time.Now().Unix()
	qryDeleteExpiredAirings := fmt.Sprintf(templates["deleteExpiredAirings"], now)
	_, err := db.database.Exec(qryDeleteExpiredAirings)
	if err != nil {
		db.log.Println(qryDeleteExpiredAirings)
		db.log.Println(err)
		return err
	}
	db.log.Println("Expired airings deleted")
	return nil
}

func (db *TabloDB) DeleteAiring(airingID int) error {
	db.log.Printf("deleting airingID %d\n", airingID)
	qryDeleteAiringByID := fmt.Sprintf(templates["deleteAiringByID"], strconv.Itoa(airingID))
	_, err := db.database.Exec(qryDeleteAiringByID)
	if err != nil {
		db.log.Println(qryDeleteAiringByID)
		db.log.Println(err)
		return err
	}

	db.log.Println("airing deleted")
	return err
}

func (db *TabloDB) UpsertSingleAiring(airing tabloapi.Airing) error {
	db.log.Printf("updating airing %d\n", airing.ObjectID)

	var showID string
	var episodeID string
	airdate, err := dateStringToInt(airing.AiringDetails.Datetime)
	if err != nil {
		db.log.Println(err)
		return err
	}

	if airing.SeriesPath != "" {
		showID = strings.Split(airing.SeriesPath, "/")[3]
		episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, strconv.Itoa(airing.Episode.SeasonNumber), airing.Episode.Number, airdate)) + "'"
	} else if airing.MoviePath != "" {
		showID = strings.Split(airing.MoviePath, "/")[3]
		episodeID = "null"
	} else if airing.SportPath != "" {
		showID = strings.Split(airing.SportPath, "/")[3]
		episodeID = "'" + sanitizeSqlString(getEpisodeID(showID, airing.Event.Season, 0, airdate)) + "'"
	} else {
		err := fmt.Errorf("No show path for %d", airing.ObjectID)
		db.log.Println(err)
		return err
	}

	var airingValue strings.Builder
	airingValue.WriteRune('(')
	airingValue.WriteString(strconv.Itoa(airing.ObjectID))
	airingValue.WriteRune(',')
	airingValue.WriteString(sanitizeSqlString(showID))
	airingValue.WriteRune(',')
	airingValue.WriteString(strconv.Itoa(airdate))
	airingValue.WriteRune(',')
	airingValue.WriteString(strconv.Itoa(airing.AiringDetails.Duration))
	airingValue.WriteRune(',')
	airingValue.WriteString(strconv.Itoa(airing.AiringDetails.Channel.ObjectID))
	airingValue.WriteString(",'")
	airingValue.WriteString(sanitizeSqlString(airing.Schedule.State))
	airingValue.WriteString("',")
	airingValue.WriteString(episodeID) // previously sanitized
	airingValue.WriteRune(')')

	qryUpsertAiring := fmt.Sprintf(templates["upsertAiring"], airingValue.String())
	_, err = db.database.Exec(qryUpsertAiring)
	if err != nil {
		db.log.Println(qryUpsertAiring)
		db.log.Println(err)
		return err
	}
	return nil
}

func (db *TabloDB) ResetScheduled() error {
	_, err := db.database.Exec(queries["updateAiringScheduledToNone"])
	if err != nil {
		db.log.Println(queries["updateAiringScheduledToNone"])
		db.log.Println(err)
		return err
	}
	return nil
}

func (db *TabloDB) GetPrioritizedConflicts() ([]PrioritizedConflictRecord, error) {
	db.log.Println("getting prioritized conflicts")

	rows, err := db.database.Query(queries["selectPriorityConflicts"])
	if err != nil {
		db.log.Println(err)
		return nil, err
	}

	var conflicts []PrioritizedConflictRecord
	for rows.Next() {
		var conflict PrioritizedConflictRecord
		err = rows.Scan(&conflict.AiringID, &conflict.ShowType, &conflict.Priority, &conflict.AirDate, &conflict.EndDate)
		if err != nil {
			db.log.Println(err)
			return nil, err
		}
		if conflict.ShowType == "movies" {
			conflict.Priority = 0
		}
		if conflict.Priority < 0 {
			err = fmt.Errorf("no priority for %d", conflict.AiringID)
			db.log.Println(err)
			return nil, err
		}
		conflicts = append(conflicts, conflict)
	}

	return conflicts, nil
}

func isOverlapping(c, s conflictRecord) bool {
	if c.airDate == s.airDate || c.endDate == s.endDate {
		// both shows start or end at the same time
		return true
	} else if c.airDate > s.airDate && c.airDate < s.endDate {
		// The first show start time is during the second show
		return true
	} else if c.endDate > s.airDate && c.endDate < s.endDate {
		// The first show end time is during the second show
		return true
	} else if c.airDate < s.airDate && c.endDate > s.endDate {
		// The first show wholly encompasses the second show
		return true
	}
	return false
}

func createConflictValue(c conflictRecord) string {
	var conflictValue strings.Builder
	conflictValue.WriteRune('(')
	conflictValue.WriteString(strconv.Itoa(c.airingID))
	conflictValue.WriteRune(',')
	conflictValue.WriteString(strconv.Itoa(c.showID))
	conflictValue.WriteRune(',')
	conflictValue.WriteString(strconv.Itoa(c.airDate))
	conflictValue.WriteRune(',')
	conflictValue.WriteString(strconv.Itoa(c.endDate))
	conflictValue.WriteRune(')')
	return conflictValue.String()
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
