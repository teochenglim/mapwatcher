package cmd

import (
	"fmt"

	sgdown "github.com/teochenglim/mapwatch/internal/geo/sg"
	"github.com/spf13/cobra"
)

var downloadSGBusRoutesOutDir string

var downloadSGBusRoutesCmd = &cobra.Command{
	Use:   "busroutes",
	Short: "Download Singapore bus routes GeoJSON from busrouter.sg",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG bus routes to %s …\n", downloadSGBusRoutesOutDir)
		dest, err := sgdown.FetchBusRoutes(downloadSGBusRoutesOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGBusRoutesCmd.Flags().StringVarP(&downloadSGBusRoutesOutDir, "out", "o", "./data", "Output directory")
	downloadSGCmd.AddCommand(downloadSGBusRoutesCmd)
}
