package api

import (
	"context"
	"errors"

	moviev1 "github.com/khusainovilas/go-spbu-practice/api/movie/v1"
	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCServer implements moviev1.MovieServiceServer.
type GRPCServer struct {
	moviev1.UnimplementedMovieServiceServer

	service *movie.Service
}

// NewGRPC creates a gRPC server.
func NewGRPC(service *movie.Service) *GRPCServer {
	return &GRPCServer{service: service}
}

// LoadMovies loads movies with ids in [from; to].
func (s *GRPCServer) LoadMovies(ctx context.Context, req *moviev1.LoadMoviesRequest) (*moviev1.LoadMoviesResponse, error) {
	result, err := s.service.LoadMovies(ctx, int(req.GetFrom()), int(req.GetTo()))
	if err != nil {
		return nil, grpcStatusForError(err)
	}

	if len(result.Movies) == 0 && len(result.Failed) > 0 {
		return nil, grpcStatusForFailed(result.Failed)
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

// GetLastMovie returns the latest movie available on the stand.
func (s *GRPCServer) GetLastMovie(ctx context.Context, _ *moviev1.GetLastMovieRequest) (*moviev1.Movie, error) {
	item, err := s.service.GetLastMovie(ctx)
	if err != nil {
		return nil, grpcStatusForError(err)
	}
	return toProtoMovie(item), nil
}

func toProtoMovie(item movie.Movie) *moviev1.Movie {
	return &moviev1.Movie{
		Id:          int32(item.ID),
		Title:       item.Title,
		Year:        int32(item.Year),
		Director:    item.Director,
		Description: item.Description,
		Actors:      item.Actors,
		Genres:      item.Genres,
	}
}

// grpcStatusForError converts a service error into a gRPC status.
func grpcStatusForError(err error) error {
	switch {
	case errors.Is(err, movie.ErrInvalidRange):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, movie.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, movie.ErrUpstream):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// grpcStatusForFailed returns the status for a range where every movie failed.
func grpcStatusForFailed(failed []movie.FailedMovie) error {
	for _, item := range failed {
		if !errors.Is(item.Err, movie.ErrNotFound) {
			return grpcStatusForError(item.Err)
		}
	}
	return status.Error(codes.NotFound, "no movies found in the requested range")
}
