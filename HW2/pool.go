package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
)

func run(ctx context.Context, cfg Config) {
	client := &http.Client{}

	jobs := make(chan int)
	results := make(chan Result)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(ctx, client, jobs, results, cfg.Timeout)
		}()
	}

	go func() {
		defer close(jobs)
		for id := cfg.From; id <= cfg.To; id++ {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "%d — error: %v\n", result.ID, result.Err)
			continue
		}
		printMovie(result.ID, result.Movie)
	}
}
