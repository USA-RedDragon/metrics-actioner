package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

//nolint:golint,gochecknoglobals
var (
	ipv4Server *http.Server
	ipv6Server *http.Server
)

func CreateServer(config config.ConfigMetrics) {
	http.Handle("/metrics", promhttp.Handler())
	ipv4Server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", config.IPV4Host, config.Port),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ipv6Server = &http.Server{
		Addr:              fmt.Sprintf("[%s]:%d", config.IPV6Host, config.Port),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errGrp := errgroup.Group{}
	errGrp.Go(func() error {
		return ipv4Server.ListenAndServe()
	})

	errGrp.Go(func() error {
		return ipv6Server.ListenAndServe()
	})

	slog.Info("Metrics server started", "ipv4", config.IPV4Host, "ipv6", config.IPV6Host, "port", config.Port)

	err := errGrp.Wait()
	if err != nil {
		slog.Error("Metrics server error", "error", err.Error())
	}
}

func Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errGrp := errgroup.Group{}
	if ipv4Server != nil {
		errGrp.Go(func() error {
			return ipv4Server.Shutdown(ctx)
		})
	}
	if ipv6Server != nil {
		errGrp.Go(func() error {
			return ipv6Server.Shutdown(ctx)
		})
	}

	return errGrp.Wait()
}
