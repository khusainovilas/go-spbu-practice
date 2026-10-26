// Command mock-stand runs a tiny local imitation of the external movie stand.
// It is intended for local development and demos only.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
)

func main() {
	addr := envOrDefault("MOCK_STAND_ADDR", ":18080")
	lastID, err := strconv.Atoi(envOrDefault("MOCK_STAND_LAST_ID", "10"))
	if err != nil || lastID < 0 {
		log.Fatalf("invalid MOCK_STAND_LAST_ID: must be a non-negative integer")
	}
	delay, err := time.ParseDuration(envOrDefault("MOCK_STAND_DELAY", "0s"))
	if err != nil || delay < 0 {
		log.Fatalf("invalid MOCK_STAND_DELAY: must be a non-negative duration")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 2 || parts[1] != "info.0.json" {
			http.NotFound(w, r)
			return
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil || id < 1 || id > lastID {
			http.NotFound(w, r)
			return
		}

		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(movie.Movie{
			ID:       id,
			Title:    fmt.Sprintf("Mock Movie %d", id),
			Year:     fmt.Sprintf("%d", 1990+id%30),
			Director: fmt.Sprintf("Director %d", id),
		})
	})

	log.Printf("mock movie stand started on %s (last_id=%d, delay=%s)", addr, lastID, delay)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
