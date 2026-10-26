// Package movie contains the domain model and domain errors of the service.
package movie

import (
	"errors"
	"fmt"
)

// Movie is a movie returned by the external movie stand.
type Movie struct {
	ID       int    `json:"id" example:"1"`
	Title    string `json:"title" example:"Toy Story"`
	Year     string `json:"year" example:"1995"`
	Director string `json:"director" example:"John Lasseter"`
}

// ErrNotFound means that a movie with the requested id does not exist.
var ErrNotFound = errors.New("movie not found")

// ValidationError describes an invalid request argument.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// LoadError is a per-movie error returned when a range was loaded only partially.
type LoadError struct {
	ID  int
	Err error
}

func (e LoadError) Error() string {
	return fmt.Sprintf("movie %d: %v", e.ID, e.Err)
}

func (e LoadError) Unwrap() error {
	return e.Err
}
