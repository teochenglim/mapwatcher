package sg

import geo "github.com/teochenglim/mapwatch/internal/geo"

const busRoutesURL = "https://data.busrouter.sg/v1/routes.min.geojson"

// FetchBusRoutes downloads Singapore bus routes GeoJSON from busrouter.sg
// and saves it to outDir/sg-bus-routes.geojson.
func FetchBusRoutes(outDir string) (string, error) {
	return geo.DownloadHTTP(outDir, busRoutesURL, "sg-bus-routes.geojson")
}
