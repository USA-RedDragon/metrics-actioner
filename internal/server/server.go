package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	ipv4Server *http.Server
	ipv6Server *http.Server
	stopped    bool
	config     *config.HTTP
}

func NewServer(config *config.HTTP) *Server {
	gin.SetMode(gin.ReleaseMode)
	if config.PProf.Enabled {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	if config.PProf.Enabled {
		pprof.Register(r)
	}

	writeTimeout := 5 * time.Second
	if config.PProf.Enabled {
		writeTimeout = 60 * time.Second
	}

	applyMiddleware(r, config)
	applyRoutes(r)

	return &Server{
		ipv4Server: &http.Server{
			Addr:              fmt.Sprintf("%s:%d", config.IPV4Host, config.Port),
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      writeTimeout,
			Handler:           r,
		},
		ipv6Server: &http.Server{
			Addr:              fmt.Sprintf("[%s]:%d", config.IPV6Host, config.Port),
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      writeTimeout,
			Handler:           r,
		},
		config: config,
	}
}

func (s *Server) Start() {
	errGrp := errgroup.Group{}
	errGrp.Go(func() error {
		return s.ipv4Server.ListenAndServe()
	})

	errGrp.Go(func() error {
		return s.ipv6Server.ListenAndServe()
	})

	slog.Info("HTTP server started", "ipv4", s.config.IPV4Host, "ipv6", s.config.IPV6Host, "port", s.config.Port)

	err := errGrp.Wait()
	if err != nil && !s.stopped {
		slog.Error("HTTP server error", "error", err.Error())
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
