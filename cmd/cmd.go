package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/USA-RedDragon/metrics-actioner/internal/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/ztrue/shutdown"
)

// actionShutdownTimeout is how long shutdown waits for running actions
// before cancelling them.
const actionShutdownTimeout = 20 * time.Second

var (
	ErrMissingConfig = errors.New("missing configuration")
)

func NewCommand(version, commit string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "metrics-actioner",
		Version: fmt.Sprintf("%s - %s", version, commit),
		Annotations: map[string]string{
			"version": version,
			"commit":  commit,
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	loader := config.NewLoader(cmd.Flags())
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		cfg, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return run(cmd, cfg)
	}
	return cmd
}

func run(cmd *cobra.Command, config *config.Config) error {
	slog.Info("Metrics Actioner", "version", cmd.Annotations["version"], "commit", cmd.Annotations["commit"])

	alertmanagerReceiver, err := alertmanager.NewReceiver(&config.Actions, prometheus.DefaultRegisterer)
	if err != nil {
		return err
	}

	slog.Info("Starting HTTP server")
	server := server.NewServer(&config.HTTP, alertmanagerReceiver)
	err = server.Start()
	if err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	stop := func(_ os.Signal) {
		slog.Info("Shutting down")

		err := server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), actionShutdownTimeout)
		defer cancel()
		err = errors.Join(err, alertmanagerReceiver.Shutdown(ctx))
		if err != nil {
			slog.Error("Shutdown error", "error", err.Error())
			os.Exit(1)
		}
		slog.Info("Shutdown complete")
	}

	shutdown.AddWithParam(stop)
	shutdown.Listen(syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)

	return nil
}
