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
	Layers         LayersConfig               `mapstructure:"layers"`
	GeoPriority    []string                   `mapstructure:"geo_label_priority"`
	Modules        ModulesConfig              `mapstructure:"modules"`
	LeaderboardURL string                     `mapstructure:"leaderboard_url"`
	Tiles          []TileConfig               `mapstructure:"tiles"`
	LayerDefs      []LayerDefConfig           `mapstructure:"layer_defs"`
}

// TileConfig defines a base-map tile theme served to the frontend.
// Add entries under tiles: in mapwatch.yaml to add new basemaps without
// changing any JS or HTML — the frontend builds buttons dynamically.
type TileConfig struct {
	ID          string `mapstructure:"id"          json:"id"`
	Label       string `mapstructure:"label"       json:"label"`
	URL         string `mapstructure:"url"         json:"url"`
	Attribution string `mapstructure:"attribution" json:"attribution"`
	Default     bool   `mapstructure:"default"     json:"default,omitempty"`
}

// LayerStyleConfig describes the Leaflet rendering style for a GeoJSON overlay.
// type: polygon | line | line_conditional | point
type LayerStyleConfig struct {
	Type        string  `mapstructure:"type"         json:"type"`
	Color       string  `mapstructure:"color"        json:"color"`
	FillColor   string  `mapstructure:"fill_color"   json:"fill_color,omitempty"`
	Weight      float64 `mapstructure:"weight"       json:"weight,omitempty"`
	Opacity     float64 `mapstructure:"opacity"      json:"opacity,omitempty"`
	FillOpacity float64 `mapstructure:"fill_opacity" json:"fill_opacity,omitempty"`
	DashArray   string  `mapstructure:"dash_array"   json:"dash_array,omitempty"`
	Radius      float64 `mapstructure:"radius"       json:"radius,omitempty"`
}

// LayerTooltipConfig specifies which GeoJSON feature properties appear in hover tooltips.
type LayerTooltipConfig struct {
	NameProps []string `mapstructure:"name_props" json:"name_props,omitempty"`
	SubProps  []string `mapstructure:"sub_props"  json:"sub_props,omitempty"`
}

// LayerDefConfig describes a named GeoJSON overlay layer.
// Add entries under layer_defs: in mapwatch.yaml to add new overlays without
// changing any JS or HTML — the frontend builds buttons and applies styles dynamically.
type LayerDefConfig struct {
	ID      string             `mapstructure:"id"      json:"id"`
	Label   string             `mapstructure:"label"   json:"label"`
	File    string             `mapstructure:"file"    json:"file"`
	Enabled bool               `mapstructure:"enabled" json:"enabled"`
	Style   LayerStyleConfig   `mapstructure:"style"   json:"style"`
	Tooltip LayerTooltipConfig `mapstructure:"tooltip" json:"tooltip,omitempty"`
}

// ModulesConfig controls optional frontend feature modules.
// All modules default to false (disabled). Enable per-example via mapwatch.yaml.
//
//   modules:
//     sound: true       # Web Audio API tones on marker.add
//     leaderboard: true # leaderboard sidebar (requires leaderboard_url)
//     stats: true       # live tap/alert counter overlay
type ModulesConfig struct {
	Sound       bool `mapstructure:"sound"`
	Leaderboard bool `mapstructure:"leaderboard"`
	Stats       bool `mapstructure:"stats"`
}

// LayersConfig controls which optional GeoJSON overlays are enabled at startup.
// All layers default to false; data files must be downloaded first with the
// corresponding "mapwatch download-sg-*" commands before enabling.
type LayersConfig struct {
	Division  bool `mapstructure:"division"`   // NPC police division boundaries
	Roads     bool `mapstructure:"roads"`      // expressways and major roads
	Cycling   bool `mapstructure:"cycling"`    // cycling paths
	MRT       bool `mapstructure:"mrt"`        // MRT/LRT rail lines
	BusStops  bool `mapstructure:"bus_stops"`  // bus stop points
	BusRoutes bool `mapstructure:"bus_routes"` // bus route lines
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
	if len(cfg.GeoPriority) == 0 {
		cfg.GeoPriority = []string{"geohash", "lat_lng", "datacenter", "location", "region"}
	}

	return &cfg, nil
}
