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

type Handler struct {
	service usecase.MovieUseCase
}

func New(service usecase.MovieUseCase) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", h.loadMovies)
	mux.HandleFunc("GET /last-movie", h.lastMovie)
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	return mux
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MovieResponse struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

func toResponse(item domain.Movie) MovieResponse {
	return MovieResponse{
		ID:       item.ID,
		Title:    item.Title,
		Year:     item.Year,
		Director: item.Director,
	}
}

func toResponses(items []domain.Movie) []MovieResponse {
	out := make([]MovieResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toResponse(item))
	}
	return out
}

// @Summary      Load movies
// @Description  Loads movies in [from; to]. Partial success is returned as 207 with X-Failed-Ids.
// @Tags         movies
// @Produce      json
// @Param        from  query     int  true  "first movie id"
// @Param        to    query     int  true  "last movie id"
// @Success      200   {array}   MovieResponse
// @Success      207   {array}   MovieResponse
// @Header       207   {string}  X-Failed-Ids  "ids that failed"
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
		writeJSON(w, http.StatusMultiStatus, toResponses(result.Movies))
		return
	}

	writeJSON(w, http.StatusOK, toResponses(result.Movies))
}

// @Summary      Get last movie
// @Tags         movies
// @Produce      json
// @Success      200  {object}  MovieResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      408  {object}  ErrorResponse
// @Failure      502  {object}  ErrorResponse
// @Failure      504  {object}  ErrorResponse
// @Router       /last-movie [get]
func (h *Handler) lastMovie(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.GetLastMovie(r.Context())
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(item))
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

func statusForFailed(failed []domain.FailedMovie) int {
	for _, item := range failed {
		if !errors.Is(item.Err, domain.ErrNotFound) {
			return statusForError(item.Err)
		}
	}
	return http.StatusNotFound
}

func firstFailure(failed []domain.FailedMovie) error {
	for _, item := range failed {
		if !errors.Is(item.Err, domain.ErrNotFound) {
			return item.Err
		}
	}
	return fmt.Errorf("%w: no movies found in the requested range", domain.ErrNotFound)
}

func failedIDs(failed []domain.FailedMovie) string {
	ids := make([]string, 0, len(failed))
	for _, item := range failed {
		ids = append(ids, strconv.Itoa(item.ID))
	}
	return strings.Join(ids, ",")
}
