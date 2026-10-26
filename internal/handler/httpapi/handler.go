// Package httpapi serves the movie API over HTTP.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/khusainovilas/go-spbu-practice/internal/domain"
	"github.com/khusainovilas/go-spbu-practice/internal/usecase"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Handler serves HTTP requests.
type Handler struct {
	service usecase.MovieUseCase
}

// New creates an HTTP handler.
func New(service usecase.MovieUseCase) *Handler {
	return &Handler{service: service}
}

// Routes registers HTTP routes and Swagger UI.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", h.loadMovies)
	mux.HandleFunc("GET /last-movie", h.lastMovie)
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	return mux
}

// ErrorResponse is returned when a request fails.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid movie range: from must not be greater than to"`
}

// loadMovies loads movies in the inclusive range [from; to].
//
// @Summary      Load movies by id range
// @Description  Loads movies from the external stand. If some ids fail, the response is 207 with loaded movies and failed ids in the X-Failed-Ids header.
// @Tags         movies
// @Produce      json
// @Param        from  query     int  true  "First movie id (greater than zero)"
// @Param        to    query     int  true  "Last movie id (not less than from)"
// @Success      200   {array}   domain.Movie
// @Success      207   {array}   domain.Movie  "Partial success"
// @Header       207   {string}  X-Failed-Ids  "Comma-separated movie ids that failed"
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      408   {object}  ErrorResponse
// @Failure      502   {object}  ErrorResponse
// @Failure      504   {object}  ErrorResponse
// @Router       /load-movies [get]
func (h *Handler) loadMovies(w http.ResponseWriter, r *http.Request) {
	from, err := queryInt(r, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	to, err := queryInt(r, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := h.service.LoadMovies(r.Context(), from, to)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}

	if len(result.Movies) == 0 && len(result.Failed) > 0 {
		writeError(w, statusForFailed(result.Failed), firstFailure(result.Failed))
		return
	}

	if len(result.Failed) > 0 {
		w.Header().Set("X-Failed-Ids", failedIDs(result.Failed))
		writeJSON(w, http.StatusMultiStatus, result.Movies)
		return
	}

	writeJSON(w, http.StatusOK, result.Movies)
}

// lastMovie returns the latest movie available on the stand.
//
// @Summary      Get the last movie
// @Description  Finds the latest available movie on the stand.
// @Tags         movies
// @Produce      json
// @Success      200  {object}  domain.Movie
// @Failure      404  {object}  ErrorResponse
// @Failure      408  {object}  ErrorResponse
// @Failure      502  {object}  ErrorResponse
// @Failure      504  {object}  ErrorResponse
// @Router       /last-movie [get]
func (h *Handler) lastMovie(w http.ResponseWriter, r *http.Request) {
	movie, err := h.service.GetLastMovie(r.Context())
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, movie)
}

func queryInt(r *http.Request, name string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return 0, fmt.Errorf("query parameter %q is required", name)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("query parameter %q must be an integer", name)
	}
	return value, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, ErrorResponse{Error: err.Error()})
}

// statusForError converts a domain error into an HTTP status code.
func statusForError(err error) int {
	switch {
	case errors.Is(err, domain.ErrInvalidRange):
		return http.StatusBadRequest
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrUpstream):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// statusForFailed returns the status for a range where every movie failed.
func statusForFailed(failed []usecase.FailedMovie) int {
	for _, item := range failed {
		if !errors.Is(item.Err, domain.ErrNotFound) {
			return statusForError(item.Err)
		}
	}
	return http.StatusNotFound
}

func firstFailure(failed []usecase.FailedMovie) error {
	for _, item := range failed {
		if !errors.Is(item.Err, domain.ErrNotFound) {
			return item.Err
		}
	}
	return fmt.Errorf("%w: no movies found in the requested range", domain.ErrNotFound)
}

func failedIDs(failed []usecase.FailedMovie) string {
	ids := make([]string, 0, len(failed))
	for _, item := range failed {
		ids = append(ids, strconv.Itoa(item.ID))
	}
	return strings.Join(ids, ",")
}
