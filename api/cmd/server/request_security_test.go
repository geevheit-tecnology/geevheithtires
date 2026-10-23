package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRequiresSingleJSONDocument(t *testing.T) {
	for _, body := range []string{`{"name":"ok"} {"name":"extra"}`, `{"name":"ok"} garbage`, `{"unknown":true}`, ""} {
		t.Run(body, func(t *testing.T) {
			var value struct {
				Name string `json:"name"`
			}
			w := httptest.NewRecorder()
			if decode(w, httptest.NewRequest("POST", "/", strings.NewReader(body)), &value) || w.Code != 400 {
				t.Fatalf("invalid JSON accepted: status %d", w.Code)
			}
		})
	}
	var value struct {
		Name string `json:"name"`
	}
	if !decode(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader("{\"name\":\"ok\"}\n ")), &value) || value.Name != "ok" {
		t.Fatal("single JSON document with trailing whitespace rejected")
	}
}

func TestRateSeparatesIPv6ClientsAndIgnoresSpoofedHeaders(t *testing.T) {
	a := &App{}
	h := a.rate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	request := func(remote string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/auth/login", nil)
		r.RemoteAddr = remote
		r.Header.Set("X-Forwarded-For", "198.51.100.8")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for i := 0; i < 30; i++ {
		if request("[2001:db8::1]:1234").Code != 204 {
			t.Fatal("blocked before limit")
		}
	}
	if w := request("[2001:db8::1]:5678"); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("same client bypassed limit by changing port")
	}
	if request("[2001:db8::2]:1234").Code != 204 {
		t.Fatal("independent IPv6 client blocked")
	}
}
