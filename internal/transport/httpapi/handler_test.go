package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	_ "github.com/khusainovilas/go-spbu-practice/docs"
	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/service"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
	"github.com/khusainovilas/go-spbu-practice/internal/transport/httpapi"
)

type stubService struct {
	loadResult service.LoadResult
	loadErr    error
	lastMovie  movie.Movie
	lastErr    error

	called   bool
	gotFrom  int
	gotTo    int
	lastCall bool
}

func (s *stubService) LoadMovies(_ context.Context, from, to int) (service.LoadResult, error) {
	s.called = true
	s.gotFrom, s.gotTo = from, to
	return s.loadResult, s.loadErr
}

func (s *stubService) GetLastMovie(_ context.Context) (movie.Movie, error) {
	s.lastCall = true
	return s.lastMovie, s.lastErr
}

func newServer(t *testing.T, stub *stubService) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(httpapi.New(stub, logger).Routes())
	t.Cleanup(server.Close)
	return server
}

func get(t *testing.T, server *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var value T
	if err := json.NewDecoder(resp.Body).Decode(&value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return value
}

func TestLoadMoviesSuccess(t *testing.T) {
	stub := &stubService{
		loadResult: service.LoadResult{
			Movies: []movie.Movie{
				{ID: 1, Title: "First", Year: "2001", Director: "D1"},
				{ID: 2, Title: "Second", Year: "2002", Director: "D2"},
			},
		},
	}
	server := newServer(t, stub)

	resp := get(t, server, "/load-movies?from=1&to=2")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", got)
	}
	if !stub.called || stub.gotFrom != 1 || stub.gotTo != 2 {
		t.Errorf("service called=%v from=%d to=%d", stub.called, stub.gotFrom, stub.gotTo)
	}

	movies := decodeJSON[[]movie.Movie](t, resp)
	if len(movies) != 2 || movies[0].ID != 1 || movies[1].ID != 2 {
		t.Errorf("unexpected movies: %+v", movies)
	}
}

func TestLoadMoviesBadRequests(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "missing from", path: "/load-movies?to=3"},
		{name: "missing to", path: "/load-movies?from=1"},
		{name: "non-numeric from", path: "/load-movies?from=abc&to=3"},
		{name: "non-numeric to", path: "/load-movies?from=1&to=abc"},
		{name: "zero from", path: "/load-movies?from=0&to=3"},
		{name: "zero to", path: "/load-movies?from=1&to=0"},
		{name: "reversed", path: "/load-movies?from=5&to=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubService{}
			server := newServer(t, stub)

			resp := get(t, server, tt.path)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			body := decodeJSON[httpapi.ErrorResponse](t, resp)
			if body.Code != "invalid_argument" {
				t.Errorf("code = %q, want invalid_argument", body.Code)
			}
			if stub.called {
				t.Error("service was called for invalid request")
			}
		})
	}
}

func TestLastMovieSuccess(t *testing.T) {
	stub := &stubService{
		lastMovie: movie.Movie{ID: 10, Title: "Last", Year: "2010", Director: "D"},
	}
	server := newServer(t, stub)

	resp := get(t, server, "/last-movie")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	got := decodeJSON[movie.Movie](t, resp)
	if got.ID != 10 || got.Title != "Last" {
		t.Errorf("unexpected movie: %+v", got)
	}
	if !stub.lastCall {
		t.Error("GetLastMovie was not called")
	}
}

func TestLastMovieNotFound(t *testing.T) {
	stub := &stubService{lastErr: fmt.Errorf("last movie: %w", movie.ErrNotFound)}
	server := newServer(t, stub)

	resp := get(t, server, "/last-movie")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	body := decodeJSON[httpapi.ErrorResponse](t, resp)
	if body.Code != "not_found" {
		t.Errorf("code = %q, want not_found", body.Code)
	}
}

func TestLastMovieUpstreamError(t *testing.T) {
	stub := &stubService{
		lastErr: &stand.StatusError{MovieID: 10, StatusCode: 500, Status: "500 Internal Server Error"},
	}
	server := newServer(t, stub)

	resp := get(t, server, "/last-movie")
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	body := decodeJSON[httpapi.ErrorResponse](t, resp)
	if body.Code != "upstream_error" {
		t.Errorf("code = %q, want upstream_error", body.Code)
	}
}

func TestLastMovieTimeout(t *testing.T) {
	stub := &stubService{
		lastErr: fmt.Errorf("stand timeout: %w", context.DeadlineExceeded),
	}
	server := newServer(t, stub)

	resp := get(t, server, "/last-movie")
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", resp.StatusCode)
	}
}

func TestLoadMoviesPartial(t *testing.T) {
	stub := &stubService{
		loadResult: service.LoadResult{
			Movies: []movie.Movie{
				{ID: 1, Title: "First"},
				{ID: 3, Title: "Third"},
			},
			Errors: []movie.LoadError{
				{ID: 2, Err: &stand.StatusError{MovieID: 2, StatusCode: 500, Status: "500 Internal Server Error"}},
			},
		},
	}
	server := newServer(t, stub)

	resp := get(t, server, "/load-movies?from=1&to=3")
	if resp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Failed-Ids"); got != "2" {
		t.Errorf("X-Failed-Ids = %q, want 2", got)
	}
	if got := resp.Header.Get("X-Failed-Count"); got != "1" {
		t.Errorf("X-Failed-Count = %q, want 1", got)
	}

	raw := resp.Header.Get("X-Partial-Errors")
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		t.Fatalf("decode X-Partial-Errors: %v", err)
	}
	var failures []httpapi.Failure
	if err := json.Unmarshal([]byte(decoded), &failures); err != nil {
		t.Fatalf("unmarshal X-Partial-Errors: %v", err)
	}
	if len(failures) != 1 || failures[0].ID != 2 {
		t.Errorf("unexpected partial failures: %+v", failures)
	}

	movies := decodeJSON[[]movie.Movie](t, resp)
	if len(movies) != 2 {
		t.Errorf("len(movies) = %d, want 2", len(movies))
	}
}

func TestLoadMoviesPartialHeadersTruncated(t *testing.T) {
	errs := make([]movie.LoadError, 101)
	for i := range errs {
		errs[i] = movie.LoadError{ID: i + 1, Err: fmt.Errorf("movie %d: %w", i+1, movie.ErrNotFound)}
	}
	stub := &stubService{
		loadResult: service.LoadResult{
			Movies: []movie.Movie{{ID: 500, Title: "Kept"}},
			Errors: errs,
		},
	}
	server := newServer(t, stub)

	resp := get(t, server, "/load-movies?from=1&to=101")
	if resp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Failed-Count"); got != "101" {
		t.Errorf("X-Failed-Count = %q, want 101", got)
	}
	if got := resp.Header.Get("X-Partial-Errors-Truncated"); got != "true" {
		t.Errorf("X-Partial-Errors-Truncated = %q, want true", got)
	}
	if raw := resp.Header.Get("X-Partial-Errors"); raw == "" {
		t.Error("X-Partial-Errors is empty")
	}
}

func TestLoadMoviesAllNotFound(t *testing.T) {
	stub := &stubService{
		loadResult: service.LoadResult{
			Errors: []movie.LoadError{
				{ID: 11, Err: fmt.Errorf("movie 11: %w", movie.ErrNotFound)},
				{ID: 12, Err: fmt.Errorf("movie 12: %w", movie.ErrNotFound)},
			},
		},
	}
	server := newServer(t, stub)

	resp := get(t, server, "/load-movies?from=11&to=12")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	body := decodeJSON[httpapi.ErrorResponse](t, resp)
	if len(body.Details) != 2 {
		t.Errorf("len(details) = %d, want 2", len(body.Details))
	}
}

func TestLoadMoviesServiceError(t *testing.T) {
	stub := &stubService{loadErr: errors.New("unexpected internal error")}
	server := newServer(t, stub)

	resp := get(t, server, "/load-movies?from=1&to=2")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestSwaggerEndpoints(t *testing.T) {
	server := newServer(t, &stubService{})

	for _, path := range []string{"/swagger/index.html", "/swagger/doc.json"} {
		resp := get(t, server, path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
	}

	resp := get(t, server, "/swagger/doc.json")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read swagger doc: %v", err)
	}
	if !strings.Contains(string(body), `"/load-movies"`) || !strings.Contains(string(body), `"/last-movie"`) {
		t.Errorf("swagger doc does not contain movie endpoints: %s", body)
	}
}

func TestHealth(t *testing.T) {
	server := newServer(t, &stubService{})
	resp := get(t, server, "/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestWrongMethod(t *testing.T) {
	server := newServer(t, &stubService{})
	resp, err := http.Post(server.URL+"/load-movies?from=1&to=2", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}
