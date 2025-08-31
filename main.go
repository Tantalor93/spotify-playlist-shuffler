package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/zmb3/spotify/v2"
	"github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var authenticator *spotifyauth.Authenticator

type PlayListView struct {
	Name              string `json:"name"`
	TracksTotal       int    `json:"tracksTotal"`
	TracksShuffleLink string `json:"shuffleTracksLink"`
}

func main() {
	clientId := os.Getenv("SPOTIFY_SHUFFLER_CLIENT_ID")
	if clientId == "" {
		log.Fatal("Missing SPOTIFY_SHUFFLER_CLIENT_ID environment variable")
	}
	clientSecret := os.Getenv("SPOTIFY_SHUFFLER_CLIENT_SECRET")
	if clientSecret == "" {
		log.Fatal("Missing SPOTIFY_SHUFFLER_CLIENT_SECRET environment variable")
	}
	redirectURI := os.Getenv("SPOTIFY_SHUFFLER_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://127.0.0.1:8080/callback"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	authenticator = spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithClientID(clientId),
		spotifyauth.WithClientSecret(clientSecret),
		spotifyauth.WithScopes(
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistModifyPrivate,
			spotifyauth.ScopePlaylistModifyPublic),
	)

	http.HandleFunc("/", serveHTML)

	http.HandleFunc("/list", listPlaylists)

	http.HandleFunc("/callback", completeAuth)

	http.HandleFunc("/shuffle", shufflePlaylist)

	http.HandleFunc("/auth", redirectToSpotify)

	http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}

func listPlaylists(w http.ResponseWriter, r *http.Request) {
	client, err := getClient(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	currentUser, err := client.CurrentUser(context.Background())

	playlists, err := client.CurrentUsersPlaylists(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var playlistsViews []PlayListView
	for _, playlist := range playlists.Playlists {
		if playlist.Owner.ID == currentUser.ID {
			playlistsViews = append(playlistsViews,
				PlayListView{Name: playlist.Name,
					TracksTotal:       int(playlist.Tracks.Total),
					TracksShuffleLink: "http://127.0.0.1:8080/shuffle?id=" + playlist.ID.String(),
				},
			)

		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playlistsViews)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("spotify_auth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "State mismatch or cookie not found", http.StatusForbidden)
		return
	}

	tok, err := authenticator.Token(context.Background(), stateCookie.Value, r)
	if err != nil {
		http.Error(w, "Failed to authenticate", http.StatusForbidden)
		log.Fatal(err)
	}

	tokenJSON, err := json.Marshal(tok)
	if err != nil {
		http.Error(w, "Failed to serialize token", http.StatusInternalServerError)
		return
	}

	encodedToken := base64.StdEncoding.EncodeToString(tokenJSON)

	// Set a cookie with the token data
	http.SetCookie(w, &http.Cookie{
		Name:    "spotify_token",
		Value:   encodedToken,
		Expires: tok.Expiry,
		Path:    "/",
		// Secure: true, // For production use, uncomment this line
		HttpOnly: true, // Prevents JavaScript from accessing the cookie
	})

	log.Println("Token obtained, redirecting to list of playlists")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// getClient retrieves the Spotify client from the user's cookie.
func getClient(r *http.Request) (*spotify.Client, error) {
	cookie, err := r.Cookie("spotify_token")
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil, err
	}

	var tok oauth2.Token
	err = json.Unmarshal(decoded, &tok)
	if err != nil {
		return nil, err
	}

	// Create a new client from the token, handling token refreshes automatically.
	return spotify.New(authenticator.Client(context.Background(), &tok)), nil
}

func shufflePlaylist(w http.ResponseWriter, r *http.Request) {
	client, err := getClient(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	id := r.URL.Query().Get("id")

	var allTracks []spotify.PlaylistItem
	itemsPage, err := client.GetPlaylistItems(context.Background(), spotify.ID(id))
	if err != nil {
		log.Println("Failed to get playlist tracks:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for itemsPage != nil {
		for _, item := range itemsPage.Items {
			allTracks = append(allTracks, item)
		}
		err = client.NextPage(context.Background(), itemsPage)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			log.Println("Failed to get next page of playlist items:", err)
			http.Error(w, "Failed to get next page of playlist items.", http.StatusInternalServerError)
			return
		}
	}
	log.Println("Got tracks:", len(allTracks))

	log.Println("Original tracks:", getTrackIds(allTracks))

	shuffledTracks := shuffleTracks(allTracks)
	trackIDs := getTrackIds(shuffledTracks)

	log.Println("Shuffled tracks:", trackIDs)

	// Delete all existing tracks in the playlist
	err = client.ReplacePlaylistTracks(context.Background(), spotify.ID(id))
	if err != nil {
		log.Println("Failed to replace playlist tracks:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Add tracks back in shuffled order, in batches of 100 (Spotify API limit)
	for i := 0; i < len(trackIDs); i += 100 {
		end := i + 100
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		_, err = client.AddTracksToPlaylist(context.Background(), spotify.ID(id), trackIDs[i:end]...)
		if err != nil {
			log.Println("Failed to add tracks to playlist:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	return
}

// redirectToSpotify redirects the browser to the Spotify authorization page.
func redirectToSpotify(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	http.SetCookie(w, &http.Cookie{
		Name:  "spotify_auth_state",
		Value: state,
		Path:  "/",
	})
	url := authenticator.AuthURL(state)
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "index.html")
}

func getTrackIds(tracks []spotify.PlaylistItem) []spotify.ID {
	var trackIDs []spotify.ID
	for _, trackItem := range tracks {
		if trackItem.Track.Track != nil {
			trackIDs = append(trackIDs, trackItem.Track.Track.ID)
		}
	}
	return trackIDs
}

// shuffleTracks shuffles the order of a slice of PlaylistTrack.
func shuffleTracks(tracks []spotify.PlaylistItem) []spotify.PlaylistItem {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(tracks), func(i, j int) {
		tracks[i], tracks[j] = tracks[j], tracks[i]
	})
	return tracks
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
