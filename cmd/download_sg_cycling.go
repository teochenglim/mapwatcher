package cmd

import (
	"fmt"

	sgdown "github.com/teochenglim/mapwatch/internal/geo/sg"
	"github.com/spf13/cobra"
)

var downloadSGCyclingOutDir string

var downloadSGCyclingCmd = &cobra.Command{
	Use:   "cycling",
	Short: "Download Singapore cycling paths GeoJSON from data.gov.sg / LTA",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG cycling paths to %s …\n", downloadSGCyclingOutDir)
		dest, err := sgdown.FetchCycling(downloadSGCyclingOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGCyclingCmd.Flags().StringVarP(&downloadSGCyclingOutDir, "out", "o", "./data", "Output directory")
	downloadSGCmd.AddCommand(downloadSGCyclingCmd)
}
