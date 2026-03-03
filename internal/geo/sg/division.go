package sg

// Dataset ID on data.gov.sg — Singapore Police Force NPC Boundary.
// Verify: https://data.gov.sg/datasets/d_89b44df21fccc4f51390eaff16aa1fe8
const divisionDatasetID = "d_89b44df21fccc4f51390eaff16aa1fe8"

// FetchDivision fetches the Singapore Police Force NPC Boundary GeoJSON
// from data.gov.sg and saves it to outDir/sg-npc-boundary.geojson.
func FetchDivision(outDir string) (string, error) {
	return fetchDataGovSG(outDir, divisionDatasetID, "sg-npc-boundary.geojson")
}
