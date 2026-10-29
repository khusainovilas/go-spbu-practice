package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type HTTPHandler struct {
	service *movie.Service
}

func NewHTTP(service *movie.Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", h.loadMovies)
	mux.HandleFunc("GET /last-movie", h.lastMovie)
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	return mux
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// @Summary      Load movies
// @Description  Loads movies in [from; to]. Partial success is returned as 207 with X-Failed-Ids.
// @Tags         movies
// @Produce      json
// @Param        from  query     int  true  "first movie id"
// @Param        to    query     int  true  "last movie id"
// @Success      200   {array}   movie.Movie
// @Success      207   {array}   movie.Movie
// @Header       207   {string}  X-Failed-Ids  "ids that failed"
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      408   {object}  ErrorResponse
// @Failure      502   {object}  ErrorResponse
// @Failure      504   {object}  ErrorResponse
// @Router       /load-movies [get]
func (h *HTTPHandler) loadMovies(w http.ResponseWriter, r *http.Request) {
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

// @Summary      Get last movie
// @Tags         movies
// @Produce      json
// @Success      200  {object}  movie.Movie
// @Failure      404  {object}  ErrorResponse
// @Failure      408  {object}  ErrorResponse
// @Failure      502  {object}  ErrorResponse
// @Failure      504  {object}  ErrorResponse
// @Router       /last-movie [get]
func (h *HTTPHandler) lastMovie(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetLastMovie(r.Context())
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, item)
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

func statusForError(err error) int {
	switch {
	case errors.Is(err, movie.ErrInvalidRange):
		return http.StatusBadRequest
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case errors.Is(err, movie.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, movie.ErrUpstream):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func statusForFailed(failed []movie.FailedMovie) int {
	for _, item := range failed {
		if !errors.Is(item.Err, movie.ErrNotFound) {
			return statusForError(item.Err)
		}
	}
	return http.StatusNotFound
}

func firstFailure(failed []movie.FailedMovie) error {
	for _, item := range failed {
		if !errors.Is(item.Err, movie.ErrNotFound) {
			return item.Err
		}
	}
	return fmt.Errorf("%w: no movies found in the requested range", movie.ErrNotFound)
}

func failedIDs(failed []movie.FailedMovie) string {
	ids := make([]string, 0, len(failed))
	for _, item := range failed {
		ids = append(ids, strconv.Itoa(item.ID))
	}
	return strings.Join(ids, ",")
}
