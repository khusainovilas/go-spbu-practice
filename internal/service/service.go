// Package service implements movie business logic on top of the stand client.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
)

const (
	// LastMovieProbeID is the largest movie id according to the stand contract.
	LastMovieProbeID = 99999
	// MaxLoadRange limits one load request to a sane number of stand calls.
	// It covers the whole id space of the stand (1..99999).
	MaxLoadRange   = 100_000
	defaultWorkers = 8
)

// MovieClient is the external movie source used by the service.
type MovieClient interface {
	GetMovie(ctx context.Context, id int) (movie.Movie, error)
}

// Service provides movie operations.
type Service struct {
	client  MovieClient
	workers int
}

// New creates a movie service. Non-positive workers defaults to 8.
func New(client MovieClient, workers int) *Service {
	if workers <= 0 {
		workers = defaultWorkers
	}
	return &Service{client: client, workers: workers}
}

// LoadResult contains successfully loaded movies and per-movie errors.
type LoadResult struct {
	Movies []movie.Movie
	Errors []movie.LoadError
}

// Partial reports whether the range was loaded only partially.
func (r LoadResult) Partial() bool {
	return len(r.Movies) > 0 && len(r.Errors) > 0
}

// ValidateRange validates from/to arguments shared by HTTP and gRPC transports.
func ValidateRange(from, to int) error {
	if from <= 0 {
		return &movie.ValidationError{Field: "from", Message: "must be greater than zero"}
	}
	if to <= 0 {
		return &movie.ValidationError{Field: "to", Message: "must be greater than zero"}
	}
	if from > to {
		return &movie.ValidationError{Field: "range", Message: "from must not be greater than to"}
	}
	if to-from+1 > MaxLoadRange {
		return &movie.ValidationError{Field: "range", Message: fmt.Sprintf("must contain at most %d ids", MaxLoadRange)}
	}
	return nil
}

// LoadMovies loads movies with ids in [from; to]. Errors for individual ids do
// not abort the whole range; they are returned in LoadResult.Errors.
func (s *Service) LoadMovies(ctx context.Context, from, to int) (LoadResult, error) {
	if err := ValidateRange(from, to); err != nil {
		return LoadResult{}, err
	}

	total := to - from + 1
	result := LoadResult{
		Movies: make([]movie.Movie, 0, total),
		Errors: make([]movie.LoadError, 0),
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type jobResult struct {
		id    int
		movie movie.Movie
		err   error
	}

	jobs := make(chan int)
	results := make(chan jobResult, s.workers)

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				m, err := s.client.GetMovie(workerCtx, id)
				select {
				case results <- jobResult{id: id, movie: m, err: err}:
				case <-workerCtx.Done():
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for id := from; id <= to; id++ {
			select {
			case jobs <- id:
			case <-workerCtx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for item := range results {
		if item.err != nil {
			result.Errors = append(result.Errors, movie.LoadError{ID: item.id, Err: item.err})
			continue
		}
		result.Movies = append(result.Movies, item.movie)
	}

	sort.Slice(result.Movies, func(i, j int) bool {
		return result.Movies[i].ID < result.Movies[j].ID
	})
	sort.Slice(result.Errors, func(i, j int) bool {
		return result.Errors[i].ID < result.Errors[j].ID
	})

	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// GetLastMovie finds the latest available movie on the stand.
//
// Movie ids are consecutive from 1 to last_id. A 404 for id 99999 guarantees
// that there are no movies with greater ids, so the search is bounded by it.
func (s *Service) GetLastMovie(ctx context.Context) (movie.Movie, error) {
	probe, err := s.client.GetMovie(ctx, LastMovieProbeID)
	if err == nil {
		return probe, nil
	}
	if !errors.Is(err, movie.ErrNotFound) {
		return movie.Movie{}, fmt.Errorf("probe last movie id %d: %w", LastMovieProbeID, err)
	}

	lo, hi := 1, LastMovieProbeID-1
	lastID := 0
	var lastMovie movie.Movie

	for lo <= hi {
		if err := ctx.Err(); err != nil {
			return movie.Movie{}, err
		}

		mid := lo + (hi-lo)/2
		found, err := s.client.GetMovie(ctx, mid)
		switch {
		case err == nil:
			lastID = mid
			lastMovie = found
			lo = mid + 1
		case errors.Is(err, movie.ErrNotFound):
			hi = mid - 1
		default:
			return movie.Movie{}, fmt.Errorf("find last movie at id %d: %w", mid, err)
		}
	}

	if lastID == 0 {
		return movie.Movie{}, fmt.Errorf("last movie: %w", movie.ErrNotFound)
	}
	return lastMovie, nil
}
