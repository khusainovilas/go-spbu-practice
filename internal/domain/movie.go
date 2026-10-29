package domain

import "errors"

type Movie struct {
	ID       int
	Title    string
	Year     int
	Director string
}

var (
	ErrNotFound     = errors.New("movie not found")
	ErrInvalidRange = errors.New("invalid movie range")
	ErrUpstream     = errors.New("movie stand error")
)

type FailedMovie struct {
	ID  int
	Err error
}

type LoadResult struct {
	Movies []Movie
	Failed []FailedMovie
}
