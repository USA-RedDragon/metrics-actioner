package actions_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/actions"
	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"golang.org/x/crypto/ssh"
)

// silentServer accepts TCP connections and never speaks SSH.
func silentServer(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func writeKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeWithin(t *testing.T, ctx context.Context, s *actions.SSH, port, key string) error {
	t.Helper()
	opts := map[string]string{
		"command":  "true",
		"host":     "127.0.0.1",
		"port":     port,
		"user":     "admin",
		"key":      key,
		"hostKeys": "ignore",
	}
	done := make(chan error, 1)
	go func() { done <- s.Execute(ctx, &models.Webhook{}, opts) }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("ssh action did not return; it hangs on a host that never answers")
		return nil
	}
}

func TestSSHHonorsContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := executeWithin(t, ctx, &actions.SSH{}, silentServer(t), writeKey(t)); err == nil {
		t.Fatal("expected an error from a host that never answers")
	}
}

func TestSSHConnectTimeout(t *testing.T) {
	t.Parallel()
	s := &actions.SSH{ConnectTimeout: 200 * time.Millisecond}
	if err := executeWithin(t, context.Background(), s, silentServer(t), writeKey(t)); err == nil {
		t.Fatal("expected an error from a host that never answers")
	}
}
