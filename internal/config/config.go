// Package config loads the service configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultMovieBaseURL is the default base URL of the external movie stand.
	DefaultMovieBaseURL = "https://homeworksite.site"
	// DefaultHTTPAddr is the default address of the HTTP server.
	DefaultHTTPAddr = ":8080"
	// DefaultGRPCAddr is the default address of the gRPC server.
	DefaultGRPCAddr = ":9090"
	// DefaultMovieRequestTimeout is the default timeout of one stand request.
	DefaultMovieRequestTimeout = 5 * time.Second
	// DefaultMovieWorkers is the default number of concurrent stand requests.
	DefaultMovieWorkers = 8
	// DefaultShutdownTimeout is the default graceful shutdown timeout.
	DefaultShutdownTimeout = 10 * time.Second
)

// Config is the service configuration.
type Config struct {
	MovieBaseURL        string
	HTTPAddr            string
	GRPCAddr            string
	MovieRequestTimeout time.Duration
	MovieWorkers        int
	ShutdownTimeout     time.Duration
}

// Load reads the configuration from the environment and applies defaults.
//
// Required variables: none. Supported variables:
//   - MOVIE_BASE_URL
//   - HTTP_ADDR
//   - GRPC_ADDR
//   - MOVIE_REQUEST_TIMEOUT
//   - MOVIE_WORKERS
//   - SHUTDOWN_TIMEOUT
func Load() (Config, error) {
	cfg := Config{
		MovieBaseURL:        envOrDefault("MOVIE_BASE_URL", DefaultMovieBaseURL),
		HTTPAddr:            envOrDefault("HTTP_ADDR", DefaultHTTPAddr),
		GRPCAddr:            envOrDefault("GRPC_ADDR", DefaultGRPCAddr),
		MovieRequestTimeout: DefaultMovieRequestTimeout,
		MovieWorkers:        DefaultMovieWorkers,
		ShutdownTimeout:     DefaultShutdownTimeout,
	}

	if err := validateBaseURL(cfg.MovieBaseURL); err != nil {
		return Config{}, fmt.Errorf("MOVIE_BASE_URL: %w", err)
	}
	cfg.MovieBaseURL = strings.TrimRight(cfg.MovieBaseURL, "/")

	if raw := strings.TrimSpace(os.Getenv("MOVIE_REQUEST_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MOVIE_REQUEST_TIMEOUT: invalid duration %q: %w", raw, err)
		}
		if timeout <= 0 {
			return Config{}, fmt.Errorf("MOVIE_REQUEST_TIMEOUT: must be greater than zero")
		}
		cfg.MovieRequestTimeout = timeout
	}

	if raw := strings.TrimSpace(os.Getenv("MOVIE_WORKERS")); raw != "" {
		workers, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("MOVIE_WORKERS: invalid integer %q: %w", raw, err)
		}
		if workers <= 0 {
			return Config{}, fmt.Errorf("MOVIE_WORKERS: must be greater than zero")
		}
		cfg.MovieWorkers = workers
	}

	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		timeout, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: invalid duration %q: %w", raw, err)
		}
		if timeout <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: must be greater than zero")
		}
		cfg.ShutdownTimeout = timeout
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid URL %q: scheme must be http or https", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid URL %q: host is empty", raw)
	}
	return nil
}
