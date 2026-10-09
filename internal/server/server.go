package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	ipv4Server        *http.Server
	ipv6Server        *http.Server
	metricsIPV4Server *http.Server
	metricsIPV6Server *http.Server
	stopped           bool
	config            *config.HTTP
}

const defTimeout = 5 * time.Second

func NewServer(config *config.HTTP, receiver *alertmanager.Receiver) *Server {
	gin.SetMode(gin.ReleaseMode)
	if config.PProf.Enabled {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	if config.PProf.Enabled {
		pprof.Register(r)
	}

	writeTimeout := defTimeout
	if config.PProf.Enabled {
		writeTimeout = 60 * time.Second
	}

	applyMiddleware(r, config, "api", receiver)
	applyRoutes(r)

	var metricsIPV4Server *http.Server
	var metricsIPV6Server *http.Server

	if config.Metrics.Enabled {
		metricsRouter := gin.New()
		applyMiddleware(metricsRouter, config, "metrics", receiver)

		metricsRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))
		metricsIPV4Server = &http.Server{
			Addr:              fmt.Sprintf("%s:%d", config.Metrics.IPV4Host, config.Metrics.Port),
			ReadHeaderTimeout: defTimeout,
			WriteTimeout:      writeTimeout,
			Handler:           metricsRouter,
		}
		metricsIPV6Server = &http.Server{
			Addr:              fmt.Sprintf("[%s]:%d", config.Metrics.IPV6Host, config.Metrics.Port),
			ReadHeaderTimeout: defTimeout,
			WriteTimeout:      defTimeout,
			Handler:           metricsRouter,
		}
	}

	return &Server{
		ipv4Server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", config.IPV4Host, config.Port),
			ReadHeaderTimeout: defTimeout,
			WriteTimeout:      writeTimeout,
			Handler:           r,
		},
		ipv6Server: &http.Server{
			Addr:              fmt.Sprintf("[%s]:%d", config.IPV6Host, config.Port),
			ReadHeaderTimeout: defTimeout,
			WriteTimeout:      defTimeout,
			Handler:           r,
		},
		metricsIPV4Server: metricsIPV4Server,
		metricsIPV6Server: metricsIPV6Server,
		config:            config,
	}
}

func (s *Server) Start() error {
	type serve struct {
		name     string
		network  string
		server   *http.Server
		listener net.Listener
	}
	serves := []serve{
		{name: "HTTP IPv4", network: "tcp4", server: s.ipv4Server},
		{name: "HTTP IPv6", network: "tcp6", server: s.ipv6Server},
	}
	if s.config.Metrics.Enabled {
		serves = append(serves,
			serve{name: "Metrics IPv4", network: "tcp4", server: s.metricsIPV4Server},
			serve{name: "Metrics IPv6", network: "tcp6", server: s.metricsIPV6Server},
		)
	}

	var lc net.ListenConfig
	for i := range serves {
		if serves[i].server == nil {
			continue
		}
		l, err := lc.Listen(context.Background(), serves[i].network, serves[i].server.Addr)
		if err != nil {
			for _, opened := range serves[:i] {
				if opened.listener != nil {
					_ = opened.listener.Close()
				}
			}
			return err
		}
		serves[i].listener = l
	}

	for _, sv := range serves {
		if sv.listener == nil {
			continue
		}
		go func() {
			if err := sv.server.Serve(sv.listener); err != nil && !s.stopped {
				slog.Error(sv.name+" server error", "error", err.Error())
			}
		}()
	}

	slog.Info("HTTP server started", "ipv4", s.config.IPV4Host, "ipv6", s.config.IPV6Host, "port", s.config.Port)
	if s.config.Metrics.Enabled {
		slog.Info("Metrics server started", "ipv4", s.config.Metrics.IPV4Host, "ipv6", s.config.Metrics.IPV6Host, "port", s.config.Metrics.Port)
	}
	return nil
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
	if s.metricsIPV4Server != nil {
		errGrp.Go(func() error {
			return s.metricsIPV4Server.Shutdown(ctx)
		})
	}
	if s.metricsIPV6Server != nil {
		errGrp.Go(func() error {
			return s.metricsIPV6Server.Shutdown(ctx)
		})
	}

	return errGrp.Wait()
}
