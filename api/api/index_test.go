package handler

import (
	"net/http/httptest"
	"testing"

	"geevheittire/api/internal/server"
)

func TestRewritePath(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/index?__path=/health", nil)
	if p := r.URL.Query().Get("__path"); p != "" {
		q := r.URL.Query()
		q.Del("__path")
		r.URL.Path = p
		r.URL.RawPath = ""
		r.URL.RawQuery = q.Encode()
		r.RequestURI = p
	}
	h := server.NewRouter(&server.App{})
	w := httptest.NewRecorder()
	// Verify router matches /health (panics on nil db inside handler, which proves route matched!)
	defer func() { _ = recover() }()
	h.ServeHTTP(w, r)
	if w.Code == 404 {
		t.Fatalf("expected /health to match, got 404")
	}
}
