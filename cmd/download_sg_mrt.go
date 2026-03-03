package cmd

import (
	"fmt"

	sgdown "github.com/teochenglim/mapwatch/internal/geo/sg"
	"github.com/spf13/cobra"
)

var downloadSGMRTOutDir string

var downloadSGMRTCmd = &cobra.Command{
	Use:   "mrt",
	Short: "Download Singapore MRT/LRT rail lines GeoJSON from data.gov.sg (URA Master Plan 2019)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG MRT/LRT rail lines to %s …\n", downloadSGMRTOutDir)
		dest, err := sgdown.FetchMRT(downloadSGMRTOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGMRTCmd.Flags().StringVarP(&downloadSGMRTOutDir, "out", "o", "./data", "Output directory")
	downloadSGCmd.AddCommand(downloadSGMRTCmd)
}
