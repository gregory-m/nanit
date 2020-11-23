package main

import "net/http"

func serve(dataDir string) {
	staticDir := "static"

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.Handle("/stream/", http.StripPrefix("/stream/", http.FileServer(http.Dir(dataDir))))
	http.ListenAndServe(":8080", nil)
}
