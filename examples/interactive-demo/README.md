# MapWatch — Interactive Demo

Full pipeline: **mobile tap** → **Redpanda** → **RisingWave** → **exporter** → **Prometheus** → **Alertmanager** → **MapWatch dashboard**

Features enabled: sound, leaderboard, stats.

## Architecture

```
mobile.html  ──POST /api/tap──►  MapWatch
                                     │
                                     ▼
                               Redpanda (Kafka)
                                     │
                                     ▼
                               RisingWave (stream SQL)
                               ├── tap_alerts (MV)
                               └── leaderboard (MV)
                                     │
                                     ▼
                               exporter.py
                               ├── /metrics  ──►  Prometheus  ──►  Alertmanager  ──►  MapWatch WS
                               └── /leaderboard ──►  MapWatch /api/leaderboard
```

## Prerequisites

- Docker Desktop (or Docker Engine + Compose plugin)
- Ports free: `8080`, `8000`, `9090`, `9093`, `4566`, `5690`, `19092`

## Quick Start

```bash
cd examples/interactive-demo
docker compose up --build
```

First run takes ~2–3 minutes: Go binary builds, GeoJSON layers download, Python packages install, RisingWave initialises.

## Open in Browser

| URL | Purpose |
|-----|---------|
| http://localhost:8080 | MapWatch dashboard |
| http://localhost:8080/mobile.html | Mobile tap page (simulate phone) |
| http://localhost:8081 | Redpanda Console — browse `user_taps` topic messages |
| http://localhost:5691 | RisingWave — streaming graph + MV query |

## Sending Taps from a Real Phone

To reach the demo from a phone on a different network, expose it with a cloudflared tunnel:

```bash
# Install once (macOS)
brew install cloudflared

# Start tunnel — prints a public HTTPS URL, e.g. https://abc-xyz.trycloudflare.com
cloudflared tunnel --url http://localhost:8080
```

Share the printed URL with anyone. Open `<tunnel-url>/mobile.html` on the phone.

> The tunnel is temporary (closes when you Ctrl-C). No account required.

### Option A — Mobile browser

Open `http://localhost:8080/mobile.html` (local) or the cloudflared URL (phone) in a browser.

1. Enter a username — prompted fresh on every page load.
2. Pick an event type (🚗 Car Accident, 🔥 Building Fire, etc.).
3. Tap anywhere on the map — change event type at any time using the **▲ change** bar at the bottom.

Each tap posts to `/api/tap` → Redpanda → RisingWave in real time.

### Option B — Script

```bash
pip install kafka-python
python3 generate-test-taps.py
```

This publishes synthetic taps directly to Redpanda on `localhost:19092`.

## What You Should See

1. **Dashboard** — Within ~4 s of tapping, an emoji marker appears on the Singapore map with an animation matching the event type:

   | Severity | Event | Animation |
   |----------|-------|-----------|
   | critical | 🚗 Car Accident | Gyrocopter spin + shake |
   | high/warning | 🔥 Building Fire | Flicker |
   | medium | 🚧 Road Congestion | Bounce |
   | low | 🌳 Fallen Tree | Sway |
   | info | 💧 Flash Flood | Ripple ring |
   | test | 🚨 Suspicious Activity | Strobe flash |

2. **Sound** — A notification tone plays for each new marker (click 🔔 in toolbar to toggle).

3. **Leaderboard** — Left sidebar updates every 5 s with top tappers, tap count, and favourite colour.

4. **Stats** — Bottom-right overlay shows total active alerts and per-minute rate.

## Pipeline Timing

| Stage | Interval |
|-------|---------|
| RisingWave `tap_alerts` window | 10 s rolling |
| exporter poll | 2 s |
| Prometheus scrape | 2 s |
| Alertmanager `group_wait` | 2 s |
| End-to-end tap → marker | ~4–6 s |

## Stopping

```bash
docker compose down
```

Add `-v` to also remove Redpanda/RisingWave volumes:

```bash
docker compose down -v
```

## Troubleshooting

**Markers never appear**

Check exporter metrics:
```bash
curl http://localhost:8000/metrics | grep tap_alert
```
If empty, check the exporter bootstrap logs:
```bash
docker compose logs exporter | head -40
```
RisingWave DDL init retries automatically; on slow machines it can take up to 60 s.

**Port conflict**

Edit the `ports:` section in `docker-compose.yml` to change the host-side port.
