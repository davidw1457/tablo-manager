package tablo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"tablo-manager/tablodb"
	"tablo-manager/utils"
)

const tabloWebUri = "https://api.tablotv.com/assocserver/getipinfo/"
const userRWX = 0700 // unix-style octal permission

type Tablo struct {
	ipAddress string
	serverID  string
	name      string
	database  tablodb.TabloDB
}

type webUriResp struct {
	Cpes []webUriRespTablo
}

type webUriRespTablo struct {
	Serverid   string
	Name       string
	Private_ip string
}

func New(databaseDir string) (*Tablo, error) {
	tablo := new(Tablo)

	_, err := os.Stat(databaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(databaseDir, userRWX)
			if err != nil {
				return tablo, fmt.Errorf("unable to create %s: %s", databaseDir, err)
			}
		} else {
			return tablo, fmt.Errorf("unable to access %s %s", databaseDir, err)
		}
	}

	files, err := os.ReadDir(databaseDir)
	if err != nil {
		return tablo, fmt.Errorf("unable to access %s %s", databaseDir, err)
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
		return tablo, fmt.Errorf("unable to connect to tablo web api %s", err)
	}

	var tabloInfo webUriResp

	err = json.Unmarshal(tabloWebResponse, &tabloInfo)
	if err != nil {
		return tablo, fmt.Errorf("error unmarshalling response %s", err)
	}

	// TODO: Figure out what to do if there is more than one Tablo
	// May need to update to return slice of Tablos
	tablo.ipAddress = tabloInfo.Cpes[0].Private_ip
	tablo.name = tabloInfo.Cpes[0].Name
	tablo.serverID = tabloInfo.Cpes[0].Serverid
	tablo.database, err = tablodb.New(tablo.ipAddress, tablo.name, tablo.serverID, databaseDir)
	if err != nil {
		return tablo, err
	}

	return tablo, nil
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

func (t *Tablo) ToString() string {
	return fmt.Sprintf("Name: %s, ID: %s, IP: %s", t.name, t.serverID, t.ipAddress)
}

func (t *Tablo) Close() {
	defer t.database.Close()
}
