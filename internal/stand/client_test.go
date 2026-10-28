package stand

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
)

func TestGetMovie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1/info.0.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"title":"Solaris","year":1972,"director":"Andrei Tarkovsky"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	item, err := client.GetMovie(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetMovie: %v", err)
	}
	if item.ID != 1 || item.Title != "Solaris" || item.Year != 1972 {
		t.Fatalf("unexpected movie: %+v", item)
	}
}

func TestGetMovieNotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	_, err := client.GetMovie(context.Background(), 2)
	if !errors.Is(err, movie.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestGetMovieUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	_, err := client.GetMovie(context.Background(), 1)
	if !errors.Is(err, movie.ErrUpstream) {
		t.Fatalf("got %v, want ErrUpstream", err)
	}
}
