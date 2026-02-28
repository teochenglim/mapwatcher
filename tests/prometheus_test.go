package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/marker"
	"github.com/teochenglim/mapwatch/internal/transformer"
)

func promProxy(tmpls map[string][]config.QueryTemplate) *transformer.PromProxy {
	return transformer.NewPromProxy(
		&config.PrometheusConfig{
			URL:         "http://prometheus:9090",
			ExternalURL: "http://localhost:9090",
			Timeout:     5 * time.Second,
		},
		tmpls,
	)
}

func markerWithAlert(alertname string, labels map[string]string) *marker.Marker {
	return &marker.Marker{ID: "t", AlertName: alertname, Labels: labels}
}

func TestRenderLinksSubstitution(t *testing.T) {
	p := promProxy(map[string][]config.QueryTemplate{
		"HighCPU": {{
			Label: "CPU %",
			Query: `rate(node_cpu_seconds_total{instance="{{.instance}}"}[5m])`,
		}},
	})
	m := markerWithAlert("HighCPU", map[string]string{"instance": "sg-prod-1"})

	links, err := p.RenderLinks(m, "http://localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if !strings.Contains(links[0].Query, "sg-prod-1") {
		t.Errorf("instance not substituted in query: %q", links[0].Query)
	}
	if !strings.HasPrefix(links[0].URL, "http://localhost:9090/graph?") {
		t.Errorf("unexpected URL: %q", links[0].URL)
	}
	if !strings.Contains(links[0].URL, "g0.expr=") {
		t.Errorf("URL missing g0.expr: %q", links[0].URL)
	}
}

func TestRenderLinksFallbackToDefault(t *testing.T) {
	p := promProxy(map[string][]config.QueryTemplate{
		"default": {{Label: "Up", Query: `up{instance="{{.instance}}"}`}},
	})
	links, err := p.RenderLinks(markerWithAlert("Unknown", map[string]string{"instance": "x"}), "http://localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Label != "Up" {
		t.Fatalf("expected 1 default link, got %v", links)
	}
}

func TestRenderLinksNoTemplates(t *testing.T) {
	p := promProxy(map[string][]config.QueryTemplate{})
	links, err := p.RenderLinks(markerWithAlert("X", nil), "http://localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(links))
	}
}

func TestRenderLinksMultiple(t *testing.T) {
	p := promProxy(map[string][]config.QueryTemplate{
		"HighCPU": {
			{Label: "CPU %", Query: `rate(node_cpu_seconds_total{instance="{{.instance}}"}[5m])`},
			{Label: "Load 1m", Query: `node_load1{instance="{{.instance}}"}`},
		},
	})
	links, err := p.RenderLinks(markerWithAlert("HighCPU", map[string]string{"instance": "x"}), "http://localhost:9090")
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
}

func TestRenderLinksExternalURL(t *testing.T) {
	p := promProxy(map[string][]config.QueryTemplate{
		"X": {{Label: "M", Query: `up`}},
	})
	links, err := p.RenderLinks(markerWithAlert("X", nil), "https://prom.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(links[0].URL, "https://prom.example.com/graph?") {
		t.Errorf("expected custom external URL, got: %q", links[0].URL)
	}
}
