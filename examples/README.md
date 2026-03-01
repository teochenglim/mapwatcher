# MapWatch — Examples

Self-contained Docker Compose stacks that each highlight specific MapWatch features.
Every example is independent: `cd` into the folder and run `docker compose up -d`.

| Example | Port | What it shows |
|---------|------|---------------|
| [blink-dot](blink-dot/) | 8080 | 5 green DC dots; 2 turn red — one with 1 alert, one with 2 alerts (badge) |

---

## blink-dot

**The DC baseline story — watch healthy infrastructure go critical.**

Five **green dots** appear on the Singapore map on page load (one per known datacenter).
Within ~15 seconds, Prometheus fires alerts across two DCs:
- **sg-dc-1 (CBD)** — 1 critical alert → dot turns red and blinks, no badge
- **sg-dc-2 (West)** — 2 critical alerts → dot turns red with badge **②**
- **sg-dc-3/4/5** — no alerts → stay green

```bash
cd blink-dot
docker compose up -d
# open http://localhost:8080
```

| Stage | Dot     | Location          | Colour | Behaviour                   |
|-------|---------|-------------------|--------|-----------------------------|
| Load  | sg-dc-1 | Singapore CBD     | Green  | Healthy baseline            |
| Load  | sg-dc-2 | Singapore West    | Green  | Healthy baseline            |
| Load  | sg-dc-3 | Singapore North   | Green  | Healthy baseline            |
| Load  | sg-dc-4 | Singapore East    | Green  | Healthy baseline            |
| Load  | sg-dc-5 | Singapore Central | Green  | Healthy baseline            |
| ~15s  | sg-dc-1 | Singapore CBD     | Red    | **Blinking** — 1 alert      |
| ~15s  | sg-dc-2 | Singapore West    | Red    | **Blinking** — 2 alerts ②   |

Click **sg-dc-2** to open the aggregated panel showing severity bar + alert list.

See [blink-dot/README.md](blink-dot/README.md) for the full walkthrough.

---

## Adding your own example

1. Copy `blink-dot/` as a starting point.
2. Edit `alerts.yml` with your alert rules and geohash or datacenter labels.
3. Update `mapwatch.yaml` → `locations` for your known DC locations (baseline dots).
4. Write a short `README.md` describing what the example demonstrates.
5. Open a PR — contributions welcome!
