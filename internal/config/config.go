package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type HTTPListener struct {
	IPV4Host string `json:"ipv4_host"`
	IPV6Host string `json:"ipv6_host"`
	Port     uint16 `json:"port"`
}

type Tracing struct {
	Enabled      bool   `json:"enabled"`
	OTLPEndpoint string `json:"otlp_endpoint"`
}

type PProf struct {
	Enabled bool `json:"enabled"`
}

type Metrics struct {
	HTTPListener
	Enabled bool `json:"enabled"`
}
type HTTP struct {
	HTTPListener
	Tracing
	PProf          PProf    `json:"pprof"`
	TrustedProxies []string `json:"trusted_proxies"`
	Metrics        Metrics  `json:"metrics"`
}

// Config is the main configuration for the application
type Config struct {
	HTTP HTTP `json:"http"`
}

//nolint:golint,gochecknoglobals
var (
	ConfigFileKey          = "config"
	HTTPHostIPV4Key        = "http.host_ipv4"
	HTTPHostIPV6Key        = "http.host_ipv6"
	HTTPPortKey            = "http.port"
	HTTPTracingEnabledKey  = "http.tracing.enabled"
	HTTPTracingOTLPEndKey  = "http.tracing.otlp_endpoint"
	HTTPPProfEnabledKey    = "http.pprof.enabled"
	HTTPTrustedProxiesKey  = "http.trusted_proxies"
	HTTPMetricsEnabledKey  = "http.metrics.enabled"
	HTTPMetricsHostIPV4Key = "http.metrics.host_ipv4"
	HTTPMetricsHostIPV6Key = "http.metrics.host_ipv6"
	HTTPMetricsPortKey     = "http.metrics.port"
)

const (
	DefaultHTTPHostIPV4        = "0.0.0.0"
	DefaultHTTPHostIPV6        = "::"
	DefaultHTTPPort            = 8080
	DefaultHTTPMetricsHostIPV4 = "127.0.0.1"
	DefaultHTTPMetricsHostIPV6 = "::1"
	DefaultHTTPMetricsPort     = 8081
)

func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(ConfigFileKey, "c", "", "Config file path")
	cmd.Flags().String(HTTPHostIPV4Key, DefaultHTTPHostIPV4, "HTTP server IPv4 host")
	cmd.Flags().String(HTTPHostIPV6Key, DefaultHTTPHostIPV6, "HTTP server IPv6 host")
	cmd.Flags().Uint16(HTTPPortKey, DefaultHTTPPort, "HTTP server port")
	cmd.Flags().Bool(HTTPTracingEnabledKey, false, "Enable Open Telemetry tracing")
	cmd.Flags().String(HTTPTracingOTLPEndKey, "", "Open Telemetry endpoint")
	cmd.Flags().Bool(HTTPPProfEnabledKey, false, "Enable pprof")
	cmd.Flags().StringSlice(HTTPTrustedProxiesKey, []string{}, "Comma-separated list of trusted proxies")
	cmd.Flags().Bool(HTTPMetricsEnabledKey, false, "Enable metrics server")
	cmd.Flags().String(HTTPMetricsHostIPV4Key, DefaultHTTPMetricsHostIPV4, "Metrics server IPv4 host")
	cmd.Flags().String(HTTPMetricsHostIPV6Key, DefaultHTTPMetricsHostIPV6, "Metrics server IPv6 host")
	cmd.Flags().Uint16(HTTPMetricsPortKey, DefaultHTTPMetricsPort, "Metrics server port")
}

func (c *Config) Validate() error {
	return nil
}

//nolint:golint,gocyclo
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	var config Config

	// Load flags from envs
	ctx, cancel := context.WithCancelCause(cmd.Context())
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if ctx.Err() != nil {
			return
		}
		optName := strings.ReplaceAll(strings.ToUpper(f.Name), ".", "__")
		if val, ok := os.LookupEnv(optName); !f.Changed && ok {
			if err := f.Value.Set(val); err != nil {
				cancel(err)
			}
			f.Changed = true
		}
	})
	if ctx.Err() != nil {
		return &config, fmt.Errorf("failed to load env: %w", context.Cause(ctx))
	}

	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return &config, fmt.Errorf("failed to get config path: %w", err)
	}
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return &config, fmt.Errorf("failed to read config: %w", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return &config, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// Flag overrides here
	if cmd.Flags().Changed(HTTPHostIPV4Key) {
		config.HTTP.IPV4Host, err = cmd.Flags().GetString(HTTPHostIPV4Key)
		if err != nil {
			return &config, fmt.Errorf("failed to get HTTP IPv4 host: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPHostIPV6Key) {
		config.HTTP.IPV6Host, err = cmd.Flags().GetString(HTTPHostIPV6Key)
		if err != nil {
			return &config, fmt.Errorf("failed to get HTTP IPv6 host: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPPortKey) {
		config.HTTP.Port, err = cmd.Flags().GetUint16(HTTPPortKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get HTTP port: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPPProfEnabledKey) {
		config.HTTP.PProf.Enabled, err = cmd.Flags().GetBool(HTTPPProfEnabledKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get pprof enabled: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPTrustedProxiesKey) {
		config.HTTP.TrustedProxies, err = cmd.Flags().GetStringSlice(HTTPTrustedProxiesKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get trusted proxies: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPMetricsEnabledKey) {
		config.HTTP.Metrics.Enabled, err = cmd.Flags().GetBool(HTTPMetricsEnabledKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics enabled: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPMetricsHostIPV4Key) {
		config.HTTP.Metrics.IPV4Host, err = cmd.Flags().GetString(HTTPMetricsHostIPV4Key)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics IPv4 host: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPMetricsHostIPV6Key) {
		config.HTTP.Metrics.IPV6Host, err = cmd.Flags().GetString(HTTPMetricsHostIPV6Key)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics IPv6 host: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPMetricsPortKey) {
		config.HTTP.Metrics.Port, err = cmd.Flags().GetUint16(HTTPMetricsPortKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics port: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPTracingEnabledKey) {
		config.HTTP.Tracing.Enabled, err = cmd.Flags().GetBool(HTTPTracingEnabledKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get tracing enabled: %w", err)
		}
	}

	if cmd.Flags().Changed(HTTPTracingOTLPEndKey) {
		config.HTTP.Tracing.OTLPEndpoint, err = cmd.Flags().GetString(HTTPTracingOTLPEndKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get tracing OTLP endpoint: %w", err)
		}
	}

	// Defaults
	if config.HTTP.IPV4Host == "" {
		config.HTTP.IPV4Host = DefaultHTTPHostIPV4
	}
	if config.HTTP.IPV6Host == "" {
		config.HTTP.IPV6Host = DefaultHTTPHostIPV6
	}
	if config.HTTP.Port == 0 {
		config.HTTP.Port = DefaultHTTPPort
	}
	if config.HTTP.Metrics.IPV4Host == "" {
		config.HTTP.Metrics.IPV4Host = DefaultHTTPMetricsHostIPV4
	}
	if config.HTTP.Metrics.IPV6Host == "" {
		config.HTTP.Metrics.IPV6Host = DefaultHTTPMetricsHostIPV6
	}
	if config.HTTP.Metrics.Port == 0 {
		config.HTTP.Metrics.Port = DefaultHTTPMetricsPort
	}

	err = config.Validate()
	if err != nil {
		return &config, fmt.Errorf("failed to validate config: %w", err)
	}

	return &config, nil
}
