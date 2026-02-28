package cmd

import (
	"fmt"

	"github.com/teochenglim/mapwatch/internal/geo"
	"github.com/spf13/cobra"
)

var downloadOutDir string

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download world GeoJSON map data from Natural Earth",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Downloading world GeoJSON to %s …\n", downloadOutDir)
		dest, err := geo.DownloadWorld(downloadOutDir)
		if err != nil {
			return err
		}
		fmt.Printf("Saved: %s\n", dest)
		return nil
	},
}

func init() {
	downloadCmd.Flags().StringVarP(&downloadOutDir, "out", "o", "./data", "Output directory")
	rootCmd.AddCommand(downloadCmd)
}
