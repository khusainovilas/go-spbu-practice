package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
)

type fakeClient struct {
	mu         sync.Mutex
	movies     map[int]movie.Movie
	errs       map[int]error
	defaultErr error
	calls      []int
}

func newFakeClient(lastID int) *fakeClient {
	client := &fakeClient{
		movies: make(map[int]movie.Movie),
		errs:   make(map[int]error),
	}
	for id := 1; id <= lastID; id++ {
		client.movies[id] = movie.Movie{
			ID:       id,
			Title:    fmt.Sprintf("Movie %d", id),
			Year:     "2000",
			Director: "Director",
		}
	}
	return client
}

func (f *fakeClient) GetMovie(ctx context.Context, id int) (movie.Movie, error) {
	if err := ctx.Err(); err != nil {
		return movie.Movie{}, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)

	if err, ok := f.errs[id]; ok {
		return movie.Movie{}, err
	}
	if f.defaultErr != nil {
		return movie.Movie{}, f.defaultErr
	}
	if item, ok := f.movies[id]; ok {
		return item, nil
	}
	return movie.Movie{}, fmt.Errorf("movie %d: %w", id, movie.ErrNotFound)
}

func (f *fakeClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestLoadMoviesSuccessInOrder(t *testing.T) {
	client := newFakeClient(5)
	svc := New(client, 4)

	result, err := svc.LoadMovies(context.Background(), 1, 5)
	if err != nil {
		t.Fatalf("LoadMovies() error = %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("Errors = %v, want none", result.Errors)
	}
	if len(result.Movies) != 5 {
		t.Fatalf("len(Movies) = %d, want 5", len(result.Movies))
	}
	for i, item := range result.Movies {
		if item.ID != i+1 {
			t.Errorf("Movies[%d].ID = %d, want %d", i, item.ID, i+1)
		}
	}
	if result.Partial() {
		t.Error("Partial() = true, want false")
	}
}

func TestLoadMoviesPartialErrors(t *testing.T) {
	client := newFakeClient(5)
	client.errs[2] = &stand.StatusError{MovieID: 2, StatusCode: 500, Status: "500 Internal Server Error"}
	client.errs[4] = fmt.Errorf("movie 4: %w", movie.ErrNotFound)
	svc := New(client, 2)

	result, err := svc.LoadMovies(context.Background(), 1, 5)
	if err != nil {
		t.Fatalf("LoadMovies() error = %v", err)
	}
	if len(result.Movies) != 3 {
		t.Fatalf("len(Movies) = %d, want 3", len(result.Movies))
	}
	if len(result.Errors) != 2 {
		t.Fatalf("len(Errors) = %d, want 2", len(result.Errors))
	}
	if result.Errors[0].ID != 2 || result.Errors[1].ID != 4 {
		t.Errorf("error ids = [%d, %d], want [2, 4]", result.Errors[0].ID, result.Errors[1].ID)
	}
	if !result.Partial() {
		t.Error("Partial() = false, want true")
	}
}

func TestValidateRange(t *testing.T) {
	tests := []struct {
		name string
		from int
		to   int
		ok   bool
	}{
		{name: "valid", from: 1, to: 10, ok: true},
		{name: "same id", from: 5, to: 5, ok: true},
		{name: "zero from", from: 0, to: 10},
		{name: "negative from", from: -1, to: 10},
		{name: "zero to", from: 1, to: 0},
		{name: "reversed", from: 10, to: 1},
		{name: "too large", from: 1, to: MaxLoadRange + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRange(tt.from, tt.to)
			if tt.ok && err != nil {
				t.Fatalf("ValidateRange() error = %v", err)
			}
			if !tt.ok {
				var validationErr *movie.ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("ValidateRange() error = %v, want *movie.ValidationError", err)
				}
			}
		})
	}
}

func TestLoadMoviesCanceledContext(t *testing.T) {
	svc := New(newFakeClient(100), 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.LoadMovies(ctx, 1, 100)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadMovies() error = %v, want context.Canceled", err)
	}
}

func TestGetLastMovieBinarySearch(t *testing.T) {
	client := newFakeClient(10)
	svc := New(client, 4)

	got, err := svc.GetLastMovie(context.Background())
	if err != nil {
		t.Fatalf("GetLastMovie() error = %v", err)
	}
	if got.ID != 10 {
		t.Errorf("ID = %d, want 10", got.ID)
	}
	if calls := client.callCount(); calls > 20 {
		t.Errorf("call count = %d, want at most 20", calls)
	}
	if !contains(client.calls, LastMovieProbeID) {
		t.Errorf("calls %v do not contain probe id %d", client.calls, LastMovieProbeID)
	}
}

func TestGetLastMovieAtUpperBound(t *testing.T) {
	client := newFakeClient(LastMovieProbeID)
	svc := New(client, 1)

	got, err := svc.GetLastMovie(context.Background())
	if err != nil {
		t.Fatalf("GetLastMovie() error = %v", err)
	}
	if got.ID != LastMovieProbeID {
		t.Errorf("ID = %d, want %d", got.ID, LastMovieProbeID)
	}
	if calls := client.callCount(); calls != 1 {
		t.Errorf("call count = %d, want 1", calls)
	}
}

func TestGetLastMovieEmptyStand(t *testing.T) {
	svc := New(newFakeClient(0), 1)

	_, err := svc.GetLastMovie(context.Background())
	if !errors.Is(err, movie.ErrNotFound) {
		t.Fatalf("GetLastMovie() error = %v, want movie.ErrNotFound", err)
	}
}

func TestGetLastMovieUpstreamError(t *testing.T) {
	client := newFakeClient(10)
	client.defaultErr = stand.ErrUnavailable
	svc := New(client, 1)

	_, err := svc.GetLastMovie(context.Background())
	if !errors.Is(err, stand.ErrUnavailable) {
		t.Fatalf("GetLastMovie() error = %v, want stand.ErrUnavailable", err)
	}
}

func contains(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
