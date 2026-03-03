# MapWatch Release Notes

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
