package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/khusainovilas/go-spbu-practice/internal/domain"
)

const (
	defaultWorkers   = 8
	LastMovieProbeID = 99999
)

type MovieProvider interface {
	GetMovie(ctx context.Context, id int) (domain.Movie, error)
}

type MovieUseCase interface {
	LoadMovies(ctx context.Context, from, to int) (domain.LoadResult, error)
	GetLastMovie(ctx context.Context) (domain.Movie, error)
}

type Service struct {
	provider MovieProvider
	workers  int
}

func NewService(provider MovieProvider) *Service {
	return &Service{provider: provider, workers: defaultWorkers}
}

var _ MovieUseCase = (*Service)(nil)

// LoadMovies loads movies in [from; to] using a small worker pool.
func (s *Service) LoadMovies(ctx context.Context, from, to int) (domain.LoadResult, error) {
	if err := validateRange(from, to); err != nil {
		return domain.LoadResult{}, err
	}

	var result domain.LoadResult

	type jobResult struct {
		id    int
		movie domain.Movie
		err   error
	}

	jobs := make(chan int)
	results := make(chan jobResult)

	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				item, err := s.provider.GetMovie(ctx, id)
				select {
				case results <- jobResult{id: id, movie: item, err: err}:
				case <-ctx.Done():
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
			case <-ctx.Done():
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
			result.Failed = append(result.Failed, domain.FailedMovie{ID: item.id, Err: item.err})
			continue
		}
		result.Movies = append(result.Movies, item.movie)
	}

	sort.Slice(result.Movies, func(i, j int) bool {
		return result.Movies[i].ID < result.Movies[j].ID
	})
	sort.Slice(result.Failed, func(i, j int) bool {
		return result.Failed[i].ID < result.Failed[j].ID
	})

	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func validateRange(from, to int) error {
	if from <= 0 {
		return fmt.Errorf("%w: from must be greater than zero", domain.ErrInvalidRange)
	}
	if to <= 0 {
		return fmt.Errorf("%w: to must be greater than zero", domain.ErrInvalidRange)
	}
	if from > to {
		return fmt.Errorf("%w: from must not be greater than to", domain.ErrInvalidRange)
	}
	return nil
}

// GetLastMovie finds the last movie using the consecutive ids rule.
func (s *Service) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	last, err := s.provider.GetMovie(ctx, LastMovieProbeID)
	if err == nil {
		return last, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Movie{}, err
	}

	lo, hi := 1, LastMovieProbeID-1
	var found domain.Movie
	foundAny := false

	for lo <= hi {
		if err := ctx.Err(); err != nil {
			return domain.Movie{}, err
		}

		mid := (lo + hi) / 2
		item, err := s.provider.GetMovie(ctx, mid)
		switch {
		case err == nil:
			found = item
			foundAny = true
			lo = mid + 1
		case errors.Is(err, domain.ErrNotFound):
			hi = mid - 1
		default:
			return domain.Movie{}, err
		}
	}

	if !foundAny {
		return domain.Movie{}, fmt.Errorf("%w: the stand has no movies", domain.ErrNotFound)
	}
	return found, nil
}
