I want to build an open source Go project called `mapwatch` — a world map
visualization tool with CLI and server modes, using Leaflet.js for interactive
maps. Help me scaffold the full project structure and implement the core foundation.

---

## Project Overview

A Go binary with two modes:
1. **CLI mode** — download map data, slice to a region (e.g. Singapore),
   build static assets, or export a self-contained static HTML file
2. **Server mode** — serve an interactive Leaflet.js dashboard that accepts
   data from external systems (Prometheus, Alertmanager, webhooks) and renders
   geo-visualizations with effects

The project must be **extensible** — users can bring their own themes,
overlays, alert rules, and effects via config or plugins.

---

## Tech Stack

- **Language**: Go (latest stable)
- **CLI framework**: cobra + viper
- **Frontend**: Single-file HTML + Leaflet.js (loaded from CDN or embedded)
- **Embedding**: Go `embed` to bundle HTML/JS/CSS into binary
- **Map data**: GeoJSON (Natural Earth or OpenStreetMap-derived)
- **Server**: chi router
- **WebSocket**: gorilla/websocket
- **Config**: YAML config file + env var overrides
- **Charts**: uPlot (lightweight, fast time-series rendering in side panel)
- **Geohash**: preferred geo encoding over raw lat/lng — use a Go geohash 
  library (e.g. mmcloughlin/geohash) for decoding

---

## Geo Encoding Strategy

**Geohash is the preferred and primary geo encoding.** Lat/lng is supported 
as a fallback only.

Geohash advantages for this use case:
- A single label value encodes both lat and lng (e.g. `w21zd3`)
- Precision is controllable — shorter geohash = larger bounding box, 
  useful for grouping nearby alerts into a region
- Naturally supports clustering — alerts sharing the same geohash prefix 
  are in the same area
- Common in infrastructure labeling (Prometheus exporters, service discovery)

### Geo resolution priority (in order):
1. `geohash` label — decode to center lat/lng + bounding box
2. `lat` + `lng` labels — use directly
3. `datacenter` or `region` label — look up in config geohash table
4. Drop marker with warning log if no geo info resolvable

### Config geohash lookup table:
```yaml
locations:
  sg-dc-1: w21zd3
  sg-dc-2: w21z8k
  us-east-1: dr5reg
  us-west-2: 9q8yy
  eu-west-1: gc6uf
```

This allows alerts without geo labels to still be placed on the map 
as long as the datacenter/region label matches a known location.

---

## Multiple Markers Per Alert Group

A single Alertmanager webhook call can contain multiple alerts firing 
simultaneously. Each alert in the `alerts[]` array must be treated as 
an independent marker with its own:
- Position (resolved individually via geo priority above)
- Severity and visual effect
- Labels and annotations
- Fingerprint as stable ID

All markers from one webhook call are reconciled atomically:
- New fingerprints → add markers
- Existing fingerprints → update markers (severity or labels may change)
- Fingerprints present in store but absent from firing set → remove markers

Each marker is broadcast individually over WebSocket so the frontend 
can handle each one independently. Do not batch into a single event.

### Multiple markers at same geohash:
When two or more markers share the same geohash (or resolve to overlapping 
positions), they must not stack invisibly on top of each other.

Handle this with **offset clustering**:
- Detect markers at identical decoded lat/lng
- Spread them in a small circle around the geohash center point 
  (configurable radius, default 0.01 degrees)
- Each marker gets a deterministic offset based on its index among 
  co-located markers so positions are stable across re-renders

Alternatively expose a **cluster toggle** in the UI:
- Clustered mode: use Leaflet.markercluster plugin — group nearby markers, 
  show count badge, expand on click
- Spread mode: apply offset as above so individual markers are all visible

Both modes must be supported and switchable from the UI toolbar.

---

## CLI Commands to Scaffold
mapwatch download              # Download world GeoJSON map data
mapwatch slice --region=SG     # Clip GeoJSON to bounding box of a region/country
mapwatch build                 # Bundle assets into the binary output dir
mapwatch export                # Export self-contained static HTML (no server needed)
mapwatch serve                 # Start HTTP server with live dashboard

---

## Server Mode — HTTP Endpoints

### Inbound data endpoints
- `POST /api/alerts` — receive standard Alertmanager webhook payload
- `POST /api/markers` — add/update a generic geo marker (custom integrations)
- `DELETE /api/markers/:id` — remove a marker
- `GET /api/markers` — return all current markers as JSON (for page reload sync)

### On-click detail endpoint (pull on demand)
- `GET /api/markers/:id/details?start=now-1h&end=now`
  - Look up marker by ID, retrieve stored labels
  - Match alertname against query_templates in config
  - Render PromQL using Go text/template with marker labels
  - Query Prometheus `/api/v1/query_range`
  - Return normalized time-series JSON for uPlot rendering
  - Return graceful error JSON if Prometheus unreachable

### WebSocket
- `GET /ws` — browser connects here on page load
- On connect: server immediately sends all current markers as individual 
  `marker.add` events so late-joining clients are fully synced
- Ongoing: broadcast individual marker events as they arrive

---

## Alertmanager Integration

Accept the standard Alertmanager webhook v4 payload:
```json
{
  "version": "4",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "fingerprint": "abc123",
      "labels": {
        "alertname": "HighCPU",
        "severity": "critical",
        "instance": "sg-prod-1",
        "datacenter": "sg-dc-1",
        "geohash": "w21zd3"
      },
      "annotations": {
        "summary": "CPU usage above 90%",
        "description": "Instance sg-prod-1 CPU at 94%"
      },
      "startsAt": "2024-01-01T00:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z"
    },
    {
      "status": "firing",
      "fingerprint": "def456",
      "labels": {
        "alertname": "DiskFull",
        "severity": "warning",
        "instance": "sg-prod-2",
        "datacenter": "sg-dc-2",
        "geohash": "w21z8k"
      },
      "annotations": {
        "summary": "Disk usage above 85%"
      },
      "startsAt": "2024-01-01T00:01:00Z",
      "endsAt": "0001-01-01T00:00:00Z"
    }
  ]
}
```

Transformer rules:
- Iterate `alerts[]` — each alert becomes one independent Marker
- Resolve geo position using priority: geohash → lat/lng → datacenter lookup
- Decode geohash to center lat/lng + store original geohash string
- Use `fingerprint` as stable marker ID
- `status=firing` → upsert marker in store
- `status=resolved` → remove marker from store
- After processing all alerts, reconcile: any stored marker whose fingerprint 
  is absent from the current firing set must be removed
- Broadcast each add/update/remove as individual WebSocket events
- Log warning and skip (do not crash) for alerts with unresolvable geo

Alertmanager config to point at mapwatch:
```yaml
receivers:
  - name: mapwatch
    webhook_configs:
      - url: http://mapwatch:8080/api/alerts
        send_resolved: true
```

Alert rule geo labeling examples:
```yaml
# Option 1: geohash label (preferred)
labels:
  severity: critical
  geohash: w21zd3

# Option 2: datacenter label (resolved via config lookup table)
labels:
  severity: warning
  datacenter: sg-dc-1

# Option 3: raw lat/lng fallback
labels:
  severity: info
  lat: "1.3521"
  lng: "103.8198"
```

---

## Prometheus On-Click Pull

When user clicks a marker, frontend calls:
GET /api/markers/:id/details?start=now-1h&end=now

Go server:
1. Retrieve marker by ID from store
2. Match `alertname` label against `query_templates` in config
3. Render each PromQL string using `text/template` with full label map
4. Query Prometheus `/api/v1/query_range` for each template entry
5. Return array of `{ label, timestamps[], values[] }` for uPlot

Config example:
```yaml
prometheus:
  url: http://prometheus:9090
  timeout: 10s

query_templates:
  HighCPU:
    - label: "CPU Usage %"
      query: 'rate(node_cpu_seconds_total{mode="user",instance="{{.instance}}"}[5m]) * 100'
    - label: "Load Average 1m"
      query: 'node_load1{instance="{{.instance}}"}'
  DiskFull:
    - label: "Disk Used %"
      query: '100 - (node_filesystem_free_bytes{instance="{{.instance}}"} / node_filesystem_size_bytes{instance="{{.instance}}"} * 100)'
  default:
    - label: "CPU Usage %"
      query: 'rate(node_cpu_seconds_total{mode="user",instance="{{.instance}}"}[5m]) * 100'
```

---

## Frontend Features (Leaflet.js)

### Map base
- Default dark tile layer (CartoDB Dark Matter)
- Theme switcher in toolbar: dark / light / satellite
- GeoJSON country border overlay (toggleable)

### DC Baseline Markers
Known datacenter locations defined in `config.locations` are shown as
permanent green "healthy" dots on map load (before any alerts arrive).

Behaviour state machine per DC location:

| State    | Condition                    | Dot         | Animation  | Badge   |
|----------|------------------------------|-------------|------------|---------|
| Healthy  | 0 active alerts              | Green 14px  | —          | —       |
| Info     | 1+ alerts, worst=info        | Blue 16px   | —          | if > 1  |
| Warning  | 1+ alerts, worst=warning     | Yellow 18px | —          | if > 1  |
| Critical | 1+ alerts, any=critical      | Red 20px    | CSS pulse  | if > 1  |

When multiple alerts share a DC, the dot shows a count badge (top-right circle).
Hovering shows a tooltip with up to 3 alert names (+"N more").
Clicking opens the DC aggregated panel.

### DC Aggregated Panel
Opens when clicking a DC baseline marker. Shows:
- Severity summary bar — proportional colour segments (red/yellow/blue)
- Severity chips — count per severity level
- Scrollable alert list sorted by: severity (worst first), then `startsAt` desc
- Each row: severity badge + alert name + instance + duration
- Click any row to drill into the individual alert detail panel

### Marker rendering (individual alerts without a matching DC)
- Each marker rendered as a Leaflet DivIcon
- Color by severity: critical=red (#f85149), warning=yellow (#e3b341), info=blue (#58a6ff), unknown=grey
- Geohash markers optionally show bounding rectangle overlay on hover
  (to visualize geohash precision)

### Marker effects (modular JS plugin system)
```js
MapWatch.registerEffect(name, handlerFn)
```
Built-in effects:
1. **blink-critical** — CSS keyframe pulse for severity=critical
2. **heatmap** — density overlay using Leaflet.heat; toggle via toolbar "Heatmap" button
3. **geohash-grid** — on marker hover, draw geohash bounding rectangle

### Clustering and overlap handling
- Toolbar toggle between **cluster mode** (Leaflet.markercluster) and
  **spread mode** (deterministic circular offset for co-located markers)
- In cluster mode: click cluster to expand, badge shows count
- In spread mode: all markers individually visible with small positional offset
- DC baseline markers are on a separate layer — always visible, not clustered

### On-click side panel (right drawer)
Opens when user clicks any individual alert marker. Shows:
- Alert name + severity badge (color-coded)
- Instance / datacenter labels
- Summary and description from annotations
- Duration: `startsAt` to now, human readable
- Prometheus metric links from `query_templates` config
- Raw labels section (collapsible)
- Close button or Escape key to dismiss

### WebSocket event contract
```js
// Frontend handles these event types:
{ "type": "marker.add",    "marker": { id, lat, lng, geohash, severity, alertname, labels, annotations, startsAt } }
{ "type": "marker.update", "marker": { ... } }
{ "type": "marker.remove", "id": "fingerprint" }
```

On WebSocket connect: server replays all current markers as `marker.add` 
events for full state sync on page load or reconnect.

---

## Logging and Observability

### Server-side logs (Go `log` package, stderr)

All server logs use the format: `prefix: key=value key=value`

| Event | Log prefix | Required fields |
|-------|-----------|-----------------|
| WS upgrade request | `ws:` | `remote=<addr>` |
| WS upgrade failure | `ws:` | `remote=<addr>`, `err=<msg>` |
| WS client connected | `ws:` | `remote=<addr>`, `total=<n>` |
| WS client disconnected | `ws:` | `remote=<addr>`, `remaining=<n>` |
| WS marker replay on connect | `ws:` | `replaying=<n> markers` |
| Hub broadcast | `hub:` | `type=<event-type>`, `clients=<n>` |
| Hub buffer full | `hub:` | drop notice |
| Alertmanager webhook | `alertmanager:` | `status=<firing\|resolved>`, `alerts=<n>` |
| Alert geo resolved | `alertmanager:` | `fingerprint=`, `alertname=`, `geohash=`, `severity=` |
| Alert geo skipped | `alertmanager:` | `fingerprint=`, `alertname=`, reason |
| WS broadcast add | `ws:` | `id=`, `alertname=`, `severity=` |
| WS broadcast remove | `ws:` | `id=` |
| Processed webhook | `alertmanager:` | `added=<n>`, `updated=<n>`, `removed=<n>` |

### Browser console logs (JS `console.log/warn/error`)

All browser logs are prefixed with `[MapWatch]`.

| Event | Level | Format |
|-------|-------|--------|
| WS connecting | `log` | `[MapWatch] WS connecting to <url>` |
| WS connected | `log` | `[MapWatch] WS connected` |
| WS closed | `warn` | `[MapWatch] WS closed code=<n> reason=<r> — reconnecting in 3s` |
| WS error | `error` | `[MapWatch] WS error <err>` |
| WS event received | `log` | `[MapWatch] WS event <type> id=<id> sev=<sev>` |
| Marker upsert | `log` | `[MapWatch] upsertMarker ADD\|UPDATE id=<id> sev=<sev> lat=<n> lng=<n>` |
| Marker added to layer | `log` | `[MapWatch] marker added to layer, pulse=<bool>` |
| Effect error | `error` | `effect error: <err>` |

---

## Extensibility Design

- **Themes**: YAML tile layer URLs + CSS variable overrides, loaded from config
- **Effects**: JS modules that self-register via `MapWatch.registerEffect()`
- **Transformers**: Go interface for adding new data source integrations
- **Query templates**: per-alertname PromQL in config, falls back to default
- **Location table**: geohash lookup by datacenter/region name in config
```go
type Transformer interface {
    Name() string
    Transform(payload []byte) ([]Marker, error)
}
```

---

## Marker Struct (internal)
```go
type Marker struct {
    ID          string            // fingerprint or generated UUID
    Geohash     string            // original geohash string if provided
    Lat         float64           // decoded or direct
    Lng         float64           // decoded or direct
    GeoBounds   *GeoBounds        // decoded geohash bounding box, nil if from lat/lng
    Severity    string            // critical, warning, info
    AlertName   string
    Labels      map[string]string // full label set for PromQL templating
    Annotations map[string]string
    StartsAt    time.Time
    UpdatedAt   time.Time
    Source      string            // "alertmanager", "api"
    Offset      *LatLng           // computed spread offset if co-located
}

type GeoBounds struct {
    MinLat, MaxLat float64
    MinLng, MaxLng float64
}

type LatLng struct {
    Lat, Lng float64
}
```

---

## Project Structure
mapwatch/
├── cmd/
│   ├── root.go
│   ├── download.go
│   ├── slice.go
│   ├── export.go
│   └── serve.go
├── internal/
│   ├── server/
│   │   ├── server.go          # HTTP server setup, chi router
│   │   ├── handlers.go        # REST API handlers
│   │   └── hub.go             # WebSocket broadcast hub
│   ├── geo/
│   │   ├── download.go        # Fetch GeoJSON from Natural Earth
│   │   ├── slice.go           # Clip GeoJSON by bounding box
│   │   └── geohash.go         # Geohash decode, bounds, offset calculation
│   ├── marker/
│   │   └── store.go           # In-memory store, reconcile, offset assignment
│   ├── transformer/
│   │   ├── interface.go       # Transformer interface
│   │   ├── alertmanager.go    # Alertmanager webhook → []Marker
│   │   └── prometheus.go      # Prometheus pull proxy + PromQL templating
│   └── config/
│       └── config.go          # Viper config, locations table, query templates
├── static/
│   ├── index.html             # Main dashboard HTML
│   ├── mapwatch.js            # Core JS: WS client, plugin registry,
│   │                          # marker manager, side panel, clustering
│   ├── effects/
│   │   ├── blink.js
│   │   ├── heatmap.js
│   │   └── geohash-grid.js
│   └── themes/
│       └── default.yaml
├── embed.go                   # go:embed directives
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── docker-compose.yml         # mapwatch + prometheus + alertmanager wired up
└── README.md

---

## Implementation Order

1. Scaffold full directory and file structure with all files stubbed
2. Implement `go.mod` with all dependencies: cobra, viper, chi, 
   gorilla/websocket, mmcloughlin/geohash
3. Implement config loading with locations table and query_templates
4. Implement `geo/geohash.go` — decode geohash to lat/lng center + bounds, 
   spread offset calculation for co-located markers
5. Implement `marker/store.go` — in-memory store, upsert, remove, 
   reconcile, offset assignment for co-located markers
6. Implement WebSocket hub — connect/disconnect, broadcast, 
   replay current state on new connection
7. Implement Alertmanager transformer — parse full payload, iterate alerts[], 
   resolve geo via geohash→lat/lng→datacenter lookup priority, 
   fingerprint-based reconcile, individual WS broadcast per marker event
8. Implement Prometheus proxy handler — label-based PromQL template 
   rendering, query_range call, normalized response
9. Implement `handlers.go` — wire all endpoints, connect transformer 
   to store and hub
10. Build `index.html` + `mapwatch.js`:
    - Leaflet dark tile layer
    - WebSocket client with reconnect
    - Marker add/update/remove handling
    - Severity colors + CSS pulse for critical
    - Cluster/spread toggle in toolbar
    - Right side panel with uPlot chart, time range selector, 
      loading/error states
    - Geohash bounding box hover overlay
    - Theme switcher
11. Implement `mapwatch serve` command
12. Implement `mapwatch export` — static HTML snapshot with embedded 
    marker JSON, no WebSocket, no live Prometheus
13. Implement `mapwatch download` and `mapwatch slice --region=SG`
14. Makefile: `build`, `run`, `test`, `docker-build`
15. `docker-compose.yml` with mapwatch + prometheus + alertmanager, 
    sample alertmanager.yml receiver config included as comment
16. README: ASCII architecture diagram, quickstart, Alertmanager setup, 
    geo labeling guide (geohash preferred), query_template reference, 
    effect plugin authoring guide

After implementation, summarize what is fully implemented, what is stubbed
or placeholder, and recommend the next 3 iteration priorities.

---

## Heatmap v2 — Pre-defined Region Aggregation

### Motivation

The current heatmap (`heatmap.js`) renders each alert as an independent
lat/lng point with a small intensity blob. For dense deployments, this
produces a noisy scatter of blobs rather than meaningful spatial pattern.

The desired behaviour mirrors a US state-level heatmap:
- The map is divided into named regions (states / planning zones)
- All alerts whose geohash falls inside a region **aggregate into one
  heatmap point** placed at the region's defined centroid
- The point's weight reflects the combined severity of all alerts in that
  region — not just count

### Config schema

Regions are defined in `mapwatch.yaml` under a new `heatmap.regions` key:

```yaml
heatmap:
  regions:
    - name: "North"
      center: [1.432, 103.820]       # lat, lng — where the heatmap blob is placed
      geohash_prefixes: ["w22", "w23"]
    - name: "East"
      center: [1.352, 103.940]
      geohash_prefixes: ["w21z", "w21x"]
    - name: "West"
      center: [1.352, 103.700]
      geohash_prefixes: ["w21y", "w21w"]
    - name: "South"
      center: [1.275, 103.820]
      geohash_prefixes: ["w21t", "w21s", "w21q"]
    - name: "Central"
      center: [1.352, 103.820]
      geohash_prefixes: ["w21z8", "w21z9", "w21zd"]
```

Rules:
- `geohash_prefixes` is an ordered list; matching is `marker.geohash.startsWith(prefix)`.
- Prefixes are checked in list order — first match wins.
- A shorter prefix subsumes all longer ones (e.g. `"w22"` matches `"w221"`, `"w22z"`).
- Regions must not have overlapping prefixes; the config author is responsible for
  ensuring disjoint coverage.
- A marker whose geohash matches no region falls back to **point mode** (rendered
  individually, same as current behaviour).
- If `heatmap.regions` is empty or absent, the heatmap runs in point mode only.

### Region-match function (client-side JS)

```js
// Returns the first matching region or null.
function geohashToRegion(geohash, regions) {
  if (!geohash) return null;
  for (const region of regions) {
    for (const prefix of region.geohash_prefixes) {
      if (geohash.startsWith(prefix)) return region;
    }
  }
  return null;
}
```

This is the single function that decides aggregation. It runs for every
marker on every heatmap refresh — O(markers × regions × avg_prefixes),
which is negligible at typical alert volumes.

### Aggregation algorithm

On every `marker.add / marker.update / marker.remove` event:

```
regionBuckets = {}   // region.name → accumulated weight

for each marker in markerMap:
  intensity = severityIntensity(marker.severity)  // critical=1.0, warning=0.6, info=0.3
  region = geohashToRegion(marker.geohash, configRegions)

  if region:
    regionBuckets[region.name] += intensity
  else:
    pointFallback.push([marker.lat, marker.lng, intensity])   // individual point

// Build heatmap input
points = []
for each (name, weight) in regionBuckets:
  region = regionsByName[name]
  points.push([region.center[0], region.center[1], weight])

points.push(...pointFallback)

heatLayer.setLatLngs(points)
```

Weight is **cumulative severity**, not raw count. A region with one
critical alert (weight 1.0) shows hotter than one with five info alerts
(weight 1.5), because a single critical incident is operationally worse.

### Leaflet.heat parameter tuning for region mode

| Parameter | Point mode (current) | Region mode (new) |
|-----------|---------------------|-------------------|
| `radius`  | 25                  | 60                |
| `blur`    | 15                  | 40                |
| `maxZoom` | 10                  | 17 (no cap)       |

Larger radius and blur cause each region centroid to visually fill the
region's geographic extent and blend into neighbours, producing the smooth
choropleth-like gradient the user expects. When zoomed in, the region blob
still sits at the centroid — this is intentional (the heatmap is a
high-level summary view, not a precise locator).

### API: expose regions to frontend

`GET /api/config` already returns `locations`. Extend it to also return
`heatmapRegions` (empty array if unconfigured):

```json
{
  "prometheusUrl": "http://localhost:9090",
  "locations": [{ "name": "sg-dc-1", "lat": 1.35, "lng": 103.82 }],
  "heatmapRegions": [
    { "name": "North",   "center": [1.432, 103.820], "geohash_prefixes": ["w22", "w23"] },
    { "name": "East",    "center": [1.352, 103.940], "geohash_prefixes": ["w21z", "w21x"] },
    { "name": "West",    "center": [1.352, 103.700], "geohash_prefixes": ["w21y", "w21w"] },
    { "name": "South",   "center": [1.275, 103.820], "geohash_prefixes": ["w21t", "w21s", "w21q"] },
    { "name": "Central", "center": [1.352, 103.820], "geohash_prefixes": ["w21z8", "w21z9", "w21zd"] }
  ]
}
```

`heatmap.js` fetches this once on load (or reads from `MapWatch.config`
if the main JS already fetched it), then uses the regions array on every
refresh.

### Go config struct changes

```go
type HeatmapRegion struct {
    Name            string     `yaml:"name"`
    Center          [2]float64 `yaml:"center"`   // [lat, lng]
    GeohashPrefixes []string   `yaml:"geohash_prefixes"`
}

type HeatmapConfig struct {
    Regions []HeatmapRegion `yaml:"regions"`
}

type Config struct {
    // existing fields ...
    Heatmap HeatmapConfig `yaml:"heatmap"`
}
```

`GetConfig` handler serialises `cfg.Heatmap.Regions` → `heatmapRegions`
in the JSON response.

### Implementation plan

1. Add `HeatmapConfig` / `HeatmapRegion` to `internal/config/config.go`
2. Extend `Handlers.GetConfig` to include `heatmapRegions` in response
3. Update `TestAPIGetConfig` to assert `heatmapRegions` field is present
4. Rewrite `static/effects/heatmap.js`:
   - Fetch regions from `MapWatch.config.heatmapRegions` (populated at
     page load alongside `prometheusUrl`)
   - Implement `geohashToRegion()` and the bucket aggregation loop
   - Switch Leaflet.heat params based on whether regions are configured
   - Keep `MapWatch.toggleHeatmap()` interface unchanged (toolbar button
     still works the same way)
5. Update `mapwatch.yaml` example and deploy configs with SG region table
6. Document the `heatmap.regions` config key in README