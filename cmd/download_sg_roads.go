package cmd

import (
	"fmt"

	sgdown "github.com/teochenglim/mapwatch/internal/geo/sg"
	"github.com/spf13/cobra"
)

var downloadSGRoadsOutDir string

var downloadSGRoadsCmd = &cobra.Command{
	Use:   "roads",
	Short: "Download Singapore road network GeoJSON from data.gov.sg (SLA National Map Line)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG road network to %s …\n", downloadSGRoadsOutDir)
		dest, err := sgdown.FetchRoads(downloadSGRoadsOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGRoadsCmd.Flags().StringVarP(&downloadSGRoadsOutDir, "out", "o", "./data", "Output directory")
	downloadSGCmd.AddCommand(downloadSGRoadsCmd)
}
