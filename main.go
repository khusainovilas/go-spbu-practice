// @title           Movie Service API
// @version         1.0
// @description     HTTP and gRPC API over the external movie stand.
// @description     HTTP: GET /load-movies, GET /last-movie. gRPC: MovieService/LoadMovies, MovieService/GetLastMovie.
// @BasePath        /
// @schemes         http
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	moviev1 "github.com/khusainovilas/go-spbu-practice/api/movie/v1"
	_ "github.com/khusainovilas/go-spbu-practice/docs"
	"github.com/khusainovilas/go-spbu-practice/internal/config"
	"github.com/khusainovilas/go-spbu-practice/internal/service"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
	"github.com/khusainovilas/go-spbu-practice/internal/transport/grpcapi"
	"github.com/khusainovilas/go-spbu-practice/internal/transport/httpapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(logger); err != nil {
		logger.Error("service stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger.Info("configuration loaded",
		slog.String("movie_base_url", cfg.MovieBaseURL),
		slog.String("http_addr", cfg.HTTPAddr),
		slog.String("grpc_addr", cfg.GRPCAddr),
		slog.Duration("movie_request_timeout", cfg.MovieRequestTimeout),
		slog.Int("movie_workers", cfg.MovieWorkers),
	)

	standClient, err := stand.NewClient(cfg.MovieBaseURL, cfg.MovieRequestTimeout)
	if err != nil {
		return fmt.Errorf("create movie stand client: %w", err)
	}
	movieService := service.New(standClient, cfg.MovieWorkers)

	httpHandler := httpapi.New(movieService, logger).Routes()
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen on gRPC address %s: %w", cfg.GRPCAddr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcapi.RecoveryUnaryInterceptor(logger),
			grpcapi.LoggingUnaryInterceptor(logger),
		),
	)
	moviev1.RegisterMovieServiceServer(grpcServer, grpcapi.NewServer(movieService, logger))
	reflection.Register(grpcServer)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	serverErrors := make(chan error, 2)
	go func() {
		logger.Info("HTTP server started",
			slog.String("addr", cfg.HTTPAddr),
			slog.String("swagger", httpBaseURL(cfg.HTTPAddr)+"/swagger/index.html"),
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("HTTP server: %w", err)
		}
	}()
	go func() {
		logger.Info("gRPC server started", slog.String("addr", cfg.GRPCAddr))
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErrors <- fmt.Errorf("gRPC server: %w", err)
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-serverErrors:
		logger.Error("server failed", slog.Any("error", runErr))
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("HTTP server shutdown failed", slog.Any("error", err))
		if runErr == nil {
			runErr = fmt.Errorf("shutdown HTTP server: %w", err)
		}
	}

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
		logger.Info("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("gRPC graceful shutdown timed out, forcing stop")
		grpcServer.Stop()
		<-grpcStopped
	}

	logger.Info("service stopped")
	return runErr
}

func httpBaseURL(addr string) string {
	switch {
	case strings.HasPrefix(addr, ":"):
		return "http://localhost" + addr
	case strings.HasPrefix(addr, "0.0.0.0:"):
		return "http://localhost:" + strings.TrimPrefix(addr, "0.0.0.0:")
	case strings.HasPrefix(addr, "[::]:"):
		return "http://localhost:" + strings.TrimPrefix(addr, "[::]:")
	default:
		return "http://" + addr
	}
}
