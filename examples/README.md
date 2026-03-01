# MapWatch — Examples

Self-contained Docker Compose stacks that each highlight specific MapWatch features.
Every example is independent: `cd` into the folder and run `docker compose up -d`.

| Example | Port | What it shows |
|---------|------|---------------|
| [blink-dot](blink-dot/) | 8081 | 5 green DC dots; 2 turn red — one with 1 alert, one with 2 alerts (badge) |
| [heatmap](heatmap/)     | 8082 | 4 green DC dots + heat density overlay with Singapore CBD hotspot |

---

## blink-dot

**The DC baseline story — watch healthy infrastructure go critical.**

Five **green dots** appear on the Singapore map on page load (one per known datacenter).
Within ~15 seconds, Prometheus fires three alerts across two DCs:
- **sg-dc-1 (CBD)** — 1 critical alert → dot turns red and blinks, no badge
- **sg-dc-2 (West)** — 2 critical alerts → dot turns red with badge **②**
- **sg-dc-3/4/5** — no alerts → stay green

```bash
cd blink-dot
docker compose up -d
# open http://localhost:8081
```

| Stage | Dot     | Location         | Colour | Behaviour                   |
|-------|---------|------------------|--------|-----------------------------|
| Load  | sg-dc-1 | Singapore CBD    | Green  | Healthy baseline            |
| Load  | sg-dc-2 | Singapore West   | Green  | Healthy baseline            |
| Load  | sg-dc-3 | Singapore North  | Green  | Healthy baseline            |
| Load  | sg-dc-4 | Singapore East   | Green  | Healthy baseline            |
| Load  | sg-dc-5 | Singapore Central| Green  | Healthy baseline            |
| ~15s  | sg-dc-1 | Singapore CBD    | Red    | **Blinking** — 1 alert      |
| ~15s  | sg-dc-2 | Singapore West   | Red    | **Blinking** — 2 alerts ②   |

Click **sg-dc-2** to open the aggregated panel showing severity bar + alert list.

See [blink-dot/README.md](blink-dot/README.md) for the full walkthrough.

---

## heatmap

**Seven Singapore alerts — visualised as a heat density layer.**

Four DC baseline dots appear on load (CBD, East, North, West).
Seven alerts fire across Singapore with a dense critical cluster in the CBD,
creating a visible hotspot gradient when the **Heatmap** toolbar button is clicked.

```bash
cd heatmap
docker compose up -d
# open http://localhost:8082  →  click the "Heatmap" button in the toolbar
```

| Area     | Severity | Count | Effect in heatmap          |
|----------|----------|-------|----------------------------|
| CBD      | critical | 3     | Red hotspot                |
| Central  | warning  | 2     | Orange/warm area           |
| West     | info     | 2     | Cool blue fringe           |

Severity drives heat intensity: `critical=1.0`, `warning=0.6`, `info=0.3`.

See [heatmap/README.md](heatmap/README.md) for the full walkthrough.

---

## Running both at once

Each example uses different host ports so you can run them simultaneously:

```bash
# Terminal 1
cd blink-dot && docker compose up

# Terminal 2
cd heatmap && docker compose up
```

MapWatch (blink-dot) → http://localhost:8081
MapWatch (heatmap)   → http://localhost:8082

---

## Adding your own example

1. Copy an existing folder as a starting point.
2. Edit `alerts.yml` with your alert rules and geohash or datacenter labels.
3. Update `mapwatch.yaml` → `locations` for your known DC locations (baseline dots).
4. Write a short `README.md` describing what the example demonstrates.
5. Open a PR — contributions welcome!
