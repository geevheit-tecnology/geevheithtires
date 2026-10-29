package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestAdministrativeControlPersistenceAndIsolation(t *testing.T) {
	database := os.Getenv("TEST_DATABASE_URL")
	if database == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", database)
	ctx := context.Background()
	owner, e := pgx.Connect(ctx, database)
	if e != nil {
		t.Fatal(e)
	}
	defer owner.Close(ctx)
	db, e := newPool(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := &App{db: db, secret: []byte("admin-control-test-secret-at-least-32-characters")}
	companies, users, tokens := []string{}, []string{}, []string{}
	password := "Administrative-QA-2026"
	for i := 0; i < 3; i++ {
		company, user := id(), id()
		companies = append(companies, company)
		users = append(users, user)
		if _, e = owner.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'Control QA',$2)", company, "control-"+company); e != nil {
			t.Fatal(e)
		}
		if _, e = owner.Exec(ctx, "select set_config('app.company_id',$1,false)", company); e != nil {
			t.Fatal(e)
		}
		if _, e = owner.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,'Control QA',$3,$4,'owner')", user, company, user+"@qa.invalid", passwordHash(password)); e != nil {
			t.Fatal(e)
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{Company: company, Role: "owner", RegisteredClaims: jwt.RegisteredClaims{Subject: user, Issuer: "geevheittire", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString(a.secret)
		tokens = append(tokens, token)
	}
	defer func() {
		_, _ = owner.Exec(ctx, "delete from platform_audit where actor_id=any($1::uuid[])", users)
		_, _ = owner.Exec(ctx, "delete from billing_events where payment_id in(select provider_id from billing_invoices where company_id=any($1::uuid[]))", companies)
		_, _ = owner.Exec(ctx, "delete from platform_admins where user_id=any($1::uuid[])", users)
		for _, company := range companies {
			_, _ = owner.Exec(ctx, "select set_config('app.company_id',$1,false)", company)
			for _, table := range []string{"billing_invoices", "refresh_tokens", "account_tokens", "users", "subscriptions"} {
				if _, err := owner.Exec(ctx, "delete from "+table+" where company_id=$1", company); err != nil {
					t.Error(err)
				}
			}
			_, _ = owner.Exec(ctx, "delete from companies where id=$1", company)
		}
	}()
	if _, e = owner.Exec(ctx, "insert into platform_admins(user_id) values($1)", users[0]); e != nil {
		t.Fatal(e)
	}
	router := chi.NewRouter()
	router.Get("/plans", a.publicPlans)
	router.Post("/login", a.login)
	router.Post("/refresh", a.refresh)
	router.Group(func(r chi.Router) {
		r.Use(a.auth)
		r.Get("/platform", a.platformDashboard)
		r.Put("/platform/plans/{id}", a.platformConfigurePlan)
		r.Get("/platform/finance", a.platformFinance)
		r.Post("/platform/finance", a.platformCreateReceivable)
		r.Patch("/platform/finance/{id}", a.platformSettleReceivable)
		r.Patch("/platform/profile", a.platformProfile)
		r.Get("/billing", a.billingStatus)
		r.Get("/entitlements", a.entitlementStatus)
	})
	request := func(who int, method, path string, body any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		if who >= 0 {
			req.Header.Set("Authorization", "Bearer "+tokens[who])
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s want %d got %d: %s", method, path, want, rec.Code, rec.Body.String())
		}
		out := map[string]any{}
		if json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatal("invalid JSON")
		}
		return out
	}
	request(1, "GET", "/platform/finance?from=2026-01-01&to=2026-12-31", nil, 403)
	request(1, "PATCH", "/platform/profile", map[string]any{}, 403)
	request(0, "GET", "/platform/finance?from=invalid&to=2026-01-01", nil, 422)
	var originalPrice, originalVehicles, originalUsers int
	var originalPublished bool
	if e = owner.QueryRow(ctx, "select price_cents,vehicle_limit,user_limit,published from billing_plans where id='pro'").Scan(&originalPrice, &originalVehicles, &originalUsers, &originalPublished); e != nil {
		t.Fatal(e)
	}
	defer owner.Exec(ctx, "update billing_plans set price_cents=$1,vehicle_limit=$2,user_limit=$3,published=$4 where id='pro'", originalPrice, originalVehicles, originalUsers, originalPublished)
	if _, e = owner.Exec(ctx, "update subscriptions set plan_id='pro',paid_until=now()+interval '1 day' where company_id=$1", companies[1]); e != nil {
		t.Fatal(e)
	}
	var version int64
	if e = owner.QueryRow(ctx, "select version from billing_plans where id='pro'").Scan(&version); e != nil {
		t.Fatal(e)
	}
	out := request(0, "PUT", "/platform/plans/pro", map[string]any{"expected_version": version, "price_cents": 32100, "vehicle_limit": 72, "user_limit": 12, "published": true, "apply_limits": true, "reason": "Publicação para validação isolada"}, 200)
	if out["clients_updated"].(float64) < 1 {
		t.Fatal("existing client limits not synchronized")
	}
	rights := request(1, "GET", "/entitlements", nil, 200)
	if rights["vehicle_limit"] != float64(72) || rights["user_limit"] != float64(12) {
		t.Fatal("client did not receive limits", rights)
	}
	offers := request(-1, "GET", "/plans", nil, 200)["plans"].([]any)
	found := false
	for _, raw := range offers {
		p := raw.(map[string]any)
		if p["id"] == "pro" {
			found = p["display_price_cents"] == float64(32100)
		}
	}
	if !found {
		t.Fatal("published price not reflected without gateway")
	}
	request(0, "PUT", "/platform/plans/pro", map[string]any{"expected_version": version, "price_cents": 1, "vehicle_limit": 72, "user_limit": 12, "published": true, "reason": "Stale update must be rejected"}, 409)
	invoice := id()
	today := time.Now().UTC().Format("2006-01-02")
	create := map[string]any{"id": invoice, "company_id": companies[2], "plan_id": "pro", "price_cents": 32100, "due_date": today, "checkout_url": "", "reason": "Mensalidade contratada para teste"}
	request(1, "POST", "/platform/finance", create, 403)
	request(0, "POST", "/platform/finance", create, 201)
	ownBilling := request(2, "GET", "/billing", nil, 200)
	if len(ownBilling["invoices"].([]any)) != 1 {
		t.Fatal("manual receivable absent for customer")
	}
	otherBilling := request(1, "GET", "/billing", nil, 200)
	if len(otherBilling["invoices"].([]any)) != 0 {
		t.Fatal("receivable leaked across tenants")
	}
	settlement := map[string]any{"expected_version": 1, "operation": "pay", "reference": "COMPROVANTE-QA", "paid_on": today, "reason": "Pagamento conferido para teste"}
	request(1, "PATCH", "/platform/finance/"+invoice, settlement, 403)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, _ := json.Marshal(settlement)
			r := httptest.NewRequest("PATCH", "/platform/finance/"+invoice, bytes.NewReader(b))
			r.Header.Set("Authorization", "Bearer "+tokens[0])
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, r)
			statuses <- rec.Code
		}()
	}
	wg.Wait()
	close(statuses)
	success, conflict := 0, 0
	for s := range statuses {
		if s == 200 {
			success++
		}
		if s == 409 {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("double receipt: success=%d conflict=%d", success, conflict)
	}
	rights = request(2, "GET", "/entitlements", nil, 200)
	if rights["plan_id"] != "pro" {
		t.Fatal("payment did not apply plan")
	}
	finance := request(0, "GET", "/platform/finance?from="+today+"&to="+today+"&search=control-"+companies[2], nil, 200)
	if finance["summary"].(map[string]any)["received_cents"] != float64(32100) {
		t.Fatal("financial total mismatch", finance)
	}
	request(0, "PATCH", "/platform/finance/"+invoice, map[string]any{"expected_version": 2, "operation": "refund", "reason": "Estorno conferido para teste"}, 200)
	rights = request(2, "GET", "/entitlements", nil, 200)
	if rights["plan_id"] == "pro" {
		t.Fatal("refund retained paid access")
	}
	session := request(-1, "POST", "/login", map[string]any{"email": users[0] + "@qa.invalid", "password": password}, 200)
	changed := request(0, "PATCH", "/platform/profile", map[string]any{"name": "Administrador Atualizado", "current_password": password, "new_password": "New-Administrative-QA-2026"}, 200)
	request(0, "GET", "/platform", nil, 401)
	request(-1, "POST", "/refresh", map[string]any{"refresh_token": session["refresh_token"]}, 401)
	tokens[0] = changed["access_token"].(string)
	request(0, "GET", "/platform", nil, 200)
	request(-1, "POST", "/login", map[string]any{"email": users[0] + "@qa.invalid", "password": password}, 401)
	request(-1, "POST", "/login", map[string]any{"email": users[0] + "@qa.invalid", "password": "New-Administrative-QA-2026"}, 200)
}
