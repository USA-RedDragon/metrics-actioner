package config_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/spf13/pflag"
)

const (
	allIPv4     = "0.0.0.0"
	proxyIP     = "10.0.0.1"
	proxyCIDR   = "10.0.0.0/8"
	otelAddress = "otel:4317"
)

func load(t *testing.T, env map[string]string, args ...string) (*config.Config, error) {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	l := config.NewLoader(fs).WithEnviron(func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	})
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return l.Load()
}

func TestDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := load(t, nil, "-c", "testdata/empty.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := config.HTTP{
		IPV4Host: allIPv4,
		IPV6Host: "::",
		Port:     8080,
		Metrics: config.Metrics{
			IPV4Host: "127.0.0.1",
			IPV6Host: "::1",
			Port:     8081,
		},
	}
	if !reflect.DeepEqual(cfg.HTTP, want) {
		t.Errorf("defaults:\n got %+v\nwant %+v", cfg.HTTP, want)
	}
	if len(cfg.Actions) != 0 {
		t.Errorf("actions = %v, want none", cfg.Actions)
	}
}

func TestLoadFullFile(t *testing.T) {
	t.Parallel()
	cfg, err := load(t, nil, "--config", "testdata/full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := config.Config{
		HTTP: config.HTTP{
			IPV4Host:       proxyIP,
			IPV6Host:       "fd00::1",
			Port:           9080,
			TrustedProxies: []string{proxyCIDR, "192.168.1.1"},
			Tracing:        config.Tracing{Enabled: true, OTLPEndpoint: "otel-collector:4317"},
			PProf:          config.PProf{Enabled: true},
			Metrics: config.Metrics{
				Enabled:  true,
				IPV4Host: allIPv4,
				IPV6Host: "::",
				Port:     9081,
			},
		},
		Actions: []config.Action{
			{
				MatchCommonLabels: config.Labels{"remediation": "restart-trunk-recorder"},
				MatchGroupLabels:  config.Labels{"namespace": "trunk-recorder"},
				Action:            "rollout-restart-deployment",
				Options:           config.Options{"deployment": "trunk-recorder-app", "namespace": "trunk-recorder"},
			},
			{
				MatchCommonLabels: config.Labels{"alertname": "TargetDown", "job": "aredn-manager"},
				Action:            "ssh",
				Options: config.Options{
					"command":  "docker restart tunnel",
					"host":     "host.example.com",
					"port":     "333",
					"user":     "admin",
					"key":      "/secrets/admin.pem",
					"hostKeys": "[host.example.com]:333 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID8Dy1TZ1lN8WAZcgdODY9SNVWqWB+hGfsVEzMaUV7n+\n",
				},
			},
		},
	}
	if !reflect.DeepEqual(*cfg, want) {
		t.Errorf("full file:\n got %+v\nwant %+v", *cfg, want)
	}
}

func TestLoadExampleFile(t *testing.T) {
	t.Parallel()
	example, err := load(t, nil, "-c", "../../config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := load(t, nil, "-c", "testdata/empty.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(example.HTTP, defaults.HTTP) {
		t.Errorf("config.example.yaml should load the defaults:\n got %+v\nwant %+v", example.HTTP, defaults.HTTP)
	}
}

func TestLegacyInlineTracingKeys(t *testing.T) {
	t.Parallel()
	cfg, err := load(t, nil, "-c", "testdata/legacy-tracing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := config.Tracing{Enabled: true, OTLPEndpoint: otelAddress}
	if cfg.HTTP.Tracing != want {
		t.Errorf("tracing = %+v, want %+v", cfg.HTTP.Tracing, want)
	}
}

func TestEnvNames(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"HTTP__IPV4_HOST":              "1.2.3.4",
		"HTTP__IPV6_HOST":              "::2",
		"HTTP__PORT":                   "9000",
		"HTTP__TRACING__ENABLED":       strconv.FormatBool(true),
		"HTTP__TRACING__OTLP_ENDPOINT": otelAddress,
		"HTTP__PPROF__ENABLED":         strconv.FormatBool(true),
		"HTTP__TRUSTED_PROXIES":        proxyCIDR + ",1.1.1.1",
		"HTTP__METRICS__ENABLED":       strconv.FormatBool(true),
		"HTTP__METRICS__IPV4_HOST":     allIPv4,
		"HTTP__METRICS__IPV6_HOST":     "::",
		"HTTP__METRICS__PORT":          "9001",
	}
	cfg, err := load(t, env, "-c", "testdata/empty.yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := config.HTTP{
		IPV4Host:       "1.2.3.4",
		IPV6Host:       "::2",
		Port:           9000,
		Tracing:        config.Tracing{Enabled: true, OTLPEndpoint: otelAddress},
		PProf:          config.PProf{Enabled: true},
		TrustedProxies: []string{proxyCIDR, "1.1.1.1"},
		Metrics:        config.Metrics{Enabled: true, IPV4Host: allIPv4, IPV6Host: "::", Port: 9001},
	}
	if !reflect.DeepEqual(cfg.HTTP, want) {
		t.Errorf("env:\n got %+v\nwant %+v", cfg.HTTP, want)
	}
}

func TestPrecedence(t *testing.T) {
	t.Parallel()
	cfg, err := load(t, map[string]string{"HTTP__PORT": "9100", "HTTP__METRICS__PORT": "9101"},
		"-c", "testdata/full.yaml", "--http.port", "9200")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 9200 {
		t.Errorf("port = %d, want the flag value 9200", cfg.HTTP.Port)
	}
	if cfg.HTTP.Metrics.Port != 9101 {
		t.Errorf("metrics port = %d, want the env value 9101", cfg.HTTP.Metrics.Port)
	}
}

func TestMissingConfigFlagFile(t *testing.T) {
	t.Parallel()
	if _, err := load(t, nil, "-c", "testdata/missing.yaml"); err == nil {
		t.Error("expected an error for a missing --config file")
	}
}

func TestConfigPathEnv(t *testing.T) {
	t.Setenv(config.ConfigPathEnv, "testdata/full.yaml")
	cfg, err := load(t, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTP.Port != 9080 {
		t.Errorf("port = %d, want 9080 from the CONFIG file", cfg.HTTP.Port)
	}
}
