package cmd

import "github.com/spf13/cobra"

// downloadSGCmd is the parent for all Singapore geospatial layer downloads.
// Sub-commands are registered by their own files (download_sg_*.go).
//
// Usage:
//
//	mapwatch download-sg division   — NPC police division boundaries (data.gov.sg)
//	mapwatch download-sg roads      — national map line / roads (data.gov.sg)
//	mapwatch download-sg cycling    — cycling paths (data.gov.sg)
//	mapwatch download-sg mrt        — MRT/LRT rail lines (data.gov.sg)
//	mapwatch download-sg busstops   — bus stops (busrouter.sg)
//	mapwatch download-sg busroutes  — bus routes (busrouter.sg)
var downloadSGCmd = &cobra.Command{
	Use:   "download-sg",
	Short: "Download Singapore geospatial data layers (all optional)",
	Long: `Download Singapore geospatial data layers for use as map overlays.

All layers are optional — MapWatch works without them.  Run a sub-command
to download a specific layer into the data directory (default: ./data).

Data files are served automatically by the mapwatch server via:
  GET /api/geojson/<name>

Enable layers at startup by setting the [layers] section in mapwatch.yaml.`,
}

func init() {
	rootCmd.AddCommand(downloadSGCmd)
}
