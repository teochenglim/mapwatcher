package cmd

import (
	"fmt"

	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/spf13/cobra"
)

var downloadSGOutDir string

var downloadSGCmd = &cobra.Command{
	Use:   "download-sg",
	Short: "Download Singapore Police Force NPC Boundary GeoJSON from data.gov.sg",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading SG NPC Boundary GeoJSON to %s …\n", downloadSGOutDir)
		dest, err := geo.DownloadSGNPC(downloadSGOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadSGCmd.Flags().StringVarP(&downloadSGOutDir, "out", "o", "./data", "Output directory")
	rootCmd.AddCommand(downloadSGCmd)
}
