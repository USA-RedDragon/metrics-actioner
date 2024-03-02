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

type Server struct {
	ipv4Server *http.Server
	ipv6Server *http.Server
	stopped    bool
	config     *config.Metrics
}

func NewServer(config *config.Metrics) *Server {
	return &Server{
		ipv4Server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", config.IPV4Host, config.Port),
			ReadHeaderTimeout: 5 * time.Second,
		},
		ipv6Server: &http.Server{
			Addr:              fmt.Sprintf("[%s]:%d", config.IPV6Host, config.Port),
			ReadHeaderTimeout: 5 * time.Second,
		},
		config: config,
	}
}

func (s *Server) Start() {
	http.Handle("/metrics", promhttp.Handler())

	errGrp := errgroup.Group{}
	errGrp.Go(func() error {
		return s.ipv4Server.ListenAndServe()
	})

	errGrp.Go(func() error {
		return s.ipv6Server.ListenAndServe()
	})

	slog.Info("Metrics server started", "ipv4", s.config.IPV4Host, "ipv6", s.config.IPV6Host, "port", s.config.Port)

	err := errGrp.Wait()
	if err != nil && !s.stopped {
		slog.Error("Metrics server error", "error", err.Error())
	}
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.stopped = true

	errGrp := errgroup.Group{}
	if s.ipv4Server != nil {
		errGrp.Go(func() error {
			return s.ipv4Server.Shutdown(ctx)
		})
	}
	if s.ipv6Server != nil {
		errGrp.Go(func() error {
			return s.ipv6Server.Shutdown(ctx)
		})
	}

	return errGrp.Wait()
}
