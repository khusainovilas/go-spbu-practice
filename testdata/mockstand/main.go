// Command mockstand runs a small local imitation of the external movie stand.
// It is used only for manual testing and is ignored by the Go tooling because
// it lives in the testdata directory.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type movie struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Year        int      `json:"year"`
	Director    string   `json:"director"`
	Description string   `json:"description"`
	Actors      []string `json:"actors"`
	Genres      []string `json:"genres"`
}

func main() {
	addr := env("MOCK_STAND_ADDR", ":18080")
	lastID, err := strconv.Atoi(env("MOCK_STAND_LAST_ID", "10"))
	if err != nil || lastID < 0 {
		log.Fatalf("MOCK_STAND_LAST_ID must be a non-negative integer")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(movie{
			ID:          id,
			Title:       fmt.Sprintf("Mock Movie %d", id),
			Year:        1990 + id%30,
			Director:    fmt.Sprintf("Director %d", id),
			Description: "Mock description",
			Actors:      []string{"Mock Actor"},
			Genres:      []string{"mock"},
		})
	})

	log.Printf("mock movie stand on %s (last_id=%d)", addr, lastID)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
