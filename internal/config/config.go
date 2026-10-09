package config

//go:generate go tool configulator -type Config

// Tracing configures OpenTelemetry tracing of HTTP requests.
type Tracing struct {
	Enabled      bool   `name:"enabled" default:"false" description:"Enable OpenTelemetry tracing"`
	OTLPEndpoint string `name:"otlp_endpoint" description:"OpenTelemetry OTLP endpoint"`
}

// PProf configures the pprof endpoints.
type PProf struct {
	Enabled bool `name:"enabled" default:"false" description:"Enable pprof on the HTTP server"`
}

// Metrics configures the Prometheus metrics server.
type Metrics struct {
	IPV4Host string `name:"ipv4_host" default:"127.0.0.1" description:"Metrics server IPv4 host"`
	IPV6Host string `name:"ipv6_host" default:"::1" description:"Metrics server IPv6 host"`
	Port     uint16 `name:"port" default:"8081" description:"Metrics server port"`
	Enabled  bool   `name:"enabled" default:"false" description:"Enable the metrics server"`
}

// HTTP configures the HTTP server that receives AlertManager webhooks.
type HTTP struct {
	IPV4Host       string   `name:"ipv4_host" default:"0.0.0.0" description:"HTTP server IPv4 host"`
	IPV6Host       string   `name:"ipv6_host" default:"::" description:"HTTP server IPv6 host"`
	Port           uint16   `name:"port" default:"8080" description:"HTTP server port"`
	Tracing        Tracing  `name:"tracing"`
	PProf          PProf    `name:"pprof"`
	TrustedProxies []string `name:"trusted_proxies" description:"Comma-separated list of trusted proxy IPs or CIDRs"`
	Metrics        Metrics  `name:"metrics"`

	LegacyTracingEnabled      bool   `name:"enabled" flag:"-" env:"-" description:"Deprecated alias of http.tracing.enabled"`
	LegacyTracingOTLPEndpoint string `name:"otlp_endpoint" flag:"-" env:"-" description:"Deprecated alias of http.tracing.otlp_endpoint"`
}

// Labels are label names and values that must all match.
type Labels map[string]string

// Options are the options passed to an action.
type Options map[string]string

// Action is a rule that runs an action when an AlertManager webhook matches it.
type Action struct {
	MatchCommonLabels Labels  `name:"match_common_labels" description:"Labels that must all match the webhook's common labels"`
	MatchGroupLabels  Labels  `name:"match_group_labels" description:"Labels that must all match the webhook's group labels"`
	Action            string  `name:"action" description:"Action to run: rollout-restart-deployment (kubectl rollout restart) or ssh (run a command over SSH)"`
	Options           Options `name:"options" description:"Options for the action, all strings. rollout-restart-deployment: deployment (required), namespace (defaults to the alert's namespace common label). ssh: command, host, user, key (path to a private key file), all required; port (default 22); hostKeys (known_hosts lines for the host, or ignore to skip host key checks)"`
}

// Config is the main configuration for the application
type Config struct {
	HTTP    HTTP     `name:"http"`
	Actions []Action `name:"actions" description:"Rules that run an action when a firing AlertManager webhook matches them. Only settable in the config file."`
}

// Validate checks the configuration and folds the deprecated tracing keys
// into http.tracing.
func (c *Config) Validate() error {
	if c.HTTP.LegacyTracingEnabled {
		c.HTTP.Tracing.Enabled = true
	}
	if c.HTTP.Tracing.OTLPEndpoint == "" {
		c.HTTP.Tracing.OTLPEndpoint = c.HTTP.LegacyTracingOTLPEndpoint
	}
	return nil
}
