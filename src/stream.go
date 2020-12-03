package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type StreamProcess struct {
	CommandTemplate string
	BabyUID         string
	Attempter       *Attempter
	API             *NanitClient
	Session         *AppSession
	DataDirectories DataDirectories
}

func NewStreamProcess(cmdTemplate string, babyUID string, session *AppSession, api *NanitClient, dataDirs DataDirectories) *StreamProcess {
	// Check babyUID does not contain and bad characters (we use it as part of the file paths)
	ensureValidBabyUID(babyUID)

	sp := &StreamProcess{
		CommandTemplate: cmdTemplate,
		BabyUID:         babyUID,
		Session:         session,
		DataDirectories: dataDirs,
		API:             api,
	}

	return sp
}

func (sp *StreamProcess) Start() {
	sp.Attempter = NewAttempter(
		func(attempt *Attempt) error {
			return execStreamProcess(sp, attempt)
		},
		[]time.Duration{
			2 * time.Second,
			30 * time.Second,
			2 * time.Minute,
			15 * time.Minute,
			1 * time.Hour,
		},
		2*time.Second,
	)

	go sp.Attempter.Run()
}

func (sp *StreamProcess) Stop() {
	sp.Attempter.Stop()
}

func execStreamProcess(sp *StreamProcess, attempt *Attempt) error {
	// Reauthorize if it is not a first try or if the session is older then 10 minutes
	if attempt.Number > 1 || time.Since(sp.Session.AuthTime) > 10*time.Minute {
		sp.API.Authorize()
	}

	logFilename := filepath.Join(sp.DataDirectories.LogDir, fmt.Sprintf("process-%v-%v.log", sp.BabyUID, time.Now().Format(time.RFC3339)))
	url := fmt.Sprintf("rtmps://media-secured.nanit.com/nanit/%v.%v", sp.BabyUID, sp.Session.AuthToken)

	r := strings.NewReplacer("{sourceUrl}", url, "{babyUid}", sp.BabyUID)
	cmdTokens := strings.Split(r.Replace(sp.CommandTemplate), " ")

	logFile, fileErr := os.Create(logFilename)
	if fileErr != nil {
		log.Fatal().Str("filename", logFilename).Err(fileErr).Msg("Unable to create log file")
	}

	defer logFile.Close()

	log.Info().Str("cmd", strings.Join(cmdTokens, " ")).Str("logfile", logFilename).Msg("Starting stream processor")

	cmd := exec.Command(cmdTokens[0], cmdTokens[1:]...)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	cmd.Dir = sp.DataDirectories.VideoDir

	err := cmd.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to start stream processor")
	}

	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	for {
		select {
		case err := <-done:
			if err != nil {
				log.Error().Err(err).Msg("Stream processor exited")
				return err
			}

			log.Warn().Msg("Stream processor exited with status 0")
			return errors.New("Stream processor exited with status 0")

		case <-attempt.InterruptC:
			log.Info().Msg("Terminating stream processor")
			if err := cmd.Process.Kill(); err != nil {
				log.Error().Err(err).Msg("Unable to kill process")
			}

			return nil
		}
	}
}
