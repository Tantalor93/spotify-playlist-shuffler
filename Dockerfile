FROM golang:1.24-alpine AS builder

# Nastavte pracovní adresář
WORKDIR /app

# Zkopírujte zdrojový kód
COPY . .

RUN go build -o /app/spotify-playlist-shuffler .

# Použijte menší obraz pro finální kontejner
FROM alpine:latest

# Nastavte pracovní adresář
WORKDIR /app

# Zkopírujte zkompilovanou binárku z předchozí fáze
COPY --from=builder /app/spotify-playlist-shuffler .

# Spusťte aplikaci
CMD ["./spotify-playlist-shuffler"]
