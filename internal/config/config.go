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

// Config is the main configuration for the application
type Config struct {
}

//nolint:golint,gochecknoglobals
var (
	ConfigFileKey = "config"
)

func RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(ConfigFileKey, "c", "", "Config file path")
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
	// if cmd.Flags().Changed(GitHubEnterpriseURLKey) {
	// 	config.GitHubAuth.EnterpriseURL, err = cmd.Flags().GetString(GitHubEnterpriseURLKey)
	// 	if err != nil {
	// 		return &config, fmt.Errorf("failed to get GitHub Enterprise URL: %w", err)
	// 	}
	// }

	err = config.Validate()
	if err != nil {
		return &config, fmt.Errorf("failed to validate config: %w", err)
	}

	return &config, nil
}
