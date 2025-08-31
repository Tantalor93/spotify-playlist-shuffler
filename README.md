# Simple spotify playlist shuffling web app

You need to create a spotify app to get client id and client secret: https://developer.spotify.com/dashboard/applications

playlist shuffler is configured using env variables:
* `SPOTIFY_SHUFFLER_CLIENT_ID` (**required**) - your spotify app client id
* `SPOTIFY_SHUFFLER_CLIENT_SECRET` (**required**) - your spotify app client secret
* `SPOTIFY_SHUFFLER_REDIRECT_URI` (default `http://127.0.0.1:8080/callback`) - your spotify app redirect uri
* `PORT` (default `8080`) - port to run the web app on
