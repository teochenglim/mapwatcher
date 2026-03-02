package geo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const naturalEarthURL = "https://raw.githubusercontent.com/datasets/geo-countries/master/data/countries.geojson"

const sgNPCPollURL = "https://api-open.data.gov.sg/v1/public/api/datasets/d_89b44df21fccc4f51390eaff16aa1fe8/poll-download"

// DownloadSGNPC fetches the Singapore Police Force NPC Boundary GeoJSON
// from data.gov.sg and saves it to outDir/sg-npc-boundary.geojson.
func DownloadSGNPC(outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	// Step 1: poll the data.gov.sg API for a presigned download URL.
	resp, err := http.Get(sgNPCPollURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("polling data.gov.sg: %w", err)
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

	// Step 2: download the actual GeoJSON from the presigned S3 URL.
	geoResp, err := http.Get(pollResp.Data.URL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("downloading GeoJSON: %w", err)
	}
	defer geoResp.Body.Close()

	if geoResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d downloading GeoJSON", geoResp.StatusCode)
	}

	dest := filepath.Join(outDir, "sg-npc-boundary.geojson")
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, geoResp.Body); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return dest, nil
}

// DownloadWorld fetches the world GeoJSON from Natural Earth and saves it to outDir.
func DownloadWorld(outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	dest := filepath.Join(outDir, "world.geojson")

	resp, err := http.Get(naturalEarthURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("downloading GeoJSON: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, naturalEarthURL)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return dest, nil
}
