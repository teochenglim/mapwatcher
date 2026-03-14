#!/usr/bin/env bash
# Download Leaflet vendor assets for offline / Docker use.
# Run once before `go build` or `make build` to bundle Leaflet into the binary.
#
# Usage:
#   ./scripts/download-vendor.sh
#   make download-vendor
#
# Files written to static/vendor/ (git-ignored by default; committed in Docker build).

set -euo pipefail

LEAFLET_VER="1.9.4"
MC_VER="1.5.3"
OUT="$(dirname "$0")/../static/vendor"

mkdir -p "$OUT"

echo "→ Downloading Leaflet ${LEAFLET_VER} …"
curl -fsSL "https://unpkg.com/leaflet@${LEAFLET_VER}/dist/leaflet.css"    -o "$OUT/leaflet.css"
curl -fsSL "https://unpkg.com/leaflet@${LEAFLET_VER}/dist/leaflet.js"     -o "$OUT/leaflet.js"

echo "→ Downloading Leaflet.markercluster ${MC_VER} …"
curl -fsSL "https://unpkg.com/leaflet.markercluster@${MC_VER}/dist/MarkerCluster.css"         -o "$OUT/MarkerCluster.css"
curl -fsSL "https://unpkg.com/leaflet.markercluster@${MC_VER}/dist/MarkerCluster.Default.css" -o "$OUT/MarkerCluster.Default.css"
curl -fsSL "https://unpkg.com/leaflet.markercluster@${MC_VER}/dist/leaflet.markercluster.js"  -o "$OUT/leaflet.markercluster.js"

echo "✓ Vendor assets written to $OUT"
ls -lh "$OUT"
