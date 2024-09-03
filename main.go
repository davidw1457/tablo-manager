package main

import (
	"fmt"
	"os"
)

func main() {
	databaseDir, err = databaseDirectory()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func databaseDirectory() (string, error) {
	directoryRoot, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	directoryRoot += string(os.PathSeparator) + ".tablomanager"
	return directoryRoot, nil
}
