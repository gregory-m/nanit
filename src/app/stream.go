package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func startStream(url string, dataDir string) {
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
		fmt.Sprintf("%v/stream.m3u8", dataDir),
	}

	log.Println(fmt.Sprintf("Starting FFMPEG with args: %v", strings.Join(args, " ")))

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Start()
	if err != nil {
		log.Fatal(fmt.Errorf("Run failed: %v", err))
	}
}
