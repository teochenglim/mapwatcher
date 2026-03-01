# MapWatch Release Notes

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
