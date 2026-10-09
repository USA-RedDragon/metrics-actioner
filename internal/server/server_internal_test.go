package server

import (
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
)

func TestPProfWriteTimeoutOnBothStacks(t *testing.T) {
	t.Parallel()
	cfg := &config.HTTP{PProf: config.PProf{Enabled: true}}
	s := NewServer(cfg, alertmanager.NewReceiver(&[]config.Action{}))
	if s.ipv4Server.WriteTimeout != pprofWriteTimeout {
		t.Errorf("IPv4 write timeout = %s, want %s", s.ipv4Server.WriteTimeout, pprofWriteTimeout)
	}
	if s.ipv6Server.WriteTimeout != pprofWriteTimeout {
		t.Errorf("IPv6 write timeout = %s, want %s", s.ipv6Server.WriteTimeout, pprofWriteTimeout)
	}
}
