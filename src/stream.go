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

func startStream(babyUID string, authToken string, dataDirs DataDirectories) func() {
	ensureValidBabyUID(babyUID)

	outFilename := filepath.Join(dataDirs.VideoDir, fmt.Sprintf("%v.m3u8", babyUID))
	logFilename := filepath.Join(dataDirs.LogDir, fmt.Sprintf("ffmpeg-%v-%v.log", babyUID, time.Now().Format(time.RFC3339)))
	url := fmt.Sprintf("rtmps://media-secured.nanit.com/nanit/%v.%v", babyUID, authToken)

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

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Error().Err(err).Msg("FFMPEG exited")
		}
	}()

	return func() {
		if !cmd.ProcessState.Exited() {
			log.Info().Msg("Terminating FFMPEG")
			if err := cmd.Process.Kill(); err != nil {
				log.Error().Err(err).Msg("Unable to kill process")
			}
		}
	}
}
