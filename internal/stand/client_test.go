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

func newTestClient(t *testing.T, handler http.Handler, timeout time.Duration) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL+"/", timeout)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func TestGetMovieSuccess(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/42/info.0.json" {
			t.Errorf("path = %q, want /42/info.0.json", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"Toy Story","year":"1995","director":"John Lasseter"}`))
	}), time.Second)

	got, err := client.GetMovie(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMovie() error = %v", err)
	}

	if got.ID != 42 {
		t.Errorf("ID = %d, want 42", got.ID)
	}
	if got.Title != "Toy Story" || got.Year != "1995" || got.Director != "John Lasseter" {
		t.Errorf("unexpected movie: %+v", got)
	}
}

func TestGetMovieNotFound(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}), time.Second)

	_, err := client.GetMovie(context.Background(), 99999)
	if !errors.Is(err, movie.ErrNotFound) {
		t.Fatalf("GetMovie() error = %v, want movie.ErrNotFound", err)
	}
}

func TestGetMovieStatusError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}), time.Second)

	_, err := client.GetMovie(context.Background(), 7)
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("GetMovie() error = %v, want *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusInternalServerError || statusErr.MovieID != 7 {
		t.Errorf("unexpected StatusError: %+v", statusErr)
	}
}

func TestGetMovieInvalidJSON(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"title":`))
	}), time.Second)

	_, err := client.GetMovie(context.Background(), 1)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("GetMovie() error = %v, want ErrInvalidResponse", err)
	}
}

func TestGetMovieTimeout(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
	}), 20*time.Millisecond)

	_, err := client.GetMovie(context.Background(), 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetMovie() error = %v, want context.DeadlineExceeded", err)
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("GetMovie() error = %v, want ErrUnavailable", err)
	}
}

func TestGetMovieContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}), time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetMovie(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetMovie() error = %v, want context.Canceled", err)
	}
}
