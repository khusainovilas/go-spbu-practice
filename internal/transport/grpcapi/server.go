// Package grpcapi implements the gRPC transport of the movie service.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"

	moviev1 "github.com/khusainovilas/go-spbu-practice/api/movie/v1"
	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/service"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MovieService is the business logic required by gRPC handlers.
type MovieService interface {
	LoadMovies(ctx context.Context, from, to int) (service.LoadResult, error)
	GetLastMovie(ctx context.Context) (movie.Movie, error)
}

// Server implements moviev1.MovieServiceServer.
type Server struct {
	moviev1.UnimplementedMovieServiceServer

	service MovieService
	logger  *slog.Logger
}

// NewServer creates a gRPC movie server. A nil logger is replaced with slog.Default.
func NewServer(movieService MovieService, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{service: movieService, logger: logger}
}

// LoadMovies loads movies with ids in [from; to].
func (s *Server) LoadMovies(ctx context.Context, req *moviev1.LoadMoviesRequest) (*moviev1.LoadMoviesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	from, to := int(req.GetFrom()), int(req.GetTo())
	if err := service.ValidateRange(from, to); err != nil {
		return nil, toStatusError(err)
	}

	result, err := s.service.LoadMovies(ctx, from, to)
	if err != nil {
		return nil, toStatusError(err)
	}

	if len(result.Movies) == 0 {
		if len(result.Errors) == 0 {
			return &moviev1.LoadMoviesResponse{Movies: []*moviev1.Movie{}}, nil
		}
		return nil, allFailedStatus(result.Errors)
	}

	response := &moviev1.LoadMoviesResponse{
		Movies:  make([]*moviev1.Movie, 0, len(result.Movies)),
		Errors:  make([]*moviev1.MovieError, 0, len(result.Errors)),
		Partial: len(result.Errors) > 0,
	}
	for _, item := range result.Movies {
		response.Movies = append(response.Movies, toProtoMovie(item))
	}
	for _, item := range result.Errors {
		response.Errors = append(response.Errors, &moviev1.MovieError{
			Id:      int32(item.ID),
			Message: item.Err.Error(),
		})
	}

	if response.Partial {
		s.logger.WarnContext(ctx, "movies loaded partially over gRPC",
			"from", from,
			"to", to,
			"loaded", len(result.Movies),
			"failed", len(result.Errors),
		)
	}
	return response, nil
}

// GetLastMovie returns the latest available movie on the stand.
func (s *Server) GetLastMovie(ctx context.Context, req *moviev1.GetLastMovieRequest) (*moviev1.Movie, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	result, err := s.service.GetLastMovie(ctx)
	if err != nil {
		return nil, toStatusError(err)
	}
	return toProtoMovie(result), nil
}

func toProtoMovie(item movie.Movie) *moviev1.Movie {
	return &moviev1.Movie{
		Id:       int32(item.ID),
		Title:    item.Title,
		Year:     item.Year,
		Director: item.Director,
	}
}

func allFailedStatus(errs []movie.LoadError) error {
	allNotFound := true
	var firstErr error
	for _, item := range errs {
		if !errors.Is(item.Err, movie.ErrNotFound) {
			allNotFound = false
			if firstErr == nil {
				firstErr = item.Err
			}
		}
	}
	if allNotFound {
		return status.Error(codes.NotFound, "no movies found in the requested range")
	}
	return toStatusError(firstErr)
}

// toStatusError maps service and stand errors to meaningful gRPC status codes.
func toStatusError(err error) error {
	if err == nil {
		return nil
	}

	var validationErr *movie.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return status.Error(codes.InvalidArgument, validationErr.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, movie.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, stand.ErrInvalidResponse):
		return status.Error(codes.Internal, err.Error())
	case errors.Is(err, stand.ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	}

	var statusErr *stand.StatusError
	if errors.As(err, &statusErr) {
		switch {
		case statusErr.StatusCode == 429:
			return status.Error(codes.ResourceExhausted, err.Error())
		case statusErr.StatusCode >= 500:
			return status.Error(codes.Unavailable, err.Error())
		default:
			return status.Error(codes.FailedPrecondition, err.Error())
		}
	}

	return status.Error(codes.Internal, err.Error())
}
