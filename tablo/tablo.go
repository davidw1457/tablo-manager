package tablo

import (
	"fmt"
	"os"
	"tablo-manager/utils"
)

const tabloWebUri = "https://api.tablotv.com/assocserver/getipinfo/"
const userRWX = 0700 // unix-style octal permission

type Tablo struct {
	ipAddress string
	serverID  string
	// database  tablodb.TabloDB
}

func New(databaseDir string) (Tablo, error) {
	tablo := *new(Tablo)

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

	return *new(Tablo), nil // TODO: Placeholder return
}

func get(uri string) (string, error) {

}
