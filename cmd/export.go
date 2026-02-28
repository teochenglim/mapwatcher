package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/teochenglim/mapwatch"
	"github.com/spf13/cobra"
)

var exportOut string

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export a self-contained static HTML snapshot (no server required)",
	Long: `Bundles the dashboard HTML, JS, and any current marker state into a single
self-contained HTML file that can be opened directly in a browser without a server.
WebSocket and live Prometheus features are disabled in export mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read embedded index.html
		htmlBytes, err := fs.ReadFile(mapwatch.StaticFiles, "static/index.html")
		if err != nil {
			return fmt.Errorf("reading index.html: %w", err)
		}

		// Read embedded mapwatch.js
		jsBytes, err := fs.ReadFile(mapwatch.StaticFiles, "static/mapwatch.js")
		if err != nil {
			return fmt.Errorf("reading mapwatch.js: %w", err)
		}

		// Inline JS: disable WS, pre-load empty markers (can be extended to accept JSON input)
		inlineInit := `
<script>
// Static export mode — WebSocket disabled.
document.addEventListener('DOMContentLoaded', function() {
  var staticMarkers = ` + string(staticMarkersJSON()) + `;
  MapWatch.loadStaticMarkers(staticMarkers);
});
</script>`

		html := string(htmlBytes)
		// Replace external mapwatch.js script tag with inline version
		html = strings.Replace(html,
			`<script src="mapwatch.js"></script>`,
			`<script>`+string(jsBytes)+`</script>`,
			1,
		)
		html = strings.Replace(html, `</body>`, inlineInit+"\n</body>", 1)

		if exportOut == "" {
			exportOut = "mapwatch-export.html"
		}
		if err := os.WriteFile(exportOut, []byte(html), 0o644); err != nil {
			return fmt.Errorf("writing export: %w", err)
		}
		fmt.Printf("Exported: %s\n", exportOut)
		return nil
	},
}

func staticMarkersJSON() []byte {
	b, _ := json.Marshal([]interface{}{})
	return b
}

func init() {
	exportCmd.Flags().StringVarP(&exportOut, "out", "o", "mapwatch-export.html", "Output HTML file path")
	rootCmd.AddCommand(exportCmd)
}
