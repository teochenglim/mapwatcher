package cmd

import (
	"fmt"

	sgdown "github.com/teochenglim/mapwatch/internal/geo/sg"
	"github.com/spf13/cobra"
)

var downloadSGBusStopsOutDir string

var downloadSGBusStopsCmd = &cobra.Command{
	Use:   "busstops",
	Short: "Download Singapore bus stops GeoJSON from busrouter.sg",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG bus stops to %s …\n", downloadSGBusStopsOutDir)
		dest, err := sgdown.FetchBusStops(downloadSGBusStopsOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGBusStopsCmd.Flags().StringVarP(&downloadSGBusStopsOutDir, "out", "o", "./data", "Output directory")
	downloadSGCmd.AddCommand(downloadSGBusStopsCmd)
}
