package cmd

import (
	"fmt"

	sgdown "github.com/teochenglim/mapwatch/internal/geo/sg"
	"github.com/spf13/cobra"
)

var downloadSGDivisionOutDir string

var downloadSGDivisionCmd = &cobra.Command{
	Use:   "division",
	Short: "Download Singapore Police Force NPC Boundary GeoJSON from data.gov.sg",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG NPC division boundary to %s …\n", downloadSGDivisionOutDir)
		dest, err := sgdown.FetchDivision(downloadSGDivisionOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGDivisionCmd.Flags().StringVarP(&downloadSGDivisionOutDir, "out", "o", "./data", "Output directory")
	downloadSGCmd.AddCommand(downloadSGDivisionCmd)
}
