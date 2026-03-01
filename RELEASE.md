# MapWatch Release Notes

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
