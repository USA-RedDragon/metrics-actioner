package config_test

import (
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/config"
)

func TestValidateRejectsUnknownAction(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"", "rollout-restart", "SSH"} {
		cfg := config.Config{Actions: []config.Action{{Action: action}}}
		if err := cfg.Validate(); err == nil {
			t.Errorf("action %q: expected an error", action)
		}
	}
	for _, action := range []string{"rollout-restart-deployment", "ssh"} {
		cfg := config.Config{Actions: []config.Action{{Action: action}}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("action %q: %v", action, err)
		}
	}
}

func TestValidateRejectsBadTrustedProxies(t *testing.T) {
	t.Parallel()
	for _, proxy := range []string{"not-an-ip", "10.0.0.0/33", "1.2.3"} {
		cfg := config.Config{HTTP: config.HTTP{TrustedProxies: []string{proxyIP, proxy}}}
		if err := cfg.Validate(); err == nil {
			t.Errorf("trusted proxy %q: expected an error", proxy)
		}
	}
	cfg := config.Config{HTTP: config.HTTP{TrustedProxies: []string{proxyIP, proxyCIDR, "::1", "fd00::/8"}}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid trusted proxies: %v", err)
	}
}
