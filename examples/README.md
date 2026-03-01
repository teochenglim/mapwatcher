# MapWatch — Examples

Self-contained Docker Compose stacks that each highlight a specific MapWatch feature.
Every example is independent: `cd` into the folder and run `docker compose up -d`.

| Example | Port | What it shows |
|---------|------|---------------|
| [blink-dot](blink-dot/) | 8081 | CSS pulse animation on `severity=critical` markers |
| [heatmap](heatmap/)     | 8082 | Leaflet.heat density overlay across 13 global alerts |

---

## blink-dot

**Four global alerts — three blink, one does not.**

```bash
cd blink-dot
docker compose up -d
# open http://localhost:8081
```

| Alert       | Location      | Colour | Animation          |
|-------------|---------------|--------|--------------------|
| HighCPU     | Singapore CBD | Red    | **Blinking/pulse** |
| MemoryLeak  | New York      | Red    | **Blinking/pulse** |
| ServiceDown | Dublin        | Red    | **Blinking/pulse** |
| DiskWarning | Los Angeles   | Yellow | Solid (no blink)   |

Any marker with `severity=critical` (or `labels.priority=P1`) automatically
gets the `mw-pulse` CSS class — no extra config needed.

See [blink-dot/README.md](blink-dot/README.md) for the full walkthrough.

---

## heatmap

**Thirteen alerts across APAC, US, and EU — visualised as a heat density layer.**

```bash
cd heatmap
docker compose up -d
# open http://localhost:8082
# then enable the overlay: MapWatch.toggleHeatmap()
```

Singapore and US East Coast are intentionally dense so you can see clear
hot-spots on the heatmap.  Severity drives heat intensity:
`critical=1.0`, `warning=0.6`, `info=0.3`.

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
2. Edit `alerts.yml` with your alert rules and geohash labels.
3. Update `mapwatch.yaml` → `locations` if you use datacenter labels.
4. Write a short `README.md` describing what the example demonstrates.
5. Open a PR — contributions welcome!
