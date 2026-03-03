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
	return DownloadHTTP(outDir, naturalEarthURL, "world.geojson")
}

// DownloadHTTP downloads rawURL and writes the response body to outDir/filename.
// Used by country sub-packages (e.g. internal/geo/sg) and the world download.
func DownloadHTTP(outDir, rawURL, filename string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	resp, err := http.Get(rawURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, rawURL)
	}

	return SaveBody(outDir, filename, resp.Body)
}

// OverpassFetch posts an Overpass QL query and saves the response to outDir/filename.
// Include [out:geojson] in your query for direct GeoJSON output.
// Used by country sub-packages to fetch OpenStreetMap data.
func OverpassFetch(outDir, query, filename string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}

	resp, err := http.PostForm("https://overpass-api.de/api/interpreter",
		map[string][]string{"data": {query}})
	if err != nil {
		return "", fmt.Errorf("overpass request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("overpass returned HTTP %d", resp.StatusCode)
	}

	return SaveBody(outDir, filename, resp.Body)
}

// SaveBody writes r to outDir/filename and returns the full path.
func SaveBody(outDir, filename string, r io.Reader) (string, error) {
	dest := filepath.Join(outDir, filename)
	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}
	return dest, nil
}
