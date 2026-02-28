package cmd

import (
	"fmt"

	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/spf13/cobra"
)

var (
	sliceRegion string
	sliceSrc    string
	sliceDst    string
)

var sliceCmd = &cobra.Command{
	Use:   "slice",
	Short: "Clip a GeoJSON file to a region bounding box",
	Long: `Reads a world GeoJSON file and outputs only the features that
intersect the bounding box of the given region code.

Supported region codes: SG, US, EU, JP, AU`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sliceRegion == "" {
			return fmt.Errorf("--region is required (e.g. --region=SG)")
		}
		if sliceDst == "" {
			sliceDst = fmt.Sprintf("./%s.geojson", sliceRegion)
		}
		fmt.Printf("Slicing %s → %s (region: %s) …\n", sliceSrc, sliceDst, sliceRegion)
		if err := geo.SliceGeoJSON(sliceSrc, sliceDst, sliceRegion); err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", sliceDst)
		return nil
	},
}

func init() {
	sliceCmd.Flags().StringVar(&sliceRegion, "region", "", "Region code to clip to (SG, US, EU, JP, AU)")
	sliceCmd.Flags().StringVar(&sliceSrc, "src", "./data/world.geojson", "Source GeoJSON file")
	sliceCmd.Flags().StringVar(&sliceDst, "dst", "", "Output path (default: ./<REGION>.geojson)")
	rootCmd.AddCommand(sliceCmd)
}
