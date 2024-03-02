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

type ConfigHTTP struct {
	IPV4Host string `json:"ipv4_host"`
	IPV6Host string `json:"ipv6_host"`
	Port     uint16 `json:"port"`
}

type ConfigMetrics struct {
	ConfigHTTP
	Enabled bool `json:"enabled"`
}

// Config is the main configuration for the application
type Config struct {
	HTTP    ConfigHTTP    `json:"http"`
	Metrics ConfigMetrics `json:"metrics"`
}

//nolint:golint,gochecknoglobals
var (
	ConfigFileKey      = "config"
	HTTPHostIPV4Key    = "http.host_ipv4"
	HTTPHostIPV6Key    = "http.host_ipv6"
	HTTPPortKey        = "http.port"
	MetricsEnabledKey  = "metrics.enabled"
	MetricsHostIPV4Key = "metrics.host_ipv4"
	MetricsHostIPV6Key = "metrics.host_ipv6"
	MetricsPortKey     = "metrics.port"
)

func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(ConfigFileKey, "c", "", "Config file path")
	cmd.Flags().String(HTTPHostIPV4Key, "0.0.0.0", "HTTP server IPv4 host")
	cmd.Flags().String(HTTPHostIPV6Key, "::", "HTTP server IPv6 host")
	cmd.Flags().Uint16(HTTPPortKey, 8080, "HTTP server port")
	cmd.Flags().Bool(MetricsEnabledKey, false, "Enable metrics server")
	cmd.Flags().String(MetricsHostIPV4Key, "127.0.0.1", "Metrics server IPv4 host")
	cmd.Flags().String(MetricsHostIPV6Key, "::1", "Metrics server IPv6 host")
	cmd.Flags().Uint16(MetricsPortKey, 8081, "Metrics server port")
}

func (c *Config) Validate() error {
	return nil
}

func LoadConfig(cmd *cobra.Command) (*Config, error) {
	var config Config

	// Load flags from envs
	ctx, cancel := context.WithCancelCause(cmd.Context())
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if ctx.Err() != nil {
			return
		}
		optName := strings.ReplaceAll(strings.ToUpper(f.Name), "-", "_")
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

	if cmd.Flags().Changed(MetricsEnabledKey) {
		config.Metrics.Enabled, err = cmd.Flags().GetBool(MetricsEnabledKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics enabled: %w", err)
		}
	}

	if cmd.Flags().Changed(MetricsHostIPV4Key) {
		config.Metrics.IPV4Host, err = cmd.Flags().GetString(MetricsHostIPV4Key)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics IPv4 host: %w", err)
		}
	}

	if cmd.Flags().Changed(MetricsHostIPV6Key) {
		config.Metrics.IPV6Host, err = cmd.Flags().GetString(MetricsHostIPV6Key)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics IPv6 host: %w", err)
		}
	}

	if cmd.Flags().Changed(MetricsPortKey) {
		config.Metrics.Port, err = cmd.Flags().GetUint16(MetricsPortKey)
		if err != nil {
			return &config, fmt.Errorf("failed to get metrics port: %w", err)
		}
	}

	err = config.Validate()
	if err != nil {
		return &config, fmt.Errorf("failed to validate config: %w", err)
	}

	return &config, nil
}
