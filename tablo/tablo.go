package tablo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	queue                 []tablodb.QueueRecord
	log                   *log.Logger
}

func New(databaseDir string) ([]*Tablo, error) {
	logFile, err := os.OpenFile(databaseDir+string(os.PathSeparator)+"main.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, userRWX)
	if err != nil {
		return nil, err
	}
	tabloFactoryLog := log.New(io.MultiWriter(logFile, os.Stdout), "tablo: ", log.LstdFlags)

	var tablos []*Tablo

	tabloFactoryLog.Println("checking for existing caches")
	files, err := os.ReadDir(databaseDir)
	if err != nil {
		tabloFactoryLog.Println(err.Error())
		return nil, err
	}
	for _, v := range files {
		fileName := v.Name()
		if utils.Substring(fileName, -6, 6) == ".cache" {
			// TODO: Open the database and check it
			// If it is valid, we can add it to the Tablo & return
		}
	}

	tabloFactoryLog.Println("getting tablo info from web")
	tabloWebResponse, err := get(tabloWebUri)
	if err != nil {
		tabloFactoryLog.Println(err.Error())
		return nil, err
	}

	tabloFactoryLog.Println("unmarshalling tablo info")
	var tabloInfo tabloapi.WebApiResp
	err = json.Unmarshal(tabloWebResponse, &tabloInfo)
	if err != nil {
		tabloFactoryLog.Println(err.Error())
		return nil, err
	}

	tabloFactoryLog.Println("creating Tablo object for each tablo retrieved")
	var errMessage strings.Builder
	for _, v := range tabloInfo.Cpes {
		tablo := &Tablo{
			ipAddress:             v.Private_ip,
			name:                  v.Name,
			serverID:              v.Serverid,
			guideLastUpdated:      time.Unix(0, 0),
			scheduledLastUpdated:  time.Unix(0, 0),
			recordingsLastUpdated: time.Unix(0, 0),
			log:                   log.New(io.MultiWriter(logFile, os.Stdout), "tablo "+v.Serverid+": ", log.LstdFlags),
		}
		tablo.database, err = tablodb.New(tablo.ipAddress, tablo.name, tablo.serverID, databaseDir)
		if err != nil {
			tabloFactoryLog.Println(err.Error())
			errMessage.WriteString(v.Serverid + ": " + err.Error())
		} else {
			tablos = append(tablos, tablo)
		}
	}

	if errMessage.String() != "" {
		// errors logged during tabloInfo.Cpes iteration don't need to be logged now
		return tablos, errors.New(errMessage.String())
	}

	tabloFactoryLog.Printf("%d tablos created\n", len(tablos))
	return tablos, nil
}

func (t *Tablo) String() string {
	return fmt.Sprintf("Name: %s, ID: %s, IP: %s", t.name, t.serverID, t.ipAddress)
}

func (t *Tablo) Close() {
	t.log.Println("closing tablo database")
	defer t.database.Close()
}

func (t *Tablo) NeedUpdate() bool {
	now := time.Now()

	return now.After(t.scheduledLastUpdated.Add(6*time.Hour)) || now.After(t.guideLastUpdated.Add(24*time.Hour)) || now.After(t.recordingsLastUpdated.Add(6*time.Hour))
}

func (t *Tablo) EnqueueUpdate() error {
	t.log.Println("enqueueing update tasks")
	now := time.Now()

	if now.After(t.recordingsLastUpdated.Add(6 * time.Hour)) {
		t.log.Printf("last recording update at %v. enqueueing recording update\n", t.recordingsLastUpdated)
		err := t.database.Enqueue("UPDATERECORDINGS", "", "")
		if err != nil {
			t.log.Println(err)
			return err
		}
	}

	if now.After(t.guideLastUpdated.Add(24 * time.Hour)) {
		t.log.Printf("last guide update at %v. enqueueing guide update\n", t.guideLastUpdated)
		err := t.database.Enqueue("UPDATEGUIDE", "", "")
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else if now.After(t.scheduledLastUpdated.Add(6 * time.Hour)) {
		t.log.Printf("last schedule update at %v. enqueueing schedule update\n", t.scheduledLastUpdated)
		err := t.database.Enqueue("UPDATESCHEDULED", "", "")
		if err != nil {
			t.log.Println(err)
			return err
		}
	}

	t.log.Println("update tasks enqueued")

	return nil
}

func (t *Tablo) HasQueueItems() bool {
	return len(t.queue) > 0
}

func (t *Tablo) LoadQueue() error {
	t.log.Println("loading queue from cache")
	q, err := t.database.GetQueue()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.queue = q
	t.log.Printf("loaded %d queue records\n", len(q))
	return nil
}

func (t *Tablo) ProcessQueue() error {
	t.log.Println("processing all queue records")
	for _, q := range t.queue {
		switch q.Action {
		case "UPDATEGUIDE":
			t.log.Println("updating guide")
			err := t.updateGuide()
			if err != nil {
				t.log.Println(err)
				return err
			}
		case "UPDATESCHEDULED":
			t.log.Println("updating schedule")
			err := t.updateScheduled()
			if err != nil {
				t.log.Println(err)
				return err
			}
		case "UPDATERECORDINGS":
			t.log.Println("updating recordings")
			err := t.updateRecordings()
			if err != nil {
				t.log.Println(err)
				return err
			}
		case "EXPORT":
			t.log.Printf("exporting %s\n", q.Details)
			err := t.exportRecording(q.Details, q.ExportPath)
			if err != nil {
				t.log.Println(err)
				return err
			}
		default:
			t.log.Printf("invalid action: %s\n", q.Action)
		}
		t.log.Printf("deleting queue record %d %s %s\n", q.QueueID, q.Action, q.Details)
		err := t.database.DeleteQueueRecord(q.QueueID)
		if err != nil {
			t.log.Println(err)
			return err
		}
	}
	t.queue = nil
	t.log.Println("all queue records processed")
	return nil
}

func (t *Tablo) updateGuide() error {
	t.log.Println("updating channels")
	err := t.updateChannels()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("updating shows")
	err = t.updateShows()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("updating airings")
	err = t.updateAirings()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("guide updated")
	return nil
}

func (t *Tablo) updateScheduled() error {
	t.log.Println("updating channels")
	err := t.updateChannels()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("updating shows")
	err = t.updateShows()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("updating scheduled airings")
	err = t.updateScheduledAirings()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("channels updated")
	return nil
}

func (t *Tablo) updateRecordings() error {
	t.log.Println("updating recording channels")
	err := t.updateRecordingChannels()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("updating recording shows")
	err = t.updateRecordingShows()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("updating recordings")
	err = t.updateRecordingAirings()
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("recording channels updated")
	return nil
}

func (t *Tablo) updateChannels() error {
	t.log.Println("updating channels")

	uri := "http://" + t.ipAddress + ":8885"
	response, err := get(uri + "/guide/channels")
	if err != nil {
		t.log.Println(err)
		return err
	}

	var channels []string
	err = json.Unmarshal(response, &channels)
	if err != nil {
		t.log.Println(err)
		return err
	}

	if len(channels) > 0 {
		t.log.Printf("getting details for %d channels\n", len(channels))
		response, err = batch(uri, channels)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no channels returned")
		t.log.Println(err)
		return err
	}

	var channelDetails map[string]tabloapi.Channel
	err = json.Unmarshal(response, &channelDetails)
	if err != nil {
		t.log.Println(err)
		return err
	}
	if len(channelDetails) > 0 {
		t.log.Printf("adding %d channels to database\n", len(channelDetails))
		err = t.database.InsertChannels(channelDetails)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no channel details returned")
		t.log.Println(err)
		return err
	}
	t.log.Println("channels updated")
	return nil
}

func (t *Tablo) updateShows() error {
	t.log.Println("updating shows")

	uri := "http://" + t.ipAddress + ":8885"
	response, err := get(uri + "/guide/shows")
	if err != nil {
		t.log.Println(err)
		return err
	}

	var shows []string
	err = json.Unmarshal(response, &shows)
	if err != nil {
		t.log.Println(err)
		return err
	}

	if len(shows) > 0 {
		t.log.Printf("getting details for %d shows\n", len(shows))
		response, err = batch(uri, shows)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no shows returned")
		t.log.Println(err)
		return err
	}

	var showDetails map[string]tabloapi.Show
	err = json.Unmarshal(response, &showDetails)
	if err != nil {
		t.log.Println(err)
		return err
	}
	if len(showDetails) > 0 {
		t.log.Printf("adding %d shows to database\n", len(showDetails))
		err = t.database.InsertShows(showDetails)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no show details returned")
		t.log.Println(err)
		return err
	}

	t.log.Println("shows updated")
	return nil
}

func (t *Tablo) exportRecording(toExport string, exportPath string) error {
	// TODO: Write up export
	t.log.Printf("%s : %s\n", toExport, exportPath)
	t.log.Println("not yet implemented")
	return nil
}

func (t *Tablo) updateScheduledAirings() error {
	// TODO: Write up update airings
	t.log.Println("not yet implemented")
	return nil
}

func (t *Tablo) updateAirings() error {
	// TODO: Write up update airings
	t.log.Println("not yet implemented")
	return nil
}

func (t *Tablo) updateRecordingChannels() error {
	// TODO: Write up update recording channels
	t.log.Println("not yet implemented")
	return nil
}

func (t *Tablo) updateRecordingShows() error {
	// TODO: Write up update recording shows
	t.log.Println("not yet implemented")
	return nil
}

func (t *Tablo) updateRecordingAirings() error {
	// TODO: Write up update recording airings
	t.log.Println("not yet implemented")
	return nil
}

func batch(uri string, input []string) ([]byte, error) {
	// fmt.Printf("batch processing %d items\n", len(input))
	var data strings.Builder
	var output []byte
	output = append(output, byte('{'))
	data.WriteRune('[')
	for i, v := range input {
		data.WriteString("\"" + v + "\"")
		if (i+1)%50 == 0 {
			data.WriteRune(']')
			// fmt.Println(data.String())
			tempOutput, err := post(uri+"/batch", data.String())
			if err != nil {
				// fmt.Println(err)
				return nil, err
			}
			trimmedOutput := tempOutput[1 : len(tempOutput)-1]
			// fmt.Println(string(trimmedOutput))
			output = append(output, trimmedOutput...)
			output = append(output, byte(','))
			data.Reset()
			data.WriteRune('[')
		} else {
			data.WriteRune(',')
		}
	}

	if len(data.String()) > 1 {
		data.WriteRune(']')
		// fmt.Println(data.String())
		tempOutput, err := post(uri+"/batch", data.String())
		if err != nil {
			return nil, err
		}

		output = append(output, tempOutput[1:len(tempOutput)-1]...)
	}

	output = append(output, byte('}'))
	// fmt.Println(string(output))
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
