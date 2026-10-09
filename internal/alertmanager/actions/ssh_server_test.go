package actions_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/actions"
	"golang.org/x/crypto/ssh"
)

type sshServer struct {
	port    string
	hostKey ssh.PublicKey

	mu       sync.Mutex
	commands []string
}

func (s *sshServer) ran() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

// startSSHServer runs an SSH server that records exec requests. Commands
// named "hang" never exit; any other command exits 0.
func startSSHServer(t *testing.T) *sshServer {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	conf := &ssh.ServerConfig{
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	conf.AddHostKey(signer)

	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	_, port, _ := net.SplitHostPort(l.Addr().String())
	srv := &sshServer{port: port, hostKey: signer.PublicKey()}

	go func() {
		for {
			nc, err := l.Accept()
			if err != nil {
				return
			}
			go srv.serve(nc, conf)
		}
	}()
	return srv
}

func (s *sshServer) serve(nc net.Conn, conf *ssh.ServerConfig) {
	defer nc.Close()
	_, chans, reqs, err := ssh.NewServerConn(nc, conf)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nch := range chans {
		ch, chReqs, err := nch.Accept()
		if err != nil {
			return
		}
		go func() {
			defer ch.Close()
			for req := range chReqs {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				cmd := string(req.Payload[4:])
				s.mu.Lock()
				s.commands = append(s.commands, cmd)
				s.mu.Unlock()
				_ = req.Reply(true, nil)
				if cmd == "hang" {
					continue
				}
				_, _ = ch.Write([]byte("ran " + cmd + "\n"))
				status := make([]byte, 4)
				binary.BigEndian.PutUint32(status, 0)
				_, _ = ch.SendRequest("exit-status", false, status)
				return
			}
		}()
	}
}

func (s *sshServer) knownHostsLine() string {
	return "[127.0.0.1]:" + s.port + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.hostKey)))
}

func (s *sshServer) options(t *testing.T, command, hostKeys string) map[string]string {
	t.Helper()
	return dialOptions(s.port, writeKey(t), command, hostKeys)
}

func TestSSHRunsCommandWithKnownHostKey(t *testing.T) {
	t.Parallel()
	srv := startSSHServer(t)
	err := (&actions.SSH{}).Execute(context.Background(), nil, srv.options(t, "restart tunnel", srv.knownHostsLine()))
	if err != nil {
		t.Fatal(err)
	}
	if got := srv.ran(); len(got) != 1 || got[0] != "restart tunnel" {
		t.Errorf("commands run = %q, want [\"restart tunnel\"]", got)
	}
}

func TestSSHRejectsUnknownHostKey(t *testing.T) {
	t.Parallel()
	srv := startSSHServer(t)
	other := startSSHServer(t)
	hostKeys := "[127.0.0.1]:" + srv.port + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(other.hostKey)))
	if err := (&actions.SSH{}).Execute(context.Background(), nil, srv.options(t, "true", hostKeys)); err == nil {
		t.Fatal("expected a host key mismatch error")
	}
	if got := srv.ran(); len(got) != 0 {
		t.Errorf("commands ran despite a host key mismatch: %q", got)
	}
}

func TestSSHCancelStopsRunningCommand(t *testing.T) {
	t.Parallel()
	srv := startSSHServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := executeWithin(ctx, t, &actions.SSH{}, srv.options(t, "hang", "ignore")); err == nil {
		t.Fatal("expected an error when the context ends during the command")
	}
}
