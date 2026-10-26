package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/khusainovilas/go-spbu-practice/internal/movie"
	"github.com/khusainovilas/go-spbu-practice/internal/stand"
)

// Failure describes one failed movie in an error response or a partial response header.
type Failure struct {
	ID    int    `json:"id" example:"3"`
	Error string `json:"error" example:"movie stand returned 500 Internal Server Error for movie 3"`
}

// ErrorResponse is the JSON error envelope of the HTTP API.
type ErrorResponse struct {
	Error   string    `json:"error" example:"invalid from parameter"`
	Code    string    `json:"code" example:"invalid_argument"`
	Details []Failure `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status line is already sent, so encoding errors cannot be reported
	// with a different status. All payload types used here are JSON-safe.
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string, details []Failure) {
	writeJSON(w, status, ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	})
}

func queryInt(r *http.Request, name string) (int, error) {
	values, ok := r.URL.Query()[name]
	if !ok || len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return 0, fmt.Errorf("query parameter %q is required", name)
	}
	value, err := strconv.Atoi(strings.TrimSpace(values[0]))
	if err != nil {
		return 0, fmt.Errorf("query parameter %q must be an integer", name)
	}
	return value, nil
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	status, code := classifyError(err)
	writeError(w, status, code, err.Error(), nil)
}

func (h *Handler) writeAllFailed(w http.ResponseWriter, errs []movie.LoadError) {
	details := make([]Failure, 0, len(errs))
	allNotFound := true
	var statusErr error
	for _, item := range errs {
		details = append(details, Failure{ID: item.ID, Error: item.Err.Error()})
		if !errors.Is(item.Err, movie.ErrNotFound) {
			allNotFound = false
			if statusErr == nil {
				statusErr = item.Err
			}
		}
	}

	if allNotFound {
		writeError(w, http.StatusNotFound, "not_found", "no movies found in the requested range", details)
		return
	}

	status, code := classifyError(statusErr)
	writeError(w, status, code, "failed to load movies", details)
}

func classifyError(err error) (status int, code string) {
	var validationErr *movie.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return http.StatusBadRequest, "invalid_argument"
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "upstream_timeout"
	case errors.Is(err, movie.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, stand.ErrInvalidResponse):
		return http.StatusBadGateway, "invalid_upstream_response"
	case errors.Is(err, stand.ErrUnavailable):
		return http.StatusBadGateway, "upstream_unavailable"
	default:
		var statusErr *stand.StatusError
		if errors.As(err, &statusErr) {
			return http.StatusBadGateway, "upstream_error"
		}
		return http.StatusInternalServerError, "internal_error"
	}
}

func setPartialHeaders(w http.ResponseWriter, errs []movie.LoadError) {
	const maxDetails = 100

	headerErrs := errs
	truncated := false
	if len(headerErrs) > maxDetails {
		headerErrs = headerErrs[:maxDetails]
		truncated = true
	}

	ids := make([]string, 0, len(headerErrs))
	details := make([]Failure, 0, len(headerErrs))
	for _, item := range headerErrs {
		ids = append(ids, strconv.Itoa(item.ID))
		details = append(details, Failure{ID: item.ID, Error: item.Err.Error()})
	}

	encoded, err := json.Marshal(details)
	if err != nil {
		encoded = []byte("[]")
	}

	w.Header().Set("X-Failed-Ids", strings.Join(ids, ","))
	w.Header().Set("X-Failed-Count", strconv.Itoa(len(errs)))
	w.Header().Set("X-Partial-Errors", url.QueryEscape(string(encoded)))
	if truncated {
		w.Header().Set("X-Partial-Errors-Truncated", "true")
	}
}
