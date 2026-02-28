# Contributing to MapWatch

MapWatch is an open-source Prometheus Alertmanager receiver that renders firing alerts as live, geo-located markers on a map.

This guide covers the two most common extension points: **effects** and **themes**.

---

## Table of contents

1. [Development setup](#development-setup)
2. [Writing a custom effect](#writing-a-custom-effect)
3. [Adding a map theme](#adding-a-map-theme)
4. [Running tests](#running-tests)
5. [Submitting a pull request](#submitting-a-pull-request)

---

## Development setup

```bash
git clone https://github.com/teochenglim/mapwatch.git
cd mapwatch

# Build binary (embeds static assets)
make build

# Run server locally (reachable at http://localhost:8080)
./bin/mapwatch serve

# Run tests (race detector on)
make test
```

Static files in `static/` are embedded at build time via `go:embed`.
While developing effects, run `./bin/mapwatch serve` and hard-refresh the browser
after each JS change (no hot-reload is built in).

---

## Writing a custom effect

Effects are JavaScript functions that run on every WebSocket event.
They can animate markers, draw overlays, play sounds — anything the browser supports.

### How effects work

```
WebSocket message ──► handleWSEvent() ──► runEffects(msg)
                                              │
                        ┌─────────────────────┼──────────────────────┐
                        ▼                     ▼                      ▼
                  blink-critical          heatmap              your-effect
```

Each effect is called with:

| Argument    | Type                  | Description                            |
|-------------|-----------------------|----------------------------------------|
| `event`     | `{ type, marker, id }` | The WebSocket event (`marker.add` etc) |
| `map`       | `L.Map`               | The Leaflet map instance               |
| `markerMap` | `{ id → { leafletMarker, data } }` | All current markers      |

### Event types

| `event.type`    | When fired                         | `event.marker` | `event.id` |
|-----------------|------------------------------------|----------------|------------|
| `marker.add`    | New alert arrives                  | ✓              | —          |
| `marker.update` | Existing alert re-fires or changes | ✓              | —          |
| `marker.remove` | Alert resolves                     | —              | ✓          |

### Step-by-step: create an effect

**1. Create the file**

```
static/effects/my-effect.js
```

**2. Register your effect**

```js
MapWatch.registerEffect('my-effect', function (event, map, markerMap) {
  // Guard: only run on the event types you care about.
  if (event.type !== 'marker.add' && event.type !== 'marker.update') return;

  const m = event.marker;
  if (!m) return;

  // Example: log every critical alert to the console.
  if (m.severity === 'critical') {
    console.log('Critical alert!', m.alertname, 'at', m.lat, m.lng);
  }
});
```

**3. Include it in `static/index.html`** (after `mapwatch.js`)

```html
<script src="effects/my-effect.js"></script>
```

### Accessing the Leaflet marker

```js
MapWatch.registerEffect('highlight-warning', function (event, map, markerMap) {
  if (event.type !== 'marker.add') return;
  const m = event.marker;
  const entry = markerMap[m.id];
  if (!entry) return;

  // entry.leafletMarker is the L.marker instance.
  const el = entry.leafletMarker.getElement();
  if (!el) return;
  const dot = el.querySelector('.mw-marker');
  if (!dot) return;

  // Toggle a CSS class based on severity.
  dot.classList.toggle('mw-highlight', m.severity === 'warning');
});
```

Add your CSS class to `static/index.html`:

```css
.mw-highlight {
  outline: 3px solid #e3b341;
  outline-offset: 3px;
}
```

### Built-in effects to reference

| File | Technique |
|------|-----------|
| [`static/effects/blink.js`](static/effects/blink.js) | Toggles `.mw-pulse` CSS class on critical markers |
| [`static/effects/heatmap.js`](static/effects/heatmap.js) | Builds a `L.heatLayer` from all marker positions |
| [`static/effects/geohash-grid.js`](static/effects/geohash-grid.js) | Draws `L.rectangle` on marker hover using geoBounds |

---

## Adding a map theme

Themes are tile-layer URLs registered in `mapwatch.js`.

### Step-by-step: add a theme

**1. Find a tile provider**

Popular free options:
- [Stadia Maps](https://stadiamaps.com/products/map-tiles/) — `https://tiles.stadiamaps.com/...`
- [OpenStreetMap](https://wiki.openstreetmap.org/wiki/Tiles) — `https://tile.openstreetmap.org/{z}/{x}/{y}.png`
- [CartoDB](https://github.com/CartoDB/basemap-styles) — used by default

**2. Add your theme to the `THEMES` constant in `static/mapwatch.js`**

```js
const THEMES = {
  dark:      { url: '...', attribution: '...' },
  light:     { url: '...', attribution: '...' },
  satellite: { url: '...', attribution: '...' },

  // Add yours:
  topo: {
    url: 'https://tile.opentopomap.org/{z}/{x}/{y}.png',
    attribution: '&copy; OpenTopoMap',
  },
};
```

**3. Add a toolbar button in `static/index.html`**

```html
<button class="tb-btn" id="btn-topo" onclick="MapWatch.setTheme('topo')">Topo</button>
```

**4. Update `setTheme()` in `mapwatch.js`** to include the new theme ID in the toggle loop

```js
['dark', 'light', 'satellite', 'topo'].forEach((n) => {
  document.getElementById('btn-' + n).classList.toggle('active', n === name);
});
```

**5. Rebuild**

```bash
make build
./bin/mapwatch serve
```

---

## Running tests

All tests live in [`tests/`](tests/).

```bash
# Run all tests with race detector
make test

# Verbose output
make test-verbose
```

| File | Coverage |
|------|----------|
| `store_test.go` | Marker store CRUD, reconcile, spread offsets |
| `alertmanager_test.go` | Geo resolution priority (geohash → lat/lng → datacenter) |
| `prometheus_test.go` | PromQL template rendering, external URL generation |
| `api_test.go` | All REST endpoints via real `httptest.Server` |

When adding a new feature, add a test in `tests/` that exercises the happy path
and at least one error path.

---

## Submitting a pull request

1. Fork the repo and create a branch: `git checkout -b feat/my-feature`
2. Make your changes
3. Run `make test` — all tests must pass
4. Run `make build` — binary must compile
5. Open a pull request against `main`

Please keep PRs focused: one feature or fix per PR.
Include a short description of what changed and why.
