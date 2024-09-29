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
	"strconv"
	"strings"
	"time"

	"tablo-manager/tabloapi"
	"tablo-manager/tablodb"
	"tablo-manager/utils"
)

const tabloWebUri = "https://api.tablotv.com/assocserver/getipinfo/"
const userRWX = 0700 // unix-style octal permission
var showTypeSubpath = map[string]string{
	"series": "/series/episodes",
	"movies": "/movies/airings",
	"sports": "/sports/events",
}

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
	defaultExportPath     string
}

type exportAiring struct {
	airingID   int
	showType   string
	exportFile string
}

func New(databaseDir string) ([]*Tablo, error) {
	logFile, err := os.OpenFile(databaseDir+string(os.PathSeparator)+"main.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, userRWX)
	if err != nil {
		return nil, err
	}
	tabloFactoryLog := log.New(io.MultiWriter(logFile, os.Stdout), "tablo: ", log.LstdFlags)

	var tablos []*Tablo

	var localDBs = make(map[string]string)

	tabloFactoryLog.Println("checking for existing caches")
	files, err := os.ReadDir(databaseDir)
	if err != nil {
		tabloFactoryLog.Println(err.Error())
		return nil, err
	}
	for _, v := range files {
		fileName := v.Name()
		if utils.Substring(fileName, -6, 6) == ".cache" {
			localDBs[utils.Substring(fileName, 0, len(fileName)-6)] = fileName
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
		var tablo *Tablo
		if localDBs[v.ServerID] != "" {
			tablo = &Tablo{
				ipAddress: v.Private_IP,
				name:      v.Name,
				serverID:  v.ServerID,
				log:       log.New(io.MultiWriter(logFile, os.Stdout), "tablo "+v.ServerID+": ", log.LstdFlags),
			}
			tablo.database, err = tablodb.Open(v.ServerID, v.Private_IP, v.Name, databaseDir)
			if err != nil {
				tabloFactoryLog.Println(err)
				err = os.Remove(databaseDir + string(os.PathSeparator) + localDBs[v.ServerID])
				if err != nil {
					tabloFactoryLog.Println(err)
					return nil, err
				}
				tablo = nil
			} else {
				tablo.guideLastUpdated, tablo.scheduledLastUpdated, tablo.recordingsLastUpdated, err = tablo.database.GetLastUpdated()
				if err != nil {
					tabloFactoryLog.Println(err)
					return nil, err
				}
				tablo.defaultExportPath, err = tablo.database.GetDefaultExportPath()
				if err != nil {
					tabloFactoryLog.Println(err)
					return nil, err
				}
				tablos = append(tablos, tablo)
			}

		}
		if tablo == nil {
			tablo = &Tablo{
				ipAddress:             v.Private_IP,
				name:                  v.Name,
				serverID:              v.ServerID,
				guideLastUpdated:      time.Unix(0, 0),
				scheduledLastUpdated:  time.Unix(0, 0),
				recordingsLastUpdated: time.Unix(0, 0),
				log:                   log.New(io.MultiWriter(logFile, os.Stdout), "tablo "+v.ServerID+": ", log.LstdFlags),
			}
			tablo.database, err = tablodb.New(tablo.ipAddress, tablo.name, tablo.serverID, databaseDir)
			if err != nil {
				tabloFactoryLog.Println(err.Error())
				errMessage.WriteString(v.ServerID + ": " + err.Error())
			} else {
				tablos = append(tablos, tablo)
			}
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
		if t.NeedUpdate() {
			t.log.Println("aborting queue processing to queue update")
			break
		}
	}
	t.queue = nil
	t.log.Println("all queue records processed")
	return nil
}

func (t *Tablo) updateGuide() error {
	t.log.Println("updating channels")
	unscheduledCount := 1
	for unscheduledCount > 0 {
		err := t.database.PurgeExpiredAirings()
		if err != nil {
			t.log.Println(err)
			return err
		}
		err = t.updateChannels("/guide/channels")
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating shows")
		err = t.updateShows("/guide/shows")
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating airings")
		err = t.updateAirings("/guide/airings")
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating conflicts")
		err = t.updateConflicts()
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating space")
		err = t.updateSpace()
		if err != nil {
			t.log.Println(err)
			return err
		}
		if t.defaultExportPath != "" {
			t.log.Println("updating exported files")
			unscheduledCount, err = t.updateExported("")
			if err != nil {
				t.log.Println(err)
				return err
			}
		} else {
			unscheduledCount = 0
		}
	}
	t.guideLastUpdated = time.Now()
	t.scheduledLastUpdated = time.Now()
	err := t.database.UpdateGuideLastUpdated(t.guideLastUpdated)
	if err != nil {
		t.log.Println(err)
		return err
	}
	err = t.database.UpdateScheduledLastUpdated(t.scheduledLastUpdated)
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("guide updated")
	return nil
}

func (t *Tablo) updateScheduled() error {
	t.log.Println("updating channels")
	unscheduledCount := 1
	for unscheduledCount > 0 {
		err := t.database.PurgeExpiredAirings()
		if err != nil {
			t.log.Println(err)
			return err
		}
		err = t.updateChannels("/guide/channels")
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating shows")
		err = t.updateShows("/guide/shows")
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("resetting schedule status")
		err = t.database.ResetScheduled()
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating scheduled airings")
		err = t.updateAirings("/guide/airings?state=scheduled")
		if err != nil {
			t.log.Println(err)
			if err.Error() != "no airings returned" {
				return err
			}
		}
		t.log.Println("updating conflicted airings")
		err = t.updateAirings("/guide/airings?state=conflicted")
		if err != nil {
			t.log.Println(err)
			if err.Error() != "no airings returned" {
				return err
			}
		}
		t.log.Println("updating conflicts")
		err = t.updateConflicts()
		if err != nil {
			t.log.Println(err)
			return err
		}
		t.log.Println("updating space")
		err = t.updateSpace()
		if err != nil {
			t.log.Println(err)
			return err
		}
		if t.defaultExportPath != "" {
			t.log.Println("updating exported files")
			unscheduledCount, err = t.updateExported("")
			if err != nil {
				t.log.Println(err)
				return err
			}
		} else {
			unscheduledCount = 0
		}
	}

	t.scheduledLastUpdated = time.Now()
	err := t.database.UpdateScheduledLastUpdated(t.scheduledLastUpdated)
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("scheduled updated")
	return nil
}

func (t *Tablo) updateRecordings() error {
	t.log.Println("updating recording channels")
	err := t.updateChannels("/recordings/channels")
	if err != nil {
		t.log.Println(err)
		if err.Error() != "no channels returned" {
			return err
		}
	}

	t.log.Println("updating recording shows")
	err = t.updateShows("/recordings/shows")
	if err != nil {
		t.log.Println(err)
		if err.Error() != "no shows returned" {
			return err
		}
	}

	t.log.Println("updating recording airings")
	err = t.updateRecordingAirings()
	if err != nil {
		t.log.Println(err)
		if err.Error() != "no recording airings returned" {
			return err
		}
	}

	t.recordingsLastUpdated = time.Now()
	err = t.database.UpdateRecordingsLastUpdated(t.recordingsLastUpdated)
	if err != nil {
		t.log.Println(err)
		return err
	}
	t.log.Println("recordings updated")
	return nil
}

func (t *Tablo) updateChannels(suffix string) error {
	t.log.Println("updating channels")

	uri := "http://" + t.ipAddress + ":8885"
	response, err := get(uri + suffix)
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
		err = t.database.UpsertChannels(channelDetails)
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

func (t *Tablo) updateShows(suffix string) error {
	t.log.Println("updating shows")

	uri := "http://" + t.ipAddress + ":8885"
	response, err := get(uri + suffix)
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
		err = t.database.UpsertShows(showDetails)
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

func (t *Tablo) updateAirings(suffix string) error {
	t.log.Println("updating airings")

	uri := "http://" + t.ipAddress + ":8885"
	response, err := get(uri + suffix)
	if err != nil {
		t.log.Println(err)
		return err
	}

	var airings []string
	err = json.Unmarshal(response, &airings)
	if err != nil {
		t.log.Println(err)
		return err
	}

	if len(airings) > 0 {
		t.log.Printf("getting details for %d airings\n", len(airings))
		response, err = batch(uri, airings)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no airings returned")
		t.log.Println(err)
		return err
	}

	var airingDetails map[string]tabloapi.Airing
	err = json.Unmarshal(response, &airingDetails)
	if err != nil {
		t.log.Println(err)
		return err
	}
	if len(airingDetails) > 0 {
		t.log.Printf("adding %d airings to database\n", len(airingDetails))
		err = t.database.UpsertAirings(airingDetails)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no airing details returned")
		t.log.Println(err)
		return err
	}

	t.log.Println("airings updated")
	return nil
}

func (t *Tablo) exportRecording(toExport string, exportPath string) error {
	// TODO: Write up export
	t.log.Printf("%s : %s\n", toExport, exportPath)
	t.log.Println("not yet implemented")
	return nil
}

func (t *Tablo) updateRecordingAirings() error {
	t.log.Println("updating recording airings")

	uri := "http://" + t.ipAddress + ":8885"
	response, err := get(uri + "/recordings/airings")
	if err != nil {
		t.log.Println(err)
		return err
	}

	var recordings []string
	err = json.Unmarshal(response, &recordings)
	if err != nil {
		t.log.Println(err)
		return err
	}

	if len(recordings) > 0 {
		t.log.Printf("getting details for %d recording airings\n", len(recordings))
		response, err = batch(uri, recordings)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no recording airings returned")
		t.log.Println(err)
		return err
	}

	var recordingDetails map[string]tabloapi.Recording
	err = json.Unmarshal(response, &recordingDetails)
	if err != nil {
		t.log.Println(err)
		return err
	}
	if len(recordingDetails) > 0 {
		t.log.Printf("adding %d recording airings to database\n", len(recordingDetails))
		err = t.database.UpsertRecordings(recordingDetails)
		if err != nil {
			t.log.Println(err)
			return err
		}
	} else {
		err = fmt.Errorf("no recording airing details returned")
		t.log.Println(err)
		return err
	}

	t.log.Println("recording airings updated")
	return nil
}

func (t *Tablo) updateConflicts() error {
	return t.database.UpdateConflicts()
}

func (t *Tablo) updateSpace() error {
	uri := "http://" + t.ipAddress + ":8885"

	response, err := get(uri + "/server/harddrives")
	if err != nil {
		t.log.Println(err)
		return err
	}

	var drives []tabloapi.Drive
	err = json.Unmarshal(response, &drives)
	if err != nil {
		t.log.Println(err)
		return err
	}

	totalSpace := int64(0)
	freeSpace := int64(0)
	for _, d := range drives {
		totalSpace += d.Size
		freeSpace += d.Free
	}

	err = t.database.UpdateSpace(totalSpace, freeSpace)
	if err != nil {
		t.log.Println(err)
		return err
	}

	return nil
}

func (t *Tablo) updateExported(alternatePath string) (int, error) {
	t.log.Println("updating exported records")
	if t.defaultExportPath == "" && alternatePath == "" {
		err := fmt.Errorf("no export path specified")
		t.log.Println(err)
		return 0, err
	}

	path := t.defaultExportPath
	if alternatePath != "" {
		path = alternatePath
	}

	currentExported, err := t.database.GetExported()
	if err != nil {
		t.log.Println(err)
		return 0, err
	}

	exportedMissing, exportedFound, err := checkExported(currentExported, path)
	if err != nil {
		t.log.Println(err)
		return 0, err
	}
	if len(exportedMissing) > 0 {
		err = t.database.DeleteExported(exportedMissing)
		if err != nil {
			t.log.Println(err)
			return 0, err
		}
	}

	var unscheduledCount int
	if len(exportedFound) > 0 {
		err = t.database.InsertExported(exportedFound)
		if err != nil {
			t.log.Println(err)
			return 0, err
		}

		exportedFoundMap := make(map[string]bool)
		for _, e := range exportedFound {
			exportedFoundMap[e] = true
		}

		scheduled, err := t.database.GetScheduledAirings()
		if err != nil {
			t.log.Println(err)
			return 0, err
		}

		toExportFileNames := t.getExportFilenames(scheduled)

		var toUnschedule []exportAiring
		for _, f := range toExportFileNames {
			if exportedFoundMap[f.exportFile] {
				toUnschedule = append(toUnschedule, f)
			}
		}

		if len(toUnschedule) > 0 {
			unscheduledCount, err = t.unscheduleAirings(toUnschedule)
			if err != nil {
				t.log.Println(err)
				return unscheduledCount, err
			}
		}
	}

	t.log.Println("exported updated")
	return unscheduledCount, nil
}

func (t *Tablo) getExportFilenames(airings []tablodb.ScheduledAiringRecord) []exportAiring {
	var exportFilenames []exportAiring
	for _, a := range airings {
		var export exportAiring
		export.airingID = a.AiringID
		export.showType = a.ShowType
		export.exportFile = getExportFilename(a, t.defaultExportPath)
		exportFilenames = append(exportFilenames, export)
	}

	return exportFilenames
}

func (t *Tablo) unscheduleAirings(airings []exportAiring) (int, error) {
	t.log.Printf("unscheduling %d airings\n", len(airings))
	skipped := 0
	for i, a := range airings {
		uri := "http://" + t.ipAddress + ":8885"
		subpath := "/guide" + showTypeSubpath[a.showType] + "/" + strconv.Itoa(a.airingID)
		resp, err := patch(uri+subpath, `{"scheduled": false}`)
		if err != nil {
			t.log.Println(err)
			return i, err
		}

		var airing tabloapi.Airing
		err = json.Unmarshal(resp, &airing)
		if err != nil {
			t.log.Println(err)
			return i, err
		}
		if airing.Error.Code == "object_not_found" {
			t.log.Printf("%d not found\n", a.airingID)
			skipped++
			err = t.database.DeleteAiring(a.airingID)
			if err != nil {
				t.log.Println(err)
				return i, err
			}
		} else if airing.Schedule.State != "unscheduled" && airing.Schedule.State != "none" {
			err = fmt.Errorf("unschedule failed for %d", a.airingID)
			t.log.Printf("returned: %+v/n", airing)
			t.log.Println(err)
			return i, err
		}
		err = t.database.UpsertSingleAiring(airing)
		if err != nil {
			t.log.Println(err)
			return i, err
		}
	}
	t.log.Printf("%d airings unscheduled\n", len(airings)-skipped)
	return len(airings) - skipped, nil
}

func getExportFilename(airing tablodb.ScheduledAiringRecord, path string) string {
	sep := string(os.PathSeparator)
	switch airing.ShowType {
	case "series":
		showTitle := utils.SanitizeFileString(airing.ShowTitle)
		var season string
		if len(airing.Season) == 1 {
			season = "0" + utils.SanitizeFileString(airing.Season)
		} else if airing.Season == "" {
			season = "00"
		} else {
			season = utils.SanitizeFileString(airing.Season)
		}
		episode := fmt.Sprintf("%02d", airing.Episode)
		if episode == "00" {
			airDate := time.Unix(int64(airing.AirDate), 0)
			episode = airDate.Format("200601021504")
		}
		episodeTitle := utils.SanitizeFileString(airing.EpisodeTitle)
		return path + sep + "TV" + sep + showTitle + sep + "Season " + season + sep + showTitle + " - s" + season + "e" + episode + " - " + episodeTitle + ".mp4"
	case "movies":
		showTitle := utils.SanitizeFileString(airing.ShowTitle)
		year := strconv.Itoa(airing.ReleaseYear)
		return path + sep + "Movies" + sep + showTitle + " - " + year + ".mp4"
	case "sports":
		showTitle := utils.SanitizeFileString(airing.ShowTitle)
		var season string
		if airing.Season == "" {
			season = "00"
		} else {
			season = utils.SanitizeFileString(airing.Season)
		}
		episodeTitle := utils.SanitizeFileString(airing.EpisodeTitle)
		return path + sep + "Sports" + sep + showTitle + sep + showTitle + " - " + season + " - " + episodeTitle + ".mp4"
	default:
		return ""
	}
}

func checkExported(toCheck []string, path string) ([]string, []string, error) {
	var exportedMissing []string
	for _, f := range toCheck {
		switch _, err := os.Stat(f); {
		case errors.Is(err, os.ErrNotExist):
			exportedMissing = append(exportedMissing, f)
		case err != nil:
			return nil, nil, err
		}
	}

	var exportedFound []string
	sep := string(os.PathSeparator)
	pathQueue := []string{path + sep + "Movies", path + sep + "Sports", path + sep + "TV"}
	for len(pathQueue) > 0 {
		curPath := pathQueue[0]
		files, err := os.ReadDir(curPath)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range files {
			if f.IsDir() {
				pathQueue = append(pathQueue, curPath+sep+f.Name())
			} else {
				exportedFound = append(exportedFound, curPath+sep+f.Name())
			}
		}
		if len(pathQueue) > 1 {
			pathQueue = pathQueue[1:]
		} else {
			pathQueue = make([]string, 0)
		}
	}

	return exportedMissing, exportedFound, nil
}

func batch(uri string, input []string) ([]byte, error) {
	var output []byte
	output = append(output, byte('{'))

	for i := 0; i < len(input); i += 50 {
		j := i + 50
		if j > len(input) {
			j = len(input)
		}
		data := "[\"" + strings.Join(input[i:j], "\",\"") + "\"]"
		tempOutput, err := post(uri+"/batch", data)
		if err != nil {
			return nil, err
		}
		output = append(output, tempOutput[1:len(tempOutput)-1]...)
		if j < len(input) {
			output = append(output, byte(','))
		} else {
			output = append(output, byte('}'))
		}
	}

	return output, nil
}

func patch(uri string, data string) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest(http.MethodPatch, uri, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Printf("Error connecting to %s. Waiting 30 seconds to retry\n", uri)
		time.Sleep(30 * time.Second)
		resp, err = client.Do(req)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			fmt.Printf("http.PATCH error: %v\n", err)
			return nil, err
		}
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func post(uri string, data string) ([]byte, error) {
	resp, err := http.Post(uri, "application/json", bytes.NewBuffer([]byte(data)))
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Printf("Error connecting to %s. Waiting 30 seconds to retry\n", uri)
		time.Sleep(30 * time.Second)
		resp, err = http.Post(uri, "application/json", bytes.NewBuffer([]byte(data)))
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			fmt.Printf("http.POST error: %v\n", err)
			return nil, err
		}
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
		if resp != nil {
			resp.Body.Close()
		}
		fmt.Printf("Error connecting to %s. Waiting 30 seconds to retry\n", uri)
		time.Sleep(30 * time.Second)
		resp, err = http.Get(uri)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			fmt.Printf("http.GET error: %v\n", err)
			return nil, err
		}
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
