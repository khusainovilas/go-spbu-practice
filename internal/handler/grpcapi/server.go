package grpcapi

import (
	"context"
	"errors"

	moviev1 "github.com/khusainovilas/go-spbu-practice/api/movie/v1"
	"github.com/khusainovilas/go-spbu-practice/internal/domain"
	"github.com/khusainovilas/go-spbu-practice/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	moviev1.UnimplementedMovieServiceServer

	service usecase.MovieUseCase
}

func New(service usecase.MovieUseCase) *Server {
	return &Server{service: service}
}

func (s *Server) LoadMovies(ctx context.Context, req *moviev1.LoadMoviesRequest) (*moviev1.LoadMoviesResponse, error) {
	result, err := s.service.LoadMovies(ctx, int(req.GetFrom()), int(req.GetTo()))
	if err != nil {
		return nil, statusForError(err)
	}

	if len(result.Movies) == 0 && len(result.Failed) > 0 {
		return nil, statusForFailed(result.Failed)
	}

	response := &moviev1.LoadMoviesResponse{
		Movies: make([]*moviev1.Movie, 0, len(result.Movies)),
		Errors: make([]*moviev1.MovieError, 0, len(result.Failed)),
	}
	for _, item := range result.Movies {
		response.Movies = append(response.Movies, toProtoMovie(item))
	}
	for _, item := range result.Failed {
		response.Errors = append(response.Errors, &moviev1.MovieError{
			Id:      int32(item.ID),
			Message: item.Err.Error(),
		})
	}
	return response, nil
}

func (s *Server) GetLastMovie(ctx context.Context, _ *moviev1.GetLastMovieRequest) (*moviev1.Movie, error) {
	item, err := s.service.GetLastMovie(ctx)
	if err != nil {
		return nil, statusForError(err)
	}
	return toProtoMovie(item), nil
}

func toProtoMovie(item domain.Movie) *moviev1.Movie {
	return &moviev1.Movie{
		Id:       int32(item.ID),
		Title:    item.Title,
		Year:     int32(item.Year),
		Director: item.Director,
	}
}

func statusForError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidRange):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrUpstream):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func statusForFailed(failed []domain.FailedMovie) error {
	for _, item := range failed {
		if !errors.Is(item.Err, domain.ErrNotFound) {
			return statusForError(item.Err)
		}
	}
	return status.Error(codes.NotFound, "no movies found in the requested range")
}
