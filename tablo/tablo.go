package tablo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"tablo-manager/tabloapi"
	"tablo-manager/tablodb"
	"tablo-manager/utils"
	"time"
)

const tabloWebUri = "https://api.tablotv.com/assocserver/getipinfo/"
const userRWX = 0700 // unix-style octal permission

type Tablo struct {
	ipAddress             string
	serverID              string
	name                  string
	database              tablodb.TabloDB
	guideLastUpdated      time.Time
	scheduledLastUpdated  time.Time
	recordingsLastUpdated time.Time
	queue                 []queueRecord
}

type queueRecord struct {
	action  string
	details string
}

func New(databaseDir string) ([]*Tablo, error) {
	var tablos []*Tablo

	_, err := os.Stat(databaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(databaseDir, userRWX)
			if err != nil {
				return make([]*Tablo, 0), fmt.Errorf("unable to create %s: %s", databaseDir, err)
			}
		} else {
			return make([]*Tablo, 0), fmt.Errorf("unable to access %s %s", databaseDir, err)
		}
	}

	files, err := os.ReadDir(databaseDir)
	if err != nil {
		return make([]*Tablo, 0), fmt.Errorf("unable to access %s %s", databaseDir, err)
	}
	for _, v := range files {
		fileName := v.Name()
		if utils.Substring(fileName, -6, 6) == ".cache" {
			// Open the database and check it
			// If it is valid, we can add it to the Tablo & return
		}
	}

	tabloWebResponse, err := get(tabloWebUri)
	if err != nil {
		return make([]*Tablo, 0), fmt.Errorf("unable to connect to tablo web api %s", err)
	}

	var tabloInfo tabloapi.WebApiResp

	err = json.Unmarshal(tabloWebResponse, &tabloInfo)
	if err != nil {
		return make([]*Tablo, 0), fmt.Errorf("error unmarshalling response %s", err)
	}

	var errMessage strings.Builder
	for _, v := range tabloInfo.Cpes {
		tablo := &Tablo{
			ipAddress:             v.Private_ip,
			name:                  v.Name,
			serverID:              v.Serverid,
			guideLastUpdated:      time.Unix(0, 0),
			scheduledLastUpdated:  time.Unix(0, 0),
			recordingsLastUpdated: time.Unix(0, 0),
		}
		tablo.database, err = tablodb.New(tablo.ipAddress, tablo.name, tablo.serverID, databaseDir)
		if err != nil {
			errMessage.WriteString(v.Serverid + ": " + err.Error())
		} else {
			tablos = append(tablos, tablo)
		}
	}

	if errMessage.String() != "" {
		return tablos, fmt.Errorf(errMessage.String())
	}
	return tablos, nil
}

func get(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (t *Tablo) String() string {
	return fmt.Sprintf("Name: %s, ID: %s, IP: %s", t.name, t.serverID, t.ipAddress)
}

func (t *Tablo) Close() {
	defer t.database.Close()
}

func (t *Tablo) NeedUpdate() bool {
	now := time.Now()

	return now.After(t.scheduledLastUpdated.Add(6*time.Hour)) || now.After(t.guideLastUpdated.Add(24*time.Hour)) || now.After(t.recordingsLastUpdated.Add(6*time.Hour))
}

func (t *Tablo) EnqueueUpdate() {
	now := time.Now()

	if now.After(t.guideLastUpdated.Add(24 * time.Hour)) {
		t.database.Enqueue("UPDATEGUIDE", "")
	} else if now.After(t.scheduledLastUpdated.Add(6 * time.Hour)) {
		t.database.Enqueue("UPDATESCHEDULED", "")
	}

	if now.After(t.recordingsLastUpdated.Add(6 * time.Hour)) {
		t.database.Enqueue("UPDATERECORDINGS", "")
	}
}

func (t *Tablo) HasQueueItems() bool {
	return len(t.queue) > 0
}

func (t *Tablo) LoadQueue() {
	t.queue = make([]queueRecord, 0)
	queueRecords, err := t.database.GetQueue()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, r := range queueRecords {
		for k, v := range r {
			t.queue = append(t.queue, queueRecord{k, v})
		}
	}
}

func (t *Tablo) ProcessQueue() {
	for _, q := range t.queue {
		switch q.action {
		case "UPDATEGUIDE":
			t.updateGuide()
		case "UPDATESCHEDULED":
			t.updateScheduled()
		case "UPDATERECORDINGS":
			t.updateRecordings()
		case "EXPORT":
			t.exportRecording(q.details)
		default:
			fmt.Println("Unsupported action %s", q.action)
		}
	}
}

func (t *Tablo) updateGuide() {
	t.updateChannels()
	t.updateShows()
	t.updateAirings()
	/*
		CREATE TABLE channel (
		  channelID INT NOT NULL PRIMARY KEY,
		  callSign  TEXT NOT NULL,
		  major     INT NOT NULL,
		  minor     INT NOT NULL,
		  network   TEXT
		);

		-- Create show table
		CREATE TABLE show (
		  showID        INT NOT NULL PRIMARY KEY,
		  parentShowID  INT,
		  rule          TEXT,
		  channelID     INT,
		  keepRecording TEXT NOT NULL,
		  count         INT,
		  showType      TEXT NOT NULL,
		  title         TEXT NOT NULL,
		  descript      TEXT,
		  releaseDate   INT,
		  origRunTime   INT,
		  rating        TEXT,
		  stars         INT,
		  FOREIGN KEY (parentShowID) REFERENCES show(showID),
		  FOREIGN KEY (channelID) REFERENCES channel(channelID)
		);

		-- Create showAward table
		CREATE TABLE showAward (
		  showID        INT NOT NULL,
		  won           INT NOT NULL,
		  awardName     TEXT NOT NULL,
		  awardCategory TEXT NOT NULL,
		  awardYear     INT NOT NULL,
		  nominee       TEXT,
		  PRIMARY KEY (showID, awardName, awardCategory, awardYear, nominee),
		  FOREIGN KEY (showID) REFERENCES show(showID)
		);

		-- Create showGenre table
		CREATE TABLE showGenre (
		  showID INT NOT NULL,
		  genre  TEXT NOT NULL,
		  PRIMARY KEY (showID, genre),
		  FOREIGN KEY (showID) REFERENCES show(showID)
		);

		-- Create showCast table
		CREATE TABLE showCastMember (
		  showID     INT NOT NULL,
		  castMember TEXT NOT NULL,
		  PRIMARY KEY (showID, castMember),
		  FOREIGN KEY (showID) REFERENCES show(showID)
		);

		-- Create showDirector table
		CREATE TABLE showDirector (
		  showID   INT NOT NULL,
		  director TEXT NOT NULL,
		  PRIMARY KEY (showID, director),
		  FOREIGN KEY (showID) REFERENCES show(showID)
		);

		-- Create team table
		CREATE TABLE team (
		  teamID INT NOT NULL PRIMARY KEY,
		  team   TEXT NOT NULL
		);

		-- Create episode table
		CREATE TABLE episode (
		  episodeID       TEXT NOT NULL PRIMARY KEY,
		  showID          INT NOT NULL,
		  title           TEXT,
		  descript        TEXT,
		  episode         INT,
		  season          TEXT,
		  seasonType      TEXT,
		  originalAirDate INT,
		  homeTeamID      INT,
		  FOREIGN KEY (showID) REFERENCES show(showID),
		  FOREIGN KEY (homeTeamID) REFERENCES team(teamID)
		);

		-- Create airing table
		CREATE TABLE airing (
		  airingID  INT NOT NULL PRIMARY KEY,
		  showID    INT NOT NULL,
		  airDate   INT NOT NULL,
		  duration  INT NOT NULL,
		  channelID INT NOT NULL,
		  scheduled TEXT NOT NULL,
		  episodeID TEXT,
		  FOREIGN KEY (showID) REFERENCES show(showID),
		  FOREIGN KEY (channelID) REFERENCES channel(channelID),
		  FOREIGN KEY (episodeID) REFERENCES episode(episodeID)
		);

		-- Create episodeTeam table
		CREATE TABLE episodeTeam (
		  episodeID TEXT NOT NULL,
		  teamID    INT NOT NULL,
		  PRIMARY KEY (episodeID, teamID),
		  FOREIGN KEY (episodeID) REFERENCES episode(episodeID),
		  FOREIGN KEY (teamID) REFERENCES team(teamID)
		);

		-- Create recording table
		CREATE TABLE recording (
		  recordingID       INT NOT NULL PRIMARY KEY,
		  showID            INT NOT NULL,
		  airDate           INT NOT NULL,
		  airingDuration    INT NOT NULL,
		  channelID         INT NOT NULL,
		  recordingState    TEXT NOT NULL,
		  clean             INT NOT NULL,
		  recordingDuration INT NOT NULL,
		  recordingSize     INT NOT NULL,
		  comSkipState      TEXT NOT NULL,
		  episodeID         INT,
		  FOREIGN KEY (showID) REFERENCES show(showID),
		  FOREIGN KEY (channelID) REFERENCES channel(channelID),
		  FOREIGN KEY (episodeID) REFERENCES episode(episodeID)
		);

		-- Create error table
		CREATE TABLE error (
		  errorID           INTEGER PRIMARY KEY,
		  recordingID       INT NOT NULL,
		  recordingShowID   INT NOT NULL,
		  showID            INT,
		  episodeID         INT,
		  channelID         INT NOT NULL,
		  airDate           INT NOT NULL,
		  airingDuration    INT NOT NULL,
		  recordingDuration INT NOT NULL,
		  recordingSize     INT NOT NULL,
		  recordingState    TEXT NOT NULL,
		  clean             INT NOT NULL,
		  comSkipState      TEXT NOT NULL,
		  comSkipError      TEXT,
		  errorCode         TEXT,
		  errorDetails      TEXT,
		  errorDescription  TEXT
		);

		-- Create queue table
		CREATE TABLE queue (
		  queueID INTEGER PRIMARY KEY,
		  action  TEXT NOT NULL,
		  details TEXT NOT NULL
		);`,
			"selectQueue": `
		SELECT
		  action,
		  details
		FROM
		  queue
		ORDER BY
		  queueID ASC;`,
		}

		var templates = map[string]string{
			"upsertSystemInfo": `
		INSERT INTO systemInfo (
		  serverID,
		  serverName,
		  privateIP,
		  dbVer,
		  guideLastUpdated,
		  recordingsLastUpdated,
		  scheduledLastUpdated
		)
		VALUES (
		  '%s',
		  '%s',
		  '%s',
		  %d,
		  0,
		  0,
		  0
		)
		ON CONFLICT DO UPDATE SET
		  serverName = '%[2]s',
		  privateIP = '%s';`,
			"insertQueue": `
		INSERT INTO queue (
		  action,
		  details
		)
		VALUES (
		  '%s',
		  '%s'
		);`,
			"insertQueuePriority": `
		INSERT INTO queue (
		  queueID,
		  action,
		  details
		)
		SELECT
		  MIN(queueID) - 1,
		  '%s',
		  '%s'
		FROM queue
		`,
		}

	*/
}

func (t *Tablo) updateScheduled() {
	t.updateChannels()
	t.updateShows()
	t.updateScheduledAirings()
}

func (t *Tablo) updateRecordings() {
	t.updateRecordingChannels()
	t.updateRecordingShows()
	t.updateRecordingAirings()
}

func (t *Tablo) updateChannels() {
	uri := "https://" + t.ipAddress + ":8885"
	response, err := get(uri + "/guide/channels")
	if err != nil {
		fmt.Println(err)
	}

	var channels []string
	err = json.Unmarshal(response, channels)
	if err != nil {
		fmt.Println(err)
	}

	if len(channels) > 0 {
		response, err = batch(uri, channels)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		return
	}

	var channelDetails map[string]tabloapi.Channel
	err = json.Unmarshal(response, channelDetails)
	if len(channelDetails) > 0 {
		t.database.InsertChannels(channelDetails)
	}
}

func (t *Tablo) updateShows() {
	uri := "https://" + t.ipAddress + ":8885"
	response, err := get(uri + "/guide/shows")
	if err != nil {
		fmt.Println(err)
	}

	var shows []string
	err = json.Unmarshal(response, shows)
	if err != nil {
		fmt.Println(err)
	}

	if len(shows) > 0 {
		response, err = batch(uri, shows)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		return
	}

	var showDetails map[string]tabloapi.Show
	err = json.Unmarshal(response, showDetails)
	if len(showDetails) > 0 {
		t.database.InsertShows(showDetails)
	}
}

func batch(uri string, input []string) ([]byte, error) {
	var data strings.Builder
	var output []byte
	output = append(output, byte('{'))
	data.WriteRune('[')
	for i, v := range input {
		data.WriteString("\"" + v + "\"")
		if i%50 == 0 {
			data.WriteRune(']')
			tempOutput, err := post(uri+"/batch", data.String())
			if err != nil {
				return nil, err
			}
			output = append(output, tempOutput[1:len(tempOutput)-1]...)
			data.Reset()
			data.WriteRune('[')
		} else {
			data.WriteRune(',')
		}
	}

	if len(data.String()) > 1 {
		data.WriteRune(']')
		tempOutput, err := post(uri+"/batch", data.String())
		if err != nil {
			return nil, err
		}
		output = append(output, tempOutput[1:len(tempOutput)-1]...)
	}

	output = append(output, byte('}'))

	return output, nil
}

func post(uri string, data string) ([]byte, error) {
	resp, err := http.Post(uri, "application/json", bytes.NewBuffer([]byte(data)))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
