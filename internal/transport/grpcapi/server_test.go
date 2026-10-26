package grpcapi_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	moviev1 "github.com/khusainovilas/go-spbu-practice/api/movie/v1"
	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/service"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
	"github.com/khusainovilas/go-spbu-practice/internal/transport/grpcapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubService struct {
	loadResult service.LoadResult
	loadErr    error
	lastMovie  movie.Movie
	lastErr    error

	gotFrom int
	gotTo   int
}

func (s *stubService) LoadMovies(_ context.Context, from, to int) (service.LoadResult, error) {
	s.gotFrom, s.gotTo = from, to
	return s.loadResult, s.loadErr
}

func (s *stubService) GetLastMovie(_ context.Context) (movie.Movie, error) {
	return s.lastMovie, s.lastErr
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoadMoviesSuccess(t *testing.T) {
	stub := &stubService{
		loadResult: service.LoadResult{
			Movies: []movie.Movie{{ID: 1, Title: "First", Year: "2001", Director: "D1"}},
			Errors: []movie.LoadError{
				{ID: 2, Err: &stand.StatusError{MovieID: 2, StatusCode: 500, Status: "500 Internal Server Error"}},
			},
		},
	}
	server := grpcapi.NewServer(stub, testLogger())

	resp, err := server.LoadMovies(context.Background(), &moviev1.LoadMoviesRequest{From: 1, To: 2})
	if err != nil {
		t.Fatalf("LoadMovies() error = %v", err)
	}
	if stub.gotFrom != 1 || stub.gotTo != 2 {
		t.Errorf("from=%d to=%d, want 1/2", stub.gotFrom, stub.gotTo)
	}
	if len(resp.Movies) != 1 || resp.Movies[0].Id != 1 || resp.Movies[0].Title != "First" {
		t.Errorf("unexpected movies: %v", resp.Movies)
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Id != 2 {
		t.Errorf("unexpected errors: %v", resp.Errors)
	}
	if !resp.Partial {
		t.Error("Partial = false, want true")
	}
}

func TestLoadMoviesInvalidArguments(t *testing.T) {
	tests := []struct {
		name string
		req  *moviev1.LoadMoviesRequest
	}{
		{name: "nil request", req: nil},
		{name: "zero from", req: &moviev1.LoadMoviesRequest{From: 0, To: 2}},
		{name: "zero to", req: &moviev1.LoadMoviesRequest{From: 1, To: 0}},
		{name: "reversed", req: &moviev1.LoadMoviesRequest{From: 5, To: 2}},
		{name: "negative", req: &moviev1.LoadMoviesRequest{From: -1, To: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubService{}
			server := grpcapi.NewServer(stub, testLogger())

			_, err := server.LoadMovies(context.Background(), tt.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %s, want InvalidArgument (err=%v)", status.Code(err), err)
			}
		})
	}
}

func TestLoadMoviesAllNotFound(t *testing.T) {
	stub := &stubService{
		loadResult: service.LoadResult{
			Errors: []movie.LoadError{{ID: 1, Err: fmt.Errorf("movie 1: %w", movie.ErrNotFound)}},
		},
	}
	server := grpcapi.NewServer(stub, testLogger())

	_, err := server.LoadMovies(context.Background(), &moviev1.LoadMoviesRequest{From: 1, To: 1})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want NotFound", status.Code(err))
	}
}

func TestLoadMoviesErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{
			name: "upstream status",
			err:  &stand.StatusError{MovieID: 1, StatusCode: 500, Status: "500 Internal Server Error"},
			want: codes.Unavailable,
		},
		{
			name: "invalid response",
			err:  fmt.Errorf("%w: bad JSON", stand.ErrInvalidResponse),
			want: codes.Internal,
		},
		{
			name: "unavailable",
			err:  fmt.Errorf("%w: dial failed", stand.ErrUnavailable),
			want: codes.Unavailable,
		},
		{
			name: "deadline",
			err:  fmt.Errorf("request: %w", context.DeadlineExceeded),
			want: codes.DeadlineExceeded,
		},
		{
			name: "canceled",
			err:  fmt.Errorf("request: %w", context.Canceled),
			want: codes.Canceled,
		},
		{
			name: "not found",
			err:  fmt.Errorf("movie: %w", movie.ErrNotFound),
			want: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := grpcapi.NewServer(&stubService{loadErr: tt.err}, testLogger())
			_, err := server.LoadMovies(context.Background(), &moviev1.LoadMoviesRequest{From: 1, To: 1})
			if status.Code(err) != tt.want {
				t.Fatalf("code = %s, want %s (err=%v)", status.Code(err), tt.want, err)
			}
		})
	}
}

func TestGetLastMovie(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stub := &stubService{
			lastMovie: movie.Movie{ID: 10, Title: "Last", Year: "2010", Director: "D"},
		}
		server := grpcapi.NewServer(stub, testLogger())

		resp, err := server.GetLastMovie(context.Background(), &moviev1.GetLastMovieRequest{})
		if err != nil {
			t.Fatalf("GetLastMovie() error = %v", err)
		}
		if resp.Id != 10 || resp.Title != "Last" {
			t.Errorf("unexpected movie: %v", resp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		stub := &stubService{lastErr: fmt.Errorf("last movie: %w", movie.ErrNotFound)}
		server := grpcapi.NewServer(stub, testLogger())

		_, err := server.GetLastMovie(context.Background(), &moviev1.GetLastMovieRequest{})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("code = %s, want NotFound", status.Code(err))
		}
	})

	t.Run("nil request", func(t *testing.T) {
		server := grpcapi.NewServer(&stubService{}, testLogger())
		_, err := server.GetLastMovie(context.Background(), nil)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("code = %s, want InvalidArgument", status.Code(err))
		}
	})
}

func TestGRPCEndToEnd(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	stub := &stubService{
		loadResult: service.LoadResult{
			Movies: []movie.Movie{{ID: 1, Title: "First"}},
		},
		lastMovie: movie.Movie{ID: 10, Title: "Last"},
	}
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcapi.RecoveryUnaryInterceptor(testLogger()),
			grpcapi.LoggingUnaryInterceptor(testLogger()),
		),
	)
	moviev1.RegisterMovieServiceServer(server, grpcapi.NewServer(stub, testLogger()))
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := moviev1.NewMovieServiceClient(conn)
	loadResp, err := client.LoadMovies(ctx, &moviev1.LoadMoviesRequest{From: 1, To: 1})
	if err != nil {
		t.Fatalf("LoadMovies() error = %v", err)
	}
	if len(loadResp.Movies) != 1 || loadResp.Movies[0].Id != 1 {
		t.Errorf("unexpected LoadMovies response: %v", loadResp)
	}

	lastResp, err := client.GetLastMovie(ctx, &moviev1.GetLastMovieRequest{})
	if err != nil {
		t.Fatalf("GetLastMovie() error = %v", err)
	}
	if lastResp.Id != 10 {
		t.Errorf("GetLastMovie().Id = %d, want 10", lastResp.Id)
	}

	_, err = client.LoadMovies(ctx, &moviev1.LoadMoviesRequest{From: 0, To: 1})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("invalid range code = %s, want InvalidArgument", status.Code(err))
	}
}
