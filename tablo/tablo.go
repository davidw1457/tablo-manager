package tablo

import "os"

type Tablo struct {
	ipAddress string
	serverID  string
	database  tablodb.TabloDB
}

func New(databaseDir string) Tablo {
	_, err := os.Stat(databaseDir)
	if err != nil {
		if os.IsNotExist(err) {

		} else {
			// handle the error?
		}
	}
}
