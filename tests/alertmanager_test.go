package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/teochenglim/mapwatch/internal/transformer"
)

type amTestAlert struct {
	Status      string            `json:"status"`
	Fingerprint string            `json:"fingerprint"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

type amTestPayload struct {
	Version string        `json:"version"`
	Status  string        `json:"status"`
	Alerts  []amTestAlert `json:"alerts"`
}

func buildPayload(alerts ...amTestAlert) []byte {
	b, _ := json.Marshal(amTestPayload{Version: "4", Status: "firing", Alerts: alerts})
	return b
}

func alert(fp string, labels map[string]string) amTestAlert {
	return amTestAlert{
		Status: "firing", Fingerprint: fp, Labels: labels,
		Annotations: map[string]string{"summary": "test"},
		StartsAt:    time.Now(),
	}
}

func TestAlertmanagerGeohashLabel(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(nil)
	markers, err := tr.Transform(buildPayload(alert("fp1", map[string]string{
		"alertname": "HighCPU", "severity": "critical", "geohash": "w21zd3",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}
	m := markers[0]
	if m.Geohash != "w21zd3" {
		t.Errorf("expected geohash=w21zd3, got %q", m.Geohash)
	}
	if m.Lat == 0 || m.Lng == 0 {
		t.Errorf("lat/lng not decoded from geohash: %v, %v", m.Lat, m.Lng)
	}
	if m.GeoBounds == nil {
		t.Error("expected non-nil GeoBounds")
	}
	if m.AlertName != "HighCPU" {
		t.Errorf("expected alertname=HighCPU, got %q", m.AlertName)
	}
	if m.Severity != "critical" {
		t.Errorf("expected severity=critical, got %q", m.Severity)
	}
}

func TestAlertmanagerLatLngLabels(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(nil)
	markers, err := tr.Transform(buildPayload(alert("fp2", map[string]string{
		"alertname": "DiskFull", "severity": "warning",
		"lat": "1.3521", "lng": "103.8198",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}
	m := markers[0]
	if m.Lat < 1.35 || m.Lat > 1.36 {
		t.Errorf("unexpected lat %v", m.Lat)
	}
}

func TestAlertmanagerDatacenterLookup(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(map[string]string{"sg-dc-1": "w21zd3"})
	markers, err := tr.Transform(buildPayload(alert("fp3", map[string]string{
		"alertname": "Net", "datacenter": "sg-dc-1",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}
	if markers[0].Geohash != "w21zd3" {
		t.Errorf("expected geohash from datacenter lookup, got %q", markers[0].Geohash)
	}
}

func TestAlertmanagerNoGeoSkipped(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(nil)
	markers, err := tr.Transform(buildPayload(alert("fp4", map[string]string{
		"alertname": "Mystery",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 0 {
		t.Fatalf("expected 0 markers, got %d", len(markers))
	}
}

func TestAlertmanagerGeohashPriorityOverLatLng(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(nil)
	markers, err := tr.Transform(buildPayload(alert("fp5", map[string]string{
		"geohash": "w21zd3", "lat": "0.0", "lng": "0.0",
	})))
	if err != nil {
		t.Fatal(err)
	}
	if markers[0].Geohash != "w21zd3" {
		t.Errorf("geohash should take priority, got %q", markers[0].Geohash)
	}
}

func TestAlertmanagerMixedBatch(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(nil)
	markers, err := tr.Transform(buildPayload(
		alert("ok1", map[string]string{"geohash": "w21zd3"}),
		alert("ok2", map[string]string{"geohash": "w21z8k"}),
		alert("bad", map[string]string{}), // no geo → skipped
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 2 {
		t.Fatalf("expected 2 markers (1 skipped), got %d", len(markers))
	}
}

func TestAlertmanagerInvalidJSON(t *testing.T) {
	tr := transformer.NewAlertmanagerTransformer(nil)
	if _, err := tr.Transform([]byte(`not json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
