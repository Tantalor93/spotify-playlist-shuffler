FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . .

RUN go build -o /app/spotify-playlist-shuffler .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/spotify-playlist-shuffler .

COPY web .

CMD ["./spotify-playlist-shuffler"]
