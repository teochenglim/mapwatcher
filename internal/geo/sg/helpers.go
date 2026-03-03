// Package sg is the Singapore geospatial API client.
// It fetches GeoJSON from data.gov.sg and busrouter.sg and writes
// files to the data directory consumed by the mapwatch server.
//
// Data sources:
//   - data.gov.sg poll-download API (NPC boundaries, roads, cycling paths, MRT lines)
//   - busrouter.sg pre-built GeoJSON (bus stops, bus routes)
package sg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/teochenglim/mapwatch/internal/geo"
)

const dataGovSGBase = "https://api-open.data.gov.sg/v1/public/api/datasets"

// fetchDataGovSG fetches a dataset via the data.gov.sg poll-download API.
// Retries up to 4 times with exponential backoff on HTTP 429 (rate limit).
func fetchDataGovSG(outDir, datasetID, filename string) (string, error) {
	pollURL := dataGovSGBase + "/" + datasetID + "/poll-download"

	var resp *http.Response
	var err error
	backoff := 5 * time.Second
	for attempt := 0; attempt < 5; attempt++ {
		resp, err = http.Get(pollURL) //nolint:gosec
		if err != nil {
			return "", fmt.Errorf("polling data.gov.sg: %w", err)
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			break
		}
		resp.Body.Close()
		fmt.Printf("  data.gov.sg rate limit hit (429) — waiting %s before retry %d/4 …\n", backoff, attempt+1)
		time.Sleep(backoff)
		backoff *= 2
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status %d from poll endpoint", resp.StatusCode)
	}

	var pollResp struct {
		Code     int    `json:"code"`
		ErrorMsg string `json:"errorMsg"`
		Data     struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pollResp); err != nil {
		return "", fmt.Errorf("decoding poll response: %w", err)
	}
	if pollResp.Code != 0 {
		return "", fmt.Errorf("data.gov.sg error %d: %s", pollResp.Code, pollResp.ErrorMsg)
	}
	if pollResp.Data.URL == "" {
		return "", fmt.Errorf("poll response contained no download URL")
	}

	return geo.DownloadHTTP(outDir, pollResp.Data.URL, filename)
}
