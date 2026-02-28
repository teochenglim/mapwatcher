package transformer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"text/template"
	"time"

	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/marker"
)

// SeriesResult is the normalized time-series payload returned to the frontend for uPlot.
type SeriesResult struct {
	Label      string    `json:"label"`
	Timestamps []int64   `json:"timestamps"`
	Values     []float64 `json:"values"`
}

// PromLink is a rendered Prometheus expression-browser link for a marker.
type PromLink struct {
	Label string `json:"label"`
	Query string `json:"query"`
	URL   string `json:"url"`
}

// PromProxy handles Prometheus query_range proxying for marker detail panels.
type PromProxy struct {
	cfg    *config.PrometheusConfig
	tmpls  map[string][]config.QueryTemplate
	client *http.Client
}

// NewPromProxy creates a PromProxy from config.
func NewPromProxy(cfg *config.PrometheusConfig, tmpls map[string][]config.QueryTemplate) *PromProxy {
	return &PromProxy{
		cfg:   cfg,
		tmpls: tmpls,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// QueryMarkerDetails queries Prometheus for all relevant series for a marker
// over the given time range. Returns nil slice on Prometheus unavailability (non-fatal).
func (p *PromProxy) QueryMarkerDetails(ctx context.Context, m *marker.Marker, start, end time.Time) ([]*SeriesResult, error) {
	tmpls := p.resolveTemplates(m.AlertName)
	if len(tmpls) == 0 {
		return nil, nil
	}

	results := make([]*SeriesResult, 0, len(tmpls))
	for _, t := range tmpls {
		query, err := renderQuery(t.Query, m.Labels)
		if err != nil {
			return nil, fmt.Errorf("rendering query template %q: %w", t.Label, err)
		}

		series, err := p.queryRange(ctx, query, start, end)
		if err != nil {
			// Prometheus unavailable — return graceful error.
			return nil, &PrometheusUnavailableError{Err: err}
		}
		series.Label = t.Label
		results = append(results, series)
	}
	return results, nil
}

// resolveTemplates returns the query templates for an alertname, falling back to "default".
func (p *PromProxy) resolveTemplates(alertname string) []config.QueryTemplate {
	if ts, ok := p.tmpls[alertname]; ok {
		return ts
	}
	return p.tmpls["default"]
}

// renderQuery renders a PromQL template string with the given label map.
func renderQuery(tmpl string, labels map[string]string) (string, error) {
	t, err := template.New("q").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, labels); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// promRangeResponse is the Prometheus query_range JSON response structure.
type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values []interface{}     `json:"values"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

func (p *PromProxy) queryRange(ctx context.Context, query string, start, end time.Time) (*SeriesResult, error) {
	step := "15s"
	if d := end.Sub(start); d > 6*time.Hour {
		step = "60s"
	} else if d > time.Hour {
		step = "30s"
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", step)

	reqURL := fmt.Sprintf("%s/api/v1/query_range?%s", p.cfg.URL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pr promRangeResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("parsing prometheus response: %w", err)
	}
	if pr.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s", pr.Error)
	}

	result := &SeriesResult{}
	if len(pr.Data.Result) > 0 {
		for _, v := range pr.Data.Result[0].Values {
			pair, ok := v.([]interface{})
			if !ok || len(pair) != 2 {
				continue
			}
			ts, ok := pair[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := pair[1].(string)
			if !ok {
				continue
			}
			var val float64
			fmt.Sscanf(valStr, "%f", &val)
			result.Timestamps = append(result.Timestamps, int64(ts))
			result.Values = append(result.Values, val)
		}
	}

	return result, nil
}

// RenderLinks renders PromQL templates for a marker and returns Prometheus
// expression-browser URLs (using externalURL as the base) without querying Prometheus.
func (p *PromProxy) RenderLinks(m *marker.Marker, externalURL string) ([]PromLink, error) {
	tmpls := p.resolveTemplates(m.AlertName)
	links := make([]PromLink, 0, len(tmpls))
	for _, t := range tmpls {
		query, err := renderQuery(t.Query, m.Labels)
		if err != nil {
			return nil, fmt.Errorf("rendering query %q: %w", t.Label, err)
		}
		graphURL := fmt.Sprintf("%s/graph?g0.expr=%s&g0.tab=0&g0.range_input=1h",
			externalURL, url.QueryEscape(query))
		links = append(links, PromLink{Label: t.Label, Query: query, URL: graphURL})
	}
	return links, nil
}

// PrometheusUnavailableError is returned when Prometheus cannot be reached.
type PrometheusUnavailableError struct {
	Err error
}

func (e *PrometheusUnavailableError) Error() string {
	return fmt.Sprintf("prometheus unavailable: %v", e.Err)
}
