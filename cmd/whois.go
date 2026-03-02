package cmd

import (
	"fmt"

	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/spf13/cobra"
)

var (
	whoisGeohash string
	whoisLat     float64
	whoisLng     float64
	whoisDataDir string
)

var whoisCmd = &cobra.Command{
	Use:   "whois",
	Short: "Look up which NPC division a geohash or lat/lng falls in",
	Example: `  mapwatch whois --geohash w21z7
  mapwatch whois --lat 1.3521 --lng 103.8198`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var lat, lng float64

		switch {
		case whoisGeohash != "":
			center, _, err := geo.DecodeGeohash(whoisGeohash)
			if err != nil {
				return fmt.Errorf("decoding geohash: %w", err)
			}
			lat, lng = center.Lat, center.Lng
			fmt.Printf("Geohash %q → lat=%.6f  lng=%.6f\n", whoisGeohash, lat, lng)

		case cmd.Flags().Changed("lat") && cmd.Flags().Changed("lng"):
			lat, lng = whoisLat, whoisLng

		default:
			return fmt.Errorf("provide --geohash or both --lat and --lng")
		}

		geojsonPath := whoisDataDir + "/sg-npc-boundary.geojson"
		info, err := geo.PointToNPC(lat, lng, geojsonPath)
		if err != nil {
			return err
		}
		if info == nil {
			fmt.Println("No NPC division found (point outside Singapore boundary?)")
			return nil
		}
		npc := info.NPCName
		if npc == "" {
			npc = "(none in source data)"
		}
		fmt.Printf("NPC:      %s\n", npc)
		fmt.Printf("Division: %s\n", info.Division)
		return nil
	},
}

func init() {
	whoisCmd.Flags().StringVar(&whoisGeohash, "geohash", "", "Geohash to look up")
	whoisCmd.Flags().Float64Var(&whoisLat, "lat", 0, "Latitude")
	whoisCmd.Flags().Float64Var(&whoisLng, "lng", 0, "Longitude")
	whoisCmd.Flags().StringVar(&whoisDataDir, "data-dir", "./data", "Directory containing sg-npc-boundary.geojson")
	rootCmd.AddCommand(whoisCmd)
}
