package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
)

// teamFromRequest reads the X-Team-ID header and returns the corresponding Team.
// Returns an error if the header is missing or invalid.
func teamFromRequest(r *http.Request) (entity.Team, error) {
	raw := r.Header.Get("X-Team-ID")
	if raw == "" {
		return 0, fmt.Errorf("missing X-Team-ID header")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || (n != 1 && n != 2) {
		return 0, fmt.Errorf("X-Team-ID must be 1 or 2, got %q", raw)
	}
	return entity.Team(n), nil
}

type errorBody struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:   code,
			Reason: reason,
		},
	})
}
