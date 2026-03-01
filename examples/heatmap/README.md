# Example: Heatmap

Demonstrates the **Leaflet.heat density overlay** with 13 alerts spread across
APAC, US, and EU — deliberately clustered in Singapore and US East to produce
visible hot-spots on the heatmap.

Heatmap intensity is proportional to severity:

| Severity   | Heat intensity |
|------------|---------------|
| `critical` | 1.0 (red/hot) |
| `warning`  | 0.6 (orange)  |
| `info`     | 0.3 (cool)    |

---

## Start the stack

```bash
cd examples/heatmap
docker compose up -d
```

| Service      | URL                     |
|--------------|-------------------------|
| MapWatch     | http://localhost:8082   |
| Prometheus   | http://localhost:9092   |
| Alertmanager | http://localhost:9095   |

Open **http://localhost:8082**.  Within 15–30 seconds all 13 dots appear on the map.

---

## Enable the heatmap overlay

The heatmap is **opt-in** — dots are always visible but the heat layer is hidden by default.

**Option A — toolbar button** (if your build includes it):

Click the **Heatmap** toggle button in the map toolbar.

**Option B — browser console**:

Open DevTools → Console and run:

```js
MapWatch.toggleHeatmap()
```

Call it again to hide the overlay.

---

## What you should see

| Region         | Alerts | Expected hotspot |
|----------------|--------|-----------------|
| Singapore      | 4      | Dense red cluster near 1.3°N 103.8°E |
| New York metro | 3      | Hot-spot near 40.7°N 74.0°W |
| Los Angeles    | 2      | Warm area near 34.0°N 118.2°W |
| Tokyo          | 1      | Red dot near 35.7°N 139.7°E |
| Sydney         | 1      | Warm dot near 33.9°S 151.2°E |
| Dublin         | 1      | Red dot near 53.3°N 6.2°W |
| Berlin         | 1      | Warm dot near 52.5°N 13.4°E |

---

## How the heatmap effect works

The effect is implemented in `static/effects/heatmap.js` and registered as
the `heatmap` effect.  It rebuilds the `L.heatLayer` point array on every
WebSocket event:

```js
MapWatch.registerEffect('heatmap', function (event, map, markerMap) {
  const points = Object.values(markerMap).map(({ data }) => {
    const intensity = data.severity === 'critical' ? 1.0
                    : data.severity === 'warning'  ? 0.6
                    : 0.3;
    return [lat, lng, intensity];
  });
  heatLayer.setLatLngs(points);
});
```

The overlay uses `leaflet.heat` (loaded from CDN in `index.html`) with
`radius: 25`, `blur: 15`, `maxZoom: 10`.

---

## Inject additional heat points via curl

```bash
# Add a critical marker in Mumbai
curl -s -XPOST http://localhost:8082/api/markers \
  -H 'Content-Type: application/json' \
  -d '{
    "id":        "in-mumbai-1",
    "lat":       19.08,
    "lng":       72.88,
    "severity":  "critical",
    "alertname": "MumbaiCPU",
    "labels":    { "instance": "in-prod-1" },
    "annotations": { "summary": "Mumbai node overloaded" }
  }'
```

The heatmap updates in real time — no page reload required.

---

## Stop the stack

```bash
docker compose down
```
