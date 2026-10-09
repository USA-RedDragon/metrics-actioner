package actions_test

import (
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/actions"
)

const testUser = "admin"

func sshOptions(extra map[string]string) map[string]string {
	opts := map[string]string{
		actions.SSHOptionCommand: "true",
		actions.SSHOptionHost:    "host.example.com",
		actions.SSHOptionUser:    testUser,
		actions.SSHOptionKey:     "/secrets/admin.pem",
	}
	for k, v := range extra {
		opts[k] = v
	}
	return opts
}

func TestSSHPortDefaultsTo22(t *testing.T) {
	t.Parallel()
	for name, extra := range map[string]map[string]string{
		"missing": nil,
		"empty":   {actions.SSHOptionPort: ""},
	} {
		opts, err := actions.ParseSSHOptions(sshOptions(extra))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if opts.Port != 22 {
			t.Errorf("%s port: got %d, want 22", name, opts.Port)
		}
	}
}

func TestSSHPortParsing(t *testing.T) {
	t.Parallel()
	opts, err := actions.ParseSSHOptions(sshOptions(map[string]string{actions.SSHOptionPort: "333"}))
	if err != nil {
		t.Fatal(err)
	}
	if opts.Port != 333 {
		t.Errorf("port: got %d, want 333", opts.Port)
	}
	for _, bad := range []string{"70000", "-1", "0", "ssh"} {
		if _, err := actions.ParseSSHOptions(sshOptions(map[string]string{actions.SSHOptionPort: bad})); err == nil {
			t.Errorf("port %q: expected an error", bad)
		}
	}
}

func TestSSHRequiredOptions(t *testing.T) {
	t.Parallel()
	for _, key := range []string{actions.SSHOptionCommand, actions.SSHOptionHost, actions.SSHOptionUser, actions.SSHOptionKey} {
		opts := sshOptions(nil)
		delete(opts, key)
		if _, err := actions.ParseSSHOptions(opts); err == nil {
			t.Errorf("missing %s: expected an error", key)
		}
	}
}
