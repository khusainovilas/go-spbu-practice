package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/service"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
	"github.com/khusainovilas/go-spbu-practice/internal/transport/httpapi"
)

func TestEndToEndWithRealStandClient(t *testing.T) {
	const lastID = 5

	standServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(movie.Movie{
			ID:       id,
			Title:    "Movie " + parts[0],
			Year:     "2000",
			Director: "Director",
		})
	}))
	defer standServer.Close()

	standClient, err := stand.NewClient(standServer.URL, time.Second)
	if err != nil {
		t.Fatalf("stand.NewClient() error = %v", err)
	}
	movieService := service.New(standClient, 4)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	apiServer := httptest.NewServer(httpapi.New(movieService, logger).Routes())
	defer apiServer.Close()

	// Range partially beyond the last available movie.
	resp, err := http.Get(apiServer.URL + "/load-movies?from=1&to=6")
	if err != nil {
		t.Fatalf("GET /load-movies: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Failed-Ids"); got != "6" {
		t.Errorf("X-Failed-Ids = %q, want 6", got)
	}
	movies := decodeJSON[[]movie.Movie](t, resp)
	if len(movies) != lastID {
		t.Errorf("len(movies) = %d, want %d", len(movies), lastID)
	}

	// Real binary search through the service and stand client.
	lastResp, err := http.Get(apiServer.URL + "/last-movie")
	if err != nil {
		t.Fatalf("GET /last-movie: %v", err)
	}
	defer lastResp.Body.Close()
	if lastResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", lastResp.StatusCode)
	}
	lastMovie := decodeJSON[movie.Movie](t, lastResp)
	if lastMovie.ID != lastID {
		t.Errorf("last movie id = %d, want %d", lastMovie.ID, lastID)
	}
}
