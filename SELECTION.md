# Drag-to-Select (Spatial Query)

Allows the user to draw a rectangle on the map and instantly see all features from any currently-visible GeoJSON layer, DC baseline markers, individual alert markers, and heatmap regions that fall within it.

---

## How to Use

1. Toggle on one or more overlay layers (e.g. **Bus Stops**, **Bus Routes**, **MRT**).
2. Click **Select** in the toolbar (teal button) — the cursor changes to a crosshair.
3. Click and drag on the map to draw a selection rectangle.
4. Release the mouse — selected features **highlight in cyan** and a **bottom panel slides up** listing them grouped by layer.
5. Click **Select** again (or press **Esc**) to clear the highlights and dismiss the panel.

**What is always included (regardless of toggles):**

- **DC baseline markers** (green breathing dots) — always visible on the map; included with alert count and worst severity
- **Individual alert markers** — blinking/coloured dots from live alert stream; always included

**Conditionally included:**

- **GeoJSON overlay layers** — only queried when the layer is toggled on (e.g. Bus Stops, Bus Routes, MRT, Roads, Cycling, Divisions)
- **Heatmap regions** — only included when the Heatmap toolbar button is active

---

## Code Walkthrough

### State (`mapwatch.js` lines ~52–56)

```js
let selectionMode     = false;   // true while Select mode is active
let selectStart       = null;    // L.LatLng where the drag started
let selectRect        = null;    // the L.Rectangle drawn while dragging
let selectedSubLayers = [];      // tracks highlighted sublayers for colour restoration
```

### Activation — `toggleSelectionMode` / `_enableSelect` / `_disableSelect`

- `_enableSelect` disables Leaflet map dragging (`map.dragging.disable()`) so the mouse drag draws the rectangle instead of panning the map. Sets cursor to `crosshair`.
- `_disableSelect` re-enables dragging, restores original layer colours, hides the bottom panel, and cleans up the rectangle.
- Pressing **Esc** calls `_disableSelect` (the Escape handler checks `selectionMode` first, then falls back to closing the side panel).

### Map event handlers

| Handler | Trigger | Action |
|---|---|---|
| `_onSelectMouseDown` | left-button down on map canvas | records `selectStart` |
| `_onSelectMouseMove` | mouse move while button held | creates / updates the dashed `L.Rectangle` preview |
| `_onSelectMouseUp`   | mouse released | finalises bounds, calls `_queryAndShow` |

> **Note:** Leaflet marker click events stop propagation to the map, so clicking directly on an alert dot or a DC marker will *not* start a drag — drag must begin on the map canvas. DC and alert marker click handlers also guard against `selectionMode` so they don't open the side panel while selection mode is active.

### `_queryAndShow(bounds)`

This is the core function. It queries four independent sources — GeoJSON layers, DC markers, alert markers, and heatmap regions:

```js
function _queryAndShow(bounds) {
  const groups = {};

  // ── 1. GeoJSON overlay layers (only when visible) ──────────────────────────
  for (const [key, state] of Object.entries(layerState)) {
    if (!state.visible || !state.layer) continue;
    const hits = [];
    state.layer.eachLayer(sublayer => {
      let hit = false;
      if (typeof sublayer.getLatLng === 'function') {
        // Point features (bus stops) — single coordinate check.
        hit = bounds.contains(sublayer.getLatLng());
      } else if (sublayer.feature && sublayer.feature.geometry) {
        // Line / polygon features — vertex-level check.
        hit = _geomHitsBounds(sublayer.feature.geometry, bounds);
      }
      if (!hit || !sublayer.feature) return;
      const p = sublayer.feature.properties || {};
      // Skip sea/marine sectors in the division layer (no DIVISION assignment).
      if (key === 'division' && !p.DIVISION && !p.Division) return;
      hits.push({ props: p, sublayer });
      if (typeof sublayer.setStyle === 'function') {
        sublayer.setStyle({ color: _SELECT_HIGHLIGHT, fillColor: _SELECT_HIGHLIGHT,
                            weight: 2, opacity: 1, fillOpacity: 0.75 });
      }
      selectedSubLayers.push({ sublayer, key });
    });
    if (hits.length) groups[key] = hits;
  }

  // ── 2. DC baseline markers (always visible) ─────────────────────────────────
  const dcHits = [];
  for (const [name, dc] of Object.entries(dcMarkers)) {
    if (!bounds.contains([dc.lat, dc.lng])) continue;
    const alertCount = Object.keys(dc.alerts).length;
    const worst = alertCount > 0 ? worstSeverityOf(Object.values(dc.alerts)) : null;
    dcHits.push({ name, alertCount, worst });
  }
  if (dcHits.length) groups['_dc'] = dcHits;

  // ── 3. Individual alert markers (always visible) ─────────────────────────────
  const alertHits = [];
  for (const [, entry] of Object.entries(markerMap)) {
    if (!entry.leafletMarker) continue;
    if (!bounds.contains(entry.leafletMarker.getLatLng())) continue;
    alertHits.push({ data: entry.data });
  }
  if (alertHits.length) groups['_alerts'] = alertHits;

  // ── 4. Heatmap regions (only when Heatmap toggle is active) ─────────────────
  const heatBtn = document.getElementById('btn-heatmap');
  if (heatBtn && heatBtn.classList.contains('active') && heatmapRegions.length) {
    const regionHits = [];
    for (const region of heatmapRegions) {
      if (!region.bounds) continue;
      const rb = L.latLngBounds(region.bounds[0], region.bounds[1]);
      if (bounds.intersects(rb)) regionHits.push({ region });
    }
    if (regionHits.length) groups['_heatmap'] = regionHits;
  }

  _showSelectPanel(groups);
}
```

### `_flatCoords(geom)` and `_geomHitsBounds(geom, bounds)`

Line and polygon features use a **vertex-level hit test** instead of a bounding-box check. This prevents false positives where a large polygon (e.g. a division boundary whose bounding box spans the whole island) would be selected just because its bbox overlaps the drawn rectangle.

```js
function _flatCoords(geom) {
  // Returns [[lng, lat], ...] for all geometry types
  switch (geom.type) {
    case 'Point':           return [geom.coordinates];
    case 'LineString':      return geom.coordinates;
    case 'MultiLineString': return geom.coordinates.flat();
    case 'Polygon':         return geom.coordinates.flat();
    case 'MultiPolygon':    return geom.coordinates.flat(2);
    default:                return [];
  }
}

function _geomHitsBounds(geom, bounds) {
  // True only if at least one vertex falls inside the drawn rectangle.
  return _flatCoords(geom).some(([lng, lat]) => bounds.contains([lat, lng]));
}
```

**Why vertex check over bbox?** Polygon bounding boxes can be much larger than the actual polygon shape. With `bounds.intersects(layer.getBounds())`, selecting a small area on the map would match any division whose bbox overlaps — even if the actual polygon outline is far away. The vertex check requires the drag rectangle to physically touch the geometry.

### Sea / Marine Sector Filter (Division layer)

The NPC boundary GeoJSON includes offshore marine sectors (labelled `M-Sect` etc.) that have an `NPC_NAME` but no `DIVISION` property. These are filtered out to avoid spurious results:

```js
if (key === 'division' && !p.DIVISION && !p.Division) return;
```

Only features with a meaningful `DIVISION` assignment are included in selection results.

### `_clearSelectionHighlights()`

Restores each highlighted sublayer to its original style using `_SELECT_ORIG_STYLES` (a per-key map of the default colours defined in `LAYER_DEFS`), then empties `selectedSubLayers`.

### Bottom panel (`#select-panel`)

- Fixed to the bottom of the viewport, hidden at `bottom: -260px` by default.
- Adding the `.open` CSS class transitions it to `bottom: 0` (slides up).
- Content is grouped by source with cyan count badges, and each feature is a compact chip card showing:
  - **GeoJSON layers** — feature name + secondary field (road name, operator, division, etc.)
  - **DC markers** — DC name + alert count (green if healthy, severity colour if alerting)
  - **Alert markers** — alertname + severity badge
  - **Heatmap regions** — region name

---

## WebSocket Integration

The drag-to-select feature queries both static GeoJSON layers and live WebSocket-driven markers.

### Alert markers (WS-driven) — always included

Markers arrive over the WebSocket as `marker.add` / `marker.update` / `marker.remove` events and are stored in `markerMap`. Each entry with a `leafletMarker` (i.e. the dot is visible on the map) is tested against the selection bounds:

```js
for (const [, entry] of Object.entries(markerMap)) {
  if (!entry.leafletMarker) continue;
  if (!bounds.contains(entry.leafletMarker.getLatLng())) continue;
  alertHits.push({ data: entry.data });
}
```

While **Select** mode is active, alert marker clicks are suppressed so they don't open the side panel:

```js
lm.on('click', () => { if (!selectionMode) MapWatch.openPanel(data.id); });
```

The same guard is applied to DC baseline markers.

### DC baseline markers (always included)

DC dots are always visible and always queried — regardless of any layer toggles. The query uses `dcMarkers`, which stores `{ lat, lng, alerts }` for each configured location:

```js
for (const [name, dc] of Object.entries(dcMarkers)) {
  if (!bounds.contains([dc.lat, dc.lng])) continue;
  // ...
}
```

### GeoJSON overlay layers (file-driven)

The layers queried by drag-to-select (`busStops`, `busRoutes`, `mrt`, etc.) are loaded once on first toggle from `/api/geojson/<file>` and cached in `layerState[key].layer`. They are static GeoJSON files — they do not receive WebSocket updates — so the selection result is always consistent with what is visible on screen.

### Sequence diagram

```
User drag-release
      │
      ▼
_onSelectMouseUp(bounds)
      │
      ├─► _clearSelectionHighlights()    ← restore previous highlight colours
      │
      └─► _queryAndShow(bounds)
              │
              ├─ for each layerState[key] where visible=true
              │       eachLayer → vertex hit test
              │       skip division features without DIVISION property (sea mask)
              │       setStyle(cyan) on matches
              │       collect { props, sublayer }
              │
              ├─ for each dcMarkers[name]
              │       point-in-bounds check
              │       collect { name, alertCount, worstSeverity }
              │
              ├─ for each markerMap entry where leafletMarker exists
              │       point-in-bounds check
              │       collect { data }
              │
              ├─ if heatmap button active:
              │       for each heatmapRegions entry
              │       bounds.intersects(regionBounds)
              │       collect { region }
              │
              └─ _showSelectPanel(groups) → slides up bottom panel

WebSocket (parallel, independent)
      │
      ├─ marker.add    → upsertMarker → markerMap + leafletMarker   (queried by _alerts)
      ├─ marker.update → upsertMarker → markerMap + leafletMarker   (queried by _alerts)
      └─ marker.remove → removeMarker → removes from markerMap
```

The WS stream feeds `markerMap` and `dcMarkers`; selection reads both live. GeoJSON layers in `layerState` are static.

---

## Extending to New Layers

To make a new GeoJSON layer selectable:

1. Add it to `layerState` and `LAYER_DEFS` in `mapwatch.js` (the existing pattern).
2. Add its default style to `_SELECT_ORIG_STYLES` so highlights can be cleared correctly.
3. Add a human-readable label to `_LAYER_LABELS`.
4. Add the toolbar button in `index.html`.

No changes to `_queryAndShow` are needed — it picks up new layers automatically via the `Object.entries(layerState)` loop.
