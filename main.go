package main

import (
	"fmt"
	"net/http"
	"os"
)

type PlayListView struct {
	Name              string `json:"name"`
	TracksTotal       int    `json:"tracksTotal"`
	TracksShuffleLink string `json:"shuffleTracksLink"`
}

func main() {
	initSpotifyAuthenticator()

	// register HTTP handlers
	http.HandleFunc("/", serveHTML)
	http.HandleFunc("/list", listPlaylists)
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/shuffle", shufflePlaylist)
	http.HandleFunc("/auth", redirectToSpotify)
	http.HandleFunc("/logout", logout)

	startServer()
}

func startServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}
