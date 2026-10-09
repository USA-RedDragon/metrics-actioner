package actions

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"golang.org/x/crypto/ssh"
)

type SSHOptionHostKey string

const (
	SSHOptionHostKeyIgnore SSHOptionHostKey = "ignore"
)

// Names of the ssh action's options.
const (
	SSHOptionCommand  = "command"
	SSHOptionHost     = "host"
	SSHOptionPort     = "port"
	SSHOptionUser     = "user"
	SSHOptionKey      = "key"
	SSHOptionHostKeys = "hostKeys"
)

const (
	defaultSSHPort           = 22
	defaultSSHConnectTimeout = 30 * time.Second
)

type SSH struct {
	// ConnectTimeout bounds the TCP connect and SSH handshake. Zero means
	// defaultSSHConnectTimeout.
	ConnectTimeout time.Duration
}

type SSHOptions struct {
	Command  string
	Host     string
	Port     uint16
	User     string
	Key      string
	HostKeys SSHOptionHostKey
}

// ParseSSHOptions reads and checks the ssh action's options.
func ParseSSHOptions(options map[string]string) (SSHOptions, error) {
	opts := SSHOptions{Port: defaultSSHPort}

	// Get the options
	for k, v := range options {
		switch k {
		case SSHOptionCommand:
			opts.Command = v
		case SSHOptionHost:
			opts.Host = v
		case SSHOptionPort:
			if v == "" {
				continue
			}
			port, err := strconv.ParseUint(v, 10, 16)
			if err != nil || port == 0 {
				return opts, fmt.Errorf("invalid port option: %s", v)
			}
			opts.Port = uint16(port)
		case SSHOptionUser:
			opts.User = v
		case SSHOptionKey:
			opts.Key = v
		case SSHOptionHostKeys:
			opts.HostKeys = SSHOptionHostKey(v)
		default:
			slog.Warn("Unknown option", "option", k)
		}
	}
	// Validate the options
	if opts.Command == "" {
		return opts, fmt.Errorf("missing command option")
	}
	if opts.Host == "" {
		return opts, fmt.Errorf("missing host option")
	}
	if opts.User == "" {
		return opts, fmt.Errorf("missing user option")
	}
	if opts.Key == "" {
		return opts, fmt.Errorf("missing key option")
	}
	return opts, nil
}

func (s *SSH) Execute(ctx context.Context, _ *models.Webhook, options map[string]string) error {
	slog.Info("SSH action executed")
	opts, err := ParseSSHOptions(options)
	if err != nil {
		return err
	}
	// Check if key points to a file with a private key
	pemBytes, err := os.ReadFile(opts.Key)
	if err != nil {
		return fmt.Errorf("error reading key file: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return fmt.Errorf("error parsing key: %w", err)
	}

	return s.runCommand(ctx, opts, signer)
}

func (s *SSH) runCommand(ctx context.Context, opts SSHOptions, key ssh.Signer) error {
	slog.Info("Running command", "command", opts.Command, "host", opts.Host, "port", opts.Port, "user", opts.User)

	var hostkeyCallback ssh.HostKeyCallback
	if opts.HostKeys != SSHOptionHostKeyIgnore {
		db := &hostKeyDB{
			revoked: make(map[string]*KnownKey),
		}

		if err := db.Read(strings.NewReader(string(opts.HostKeys)), "known_hosts"); err != nil {
			return err
		}

		var certChecker ssh.CertChecker
		certChecker.IsHostAuthority = db.IsHostAuthority
		certChecker.IsRevoked = db.IsRevoked
		certChecker.HostKeyFallback = db.check

		hostkeyCallback = certChecker.CheckHostKey
	} else {
		hostkeyCallback = func(string, net.Addr, ssh.PublicKey) error {
			return nil
		}
	}

	conf := &ssh.ClientConfig{
		User:            opts.User,
		HostKeyCallback: hostkeyCallback,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	conn, err := s.dial(ctx, net.JoinHostPort(opts.Host, strconv.Itoa(int(opts.Port))), conf)
	if err != nil {
		return fmt.Errorf("error dialing: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}
	defer session.Close()

	// Run the command, redirecting the output to the logger
	session.Stdout = &stdToSlogInfoWriter{}
	session.Stderr = &stdToSlogErrWriter{}

	err = session.Run(opts.Command)
	if ctx.Err() != nil {
		return fmt.Errorf("error running command: %w", context.Cause(ctx))
	}
	if err != nil {
		return fmt.Errorf("error running command: %w", err)
	}

	return nil
}

// dial connects and completes the SSH handshake within ConnectTimeout,
// stopping early if ctx ends.
func (s *SSH) dial(ctx context.Context, addr string, conf *ssh.ClientConfig) (*ssh.Client, error) {
	timeout := s.ConnectTimeout
	if timeout == 0 {
		timeout = defaultSSHConnectTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, conf)
	if !stop() {
		if err == nil {
			_ = c.Close()
		}
		return nil, context.Cause(ctx)
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = c.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

type stdToSlogInfoWriter struct {
}

func (w *stdToSlogInfoWriter) Write(p []byte) (n int, err error) {
	slog.Info(string(p))
	return len(p), nil
}

type stdToSlogErrWriter struct {
}

func (w *stdToSlogErrWriter) Write(p []byte) (n int, err error) {
	slog.Error(string(p))
	return len(p), nil
}
