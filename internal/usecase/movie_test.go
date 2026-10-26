package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/khusainovilas/go-spbu-practice/internal/domain"
)

type fakeProvider struct {
	movies map[int]domain.Movie
	fail   map[int]error
}

func (f fakeProvider) GetMovie(_ context.Context, id int) (domain.Movie, error) {
	if err := f.fail[id]; err != nil {
		return domain.Movie{}, err
	}
	movie, ok := f.movies[id]
	if !ok {
		return domain.Movie{}, fmt.Errorf("movie %d: %w", id, domain.ErrNotFound)
	}
	return movie, nil
}

func TestLoadMovies(t *testing.T) {
	service := NewService(fakeProvider{movies: map[int]domain.Movie{
		1: {ID: 1, Title: "First"},
		2: {ID: 2, Title: "Second"},
		3: {ID: 3, Title: "Third"},
	}})

	result, err := service.LoadMovies(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("LoadMovies: %v", err)
	}
	if len(result.Movies) != 3 {
		t.Fatalf("got %d movies, want 3", len(result.Movies))
	}
	if len(result.Failed) != 0 {
		t.Fatalf("got %d failures, want 0", len(result.Failed))
	}
}

func TestLoadMoviesPartialFailure(t *testing.T) {
	service := NewService(fakeProvider{
		movies: map[int]domain.Movie{1: {ID: 1}, 3: {ID: 3}},
		fail:   map[int]error{2: fmt.Errorf("stand failed: %w", domain.ErrUpstream)},
	})

	result, err := service.LoadMovies(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("LoadMovies: %v", err)
	}
	if len(result.Movies) != 2 {
		t.Fatalf("got %d movies, want 2", len(result.Movies))
	}
	if len(result.Failed) != 1 || result.Failed[0].ID != 2 {
		t.Fatalf("unexpected failures: %+v", result.Failed)
	}
}

func TestLoadMoviesInvalidRange(t *testing.T) {
	service := NewService(fakeProvider{})

	for _, test := range []struct{ from, to int }{
		{0, 3},
		{1, 0},
		{3, 1},
	} {
		_, err := service.LoadMovies(context.Background(), test.from, test.to)
		if !errors.Is(err, domain.ErrInvalidRange) {
			t.Errorf("LoadMovies(%d, %d): got %v, want ErrInvalidRange", test.from, test.to, err)
		}
	}
}

func TestGetLastMovie(t *testing.T) {
	service := NewService(fakeProvider{movies: map[int]domain.Movie{
		1: {ID: 1},
		2: {ID: 2},
		3: {ID: 3},
	}})

	movie, err := service.GetLastMovie(context.Background())
	if err != nil {
		t.Fatalf("GetLastMovie: %v", err)
	}
	if movie.ID != 3 {
		t.Fatalf("got id %d, want 3", movie.ID)
	}
}

func TestGetLastMovieEmptyStand(t *testing.T) {
	service := NewService(fakeProvider{})

	_, err := service.GetLastMovie(context.Background())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}
