package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/teochenglim/mapwatch"
	"github.com/teochenglim/mapwatch/internal/config"
	"github.com/teochenglim/mapwatch/internal/server"
	"github.com/spf13/cobra"
)

var servePort string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server with live map dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// --port flag overrides config file and env var
		if servePort != "" {
			cfg.Server.Addr = ":" + servePort
		}

		staticFS := mapwatch.StaticHTTPFS()
		srv := server.New(cfg, staticFS)

		log.Printf("mapwatch listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&servePort, "port", "p", "", "Port to listen on (overrides config and MAPWATCH_SERVER_ADDR)")
	rootCmd.AddCommand(serveCmd)
}
