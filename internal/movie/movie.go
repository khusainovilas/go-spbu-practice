// Package movie contains the movie model, errors and business logic.
package movie

import (
	"context"
	"errors"
	"fmt"
)

// Movie is a movie returned by the external stand.
type Movie struct {
	ID          int      `json:"id" example:"1"`
	Title       string   `json:"title" example:"Solaris"`
	Year        int      `json:"year" example:"1972"`
	Director    string   `json:"director" example:"Andrei Tarkovsky"`
	Description string   `json:"description,omitempty" example:"A psychologist visits a space station"`
	Actors      []string `json:"actors,omitempty"`
	Genres      []string `json:"genres,omitempty"`
}

var (
	// ErrNotFound means that the requested movie does not exist.
	ErrNotFound = errors.New("movie not found")
	// ErrInvalidRange means that the from/to arguments are invalid.
	ErrInvalidRange = errors.New("invalid movie range")
	// ErrUpstream means that the external movie stand returned an error.
	ErrUpstream = errors.New("movie stand error")
)

// LastMovieProbeID is the maximum movie id according to the stand contract.
const LastMovieProbeID = 99999

// MovieProvider loads one movie from an external source.
type MovieProvider interface {
	GetMovie(ctx context.Context, id int) (Movie, error)
}

// FailedMovie describes one movie that could not be loaded.
type FailedMovie struct {
	ID  int
	Err error
}

// LoadResult contains successfully loaded movies and per-movie failures.
type LoadResult struct {
	Movies []Movie
	Failed []FailedMovie
}

// Service implements the movie business logic.
type Service struct {
	provider MovieProvider
}

// NewService creates a movie service.
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
		Movies: make([]Movie, 0, to-from+1),
		Failed: make([]FailedMovie, 0),
	}

	for id := from; id <= to; id++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		item, err := s.provider.GetMovie(ctx, id)
		if err != nil {
			result.Failed = append(result.Failed, FailedMovie{ID: id, Err: err})
			continue
		}
		result.Movies = append(result.Movies, item)
	}

	return result, nil
}

func validateRange(from, to int) error {
	switch {
	case from <= 0:
		return fmt.Errorf("%w: from must be greater than zero", ErrInvalidRange)
	case to <= 0:
		return fmt.Errorf("%w: to must be greater than zero", ErrInvalidRange)
	case from > to:
		return fmt.Errorf("%w: from must not be greater than to", ErrInvalidRange)
	}
	return nil
}

// GetLastMovie finds the latest available movie on the stand.
//
// Movie ids are consecutive from 1 to last_id, and id 99999 is the upper
// bound. If it does not exist, binary search finds the last existing id.
func (s *Service) GetLastMovie(ctx context.Context) (Movie, error) {
	last, err := s.provider.GetMovie(ctx, LastMovieProbeID)
	if err == nil {
		return last, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Movie{}, err
	}

	lo, hi := 1, LastMovieProbeID-1
	var found Movie
	foundAny := false

	for lo <= hi {
		if err := ctx.Err(); err != nil {
			return Movie{}, err
		}

		mid := (lo + hi) / 2
		item, err := s.provider.GetMovie(ctx, mid)
		switch {
		case err == nil:
			found = item
			foundAny = true
			lo = mid + 1
		case errors.Is(err, ErrNotFound):
			hi = mid - 1
		default:
			return Movie{}, err
		}
	}

	if !foundAny {
		return Movie{}, fmt.Errorf("%w: the stand has no movies", ErrNotFound)
	}
	return found, nil
}
