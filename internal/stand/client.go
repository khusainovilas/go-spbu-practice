// Package stand implements a client for the external movie stand.
package stand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
)

const maxResponseSize = 1 << 20 // 1 MiB

// ErrInvalidResponse means that the stand returned something that is not a valid movie.
var ErrInvalidResponse = errors.New("invalid response from movie stand")

// ErrUnavailable means that the stand could not be reached or the connection failed.
var ErrUnavailable = errors.New("movie stand is unavailable")

// StatusError is returned when the stand responds with a non-200 and non-404 status.
type StatusError struct {
	MovieID    int
	StatusCode int
	Status     string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("movie stand returned %s for movie %d", e.Status, e.MovieID)
}

// Client fetches movies from the external stand.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	timeout    time.Duration
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client. It is mostly useful in tests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// NewClient creates a stand client. Every request is limited by timeout.
func NewClient(baseURL string, timeout time.Duration, opts ...Option) (*Client, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("request timeout must be greater than zero")
	}

	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse base URL %q: %w", baseURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("base URL %q must use http or https", baseURL)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("base URL %q has no host", baseURL)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second

	client := &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Transport: transport,
		},
		timeout: timeout,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client, nil
}

// GetMovie fetches a movie by id. It returns movie.ErrNotFound for HTTP 404.
func (c *Client) GetMovie(ctx context.Context, id int) (movie.Movie, error) {
	if id <= 0 {
		return movie.Movie{}, fmt.Errorf("movie id must be greater than zero: %d", id)
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	endpoint := c.baseURL.JoinPath(strconv.Itoa(id), "info.0.json")
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return movie.Movie{}, fmt.Errorf("build request for movie %d: %w", id, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-spbu-movie-service/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return movie.Movie{}, fmt.Errorf("%w: get movie %d: request timed out after %s: %w", ErrUnavailable, id, c.timeout, context.DeadlineExceeded)
		case errors.Is(err, context.Canceled):
			return movie.Movie{}, fmt.Errorf("get movie %d: request canceled: %w", id, context.Canceled)
		default:
			return movie.Movie{}, fmt.Errorf("%w: get movie %d: %w", ErrUnavailable, id, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return movie.Movie{}, fmt.Errorf("movie %d: %w", id, movie.ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return movie.Movie{}, &StatusError{
			MovieID:    id,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return movie.Movie{}, fmt.Errorf("%w: read movie %d response: %w", ErrUnavailable, id, err)
	}
	if len(body) > maxResponseSize {
		return movie.Movie{}, fmt.Errorf("%w: movie %d response exceeds %d bytes", ErrInvalidResponse, id, maxResponseSize)
	}

	var result movie.Movie
	if err := json.Unmarshal(body, &result); err != nil {
		return movie.Movie{}, fmt.Errorf("%w: movie %d: %v", ErrInvalidResponse, id, err)
	}
	result.ID = id
	return result, nil
}
