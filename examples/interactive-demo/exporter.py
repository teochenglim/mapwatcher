#!/usr/bin/env python3
"""
RisingWave → Prometheus exporter + /leaderboard API.

On startup, bootstraps the pipeline:
  1. Creates Redpanda topics (user_taps, tap_alerts) if absent.
  2. Creates RisingWave SOURCE + MATERIALIZED VIEWs if absent.
  3. Starts poll loop + HTTP server.

Environment variables:
  RW_HOST          RisingWave host           (default: risingwave)
  RW_PORT          RisingWave port           (default: 4566)
  RW_DB            database name             (default: dev)
  RW_USER          username                  (default: root)
  REDPANDA_BROKERS Kafka bootstrap servers   (default: redpanda:29092)
  EXPORTER_PORT    HTTP port                 (default: 8000)
  POLL_INTERVAL    seconds between polls     (default: 2)
"""

import json
import logging
import os
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

import psycopg2
import psycopg2.extras

logging.basicConfig(level=logging.INFO, format='%(asctime)s %(levelname)s %(message)s')
log = logging.getLogger(__name__)

# ── Config ────────────────────────────────────────────────────────────────────

RW_HOST          = os.getenv('RW_HOST', 'risingwave')
RW_PORT          = int(os.getenv('RW_PORT', '4566'))
RW_DB            = os.getenv('RW_DB', 'dev')
RW_USER          = os.getenv('RW_USER', 'root')
REDPANDA_BROKERS = os.getenv('REDPANDA_BROKERS', 'redpanda:29092')
POLL_INTERVAL    = float(os.getenv('POLL_INTERVAL', '2'))
HTTP_PORT        = int(os.getenv('EXPORTER_PORT', '8000'))

# ── Init SQL ──────────────────────────────────────────────────────────────────

INIT_SQL = """
CREATE SOURCE IF NOT EXISTS user_taps (
  username   VARCHAR,
  color      VARCHAR,
  severity   VARCHAR,
  lat        DOUBLE PRECISION,
  lng        DOUBLE PRECISION,
  session_id VARCHAR,
  timestamp  TIMESTAMPTZ
) WITH (
  connector = 'kafka',
  topic = 'user_taps',
  properties.bootstrap.server = '{brokers}',
  scan.startup.mode = 'latest'
) FORMAT PLAIN ENCODE JSON;

CREATE MATERIALIZED VIEW IF NOT EXISTS tap_alerts AS
SELECT
  ROUND(CAST(lat AS NUMERIC), 3) AS lat_bin,
  ROUND(CAST(lng AS NUMERIC), 3) AS lng_bin,
  AVG(lat) AS center_lat,
  AVG(lng) AS center_lng,
  '' AS geohash,
  COUNT(*) AS tap_count,
  COUNT(DISTINCT username) AS unique_tappers,
  string_agg(DISTINCT username, ',') AS usernames,
  min(color) AS dominant_color,
  CASE
    WHEN COUNT(*) >= 5 THEN 'critical'
    WHEN COUNT(*) >= 3 THEN 'warning'
    ELSE 'info'
  END AS severity
FROM user_taps
WHERE "timestamp" > NOW() - INTERVAL '10 seconds'
GROUP BY ROUND(CAST(lat AS NUMERIC), 3), ROUND(CAST(lng AS NUMERIC), 3);

CREATE MATERIALIZED VIEW IF NOT EXISTS leaderboard AS
SELECT
  username,
  COUNT(*) AS tap_count,
  COUNT(DISTINCT CONCAT(ROUND(CAST(lat AS NUMERIC), 3)::text, ',', ROUND(CAST(lng AS NUMERIC), 3)::text)) AS unique_locations,
  min(color) AS favorite_color,
  MAX("timestamp") AS last_tap
FROM user_taps
WHERE "timestamp" > NOW() - INTERVAL '5 minutes'
GROUP BY username;
"""

# ── Bootstrap ─────────────────────────────────────────────────────────────────

def retry(fn, label, attempts=20, delay=3):
    for i in range(attempts):
        try:
            fn()
            log.info('%s: OK', label)
            return
        except Exception as exc:
            log.warning('%s attempt %d/%d failed: %s', label, i + 1, attempts, exc)
            time.sleep(delay)
    raise RuntimeError(f'{label} failed after {attempts} attempts')


def init_redpanda():
    from kafka import KafkaAdminClient
    from kafka.admin import NewTopic
    from kafka.errors import TopicAlreadyExistsError

    def _create():
        admin = KafkaAdminClient(bootstrap_servers=REDPANDA_BROKERS,
                                 request_timeout_ms=5000)
        existing = set(admin.list_topics())
        new = [NewTopic(t, num_partitions=1, replication_factor=1)
               for t in ('user_taps', 'tap_alerts')
               if t not in existing]
        if new:
            admin.create_topics(new)
            log.info('Created topics: %s', [t.name for t in new])
        admin.close()

    retry(_create, 'Redpanda topic init')


def init_risingwave():
    def _ddl():
        conn = psycopg2.connect(
            host=RW_HOST, port=RW_PORT, dbname=RW_DB,
            user=RW_USER, password='', connect_timeout=5,
        )
        conn.autocommit = True
        with conn.cursor() as cur:
            # Execute each statement individually (RisingWave doesn't support
            # multi-statement strings in a single execute call).
            for stmt in INIT_SQL.format(brokers=REDPANDA_BROKERS).split(';'):
                stmt = stmt.strip()
                if stmt:
                    cur.execute(stmt)
        conn.close()

    retry(_ddl, 'RisingWave DDL init')


# ── Shared state (updated by background poll thread) ─────────────────────────

_lock        = threading.Lock()
_tap_rows    = []
_leader_rows = []
_cleared_at  = None   # ISO timestamp; rows with last_tap <= this are hidden


def connect():
    return psycopg2.connect(
        host=RW_HOST, port=RW_PORT, dbname=RW_DB,
        user=RW_USER, password='', connect_timeout=5,
    )


def poll_loop():
    """Background thread: query RisingWave every POLL_INTERVAL seconds."""
    conn = None
    while True:
        try:
            if conn is None or conn.closed:
                log.info('Connecting to RisingWave %s:%d/%s …', RW_HOST, RW_PORT, RW_DB)
                conn = connect()
                log.info('Connected.')

            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute("""
                    SELECT geohash, tap_count, unique_tappers, usernames,
                           dominant_color, severity, center_lat, center_lng
                    FROM tap_alerts
                    ORDER BY tap_count DESC
                """)
                taps = [dict(r) for r in cur.fetchall()]

                cur.execute("""
                    SELECT username, tap_count, unique_locations, favorite_color, last_tap
                    FROM leaderboard
                    ORDER BY tap_count DESC
                    LIMIT 20
                """)
                leaders = [dict(r) for r in cur.fetchall()]

            with _lock:
                _tap_rows[:]    = taps
                _leader_rows[:] = leaders

        except Exception as exc:
            log.warning('Poll error: %s', exc)
            if conn and not conn.closed:
                try:
                    conn.close()
                except Exception:
                    pass
            conn = None

        time.sleep(POLL_INTERVAL)


# ── Prometheus metric builder ─────────────────────────────────────────────────

def build_metrics() -> str:
    lines = [
        '# HELP tap_alert_count Number of taps in the active window for a geohash cluster',
        '# TYPE tap_alert_count gauge',
    ]

    with _lock:
        taps    = list(_tap_rows)
        leaders = list(_leader_rows)

    for row in taps:
        labels = (
            f'geohash="{row.get("geohash","")}"'
            f',severity="{row.get("severity","info")}"'
            f',dominant_color="{row.get("dominant_color","#4444FF")}"'
            f',usernames="{row.get("usernames","")}"'
        )
        lines.append(f'tap_alert_count{{{labels}}} {row.get("tap_count", 0)}')

    lines += [
        '',
        '# HELP leaderboard_position Rank of a tapper in the 5-minute leaderboard',
        '# TYPE leaderboard_position gauge',
    ]
    for i, row in enumerate(leaders):
        labels = (
            f'username="{row.get("username","")}"'
            f',tap_count="{row.get("tap_count",0)}"'
            f',favorite_color="{row.get("favorite_color","#4444FF")}"'
        )
        lines.append(f'leaderboard_position{{{labels}}} {i + 1}')

    return '\n'.join(lines) + '\n'


# ── HTTP handler ──────────────────────────────────────────────────────────────

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_GET(self):  # noqa: N802
        if self.path == '/metrics':
            body = build_metrics().encode()
            self.send_response(200)
            self.send_header('Content-Type', 'text/plain; version=0.0.4')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        elif self.path == '/leaderboard':
            with _lock:
                data = list(_leader_rows)
                cutoff = _cleared_at
            safe = []
            for row in data:
                row_safe = {k: (str(v) if not isinstance(v, (int, float, str, type(None))) else v)
                            for k, v in row.items()}
                if cutoff and row_safe.get('last_tap', '') <= cutoff:
                    continue
                safe.append(row_safe)
            body = json.dumps(safe).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        elif self.path == '/health':
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'ok')

    def do_POST(self):  # noqa: N802
        if self.path == '/leaderboard/clear':
            global _cleared_at
            from datetime import datetime, timezone
            with _lock:
                _cleared_at = datetime.now(timezone.utc).isoformat()
            log.info('Leaderboard cleared at %s', _cleared_at)
            body = json.dumps({'cleared_at': _cleared_at}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()


# ── Entry point ───────────────────────────────────────────────────────────────

if __name__ == '__main__':
    log.info('=== Bootstrap: Redpanda topics ===')
    init_redpanda()

    log.info('=== Bootstrap: RisingWave DDL ===')
    init_risingwave()

    log.info('=== Starting poll + HTTP server ===')
    t = threading.Thread(target=poll_loop, daemon=True)
    t.start()

    server = HTTPServer(('0.0.0.0', HTTP_PORT), Handler)

    print(flush=True, end='')  # flush any buffered output before banner
    print("""
╔══════════════════════════════════════════════════════════╗
║           🗺️  MapWatch Interactive Demo Ready            ║
╠══════════════════════════════════════════════════════════╣
║  MapWatch dashboard  →  http://localhost:8080            ║
║  Mobile tap page     →  http://localhost:8080/mobile.html║
╠══════════════════════════════════════════════════════════╣
║  Redpanda Console    →  http://localhost:8081            ║
║  RisingWave UI       →  http://localhost:5691            ║
╚══════════════════════════════════════════════════════════╝
""", flush=True)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
