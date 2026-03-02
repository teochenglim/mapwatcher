package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/marker"
	"github.com/teochenglim/mapwatch/internal/server"
	"github.com/teochenglim/mapwatch/internal/transformer"
)

// newTestServer spins up a full mapwatch HTTP server via httptest.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{Addr: ":0"},
		Prometheus: config.PrometheusConfig{
			URL:         "http://prometheus:9090",
			ExternalURL: "http://localhost:9090",
			Timeout:     5 * time.Second,
		},
		Spread:    config.SpreadConfig{Radius: 0.01},
		Locations: map[string]string{"sg-dc-1": "w21zd3"},
		QueryTemplates: map[string][]config.QueryTemplate{
			"HighCPU": {{Label: "CPU %", Query: `rate(node_cpu_seconds_total{instance="{{.instance}}"}[5m])`}},
			"default": {{Label: "Up", Query: `up`}},
		},
	}
	srv := server.New(cfg, nil, "")
	return httptest.NewServer(srv.Handler)
}

func get(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func postJSON(t *testing.T, ts *httptest.Server, path string, body interface{}) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// ── GET /api/config ───────────────────────────────────────────────────────────

func TestAPIGetConfig(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := get(t, ts, "/api/config")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var cfg struct {
		PrometheusURL  string `json:"prometheusUrl"`
		Locations      []struct {
			Name string  `json:"name"`
			Lat  float64 `json:"lat"`
			Lng  float64 `json:"lng"`
		} `json:"locations"`
		HeatmapRegions []struct {
			Name            string     `json:"name"`
			Center          [2]float64 `json:"center"`
			GeohashPrefixes []string   `json:"geohash_prefixes"`
		} `json:"heatmapRegions"`
	}
	decode(t, resp, &cfg)
	if cfg.PrometheusURL != "http://localhost:9090" {
		t.Errorf("unexpected prometheusUrl: %q", cfg.PrometheusURL)
	}
	if len(cfg.Locations) != 1 {
		t.Errorf("expected 1 location, got %d", len(cfg.Locations))
	}
	// No heatmap regions in the test config — field must be present but empty.
	if cfg.HeatmapRegions == nil {
		t.Error("heatmapRegions field missing from /api/config response")
	}
}

// ── GET /api/markers ──────────────────────────────────────────────────────────

func TestAPIGetMarkersEmpty(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := get(t, ts, "/api/markers")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var markers []*marker.Marker
	decode(t, resp, &markers)
	if len(markers) != 0 {
		t.Fatalf("expected empty slice, got %d markers", len(markers))
	}
}

// ── POST /api/markers ─────────────────────────────────────────────────────────

func TestAPIPostMarkersGeohash(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]interface{}{
		"id": "test-1", "geohash": "w21zd3",
		"severity": "critical", "alertname": "HighCPU",
	}
	resp := postJSON(t, ts, "/api/markers", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var m marker.Marker
	decode(t, resp, &m)
	if m.ID != "test-1" {
		t.Errorf("expected id=test-1, got %q", m.ID)
	}
	if m.Lat == 0 || m.Lng == 0 {
		t.Errorf("geohash not decoded: lat=%v lng=%v", m.Lat, m.Lng)
	}
	if m.GeoBounds == nil {
		t.Error("expected GeoBounds after geohash decode")
	}
}

func TestAPIPostMarkersMissingID(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := postJSON(t, ts, "/api/markers", map[string]string{"geohash": "w21zd3"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ── DELETE /api/markers/:id ───────────────────────────────────────────────────

func TestAPIDeleteMarker(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create first.
	postJSON(t, ts, "/api/markers", map[string]interface{}{
		"id": "del-me", "geohash": "w21zd3", "severity": "info",
	}).Body.Close()

	// Delete.
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/markers/del-me", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Confirm gone.
	r := get(t, ts, "/api/markers")
	var markers []*marker.Marker
	decode(t, r, &markers)
	if len(markers) != 0 {
		t.Fatalf("expected 0 markers after delete, got %d", len(markers))
	}
}

func TestAPIDeleteMarkerNotFound(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/markers/ghost", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ── GET /api/markers/:id/links ────────────────────────────────────────────────

func TestAPIGetMarkerLinks(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	postJSON(t, ts, "/api/markers", map[string]interface{}{
		"id": "cpu-1", "geohash": "w21zd3",
		"severity": "critical", "alertname": "HighCPU",
		"labels": map[string]string{"instance": "sg-prod-1"},
	}).Body.Close()

	resp := get(t, ts, "/api/markers/cpu-1/links")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var links []transformer.PromLink
	decode(t, resp, &links)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].Label != "CPU %" {
		t.Errorf("unexpected label: %q", links[0].Label)
	}
	if links[0].URL == "" {
		t.Error("expected non-empty URL")
	}
}

func TestAPIGetMarkerLinksNotFound(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := get(t, ts, "/api/markers/ghost/links")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ── POST /api/alerts (Alertmanager webhook) ───────────────────────────────────

func TestAPIPostAlertsFiring(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	payload := map[string]interface{}{
		"version": "4", "status": "firing",
		"groupLabels": map[string]string{"alertname": "HighCPU"},
		"alerts": []map[string]interface{}{{
			"status": "firing", "fingerprint": "abc123",
			"labels": map[string]string{
				"alertname": "HighCPU", "severity": "critical",
				"geohash": "w21zd3", "instance": "sg-prod-1",
			},
			"annotations": map[string]string{"summary": "High CPU"},
			"startsAt":    "2026-01-01T00:00:00Z",
			"endsAt":      "0001-01-01T00:00:00Z",
		}},
	}

	resp := postJSON(t, ts, "/api/alerts", payload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]int
	decode(t, resp, &result)
	if result["added"] != 1 {
		t.Errorf("expected added=1, got %v", result)
	}

	// Confirm marker appeared.
	r := get(t, ts, "/api/markers")
	var markers []*marker.Marker
	decode(t, r, &markers)
	if len(markers) != 1 || markers[0].ID != "abc123" {
		t.Fatalf("marker not found after PostAlerts: %v", markers)
	}
}

func TestAPIPostAlertsResolved(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Fire an alert.
	postJSON(t, ts, "/api/markers", map[string]interface{}{
		"id": "fp-res", "geohash": "w21zd3", "severity": "critical",
	}).Body.Close()

	// Send resolved (empty alerts list → reconcile removes all).
	resp := postJSON(t, ts, "/api/alerts", map[string]interface{}{
		"version": "4", "status": "resolved", "alerts": []interface{}{},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Round-trip: add → list → delete ──────────────────────────────────────────

func TestAPIRoundTrip(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	const n = 3
	for i := range n {
		postJSON(t, ts, "/api/markers", map[string]interface{}{
			"id": fmt.Sprintf("m%d", i), "geohash": "w21zd3", "severity": "info",
		}).Body.Close()
	}

	r := get(t, ts, "/api/markers")
	var markers []*marker.Marker
	decode(t, r, &markers)
	if len(markers) != n {
		t.Fatalf("expected %d markers, got %d", n, len(markers))
	}

	for i := range n {
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/markers/m%d", ts.URL, i), nil)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}

	r = get(t, ts, "/api/markers")
	decode(t, r, &markers)
	if len(markers) != 0 {
		t.Fatalf("expected 0 after deletes, got %d", len(markers))
	}
}
