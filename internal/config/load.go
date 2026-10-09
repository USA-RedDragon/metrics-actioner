package config

import (
	"os"

	"github.com/USA-RedDragon/configulator/v2"
	cpflag "github.com/USA-RedDragon/configulator/v2/flags/pflag"
	"github.com/goccy/go-yaml"
	"github.com/spf13/pflag"
)

// EnvSeparator joins nested levels in environment variable names, as in
// HTTP__METRICS__PORT.
const EnvSeparator = "__"

// ConfigPathEnv names the environment variable that sets the config file
// path. The --config flag takes precedence over it.
const ConfigPathEnv = "CONFIG"

// NewLoader returns a configulator for Config that reads config.yaml (or
// the file named by --config or CONFIG), environment variables and the
// flags it registers on fs.
func NewLoader(fs *pflag.FlagSet) *configulator.Configulator[Config] {
	c := configulator.New(ConfigSchema()).
		WithEnvironmentVariables(&configulator.EnvironmentVariableOptions{Separator: EnvSeparator}).
		WithFile(&configulator.FileOptions{
			Search:   []string{"config.yaml"},
			Explicit: os.Getenv(ConfigPathEnv),
			Decoders: configulator.Decoders{".yaml": yaml.Unmarshal, ".yml": yaml.Unmarshal},
		})
	return cpflag.Bind(c, fs, ConfigPFlagHooks(), nil)
}
