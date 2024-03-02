package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"

	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/USA-RedDragon/metrics-actioner/internal/server"
	"github.com/spf13/cobra"
	"github.com/ztrue/shutdown"
	"golang.org/x/sync/errgroup"
)

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
		RunE:          run,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	config.RegisterFlags(cmd)
	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	slog.Info("Metrics Actioner", "version", cmd.Annotations["version"], "commit", cmd.Annotations["commit"])

	config, err := config.LoadConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	slog.Info("Starting HTTP server")
	server := server.NewServer(&config.HTTP)
	err = server.Start()
	if err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	stop := func(sig os.Signal) {
		slog.Info("Shutting down")

		errGrp := errgroup.Group{}

		if server != nil {
			errGrp.Go(func() error {
				return server.Stop()
			})
		}

		err := errGrp.Wait()
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
