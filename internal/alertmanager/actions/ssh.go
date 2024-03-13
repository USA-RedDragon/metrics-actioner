package actions

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"golang.org/x/crypto/ssh"
)

type SSHOptionHostKey string

const (
	SSHOptionHostKeyIgnore SSHOptionHostKey = "ignore"
)

type SSH struct {
}

type SSHOptions struct {
	Command  string
	Host     string
	Port     uint16
	User     string
	Key      string
	HostKeys SSHOptionHostKey
}

func (s *SSH) Execute(webhook *models.Webhook, options map[string]string) error {
	slog.Info("SSH action executed")
	var opts SSHOptions

	// Get the options
	for k, v := range options {
		switch k {
		case "command":
			opts.Command = v
		case "host":
			opts.Host = v
		case "port":
			if v == "" {
				v = "22"
			}
			intPort, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid port option: %s", v)
			}
			opts.Port = uint16(intPort)
		case "user":
			opts.User = v
		case "key":
			opts.Key = v
		case "hostKeys":
			opts.HostKeys = SSHOptionHostKey(v)
		default:
			slog.Warn("Unknown option", "option", k)
		}
	}
	// Validate the options
	if opts.Command == "" {
		return fmt.Errorf("missing command option")
	}
	if opts.Host == "" {
		return fmt.Errorf("missing host option")
	}
	if opts.User == "" {
		return fmt.Errorf("missing user option")
	}
	if opts.Key == "" {
		return fmt.Errorf("missing key option")
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

	return s.runCommand(opts, signer)
}

func (s *SSH) runCommand(opts SSHOptions, key ssh.Signer) error {
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
		hostkeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
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

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port), conf)
	if err != nil {
		return fmt.Errorf("error dialing: %w", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}
	defer session.Close()

	// Run the command, redirecting the output to the logger
	session.Stdout = &stdToSlogInfoWriter{}
	session.Stderr = &stdToSlogErrWriter{}

	err = session.Run(opts.Command)
	if err != nil {
		return fmt.Errorf("error running command: %w", err)
	}

	return nil
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
