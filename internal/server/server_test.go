package server_test

import (
	"context"
	"net"
	"net/http"
	"net/netip"
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
	held, port := holdIPv6PortFreeOnIPv4(t)
	defer held.Close()

	cfg := testConfig()
	cfg.Port = port
	s := server.NewServer(cfg, alertmanager.NewReceiver(&[]config.Action{}))
	if err := s.Start(); err == nil {
		_ = s.Stop()
		t.Fatal("expected Start to fail while the IPv6 port is taken")
	}

	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	if err != nil {
		t.Fatalf("IPv4 listener was left open after Start failed: %v", err)
	}
	l.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatal("IPv4 server still serving after Start failed")
	}
}

// holdIPv6PortFreeOnIPv4 listens on an IPv6 loopback port whose IPv4
// loopback twin is free, so the test can tell a leaked IPv4 listener from an
// unrelated process.
func holdIPv6PortFreeOnIPv4(t *testing.T) (net.Listener, uint16) {
	t.Helper()
	var lc net.ListenConfig
	for range 20 {
		held, err := lc.Listen(context.Background(), "tcp6", "[::1]:0")
		if err != nil {
			t.Skipf("no IPv6 loopback: %v", err)
		}
		addr, err := netip.ParseAddrPort(held.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		port := addr.Port()
		probe, err := lc.Listen(context.Background(), "tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
		if err == nil {
			probe.Close()
			return held, port
		}
		held.Close()
	}
	t.Fatal("no port free on both IPv4 and IPv6 loopback")
	return nil, 0
}
