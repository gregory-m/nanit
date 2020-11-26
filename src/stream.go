package main

import (
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
	InterruptC      chan bool
	DoneC           chan bool
	Session         *AppSession
	DataDirectories DataDirectories
}

func NewStreamProcess(cmdTemplate string, babyUID string, session *AppSession, dataDirs DataDirectories) *StreamProcess {
	// Check babyUID does not contain and bad characters (we use it as part of the file paths)
	ensureValidBabyUID(babyUID)

	sp := &StreamProcess{
		CommandTemplate: cmdTemplate,
		BabyUID:         babyUID,
		Session:         session,
		DataDirectories: dataDirs,
		InterruptC:      make(chan bool, 1),
		DoneC:           make(chan bool, 1),
	}

	go execStreamProcess(sp)

	return sp
}

func (sp *StreamProcess) Stop() {
	sp.InterruptC <- true
	<-sp.DoneC
}

func execStreamProcess(sp *StreamProcess) {
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

	defer func() {
		sp.DoneC <- true
	}()

	for {
		select {
		case err := <-done:
			if err != nil {
				log.Error().Err(err).Msg("Stream processor exited")
			} else {
				log.Warn().Msg("Stream processor exited with status 0")
			}

			return

		case <-sp.InterruptC:
			log.Info().Msg("Terminating stream processor")
			if err := cmd.Process.Kill(); err != nil {
				log.Error().Err(err).Msg("Unable to kill process")
			}

			return
		}
	}
}
