// Package usecase contains the business logic of the movie service.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/khusainovilas/go-spbu-practice/internal/domain"
)

// LastMovieProbeID is the maximum movie id according to the stand contract.
const LastMovieProbeID = 99999

// MovieProvider loads one movie from an external source.
//
// The interface lives in the usecase layer, and the real implementation
// (adapter/stand) is connected in cmd/server. This keeps business logic
// independent of HTTP and the external site.
type MovieProvider interface {
	GetMovie(ctx context.Context, id int) (domain.Movie, error)
}

// MovieUseCase is the business logic used by the HTTP and gRPC handlers.
type MovieUseCase interface {
	LoadMovies(ctx context.Context, from, to int) (LoadResult, error)
	GetLastMovie(ctx context.Context) (domain.Movie, error)
}

// FailedMovie describes one movie that could not be loaded.
type FailedMovie struct {
	ID  int
	Err error
}

// LoadResult contains successfully loaded movies and per-movie failures.
type LoadResult struct {
	Movies []domain.Movie
	Failed []FailedMovie
}

// Service implements MovieUseCase.
type Service struct {
	provider MovieProvider
}

// NewService creates the movie use case.
func NewService(provider MovieProvider) *Service {
	return &Service{provider: provider}
}

// LoadMovies loads every movie id in [from; to]. If one movie fails, the rest
// are still loaded and the failure is returned in LoadResult.Failed.
func (s *Service) LoadMovies(ctx context.Context, from, to int) (LoadResult, error) {
	if err := validateRange(from, to); err != nil {
		return LoadResult{}, err
	}

	result := LoadResult{
		Movies: make([]domain.Movie, 0, to-from+1),
		Failed: make([]FailedMovie, 0),
	}

	for id := from; id <= to; id++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		movie, err := s.provider.GetMovie(ctx, id)
		if err != nil {
			result.Failed = append(result.Failed, FailedMovie{ID: id, Err: err})
			continue
		}
		result.Movies = append(result.Movies, movie)
	}

	return result, nil
}

func validateRange(from, to int) error {
	switch {
	case from <= 0:
		return fmt.Errorf("%w: from must be greater than zero", domain.ErrInvalidRange)
	case to <= 0:
		return fmt.Errorf("%w: to must be greater than zero", domain.ErrInvalidRange)
	case from > to:
		return fmt.Errorf("%w: from must not be greater than to", domain.ErrInvalidRange)
	}
	return nil
}

// GetLastMovie finds the latest available movie on the stand.
//
// Movie ids are consecutive from 1 to last_id, and id 99999 is the upper
// bound. If it does not exist, binary search finds the last existing id.
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
		movie, err := s.provider.GetMovie(ctx, mid)
		switch {
		case err == nil:
			found = movie
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
