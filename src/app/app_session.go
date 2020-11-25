package main

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
)

const REVISION = 1

type AppSession struct {
	Revision  int    `json:"revision"`
	AuthToken string `json:"authToken"`
	Babies    []Baby `json:"babies"`
}

type AppSessionStore struct {
	Filename string
	Session  *AppSession
}

func (store *AppSessionStore) Load() {
	if _, err := os.Stat(store.Filename); os.IsNotExist(err) {
		log.Info().Str("filename", store.Filename).Msg("No app session file found")
		store.Session = &AppSession{Revision: REVISION}
		return
	}

	f, err := os.Open(store.Filename)
	if err != nil {
		log.Fatal().Str("filename", store.Filename).Err(err).Msg("Unable to open app session file")
	}

	defer f.Close()

	session := &AppSession{}
	jsonErr := json.NewDecoder(f).Decode(session)
	if jsonErr != nil {
		log.Fatal().Str("filename", store.Filename).Err(jsonErr).Msg("Unable to decode app session file")
	}

	if session.Revision == REVISION {
		store.Session = session
		log.Info().Str("filename", store.Filename).Msg("Loaded app session from the file")
	} else {
		store.Session = &AppSession{Revision: REVISION}
		log.Warn().Str("filename", store.Filename).Msg("App session file contains older revision of the state, ignoring")
	}

}

func (store *AppSessionStore) Save() {
	if store.Filename == "" {
		return
	}

	log.Trace().Str("filename", store.Filename).Msg("Storing app session to the file")

	f, err := os.OpenFile(store.Filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal().Str("filename", store.Filename).Err(err).Msg("Unable to open app session file for writing")
	}

	defer f.Close()

	data, jsonErr := json.Marshal(store.Session)
	if jsonErr != nil {
		log.Fatal().Str("filename", store.Filename).Err(jsonErr).Msg("Unable to marshal contents of app session file")
	}

	_, writeErr := f.Write(data)
	if writeErr != nil {
		log.Fatal().Str("filename", store.Filename).Err(writeErr).Msg("Unable to wrote to app session file")
	}
}
