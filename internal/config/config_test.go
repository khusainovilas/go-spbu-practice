package config

import (
	"testing"
	"time"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"MOVIE_BASE_URL",
		"HTTP_ADDR",
		"GRPC_ADDR",
		"MOVIE_REQUEST_TIMEOUT",
		"MOVIE_WORKERS",
		"SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MovieBaseURL != DefaultMovieBaseURL {
		t.Errorf("MovieBaseURL = %q, want %q", cfg.MovieBaseURL, DefaultMovieBaseURL)
	}
	if cfg.HTTPAddr != DefaultHTTPAddr {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, DefaultHTTPAddr)
	}
	if cfg.GRPCAddr != DefaultGRPCAddr {
		t.Errorf("GRPCAddr = %q, want %q", cfg.GRPCAddr, DefaultGRPCAddr)
	}
	if cfg.MovieRequestTimeout != DefaultMovieRequestTimeout {
		t.Errorf("MovieRequestTimeout = %s, want %s", cfg.MovieRequestTimeout, DefaultMovieRequestTimeout)
	}
	if cfg.MovieWorkers != DefaultMovieWorkers {
		t.Errorf("MovieWorkers = %d, want %d", cfg.MovieWorkers, DefaultMovieWorkers)
	}
}

func TestLoadOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("MOVIE_BASE_URL", "http://localhost:18080/")
	t.Setenv("HTTP_ADDR", ":9091")
	t.Setenv("GRPC_ADDR", ":9092")
	t.Setenv("MOVIE_REQUEST_TIMEOUT", "2s")
	t.Setenv("MOVIE_WORKERS", "3")
	t.Setenv("SHUTDOWN_TIMEOUT", "4s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.MovieBaseURL != "http://localhost:18080" {
		t.Errorf("MovieBaseURL = %q, want trailing slash trimmed", cfg.MovieBaseURL)
	}
	if cfg.HTTPAddr != ":9091" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != ":9092" {
		t.Errorf("GRPCAddr = %q", cfg.GRPCAddr)
	}
	if cfg.MovieRequestTimeout != 2*time.Second {
		t.Errorf("MovieRequestTimeout = %s", cfg.MovieRequestTimeout)
	}
	if cfg.MovieWorkers != 3 {
		t.Errorf("MovieWorkers = %d", cfg.MovieWorkers)
	}
	if cfg.ShutdownTimeout != 4*time.Second {
		t.Errorf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "base URL scheme", key: "MOVIE_BASE_URL", value: "ftp://example.com"},
		{name: "base URL host", key: "MOVIE_BASE_URL", value: "http://"},
		{name: "request timeout", key: "MOVIE_REQUEST_TIMEOUT", value: "soon"},
		{name: "zero request timeout", key: "MOVIE_REQUEST_TIMEOUT", value: "0s"},
		{name: "workers", key: "MOVIE_WORKERS", value: "many"},
		{name: "zero workers", key: "MOVIE_WORKERS", value: "0"},
		{name: "shutdown timeout", key: "SHUTDOWN_TIMEOUT", value: "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			t.Setenv(tt.key, tt.value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() error = nil, want error for %s=%q", tt.key, tt.value)
			}
		})
	}
}
