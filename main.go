package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"tablo-manager/tablo"
)

// TODO: Create export function
// TODO: Figure out best way to purge outdated data from system
//       (e.g. remove showIDs that do not contain airings or recordings & are not scheduled)
// TODO: Populate showFilter table
// TODO: Populate priority table

const userRWX = 0700 // unix-style octal permission

func main() {

	var databaseDir string
	if len(os.Args) > 1 {
		databaseDir = os.Args[1]
	} else {
		var err error
		databaseDir, err = os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		databaseDir += string(os.PathSeparator) + ".tablomanager"
	}

	_, err := os.Stat(databaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(databaseDir, userRWX)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	logFile, err := os.OpenFile(databaseDir+string(os.PathSeparator)+"main.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, userRWX)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	mainLog := log.New(io.MultiWriter(logFile, os.Stdout), "main: ", log.LstdFlags)

	mainLog.Println("beginning tablo creation")
	tablos, err := tablo.New(databaseDir)
	if err != nil {
		mainLog.Fatal(err)
	}
	for _, t := range tablos {
		defer t.Close()
	}

	mainLog.Printf("%d tablos found. beginning process loop.\n", len(tablos))
	for len(tablos) > 0 {
		for _, t := range tablos {
			mainLog.Println(t.String())
			mainLog.Println("checking whether to update database")
			if t.NeedUpdate() {
				mainLog.Println("adding update tasks to work queue")
				err = t.EnqueueUpdate()
				if err != nil {
					mainLog.Println(err)
					continue
				}
			}

			mainLog.Println("loading queue records")
			err = t.LoadQueue()
			if err != nil {
				mainLog.Println(err)
				continue
			}

			mainLog.Println("checking whether there are queue records")
			if t.HasQueueItems() {
				mainLog.Println("processing queue records")
				err := t.ProcessQueue()
				if err != nil {
					mainLog.Println(err)
				}
			}
		}
		mainLog.Println("completed process loop. pausing for 15 minutes")
		time.Sleep(15 * time.Minute)
	}
}
