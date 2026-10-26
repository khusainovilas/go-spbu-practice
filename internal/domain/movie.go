// Package domain contains the business entities and errors of the movie service.
package domain

import "errors"

// Movie is a movie loaded from the external stand.
type Movie struct {
	ID          int      `json:"id" example:"1"`
	Title       string   `json:"title" example:"Solaris"`
	Year        int      `json:"year" example:"1972"`
	Director    string   `json:"director" example:"Andrei Tarkovsky"`
	Description string   `json:"description,omitempty" example:"A psychologist visits a space station"`
	Actors      []string `json:"actors,omitempty"`
	Genres      []string `json:"genres,omitempty"`
}

var (
	// ErrNotFound means that the requested movie does not exist.
	ErrNotFound = errors.New("movie not found")
	// ErrInvalidRange means that the from/to arguments are invalid.
	ErrInvalidRange = errors.New("invalid movie range")
	// ErrUpstream means that the external movie stand returned an error.
	ErrUpstream = errors.New("movie stand error")
)
