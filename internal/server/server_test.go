package server_test

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/USA-RedDragon/metrics-actioner/internal/server"
)

func testConfig() *config.HTTP {
	return &config.HTTP{
		IPV4Host: "127.0.0.1",
		IPV6Host: "::1",
		Metrics: config.Metrics{
			IPV4Host: "127.0.0.1",
			IPV6Host: "::1",
		},
	}
}

func TestStartStop(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.Metrics.Enabled = true
	s := server.NewServer(cfg, alertmanager.NewReceiver(&[]config.Action{}))
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestStartReleasesListenersOnError(t *testing.T) {
	t.Parallel()
	var lc net.ListenConfig
	held, err := lc.Listen(context.Background(), "tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	defer held.Close()
	port := held.Addr().(*net.TCPAddr).Port

	cfg := testConfig()
	cfg.Port = uint16(port)
	s := server.NewServer(cfg, alertmanager.NewReceiver(&[]config.Action{}))
	if err := s.Start(); err == nil {
		_ = s.Stop()
		t.Fatal("expected Start to fail while the IPv6 port is taken")
	}

	l, err := lc.Listen(context.Background(), "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("IPv4 listener was left open after Start failed: %v", err)
	}
	l.Close()

	resp, err := http.Get("http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) + "/health")
	if err == nil {
		resp.Body.Close()
		t.Fatal("IPv4 server still serving after Start failed")
	}
}
