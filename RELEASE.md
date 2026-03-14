# MapWatch Release Notes

## v0.6.0

**Interactive demo — mobile tap, emoji event markers, leaderboard, offline-capable Docker image**

### Interactive demo (`examples/interactive-demo/`)

New end-to-end demo stack: participants tap a map on their phone, events flow through
**Redpanda → RisingWave → leaderboard exporter → MapWatch** in real time.

Services: `mapwatch`, `redpanda`, `risingwave`, `exporter` (FastAPI leaderboard), `prometheus`, `alertmanager`.

```bash
cd examples/interactive-demo && docker compose up --build
# Main map:    http://localhost:8080
# Mobile tap:  http://localhost:8080/mobile.html  (open on phone)
```

### New feature — mobile tap page (`/mobile.html`)

Colour-coded tap input for mobile browsers. Participants pick a severity colour, enter
their name, and tap the Singapore map to place real-time event markers visible to all
connected clients.

### New API endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/tap` | Receives `{username,color,severity,lat,lng,session_id,timestamp}`; creates a marker and optionally publishes to Kafka (`REDPANDA_BROKERS` env var) |
| `GET`  | `/api/leaderboard` | Proxies to upstream `leaderboard_url`; returns 503 when not configured |
| `POST` | `/api/leaderboard/clear` | Proxies clear request to upstream leaderboard |

### New feature — `modules:` config section

Optional frontend modules, all off by default. Enable per-example in their `mapwatch.yaml`:

```yaml
modules:
  sound: true        # Web Audio API tones on marker.add
  leaderboard: true  # live leaderboard sidebar (requires leaderboard_url)
  stats: true        # tap / alert counter overlay
```

`GET /api/config` now returns `{"modules":{…},"leaderboardUrl":"…"}`.
Modules are loaded dynamically via `<script>` injection so the base binary stays lean.

### New feature — emoji event markers (`blink.js` rewrite)

Plain coloured dots replaced with animated emoji incident markers:

| Severity | Emoji | Incident type | Animation |
|----------|-------|---------------|-----------|
| critical | 🚗 | Car Accident | spinning dashed ring + shake |
| high / warning | 🔥 | Building Fire | rapid opacity flicker |
| medium | 🚧 | Road Congestion | bounce |
| low | 🌳 | Fallen Tree | slow sway |
| info | 💧 | Flash Flood | expanding ripple ring |
| test | 🚨 | Suspicious Activity | strobe flash |
| debug | 👁️ | General Sighting | static |

Cluster icons show the **worst-severity emoji** in the group so the most critical
incident is immediately visible without expanding the cluster.

### New feature — `clearMap()` public API

`MapWatch.clearMap()` removes all markers from the map and resets DC alert counts.
Accessible via the new **Clear map** toolbar button.

### New feature — tap marker expiry

Tap markers (`source=tap`) auto-expire after **30 seconds**. A server-side goroutine
scans every 5 s and broadcasts `marker.remove` for expired taps, keeping
`recomputeOffsets` fast under sustained tap load.

### Bug fix — sound not playing after many taps

`AudioContext.resume()` is async. Previously, beeps were scheduled immediately at
`ctx.currentTime + 0.01` before the context actually resumed (e.g. after a background
tab comes back), so those timestamps were already in the past. Fixed by scheduling
beeps inside the `.then()` callback of `ctx.resume()`.

### Offline-capable Docker image

Leaflet CSS/JS and WAV notification sounds are now **bundled into the Docker image**
at build time — no CDN required at runtime.

- `scripts/download-vendor.sh` — downloads Leaflet + MarkerCluster assets to `static/vendor/`
- `scripts/generate-sounds.py` — generates all 7 severity WAV files using Python stdlib (no deps)
- `index.html` falls back to unpkg CDN automatically when vendor files are absent (local dev)

### docker-compose.yml simplified

Root `docker-compose.yml` is now a minimal baseline (mapwatch + prometheus + alertmanager,
all modules off). Each example ships its own compose file. `docker-compose-local.yml`
removed — the main `Dockerfile` handles both vendor download and sound generation.

---

## v0.5.4

**Auto-hide SG overlay buttons when map data is not downloaded**

### Frontend

- SG overlay buttons (Divisions, Roads, Cycling, MRT, Bus Stops, Bus Routes) are now
  probed with `HEAD` requests on page load and hidden automatically when their GeoJSON
  file is absent — no click required, no alert popup
- When a `_toggleLayer` fetch returns 404, the button is hidden silently with a
  `console.warn` instead of the previous disruptive `alert()`

### blink-dot example

- Added `examples/blink-dot/Dockerfile` — builds binary only, no GeoJSON downloads
- `docker-compose.yml` updated to use the blink-dot `Dockerfile` so
  `cd examples/blink-dot && docker compose up --build` works without network access
  to data.gov.sg or busrouter.sg
- READMEs updated to document the no-download behaviour

### Release tooling

- `make release v=0.5.4` — single command bumps `VERSION`, commits, tags, and pushes
  (replaces the previous 4-step manual process)

---

## v0.5.0

**Drag-to-select spatial query**

### New feature — rectangle selection across all visible layers

A teal **Select** button in the toolbar activates a spatial selection mode.
Click and drag on the map to draw a rectangle; all features from every currently-visible
GeoJSON layer that fall within the rectangle are highlighted in cyan and listed in a
bottom panel grouped by layer type.

**Works generically with any combination of active layers:**

| Layer | Feature type | Hit test |
|-------|-------------|----------|
| Bus Stops | Point (circle marker) | Point inside rect |
| Bus Routes | LineString | Any route vertex inside rect |
| MRT Lines | LineString | Any line vertex inside rect |
| Roads | LineString | Any segment vertex inside rect |
| Cycling Paths | LineString | Any path vertex inside rect |
| Divisions (NPC) | Polygon | Any polygon vertex inside rect |

**Key behaviours:**

- Cursor changes to crosshair while Select is active; map pan is disabled during drag
- Dashed cyan rectangle previewed in real time as you drag
- On release: matched features highlight cyan; original colours restored when Select is deactivated
- Results appear in a bottom panel that slides up; dismisses on **Esc** or clicking **Select** again
- DC baseline markers and alert dot clicks are suppressed while Select is active (they resume on deactivation)
- Polygon/line features use a **vertex-level hit test** (not bounding-box) — large polygons like division boundaries are only selected when the drag rectangle physically overlaps their actual geometry

### Bug fixes

- Division polygons no longer selected by bounding-box proximity — replaced `bounds.intersects(layer.getBounds())` with `_geomHitsBounds` which checks that at least one polygon vertex falls inside the drawn rectangle
- Sea / marine sectors excluded from division selection results — NPC boundary GeoJSON includes offshore `M-Sect` features with no `DIVISION` property; these are now filtered out at query time

### Extended selection sources

- **DC baseline markers always included** — green breathing dots (and alert-firing dots) are queried regardless of any layer toggle; result chips show DC name, alert count, and worst severity
- **Individual alert markers always included** — every visible blinking/coloured alert dot in `markerMap` is tested against the selection bounds; result chips show alertname and severity
- **Heatmap regions conditionally included** — when the Heatmap toolbar button is active, heatmap rectangles that intersect the drawn selection are listed as a separate group in the results panel

### Documentation

- [SELECTION.md](SELECTION.md) — full reference: code walkthrough, geometry hit-test rationale, WebSocket integration diagram (updated to reflect alert/DC marker inclusion), extension guide

---

## v0.4.0

**Singapore transport & infrastructure overlays + map navigation controls**

### New optional map layers (no API key required)

Each layer requires a one-time download. Enable at startup via `mapwatch.yaml`
or toggle interactively from the toolbar.

| Layer | Command | Source | Output size |
|-------|---------|--------|-------------|
| Police divisions | `mapwatch download-sg division` | data.gov.sg (SPF NPC) | ~6 MB |
| Road network | `mapwatch download-sg roads` | data.gov.sg (SLA National Map Line) | ~3.4 MB (filtered) |
| Cycling paths | `mapwatch download-sg cycling` | data.gov.sg (LTA) | ~2.8 MB |
| MRT/LRT lines | `mapwatch download-sg mrt` | data.gov.sg (URA Master Plan 2019) | ~22 MB |
| Bus stops | `mapwatch download-sg busstops` | busrouter.sg | ~1.1 MB |
| Bus routes | `mapwatch download-sg busroutes` | busrouter.sg | ~8.9 MB |

All layers are downloaded from official Singapore government sources (data.gov.sg)
or busrouter.sg. No API keys required.

The `roads` command downloads the full 605 MB SLA National Map Line, then filters
it in-memory to keep only expressways, slip roads, and major roads (~9,165 features),
deleting the raw file automatically. See [ROAD.md](ROAD.md) for dataset details.

### New `layers:` config section in mapwatch.yaml

Enable layers at server startup (defaults all `false`):

```yaml
layers:
  division:   false   # NPC police division boundaries
  roads:      false   # expressways and major roads
  cycling:    false   # cycling paths
  mrt:        false   # MRT/LRT rail lines
  bus_stops:  false   # bus stop points
  bus_routes: false   # bus route lines
```

### Map navigation controls

- **Pan and zoom re-enabled** — drag to pan, scroll to zoom
- **Reset button (⊙ SG)** — returns to the default full-Singapore view instantly
- **Zoom slider** — scrub zoom level 10–19 in the toolbar; syncs bidirectionally with Leaflet zoom
- **Native zoom buttons** — Leaflet +/− control shown at bottom-right

### Architecture refactor

- Download functions moved to `internal/geo/sg/` sub-package — pattern extensible for `internal/geo/my/`, etc.
- Generic helpers (`DownloadHTTP`, `SaveBody`) exported from `internal/geo`; `OverpassFetch` retained but no longer used for SG layers
- CLI restructured: `mapwatch download-sg <layer>` sub-command tree instead of flat flags
- `GET /api/config` returns `layers` object; frontend auto-enables layers configured as `true`
- data.gov.sg poll-download helper retries up to 4× with exponential backoff on HTTP 429 (rate limit)

---

## v0.3.1

**Patch — NPC boundary stability + docs**

- Fix: test coverage for NPC boundary response parsing edge cases
- Docs: README updated with `mapwatch download-sg` usage example and screenshots
- No breaking changes from v0.3.0

---

## v0.3.0

**Singapore Police Division (NPC) boundary overlay**

- New command: `mapwatch download-sg` — fetches Singapore Police Force NPC Boundary GeoJSON from data.gov.sg (two-step poll-download API)
- New toolbar button: **NPC Zones** — lazy-loads `sg-npc-boundary.geojson` and renders cyan polygon boundaries with tooltips (NPC name + division)
- New REST endpoint: `GET /api/geojson/{name}` — serves any locally-downloaded GeoJSON file from the data directory; returns a 404 with hint when file is missing
- Styled Leaflet overlay: thin cyan outline, subtle 6% opacity fill, sticky tooltip showing NPC name and division
- Toggle is idempotent — subsequent clicks show/hide without re-fetching

**How to use:**
```bash
mapwatch download-sg --out ./data   # fetch sg-npc-boundary.geojson
mapwatch serve                       # click "NPC Zones" in toolbar
```

---

## v0.2.5

**Heatmap UX overhaul + green dot animation** (breaking config change — see migration)

- Heatmap shown by default; toggle off with the toolbar button (was opt-in)
- Empty regions always visible as grey outlines — zones are clear even before alerts fire
- Simplified region config: only `name` + `bounds` required (dropped `center`, `geohash_prefixes`)
- Region matching now uses spatial containment (lat/lng inside bounds) instead of geohash prefix matching
- Optional `color` per region overrides severity colour
- Count label floats at top edge of each active rectangle: `"West SG · 2 alerts"`
- Mouse coordinate overlay: hover map → `lat, lng` shown bottom-left (for tuning bounds)
- Green DC baseline dots now breathe gently (3 s scale + opacity cycle)
- Fix: DC aggregation timing race — markers arriving before `/api/config` returns now re-aggregate correctly
- Removed `examples/heatmap/` — root `mapwatch.yaml` is the reference config

**Migration** — replace `geohash_prefixes` + `center` with just `bounds`:
```yaml
# Before
- name: "West SG"
  center: [1.352, 103.700]
  bounds: [[1.28, 103.62], [1.42, 103.78]]
  geohash_prefixes: ["w21z8", "w21z2"]

# After
- name: "West SG"
  bounds: [[1.3203, 103.7054], [1.3958, 103.7885]]
```

---

## v0.2.4

**Heatmap: choropleth overlay** (breaking change — `bounds` field required)

- Replaced `leaflet.heat` fuzzy blobs with `L.rectangle` choropleth — solid filled regions coloured by severity, like a US state map
- New required config field: `bounds: [[lat_sw, lng_sw], [lat_ne, lng_ne]]` on each `heatmap.regions[]` entry
- Removed `leaflet.heat` CDN dependency
- Fix: heatmap timing race — `config.loaded` event dispatched after `fetchConfig` so regions are available before first redraw
- Fix: Light / Satellite theme buttons broken — dead `currentTheme` variable removed
- Fix: Toolbar Cluster, Spread, Borders buttons removed — unused on a fixed non-zoomable map
- Fix: README binary path corrected (`make build` → `bin/mapwatch`)
- Fix: `images/output.png` was untracked — now committed so screenshot renders on GitHub
- Added `[heatmap]` console debug logs and `[config]` backend log on `/api/config`

**Migration** — add `bounds` to every region in `mapwatch.yaml`:
```yaml
- name: "North SG"
  center: [1.432, 103.820]
  bounds: [[1.38, 103.70], [1.48, 103.95]]   # ← new
  geohash_prefixes: ["w22", "w23"]
```

---

## v0.2.3

- Heatmap: region aggregation with `leaflet.heat` blobs (superseded by v0.2.4)
- Fix: `store.Upsert` returns `(bool, []*Marker)` — updated tests
- Fix: `/api/config` locations field is now an array `[{name,lat,lng}]`

## v0.2.2

- DC baseline markers: green healthy dots from `locations` config
- Multi-alert panel on DC dot click
- Same-geohash spread offset fix

## v0.2.0

- Heatmap toggle button in toolbar
- `heatmap.regions` config support
- `/api/config` returns `heatmapRegions` field
