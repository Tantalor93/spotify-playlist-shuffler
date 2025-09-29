package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

const spotifyTokenCookieName = "spotify_token"

var authenticator *spotifyauth.Authenticator

// initSpotifyAuthenticator initializes spotify authenticator with client ID, secret,
// redirect URI based on env variables.
func initSpotifyAuthenticator() {
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

	authenticator = spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithClientID(clientId),
		spotifyauth.WithClientSecret(clientSecret),
		spotifyauth.WithScopes(
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistModifyPrivate,
			spotifyauth.ScopePlaylistModifyPublic,
		),
	)
}

// completes authentication redirected from spotify
func completeAuth(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("spotify_auth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "State mismatch or cookie not found", http.StatusForbidden)
		return
	}

	tok, err := authenticator.Token(r.Context(), stateCookie.Value, r)
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
		Name:    spotifyTokenCookieName,
		Value:   encodedToken,
		Expires: tok.Expiry,
		Path:    "/",
		// Secure: true, // For production use, uncomment this line
		HttpOnly: true, // Prevents JavaScript from accessing the cookie
	})

	log.Println("Token obtained, redirecting to list of playlists")

	http.Redirect(w, r, "/", http.StatusSeeOther)
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

func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    spotifyTokenCookieName,
		Value:   "",
		Expires: time.Unix(0, 0),
		Path:    "/",
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// getClientFromCookieAuth retrieves the Spotify client from the user's cookie.
func getClientFromCookieAuth(r *http.Request) (*spotify.Client, error) {
	cookie, err := r.Cookie(spotifyTokenCookieName)
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
	return spotify.New(authenticator.Client(r.Context(), &tok)), nil
}
