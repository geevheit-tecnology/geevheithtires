package handler

import (
	"encoding/json"
	"net/http"

	"geevheittire/api/internal/server"
)

// Handler is the Vercel Go Serverless Function entrypoint for the GeevheitTire API.
func Handler(w http.ResponseWriter, r *http.Request) {
	if p := r.URL.Query().Get("__path"); p != "" {
		q := r.URL.Query()
		q.Del("__path")
		r.URL.Path = p
		r.URL.RawPath = ""
		r.URL.RawQuery = q.Encode()
		if r.URL.RawQuery != "" {
			r.RequestURI = p + "?" + r.URL.RawQuery
		} else {
			r.RequestURI = p
		}
	}
	h, err := server.NewServerlessHandler()
	if err != nil {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "about:blank",
			"title":  "API indisponível: " + err.Error(),
			"status": http.StatusServiceUnavailable,
		})
		return
	}
	h.ServeHTTP(w, r)
}
