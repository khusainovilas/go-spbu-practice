// Package httpapi implements the HTTP transport of the movie service.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/service"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// MovieService is the business logic required by HTTP handlers.
type MovieService interface {
	LoadMovies(ctx context.Context, from, to int) (service.LoadResult, error)
	GetLastMovie(ctx context.Context) (movie.Movie, error)
}

// Handler serves the HTTP API.
type Handler struct {
	service MovieService
	logger  *slog.Logger
}

// New creates an HTTP handler. A nil logger is replaced with slog.Default.
func New(movieService MovieService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: movieService, logger: logger}
}

// Routes returns the HTTP router with all endpoints and middleware.
//
// Swagger UI is available at /swagger/index.html.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /load-movies", h.loadMovies)
	mux.HandleFunc("GET /last-movie", h.lastMovie)
	mux.HandleFunc("GET /healthz", h.health)
	mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))

	return h.logging(h.recovery(mux))
}

// loadMovies loads movies in the inclusive range [from; to].
//
// @Summary      Load movies by id range
// @Description  Loads movies from the external stand for ids in [from; to]. If some movies fail, the response is 207 with the successfully loaded movies and failure details in the X-Failed-Ids and X-Partial-Errors headers.
// @Tags         movies
// @Produce      json
// @Param        from  query     int  true  "First movie id (must be > 0)"
// @Param        to    query     int  true  "Last movie id (must be >= from)"
// @Success      200   {array}   movie.Movie
// @Success      207   {array}   movie.Movie  "Partial success"
// @Header       207   {string}  X-Failed-Ids     "Comma-separated ids that failed (up to 100)"
// @Header       207   {string}  X-Partial-Errors "URL-encoded JSON array of {id, error} objects (up to 100)"
// @Header       207   {string}  X-Partial-Errors-Truncated "true when headers contain only the first 100 failures"
// @Failure      400   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "No movies found in the range"
// @Failure      408   {object}  ErrorResponse  "Request canceled"
// @Failure      500   {object}  ErrorResponse
// @Failure      502   {object}  ErrorResponse  "Movie stand error"
// @Failure      504   {object}  ErrorResponse  "Movie stand timeout"
// @Router       /load-movies [get]
func (h *Handler) loadMovies(w http.ResponseWriter, r *http.Request) {
	from, err := queryInt(r, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	to, err := queryInt(r, "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}
	if err := service.ValidateRange(from, to); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), nil)
		return
	}

	result, err := h.service.LoadMovies(r.Context(), from, to)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	if len(result.Movies) == 0 {
		if len(result.Errors) == 0 {
			writeJSON(w, http.StatusOK, []movie.Movie{})
			return
		}
		h.writeAllFailed(w, result.Errors)
		return
	}

	if len(result.Errors) > 0 {
		setPartialHeaders(w, result.Errors)
		h.logger.WarnContext(r.Context(), "movies loaded partially",
			"from", from,
			"to", to,
			"loaded", len(result.Movies),
			"failed", len(result.Errors),
		)
		writeJSON(w, http.StatusMultiStatus, result.Movies)
		return
	}

	writeJSON(w, http.StatusOK, result.Movies)
}

// lastMovie returns the latest available movie on the stand.
//
// @Summary      Get the last movie
// @Description  Finds the latest available movie on the stand. Movie ids are consecutive, so the service probes id 99999 and then uses binary search.
// @Tags         movies
// @Produce      json
// @Success      200  {object}  movie.Movie
// @Failure      404  {object}  ErrorResponse
// @Failure      408  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Failure      502  {object}  ErrorResponse
// @Failure      504  {object}  ErrorResponse
// @Router       /last-movie [get]
func (h *Handler) lastMovie(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetLastMovie(r.Context())
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// health reports that the HTTP server is alive.
//
// @Summary      Health check
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /healthz [get]
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
