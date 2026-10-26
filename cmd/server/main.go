// @title           Movie Service API
// @version         1.0
// @description     HTTP and gRPC API over an external movie stand.
// @BasePath        /
// @schemes         http
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	moviev1 "github.com/khusainovilas/go-spbu-practice/api/movie/v1"
	_ "github.com/khusainovilas/go-spbu-practice/docs"
	"github.com/khusainovilas/go-spbu-practice/internal/adapter/stand"
	"github.com/khusainovilas/go-spbu-practice/internal/handler/grpcapi"
	"github.com/khusainovilas/go-spbu-practice/internal/handler/httpapi"
	"github.com/khusainovilas/go-spbu-practice/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const requestTimeout = 5 * time.Second

func main() {
	cfg := loadConfig()

	// Dependency wiring: adapter -> usecase -> handlers.
	standClient := stand.NewClient(cfg.movieBaseURL, requestTimeout)
	movieService := usecase.NewService(standClient)

	httpServer := &http.Server{
		Addr:    cfg.httpAddr,
		Handler: httpapi.New(movieService).Routes(),
	}

	listener, err := net.Listen("tcp", cfg.grpcAddr)
	if err != nil {
		log.Fatalf("listen gRPC on %s: %v", cfg.grpcAddr, err)
	}
	grpcServer := grpc.NewServer()
	moviev1.RegisterMovieServiceServer(grpcServer, grpcapi.New(movieService))
	reflection.Register(grpcServer)

	go func() {
		log.Printf("HTTP server on %s (Swagger: /swagger/index.html), gRPC on %s", cfg.httpAddr, cfg.grpcAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server: %v", err)
		}
	}()
	go func() {
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Fatalf("gRPC server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}
	grpcServer.GracefulStop()
}

type config struct {
	movieBaseURL string
	httpAddr     string
	grpcAddr     string
}

func loadConfig() config {
	return config{
		movieBaseURL: env("MOVIE_BASE_URL", "https://homeworksite.site"),
		httpAddr:     env("HTTP_ADDR", ":8080"),
		grpcAddr:     env("GRPC_ADDR", ":9090"),
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
