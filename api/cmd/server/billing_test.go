package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

func TestPaymentCents(t *testing.T) {
	for in, want := range map[string]int{"149": 14900, "449.90": 44990, "0.01": 1, "1.1": 110} {
		n, e := paymentCents(json.Number(in))
		if e != nil || n != want {
			t.Fatalf("%s: %d %v", in, n, e)
		}
	}
	for _, in := range []string{"-1", "1.999", "1e2", "NaN", "", "99999999999999"} {
		if _, e := paymentCents(json.Number(in)); e == nil {
			t.Fatalf("accepted %s", in)
		}
	}
	for _, bad := range []string{"https://evil.test/i/x", "javascript:alert(1)", "http://www.asaas.com/i/x", "https://www.asaas.com.evil.test"} {
		if validCheckoutURL(bad, "production") {
			t.Fatal(bad)
		}
	}
}
func TestPaymentsDefaultDisabled(t *testing.T) {
	t.Setenv("ASAAS_API_KEY", "test")
	t.Setenv("ASAAS_WEBHOOK_TOKEN", "unique-test-webhook-token-at-least-32")
	t.Setenv("ASAAS_ENVIRONMENT", "production")
	t.Setenv("ASAAS_LIVE_ENABLED", "")
	if asaasFromEnv() != nil {
		t.Fatal("live payments enabled without opt in")
	}
	t.Setenv("ASAAS_ENVIRONMENT", "sandbox")
	if asaasFromEnv() == nil {
		t.Fatal("sandbox unavailable")
	}
	a := &App{}
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{}`))
	req.Header.Set("asaas-access-token", "wrong")
	w := httptest.NewRecorder()
	a.asaasWebhook(w, req)
	if w.Code != 401 {
		t.Fatalf("unauthenticated webhook: %d", w.Code)
	}
}
func TestBillingAndPlatformIsolation(t *testing.T) {
	database := os.Getenv("TEST_DATABASE_URL")
	if database == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", database)
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, database)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(ctx)
	db, e := newPool(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := &App{db: db, secret: []byte("test-secret-with-at-least-32-characters")}
	companies := []string{id(), id()}
	subjects := []string{id(), id()}
	for i, c := range companies {
		if _, e = admin.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'Billing QA',$2)", c, "billing-"+c); e != nil {
			t.Fatal(e)
		}
		if _, e = admin.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,'Billing QA',$3,'unused','owner')", subjects[i], c, c+"@billing.invalid"); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		for _, c := range companies {
			_, _ = admin.Exec(ctx, `delete from billing_events where payment_id in(select provider_id from billing_invoices where company_id=$1);delete from billing_invoices where company_id=$1;delete from vehicles where company_id=$1;delete from platform_audit where actor_id in(select id from users where company_id=$1);delete from platform_admins where user_id in(select id from users where company_id=$1);delete from users where company_id=$1;delete from subscriptions where company_id=$1;delete from companies where id=$1`, c)
		}
	}()
	r := chi.NewRouter()
	r.Use(a.auth)
	r.Use(a.billingGuard)
	r.Get("/api/v1/billing", a.billingStatus)
	r.Post("/api/v1/vehicles", a.createVehicle)
	r.Get("/api/v1/platform", a.platformDashboard)
	r.Post("/api/v1/platform/change", a.platformChange)
	tokens := make([]string, 2)
	for i := range tokens {
		tokens[i], _ = jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{Company: companies[i], Role: "owner", RegisteredClaims: jwt.RegisteredClaims{Subject: subjects[i], Issuer: "geevheittire", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString(a.secret)
	}
	request := func(method, path string, who int, body string, want int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+tokens[who])
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: got %d want %d %s", method, path, w.Code, want, w.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	request("GET", "/api/v1/platform", 0, "", 403)
	if _, e = admin.Exec(ctx, "insert into platform_admins(user_id) values($1)", subjects[0]); e != nil {
		t.Fatal(e)
	}
	request("GET", "/api/v1/platform", 0, "", 200)
	request("POST", "/api/v1/platform/change", 1, `{"target":"`+companies[0]+`","operation":"suspend","reason":"unauthorized action"}`, 403)
	request("POST", "/api/v1/platform/change", 0, `{"target":"`+companies[1]+`","operation":"suspend","reason":"QA administrative suspension"}`, 200)
	request("POST", "/api/v1/vehicles", 1, `{"plate":"ABC1D23","model":"QA Truck","template_id":"tractor-6x2"}`, 402)
	if out := request("GET", "/api/v1/billing", 1, "", 200); out["status"] != "suspended" {
		t.Fatal(out)
	}
	request("POST", "/api/v1/platform/change", 0, `{"target":"`+companies[1]+`","operation":"restore","reason":"QA restore access"}`, 200)
	if _, e = admin.Exec(ctx, "update companies set trial_ends_at=now()-interval '1 day' where id=$1", companies[1]); e != nil {
		t.Fatal(e)
	}
	request("POST", "/api/v1/vehicles", 1, `{"plate":"ABC1D23","model":"QA Truck","template_id":"tractor-6x2"}`, 402)
	request("GET", "/api/v1/billing", 1, "", 200)
	request("POST", "/api/v1/platform/change", 0, `{"target":"`+companies[1]+`","operation":"trial","reason":"QA extended trial","payload":{"days":14}}`, 200)
	request("POST", "/api/v1/vehicles", 1, `{"plate":"ABC1D23","model":"QA Truck","template_id":"tractor-6x2"}`, 201)
	// DB-level guards serialize simultaneous inserts; the last trial slot is unique.
	for i := 0; i < 13; i++ {
		if _, e = admin.Exec(ctx, "insert into vehicles(id,company_id,plate,model,template_id) values($1,$2,$3,'QA','tractor-6x2')", id(), companies[1], "QA"+id()); e != nil {
			t.Fatal(e)
		}
	}
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := `{"plate":"XYZ` + []string{"1A23", "2B34"}[i] + `","model":"QA Truck","template_id":"tractor-6x2"}`
			req := httptest.NewRequest("POST", "/api/v1/vehicles", bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+tokens[1])
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			results <- w.Code
		}(i)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for result := range results {
		counts[result]++
	}
	if counts[201] != 1 || counts[402] != 1 {
		t.Fatal(counts)
	}
	// Runtime RLS cannot see the other company's invoices; payment application is once-only.
	inv, payment := id(), "pay_"+stringsWithoutDashes(id())
	if _, e = admin.Exec(ctx, `insert into billing_invoices(id,company_id,plan_id,price_cents,vehicle_limit,user_limit,provider_id,due_date,environment) values($1,$2,'starter',14900,15,3,$3,current_date,'sandbox')`, inv, companies[1], payment); e != nil {
		t.Fatal(e)
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = tx.Exec(ctx, "select set_config('app.company_id',$1,true)", companies[0])
	var n int
	e = tx.QueryRow(ctx, "select count(*) from billing_invoices where id=$1", inv).Scan(&n)
	_ = tx.Rollback(ctx)
	if e != nil || n != 0 {
		t.Fatalf("invoice RLS: %d %v", n, e)
	}
	for i := 0; i < 2; i++ {
		if _, e = db.Exec(ctx, "select billing_settle($1,$2,'CONFIRMED',14900,'sandbox')", "evt_"+inv, payment); e != nil {
			t.Fatal(e)
		}
	}
	var first, second time.Time
	if e = admin.QueryRow(ctx, "select paid_until from subscriptions where company_id=$1", companies[1]).Scan(&first); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(ctx, "select billing_settle($1,$2,'RECEIVED',14900,'sandbox')", "evt_new_"+inv, payment); e != nil {
		t.Fatal(e)
	}
	_ = admin.QueryRow(ctx, "select paid_until from subscriptions where company_id=$1", companies[1]).Scan(&second)
	if !first.Equal(second) {
		t.Fatal("duplicate payment extended entitlement twice")
	}
	if _, e = db.Exec(ctx, "select billing_settle($1,$2,'CONFIRMED',1,'sandbox')", "evt_bad_"+inv, payment); e == nil {
		t.Fatal("accepted wrong amount")
	}
	if _, e = db.Exec(ctx, "select billing_settle($1,$2,'REFUNDED',14900,'sandbox')", "evt_refund_"+inv, payment); e != nil {
		t.Fatal(e)
	}
	// A stale payment confirmation after a refund cannot re-enable the period.
	_, e = db.Exec(ctx, "select billing_settle($1,$2,'CONFIRMED',14900,'sandbox')", "evt_stale_"+inv, payment)
	if e != nil {
		t.Fatal(e)
	}
	var status string
	_ = admin.QueryRow(ctx, "select status from billing_invoices where id=$1", inv).Scan(&status)
	if status != "refunded" {
		t.Fatal(status)
	}
	_, _ = admin.Exec(ctx, "delete from platform_admins where user_id=$1", subjects[0])
	request("GET", "/api/v1/platform", 0, "", 403)
}
func stringsWithoutDashes(s string) string {
	out := ""
	for _, c := range s {
		if c != '-' {
			out += string(c)
		}
	}
	return out
}
