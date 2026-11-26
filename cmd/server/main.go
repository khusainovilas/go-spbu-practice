// @title       Movie Service API
// @version     1.0
// @description HTTP and gRPC API backed by PostgreSQL with Redis search.
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
	"github.com/khusainovilas/go-spbu-practice/internal/adapter/postgres"
	redisstore "github.com/khusainovilas/go-spbu-practice/internal/adapter/redis"
	"github.com/khusainovilas/go-spbu-practice/internal/adapter/stand"
	"github.com/khusainovilas/go-spbu-practice/internal/handler/grpcapi"
	"github.com/khusainovilas/go-spbu-practice/internal/handler/httpapi"
	"github.com/khusainovilas/go-spbu-practice/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	movieBaseURL := env("MOVIE_BASE_URL", "https://homeworksite.site")
	httpAddr := env("HTTP_ADDR", ":8080")
	grpcAddr := env("GRPC_ADDR", ":9090")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("REDIS_ADDR is required")
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelStartup()

	pool, err := postgres.Open(startupCtx, databaseURL)
	if err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	if err := postgres.ApplyMigrations(startupCtx, pool); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	redisClient, err := redisstore.Open(startupCtx, redisAddr)
	if err != nil {
		log.Fatalf("connect to Redis: %v", err)
	}
	defer redisClient.Close()

	standClient := stand.NewClient(movieBaseURL, 5*time.Second)
	movieService := usecase.NewService(standClient, postgres.NewRepository(pool), redisClient)

	if err := movieService.Sync(startupCtx); err != nil {
		log.Fatalf("sync movies: %v", err)
	}
	if err := movieService.RebuildSearchIndex(startupCtx); err != nil {
		log.Fatalf("build search index: %v", err)
	}
	cancelStartup()

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: httpapi.New(movieService, redisClient).Routes(),
	}

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listen gRPC on %s: %v", grpcAddr, err)
	}
	grpcServer := grpc.NewServer()
	moviev1.RegisterMovieServiceServer(grpcServer, grpcapi.New(movieService))
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
