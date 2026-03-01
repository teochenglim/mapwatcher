# Example: Blink-dot

Demonstrates the **CSS pulse animation** that fires on `severity=critical` markers.

Three critical alerts blink across Singapore, New York, and Dublin.
A fourth warning alert in Los Angeles renders as a solid yellow dot — no blink.

```
  Prometheus ──fires──► Alertmanager ──webhook──► MapWatch ──WebSocket──► Browser
    vector(1)                                       :8081                  (map)
```

---

## Start the stack

```bash
cd examples/blink-dot
docker compose up -d
```

| Service      | URL                     |
|--------------|-------------------------|
| MapWatch     | http://localhost:8081   |
| Prometheus   | http://localhost:9091   |
| Alertmanager | http://localhost:9094   |

Open **http://localhost:8081**.  Within 15–30 seconds you will see four dots appear:

| Alert        | Location         | Colour | Animation          |
|--------------|------------------|--------|--------------------|
| HighCPU      | Singapore CBD    | Red    | **Blinking/pulse** |
| MemoryLeak   | New York         | Red    | **Blinking/pulse** |
| ServiceDown  | Dublin / EU West | Red    | **Blinking/pulse** |
| DiskWarning  | Los Angeles      | Yellow | Solid (no blink)   |

---

## How the blink animation works

MapWatch applies the `mw-pulse` CSS class to any marker where:
- `severity == "critical"`, **or**
- `labels.priority == "P1"`

The animation is pure CSS — no JS timers or polling. It is implemented in
`static/effects/blink.js` and registered as the `blink-critical` effect:

```js
MapWatch.registerEffect('blink-critical', function (event, map, markerMap) {
  const isPulsing = m.severity === 'critical' || (m.labels && m.labels.priority === 'P1');
  dot.classList.toggle('mw-pulse', isPulsing);
});
```

---

## Send a custom blinking marker via curl

You can inject markers directly without Prometheus:

```bash
# Blinking marker at Tokyo
curl -s -XPOST http://localhost:8081/api/markers \
  -H 'Content-Type: application/json' \
  -d '{
    "id":        "manual-tokyo-1",
    "lat":       35.68,
    "lng":       139.69,
    "severity":  "critical",
    "alertname": "ManualTest",
    "labels":    { "instance": "jp-prod-1", "datacenter": "ap-northeast-1" },
    "annotations": { "summary": "Manually injected blinking dot" }
  }'
```

Remove it:

```bash
curl -s -XDELETE http://localhost:8081/api/markers/manual-tokyo-1
```

---

## Stop the stack

```bash
docker compose down
```
