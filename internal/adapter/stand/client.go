// Package stand implements a client for the external movie stand.
package stand

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/khusainovilas/go-spbu-practice/internal/domain"
)

// Client loads movies over HTTP.
type Client struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
}

// NewClient creates a stand client. Every request is limited by timeout.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
		http:    &http.Client{},
	}
}

// GetMovie loads one movie from {baseURL}/{id}/info.0.json.
func (c *Client) GetMovie(ctx context.Context, id int) (domain.Movie, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/%d/info.0.json", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.Movie{}, fmt.Errorf("%w: build request for movie %d: %v", domain.ErrUpstream, id, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// %w keeps both ErrUpstream and the original context error visible
		// to errors.Is, so handlers can return 504 for a timeout.
		return domain.Movie{}, fmt.Errorf("%w: get movie %d: %w", domain.ErrUpstream, id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.Movie{}, fmt.Errorf("movie %d: %w", id, domain.ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return domain.Movie{}, fmt.Errorf("%w: movie %d: HTTP %d", domain.ErrUpstream, id, resp.StatusCode)
	}

	var movie domain.Movie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return domain.Movie{}, fmt.Errorf("%w: decode movie %d: %v", domain.ErrUpstream, id, err)
	}
	movie.ID = id
	return movie, nil
}
