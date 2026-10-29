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

type Client struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
		http:    &http.Client{},
	}
}

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
		return domain.Movie{}, fmt.Errorf("%w: get movie %d: %w", domain.ErrUpstream, id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.Movie{}, fmt.Errorf("movie %d: %w", id, domain.ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return domain.Movie{}, fmt.Errorf("%w: movie %d: HTTP %d", domain.ErrUpstream, id, resp.StatusCode)
	}

	var item domain.Movie
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return domain.Movie{}, fmt.Errorf("%w: decode movie %d: %v", domain.ErrUpstream, id, err)
	}
	item.ID = id
	return item, nil
}
