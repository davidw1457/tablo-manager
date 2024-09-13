package main

import (
	"fmt"
	"os"
	"tablo-manager/tablo"
)

func main() {
	databaseDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	databaseDir += string(os.PathSeparator) + ".tablomanager"

	tablo, err := tablo.New(databaseDir)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(tablo)
}
