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
	BabyUID         string
	InterruptC      chan bool
	DoneC           chan bool
	Session         *AppSession
	DataDirectories DataDirectories
}

func NewStreamProcess(babyUID string, session *AppSession, dataDirs DataDirectories) *StreamProcess {
	// Check babyUID does not contain and bad characters (we use it as part of the file paths)
	ensureValidBabyUID(babyUID)

	sp := &StreamProcess{
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
	outFilename := filepath.Join(sp.DataDirectories.VideoDir, fmt.Sprintf("%v.m3u8", sp.BabyUID))
	logFilename := filepath.Join(sp.DataDirectories.LogDir, fmt.Sprintf("ffmpeg-%v-%v.log", sp.BabyUID, time.Now().Format(time.RFC3339)))
	url := fmt.Sprintf("rtmps://media-secured.nanit.com/nanit/%v.%v", sp.BabyUID, sp.Session.AuthToken)

	args := []string{
		"-i",
		url,
		"-vcodec",
		"copy",
		"-acodec",
		"copy",
		"-hls_init_time",
		"0",
		"-hls_time",
		"1",
		"-hls_list_size",
		"6",
		"-hls_wrap",
		"10",
		"-start_number",
		"1",
		outFilename,
	}

	logFile, fileErr := os.Create(logFilename)
	if fileErr != nil {
		log.Fatal().Str("filename", logFilename).Err(fileErr).Msg("Unable to create log file")
	}

	defer logFile.Close()

	log.Info().Str("args", strings.Join(args, " ")).Str("logfile", logFilename).Msg("Starting FFMPEG")

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = logFile
	cmd.Stdout = logFile

	err := cmd.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to start FFMPEG")
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
				log.Error().Err(err).Msg("FFMPEG exited")
			}

			return

		case <-sp.InterruptC:
			log.Info().Msg("Terminating FFMPEG")
			if err := cmd.Process.Kill(); err != nil {
				log.Error().Err(err).Msg("Unable to kill process")
			}

			return
		}
	}
}
