# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache dependencies separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /mapwatch ./cmd/mapwatch

# Download SG NPC boundary GeoJSON at build time (baked into the image).
RUN /mapwatch download-sg --out /data

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM scratch

# Copy TLS certs (needed for HTTPS outbound calls to Prometheus/CDN)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary and pre-downloaded GeoJSON data
COPY --from=builder /mapwatch /mapwatch
COPY --from=builder /data /data

EXPOSE 8080

ENTRYPOINT ["/mapwatch"]
CMD ["serve", "--data-dir", "/data"]
