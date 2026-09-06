package api

import (
	"encoding/json"
	"net/http"
)

// writeJSON serializa v como JSON no writer (usado por handlers simples).
func writeJSON(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(v)
}
