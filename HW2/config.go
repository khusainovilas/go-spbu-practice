package main

import (
	"errors"
	"flag"
	"fmt"
	"time"
)

type Config struct {
	From    int
	To      int
	Workers int
	Timeout time.Duration
}

func parseFlags() (Config, error) {
	var cfg Config

	flag.IntVar(&cfg.From, "from", 0, "first movie id (required)")
	flag.IntVar(&cfg.To, "to", 0, "last movie id (required)")
	flag.IntVar(&cfg.Workers, "workers", 10, "number of workers")
	flag.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "request timeout")
	flag.Parse()

	if cfg.From <= 0 {
		return Config{}, errors.New("--from is required and must be greater than 0")
	}
	if cfg.To <= 0 {
		return Config{}, errors.New("--to is required and must be greater than 0")
	}
	if cfg.From > cfg.To {
		return Config{}, fmt.Errorf("--from must not be greater than --to")
	}
	if cfg.Workers <= 0 {
		return Config{}, fmt.Errorf("--workers must be greater than 0")
	}
	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("--timeout must be greater than 0")
	}
	return cfg, nil
}
