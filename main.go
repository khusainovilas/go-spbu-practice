// @title       Movie Service API
// @version     1.0
// @description HTTP and gRPC API over an external movie stand.
// @BasePath    /
// @schemes     http
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
	"github.com/khusainovilas/go-spbu-practice/internal/api"
	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	movieBaseURL := env("MOVIE_BASE_URL", "https://homeworksite.site")
	httpAddr := env("HTTP_ADDR", ":8080")
	grpcAddr := env("GRPC_ADDR", ":9090")

	standClient := stand.NewClient(movieBaseURL, 5*time.Second)
	movieService := movie.NewService(standClient)

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: api.NewHTTP(movieService).Routes(),
	}

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listen gRPC on %s: %v", grpcAddr, err)
	}
	grpcServer := grpc.NewServer()
	moviev1.RegisterMovieServiceServer(grpcServer, api.NewGRPC(movieService))
	reflection.Register(grpcServer)

	go func() {
		log.Printf("HTTP on %s, gRPC on %s", httpAddr, grpcAddr)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	grpcServer.GracefulStop()
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
