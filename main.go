package main

import (
	"fmt"
	"os"
	"tablo-manager/tablo"
	"time"
)

func main() {
	databaseDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	databaseDir += string(os.PathSeparator) + ".tablomanager"

	err = os.RemoveAll(databaseDir) // TODO: Remove this line once database creation works

	tablos, err := tablo.New(databaseDir)
	if err != nil {
		fmt.Println(err)
	}
	for len(tablos) > 0 {
		for _, t := range tablos {
			if t.NeedUpdate() {
				t.EnqueueUpdate()
			}

			t.LoadQueue()

			if t.HasQueueItems() {
				t.ProcessQueue()
			}
		}
		time.Sleep(5 * time.Minute)
	}
}
