package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func handleLog(w http.ResponseWriter, r *http.Request) {
	log.Println("Saving log to file")
	defer r.Body.Close()

	out, err := os.Create("logs.tar.gz")
	defer out.Close()

	_, err = io.Copy(out, r.Body)

	if err != nil {
		log.Println(fmt.Errorf("Unable to save received log file: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func serve(dataDir string) {
	staticDir := "static"

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.Handle("/stream/", http.StripPrefix("/stream/", http.FileServer(http.Dir(dataDir))))
	http.HandleFunc("/log", handleLog)
	http.ListenAndServe(":8080", nil)
}
