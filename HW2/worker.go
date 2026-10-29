package main

import (
	"context"
	"net/http"
	"time"
)

func worker(ctx context.Context, client *http.Client, jobs <-chan int, results chan<- Result, timeout time.Duration) {
	for id := range jobs {
		movie, err := fetchMovie(ctx, client, id, timeout)
		select {
		case results <- Result{id, movie, err}:
		case <-ctx.Done():
			return
		}
	}
}
