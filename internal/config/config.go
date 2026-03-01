package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the full application configuration.
type Config struct {
	Server         ServerConfig               `mapstructure:"server"`
	Prometheus     PrometheusConfig           `mapstructure:"prometheus"`
	Locations      map[string]string          `mapstructure:"locations"`
	QueryTemplates map[string][]QueryTemplate `mapstructure:"query_templates"`
	Spread         SpreadConfig               `mapstructure:"spread"`
	Heatmap        HeatmapConfig              `mapstructure:"heatmap"`
}

// HeatmapRegion defines a named spatial zone for choropleth overlay.
// Markers whose lat/lng falls inside Bounds are aggregated into this region.
// Bounds: [[lat_sw, lng_sw], [lat_ne, lng_ne]] — the rectangle drawn on the map.
// Color (optional) overrides the severity-based fill colour, e.g. "#4a9eff".
type HeatmapRegion struct {
	Name   string        `mapstructure:"name"`
	Bounds [2][2]float64 `mapstructure:"bounds"` // [[lat_sw,lng_sw],[lat_ne,lng_ne]]
	Color  string        `mapstructure:"color"`  // optional custom fill colour
}

// HeatmapConfig holds optional region definitions for heatmap aggregation.
type HeatmapConfig struct {
	Regions []HeatmapRegion `mapstructure:"regions"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
}

type PrometheusConfig struct {
	URL         string        `mapstructure:"url"`
	ExternalURL string        `mapstructure:"external_url"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type QueryTemplate struct {
	Label string `mapstructure:"label"`
	Query string `mapstructure:"query"`
}

type SpreadConfig struct {
	// Radius in degrees for co-located marker offset (default 0.01)
	Radius float64 `mapstructure:"radius"`
}

// Load reads config from file and environment variables.
// cfgFile may be empty to use default search paths.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("prometheus.url", "http://localhost:9090")
	v.SetDefault("prometheus.external_url", "http://localhost:9090")
	v.SetDefault("prometheus.timeout", "10s")
	v.SetDefault("spread.radius", 0.01)

	// Env overrides: MAPWATCH_SERVER_ADDR etc.
	v.SetEnvPrefix("MAPWATCH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("mapwatch")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.mapwatch")
		v.AddConfigPath("/etc/mapwatch")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
		// Config file not found — use defaults and env only.
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if cfg.Locations == nil {
		cfg.Locations = make(map[string]string)
	}
	if cfg.QueryTemplates == nil {
		cfg.QueryTemplates = make(map[string][]QueryTemplate)
	}

	return &cfg, nil
}
