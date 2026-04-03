package server

import (
	"context"
	"net/http"
	"time"

	"hh-parser/pkg/logger"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(addr string, handler http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
		},
	}
}

func (s *Server) Start() {
	go func() {
		logger.Log.Info("Starting HTTP server", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("HTTP server failed", "error", err)
		}
	}()
}

func (s *Server) Shutdown(ctx context.Context) error {
	logger.Log.Info("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}