package sg

// FetchMRT fetches Singapore MRT/LRT rail line GeoJSON from data.gov.sg
// (URA Master Plan 2019 Rail Line layer, ~22 MB) and saves it to outDir/sg-mrt.geojson.
// Source: https://data.gov.sg/datasets/d_222bfc84eb86c7c11994d02f8939da8d/view
func FetchMRT(outDir string) (string, error) {
	const datasetID = "d_222bfc84eb86c7c11994d02f8939da8d"
	return fetchDataGovSG(outDir, datasetID, "sg-mrt.geojson")
}
