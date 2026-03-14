# ── Build stage ──────────────────────────────────────────────────────────────
#
# Blink-dot example — no GeoJSON map layer downloads.
# SG overlay buttons (Divisions, Roads, MRT, etc.) are hidden automatically
# when the files are absent. All other features work normally.
#
# Build:  docker build -f examples/blink-dot/Dockerfile -t mapwatch-blink .
# Run:    docker run -p 8080:8080 mapwatch-blink
#
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache dependencies separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Download Leaflet vendor assets for offline use (embedded via go:embed).
RUN apk add --no-cache curl && \
    sh scripts/download-vendor.sh

# Generate WAV sound files (pure Python stdlib, no pip deps).
RUN apk add --no-cache python3 && \
    python3 scripts/generate-sounds.py

# Build binary (vendor + sounds are now inside static/ and will be embedded).
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /mapwatch ./cmd/mapwatch

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

# Copy TLS certs (needed for HTTPS outbound calls to Prometheus/CDN)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary only — no GeoJSON data bundled
COPY --from=builder /mapwatch /mapwatch

EXPOSE 8080

ENTRYPOINT ["/mapwatch"]
CMD ["serve"]
