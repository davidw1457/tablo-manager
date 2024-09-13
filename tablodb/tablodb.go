package tablodb

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

type TabloDB struct {
	database *sql.DB
}

func New(ipAddress string, name string, serverID string, directory string) (TabloDB, error) {
	var tabloDB TabloDB
	databaseFile := directory + string(os.PathSeparator) + serverID + ".cache"
	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		return tabloDB, fmt.Errorf("unable to create database %s: %s", databaseFile, err)
	}

	tabloDB.database = db

	err = tabloDB.initialSetup()
	if err != nil {
		return tabloDB, fmt.Errorf("unable to initialize database: %s", err)
	}

	return tabloDB, nil
}

func (db *TabloDB) Close() {
	defer db.database.Close()
}

func (db *TabloDB) initialSetup() error {

	return nil
}
