# Example: Blink-dot

Demonstrates **DC baseline markers** and **multi-alert aggregation**.

Five known datacenter locations appear as **green healthy dots** on page load.
When Prometheus fires, two DCs turn **red and blink** — one with a single alert,
one with two alerts (count badge). The other three DCs stay green.

```
Prometheus ──(vector(1) always fires)──► Alertmanager ──webhook──► MapWatch ──WebSocket──► Browser
                                                          :9093       :8080                  (map)
```

---

## Start the stack

```bash
cd examples/blink-dot

# Pull published image (end-user):
docker compose up -d

# Build from local source (developer):
docker compose up --build
```

> **No map downloads required.**
> This example uses a minimal `Dockerfile` that ships only the binary.
> The SG overlay buttons (Divisions, Roads, MRT, etc.) are automatically
> hidden in the toolbar because the GeoJSON files are not present.

| Service      | URL                     |
|--------------|-------------------------|
| MapWatch     | http://localhost:8080   |
| Prometheus   | http://localhost:9090   |
| Alertmanager | http://localhost:9093   |

Open **http://localhost:8080** and watch the map unfold in two stages:

**Stage 1 — on page load (immediate):**

| Dot     | Location          | Colour | Meaning    |
|---------|-------------------|--------|------------|
| sg-dc-1 | Singapore CBD     | Green  | DC healthy |
| sg-dc-2 | Singapore West    | Green  | DC healthy |
| sg-dc-3 | Singapore North   | Green  | DC healthy |
| sg-dc-4 | Singapore East    | Green  | DC healthy |
| sg-dc-5 | Singapore Central | Green  | DC healthy |

**Stage 2 — after ~15 seconds:**

| Dot     | Location       | Colour | Behaviour                          |
|---------|----------------|--------|------------------------------------|
| sg-dc-1 | Singapore CBD  | Red    | **Blinking** — 1 alert             |
| sg-dc-2 | Singapore West | Red    | **Blinking** — 2 alerts, badge ②   |
| sg-dc-3 | North          | Green  | Still healthy                      |
| sg-dc-4 | East           | Green  | Still healthy                      |
| sg-dc-5 | Central        | Green  | Still healthy                      |

---

## What to try

**Hover** any dot to see a tooltip:
- Green dot: shows "HEALTHY"
- Red dot with 1 alert: shows the alert name + instance
- Red dot with 2 alerts: shows both alert names + "Click to view all ↗"

**Click sg-dc-1** (CBD, 1 alert) → individual alert detail panel with Prometheus links.

**Click sg-dc-2** (West, 2 alerts) → aggregated DC panel:
- Severity bar (proportional red segments)
- Count chip: `2 critical`
- Alert list sorted by severity — click any row to drill into that alert's details

---

## How DC baseline markers work

On page load, `mapwatch.js` fetches `/api/config` which returns the `locations` table
decoded to lat/lng.  Each location becomes a small **green Leaflet marker** added
directly to the map (separate layer — never clustered).

When an alert arrives with `datacenter: sg-dc-2`, `mapwatch.js` aggregates it onto
the `sg-dc-2` DC marker instead of creating an individual dot.  The DC marker
updates its colour, size, CSS animation, and count badge automatically.

```js
// The DC matching logic in mapwatch.js:
function getDCForAlert(data) {
  if (data.labels && data.labels.datacenter) {
    const name = data.labels.datacenter;
    if (dcMarkers[name]) return name;  // aggregate onto DC dot
  }
  return null;  // render as individual marker
}
```

The CSS pulse animation on the DC dot:

```css
@keyframes mw-pulse {
  0%   { box-shadow: 0 0 0 0    rgba(248,81,73,.85); }
  70%  { box-shadow: 0 0 0 16px rgba(248,81,73,0);   }
  100% { box-shadow: 0 0 0 0    rgba(248,81,73,0);   }
}
.mw-pulse { animation: mw-pulse 1.4s ease-out infinite; }
```

---

## Inject your own alerts via curl (no Prometheus needed)

Add a third alert to sg-dc-2 — the badge will update to ③:

```bash
curl -s -XPOST http://localhost:8080/api/markers \
  -H 'Content-Type: application/json' \
  -d '{
    "id":      "manual-west-3",
    "geohash": "w21z8k",
    "severity": "critical",
    "labels":  { "instance": "sg-manual-3", "datacenter": "sg-dc-2" },
    "annotations": { "summary": "Third alert — badge becomes 3" }
  }'
```

Add a warning to a healthy DC (sg-dc-3) — the dot turns yellow:

```bash
curl -s -XPOST http://localhost:8080/api/markers \
  -H 'Content-Type: application/json' \
  -d '{
    "id":      "manual-north-1",
    "geohash": "w21zb",
    "severity": "warning",
    "labels":  { "instance": "sg-manual-n", "datacenter": "sg-dc-3" },
    "annotations": { "summary": "Warning — North DC dot turns yellow" }
  }'
```

Remove them:

```bash
curl -s -XDELETE http://localhost:8080/api/markers/manual-west-3
curl -s -XDELETE http://localhost:8080/api/markers/manual-north-1
```

---

## Stop the stack

```bash
docker compose down
```
