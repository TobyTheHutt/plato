package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"plato/backend/internal/domain"
)

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not found")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write json failed: status=%d body_type=%T err=%v", status, body, err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeDecodeError(w http.ResponseWriter, err error) {
	message := "invalid JSON"
	if strings.Contains(err.Error(), "request body too large") {
		message = fmt.Sprintf("request body too large (max %d bytes)", maxJSONBodyBytes)
	}
	writeError(w, http.StatusBadRequest, message)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrValidation):
		message := "validation failed"
		detailed := strings.TrimSpace(err.Error())
		suffix := ": " + domain.ErrValidation.Error()
		if strings.HasSuffix(detailed, suffix) {
			detailed = strings.TrimSuffix(detailed, suffix)
		}
		if detailed != "" && detailed != domain.ErrValidation.Error() {
			message = detailed
		}
		writeError(w, http.StatusBadRequest, message)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID, X-Org-ID, X-Role")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
