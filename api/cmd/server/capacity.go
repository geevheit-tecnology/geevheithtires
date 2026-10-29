package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func (a *App) publicQuota(w http.ResponseWriter, r *http.Request, peer string) bool {
	keys := []struct {
		key   string
		limit int
	}{{"peer:" + peer, 600}}
	if r.Method != "GET" && r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1_000_001))
		if err != nil || len(body) > 1_000_000 {
			problem(w, 413, "Dados muito grandes.")
			return false
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var identity struct {
			Email string `json:"email"`
		}
		if json.Unmarshal(body, &identity) == nil && len(identity.Email) <= 254 && strings.TrimSpace(identity.Email) != "" {
			keys = append(keys, struct {
				key   string
				limit int
			}{"account:" + strings.ToLower(strings.TrimSpace(identity.Email)), 20})
		}
	}
	for _, item := range keys {
		digest := hmac.New(sha256.New, a.secret)
		_, _ = digest.Write([]byte(item.key))
		key := hex.EncodeToString(digest.Sum(nil))
		var allowed bool
		if err := a.db.QueryRow(r.Context(), "select auth_consume_quota($1,$2)", key, item.limit).Scan(&allowed); err != nil {
			problem(w, 503, "Não foi possível verificar o acesso. Tente novamente em instantes.")
			return false
		}
		if !allowed {
			w.Header().Set("Retry-After", "60")
			problem(w, 429, "Muitas tentativas. Aguarde um minuto.")
			return false
		}
	}
	return true
}

// Bound expensive password work per process so a login burst cannot allocate
// unbounded Argon2 memory. Four verifications use at most about 256 MiB.
var passwordSlots = make(chan struct{}, 4)

func passwordWorkLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}
		select {
		case passwordSlots <- struct{}{}:
			defer func() { <-passwordSlots }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "3")
			problem(w, 429, "Há muitos acessos simultâneos. Aguarde alguns segundos e tente novamente.")
		}
	})
}

func requestObservation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Request-ID", middleware.GetReqID(r.Context()))
		response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(response, r)
		if response.Status() >= 500 || time.Since(start) > time.Second {
			path := chi.RouteContext(r.Context()).RoutePattern()
			if path == "" {
				path = "unmatched"
			}
			slog.Warn("request requires attention", "request_id", middleware.GetReqID(r.Context()), "method", r.Method, "route", path, "status", response.Status(), "duration_ms", time.Since(start).Milliseconds())
		}
	})
}
