package tablo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"tablo-manager/tabloapi"
	"tablo-manager/tablodb"
	"tablo-manager/utils"
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
		return tablos, errors.New(errMessage.String())
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
			// t.exportRecording(q.details)
			return
		default:
			fmt.Printf("Unsupported action %s\n", q.action)
		}
	}
}

func (t *Tablo) updateGuide() {
	t.updateChannels()
	t.updateShows()
	// t.updateAirings()
}

func (t *Tablo) updateScheduled() {
	t.updateChannels()
	// t.updateShows()
	// t.updateScheduledAirings()
}

func (t *Tablo) updateRecordings() {
	// t.updateRecordingChannels()
	// t.updateRecordingShows()
	// t.updateRecordingAirings()
}

func (t *Tablo) updateChannels() {
	uri := "https://" + t.ipAddress + ":8885"
	response, err := get(uri + "/guide/channels")
	if err != nil {
		fmt.Println(err)
	}

	var channels []string
	err = json.Unmarshal(response, &channels)
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
	json.Unmarshal(response, &channelDetails)
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
	err = json.Unmarshal(response, &shows)
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
	json.Unmarshal(response, &showDetails)
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
