package main

import (
	"encoding/json"
	"github.com/zmb3/spotify/v2"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// listPlaylists lists the user's playlists.
func listPlaylists(w http.ResponseWriter, r *http.Request) {
	client, err := getClientFromCookieAuth(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	currentUser, err := client.CurrentUser(r.Context())

	playlists, err := client.CurrentUsersPlaylists(r.Context())
	if err != nil {
		log.Fatal(err)
	}

	var playlistsViews []PlayListView
	for _, playlist := range playlists.Playlists {
		if playlist.Owner.ID == currentUser.ID {
			playlistsViews = append(playlistsViews,
				PlayListView{Name: playlist.Name,
					TracksTotal:       int(playlist.Tracks.Total),
					TracksShuffleLink: "/shuffle?id=" + playlist.ID.String(),
				},
			)

		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playlistsViews)
}

// shufflePlaylist shuffles the tracks in the specified playlist.
func shufflePlaylist(w http.ResponseWriter, r *http.Request) {
	client, err := getClientFromCookieAuth(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	id := r.URL.Query().Get("id")

	var allTracks []spotify.PlaylistItem
	itemsPage, err := client.GetPlaylistItems(r.Context(), spotify.ID(id))
	if err != nil {
		log.Println("Failed to get playlist tracks:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for itemsPage != nil {
		for _, item := range itemsPage.Items {
			allTracks = append(allTracks, item)
		}
		err = client.NextPage(r.Context(), itemsPage)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			log.Println("Failed to get next page of playlist items:", err)
			http.Error(w, "Failed to get next page of playlist items.", http.StatusInternalServerError)
			return
		}
	}

	log.Println("Original tracks:", getTrackIds(allTracks))

	shuffledTracks := shuffleTracks(allTracks)
	trackIDs := getTrackIds(shuffledTracks)

	log.Println("Shuffled tracks:", trackIDs)

	// Delete all existing tracks in the playlist
	err = client.ReplacePlaylistTracks(r.Context(), spotify.ID(id))
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
		_, err = client.AddTracksToPlaylist(r.Context(), spotify.ID(id), trackIDs[i:end]...)
		if err != nil {
			log.Println("Failed to add tracks to playlist:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	return
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
