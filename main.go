package main

import (
	"fmt"
	"os"
)

func main() {
	databaseDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	databaseDir += string(os.PathSeparator) + ".tablomanager"

	// tablo := tablo.New(databaseDir)
}
