package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "mapwatch",
	Short: "World map visualization tool for Prometheus and Alertmanager",
	Long: `mapwatch is a geo-visualization server and CLI tool.
It accepts Alertmanager webhooks, custom marker events, and Prometheus metrics,
and renders them on an interactive Leaflet.js world map.`,
}

// SetVersion configures the version string shown by --version.
// Called from main() with the value injected at build time.
func SetVersion(v string) {
	rootCmd.Version = v
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./mapwatch.yaml)")
}
