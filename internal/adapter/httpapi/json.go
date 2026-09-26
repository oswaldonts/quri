package httpapi

import (
	"encoding/json"
	"net/http"
)

// maxBodyBytes limits request bodies to protect the server from huge payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON sends v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends a JSON error body: {"error": "message"}.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// decodeJSON reads a single JSON object from the request body into dst,
// rejecting unknown fields and bodies larger than maxBodyBytes.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
