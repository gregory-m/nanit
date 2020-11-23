package main

import (
	"os"
)

func main() {
	dataDir := "data"

	api := NanitClient{
		Email:    os.Getenv("EMAIL"),
		Password: os.Getenv("PASSWORD"),
	}

	startStream(api.GetStreamURL(), dataDir)
	serve(dataDir)
}
