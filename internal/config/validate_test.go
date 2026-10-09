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
