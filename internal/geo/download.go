package geo

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const naturalEarthURL = "https://raw.githubusercontent.com/datasets/geo-countries/master/data/countries.geojson"

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
