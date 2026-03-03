package sg

import geo "github.com/teochenglim/mapwatch/internal/geo"

const busStopsURL = "https://data.busrouter.sg/v1/stops.min.geojson"

// FetchBusStops downloads Singapore bus stops GeoJSON from busrouter.sg
// and saves it to outDir/sg-bus-stops.geojson.
func FetchBusStops(outDir string) (string, error) {
	return geo.DownloadHTTP(outDir, busStopsURL, "sg-bus-stops.geojson")
}
