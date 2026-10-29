package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSharedAuthenticationQuotaAcrossInstances(t *testing.T) {
	database := os.Getenv("TEST_DATABASE_URL")
	if database == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", database)
	db, err := newPool(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	secret := []byte("quota-test-" + id())
	instances := []*App{{db: db, secret: secret}, {db: db, secret: secret}}
	request := func(instance int, email string) int {
		handler := instances[instance].rate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
		req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(`{"email":"`+email+`"}`))
		req.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response.Code
	}
	for i := 0; i < 35; i++ {
		if request(i%2, id()+"@qa.invalid") != 200 {
			t.Fatal("distinct clients sharing proxy were blocked")
		}
	}
	email := id() + "@qa.invalid"
	for i := 0; i < 20; i++ {
		if request(i%2, email) != 200 {
			t.Fatal("account quota exhausted early")
		}
	}
	if request(1, strings.ToUpper(email)) != 429 {
		t.Fatal("quota was bypassed across replicas or email casing")
	}
	if _, err = db.Exec(context.Background(), "select * from auth_quotas limit 1"); err == nil {
		t.Fatal("runtime can read private quota table")
	}
}

func TestPasswordWorkLimitBoundsMemoryWork(t *testing.T) {
	for i := 0; i < cap(passwordSlots); i++ {
		passwordSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(passwordSlots); i++ {
			<-passwordSlots
		}
	}()
	reached := false
	handler := passwordWorkLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(200) }))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("POST", "/auth/login", nil))
	if reached || rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatal("unbounded password work accepted")
	}
}

func TestAuthenticatedRateLimitDoesNotShareClientQuota(t *testing.T) {
	a := &App{}
	handler := a.rate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	request := func(user string) int {
		req := httptest.NewRequest("POST", "/support/tickets", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		identity := &Claims{}
		identity.Subject = user
		req = req.WithContext(context.WithValue(req.Context(), claimsKey{}, identity))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	a.limiter.Store("expired", &rateWindow{at: time.Now().Add(-3 * time.Minute), count: 30})
	for i := 0; i < 30; i++ {
		if request("user-a") != 200 {
			t.Fatal("quota exhausted too early")
		}
	}
	if request("user-a") != 429 {
		t.Fatal("quota was not enforced")
	}
	if request("user-b") != 200 {
		t.Fatal("unrelated customer shares rate limit")
	}
	if _, found := a.limiter.Load("expired"); found {
		t.Fatal("stale rate-limit entry retained")
	}
}
