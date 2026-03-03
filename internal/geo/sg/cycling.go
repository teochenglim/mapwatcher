package sg

// Dataset ID on data.gov.sg — LTA Cycling Path Network (GEOJSON).
// Verify: https://data.gov.sg/datasets/d_8f468b25193f64be8a16fa7d8f60f553/view
const cyclingDatasetID = "d_8f468b25193f64be8a16fa7d8f60f553"

// FetchCycling fetches Singapore cycling path GeoJSON from data.gov.sg
// (LTA CyclingPathGazette dataset, ~2.9 MB) and saves it to outDir/sg-cycling.geojson.
func FetchCycling(outDir string) (string, error) {
	return fetchDataGovSG(outDir, cyclingDatasetID, "sg-cycling.geojson")
}
