# ── Build stage ──────────────────────────────────────────────────────────────
#
# Interactive-demo — includes sound, leaderboard, stats modules.
# Vendor assets and WAV files are generated and embedded at build time.
#
# Build:  docker build -f examples/interactive-demo/Dockerfile -t mapwatch-demo .
# Run:    docker run -p 8080:8080 mapwatch-demo
#
FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Download Leaflet vendor assets (offline map tiles CSS/JS).
RUN apk add --no-cache curl && \
    sh scripts/download-vendor.sh

# Generate WAV notification sounds (pure Python stdlib).
RUN apk add --no-cache python3 && \
    python3 scripts/generate-sounds.py

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /mapwatch ./cmd/mapwatch

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /mapwatch /mapwatch

EXPOSE 8080

ENTRYPOINT ["/mapwatch"]
CMD ["serve"]
