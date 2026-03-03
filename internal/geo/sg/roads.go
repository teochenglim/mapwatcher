package sg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// roadFolders are the FOLDERPATH values in the SLA National Map Line dataset
// that represent drivable road geometry. Contour lines and boundaries are excluded.
var roadFolders = map[string]bool{
	"Layers/Expressway":          true,
	"Layers/Expressway_Sliproad": true,
	"Layers/Major_Road":          true,
}

// FetchRoads fetches Singapore's National Map Line dataset from data.gov.sg,
// filters it to road features only (drops contour lines and boundary), and saves
// the result to outDir/sg-roads.geojson.
// Source: SLA National Map Line — https://data.gov.sg/datasets/d_10480c0b59e65663dfae1028ff4aa8bb/view
func FetchRoads(outDir string) (string, error) {
	const datasetID = "d_10480c0b59e65663dfae1028ff4aa8bb"

	// Download the full file to a temp name, filter it, then remove the temp.
	tmp := filepath.Join(outDir, "sg-roads-raw.geojson")
	if _, err := fetchDataGovSG(outDir, datasetID, "sg-roads-raw.geojson"); err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	fmt.Println("  filtering road features (removing contours and boundary) …")
	dest, err := filterRoadFeatures(tmp, filepath.Join(outDir, "sg-roads.geojson"))
	if err != nil {
		return "", err
	}
	return dest, nil
}

// filterRoadFeatures reads src GeoJSON, keeps only features whose FOLDERPATH is
// a road layer, and writes the result to dst.
func filterRoadFeatures(src, dst string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("reading raw roads: %w", err)
	}

	var fc struct {
		Type     string            `json:"type"`
		Name     string            `json:"name,omitempty"`
		Features []json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		return "", fmt.Errorf("parsing GeoJSON: %w", err)
	}

	// Quick struct just to read FOLDERPATH without allocating the full feature.
	var props struct {
		Properties struct {
			FolderPath string `json:"FOLDERPATH"`
		} `json:"properties"`
	}

	kept := fc.Features[:0]
	for _, raw := range fc.Features {
		if err := json.Unmarshal(raw, &props); err != nil {
			continue
		}
		fp := strings.TrimSpace(props.Properties.FolderPath)
		if roadFolders[fp] {
			kept = append(kept, raw)
		}
	}

	out, err := json.Marshal(map[string]any{
		"type":     fc.Type,
		"name":     fc.Name,
		"features": kept,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling filtered GeoJSON: %w", err)
	}

	if err := os.WriteFile(dst, out, 0o644); err != nil {
		return "", fmt.Errorf("writing filtered roads: %w", err)
	}
	fmt.Printf("  kept %d road features\n", len(kept))
	return dst, nil
}
