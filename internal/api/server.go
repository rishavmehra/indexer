package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/rishavmehra/indexer/internal/config"
)

// Server represents the API server
type Server struct {
	router *gin.Engine
	server *http.Server
	cfg    config.ServerConfig
}

// NewServer creates a new API server
func NewServer(cfg config.ServerConfig) *Server {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	return &Server{
		router: router,
		cfg:    cfg,
	}
}

// Router returns the gin router
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Start starts the server
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%s", s.cfg.Port),
		Handler: s.router,
	}

	// Start the server in a goroutine
	go func() {
		log.Info().Str("port", s.cfg.Port).Msg("Starting server")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
		return err
	}

	log.Info().Msg("Server shutdown gracefully")
	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}
