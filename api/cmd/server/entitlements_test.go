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
	"testing"
	"time"
)

func TestPlanEntitlementsCannotBeElevatedByClient(t *testing.T) {
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
	a := &App{db: db, secret: []byte("entitlement-test-secret-at-least-32-characters")}
	company, user := id(), id()
	if _, e = admin.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'Plan QA',$2)", company, "plan-"+company); e != nil {
		t.Fatal(e)
	}
	if _, e = admin.Exec(ctx, "select set_config('app.company_id',$1,false)", company); e != nil {
		t.Fatal(e)
	}
	defer func() {
		for _, table := range []string{"tire_operations", "tire_movements", "tire_policies", "audit_log", "alerts", "tires", "vehicles", "users", "subscriptions", "companies"} {
			column := "company_id"
			if table == "companies" {
				column = "id"
			}
			if _, err := admin.Exec(ctx, "delete from "+table+" where "+column+"=$1", company); err != nil {
				t.Error(err)
			}
		}
	}()
	if _, e = admin.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,'Plan QA',$3,'unused','owner')", user, company, user+"@qa.invalid"); e != nil {
		t.Fatal(e)
	}
	// Deliberately forged extra claims must not change server-owned capabilities.
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"company_id": company, "sub": user, "role": "owner", "plan_id": "enterprise", "features": map[string]bool{"brand_analytics": true}, "iss": "geevheittire", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(a.secret)
	r := chi.NewRouter()
	r.Use(a.auth)
	r.Use(a.billingGuard)
	r.Use(a.entitlementContext)
	r.Get("/api/v1/entitlements", a.entitlementStatus)
	r.Get("/api/v1/dashboard", a.dashboard)
	r.Get("/api/v1/{resource:tires}", a.list)
	r.Get("/api/v1/tire-reports", a.tireReports)
	r.Patch("/api/v1/tire-policy", a.setTirePolicy)
	r.Get("/api/v1/tire-policy", a.getTirePolicy)
	r.Post("/api/v1/tires", a.createTire)
	r.Post("/api/v1/tires/{id}/movements", a.moveTire)
	request := func(method, path string, body any, want int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/api/v1/"+path, bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Plan", "enterprise")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s want %d got %d: %s", method, path, want, rec.Code, rec.Body.String())
		}
		var out map[string]any
		if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	premiumDenied := func() {
		t.Helper()
		out := request("GET", "entitlements?plan_id=pro", nil, 200)
		if out["plan_id"] != "starter" || out["features"].(map[string]any)["brand_analytics"] != false {
			t.Fatal("Starter acquired premium access", out)
		}
		for _, action := range []string{"RECAP_SEND", "RECAP_RETURN", "STOCK_TRANSFER"} {
			out = request("POST", "tires/"+id()+"/movements", map[string]any{"action": action}, 403)
			if out["code"] != "feature_unavailable" {
				t.Fatal("missing feature denial", out)
			}
		}
		request("PATCH", "tire-policy", map[string]any{}, 403)
		for _, q := range []string{"from=2026-01-01", "to=2026-12-31", "before=" + id()} {
			request("GET", "tire-reports?"+q, nil, 403)
		}
		out = request("GET", "tire-reports?plan_id=pro&scope=brands", nil, 200)
		if len(out["brands"].([]any)) != 0 || out["can_compare_brands"] != false || out["limited"] != true {
			t.Fatal("premium analytics leaked", out)
		}
	}
	premiumDenied() // Default trial is Starter.
	if _, e = admin.Exec(ctx, "update subscriptions set plan_id='starter',paid_until=now()+interval '1 day' where company_id=$1", company); e != nil {
		t.Fatal(e)
	}
	premiumDenied() // Paid Starter cannot elevate itself either.
	if _, e = admin.Exec(ctx, "update subscriptions set plan_id='pro',paid_until=now()+interval '1 day',vehicle_limit=60,user_limit=10 where company_id=$1", company); e != nil {
		t.Fatal(e)
	}
	out := request("GET", "entitlements", nil, 200)
	features := out["features"].(map[string]any)
	if out["plan_id"] != "pro" || features["brand_analytics"] != true || features["ai_assistant"] != false || features["public_api"] != false {
		t.Fatal("incorrect Pro capabilities", out)
	}
	rules := map[string]any{"warning_mm": 6, "critical_mm": 4, "rotation_km": 5000, "inspection_days": 20, "minimum_stock": 3, "max_retreads": 3}
	request("PATCH", "tire-policy", rules, 200)
	if out = request("GET", "tire-policy", nil, 200); out["rotation_km"] != float64(5000) {
		t.Fatal("Pro custom policy missing")
	}
	tire := request("POST", "tires", map[string]any{"code": "PLAN-TIRE", "serial_number": "PLAN-SERIAL", "dot_code": "1025", "expires_on": "2035-12-31", "road_use": "MISTO", "brand": "Plan Brand", "size": "295/80 R22.5", "tread_mm": 3.5, "acquisition_cost": 1000}, 201)["id"].(string)
	if request("GET", "dashboard", nil, 200)["critical"] != float64(1) {
		t.Fatal("Pro dashboard ignored custom rules")
	}
	if request("GET", "tires", nil, 200)["data"].([]any)[0].(map[string]any)["status"] != "critico" {
		t.Fatal("Pro inventory ignored custom rules")
	}
	// More than one page proves that Starter cannot paginate through advanced reports.
	for i := 0; i < 55; i++ {
		if _, e = admin.Exec(ctx, `insert into tire_movements(id,company_id,tire_id,action,to_state,reason,tread_mm) values($1,$2,$3,'REMOVE','EM_ESTOQUE','QA report occurrence',9)`, id(), company, tire); e != nil {
			t.Fatal(e)
		}
	}
	out = request("GET", "tire-reports?from=2020-01-01", nil, 200)
	if out["limited"] != false || len(out["brands"].([]any)) != 1 || out["next_cursor"] == "" {
		t.Fatal("Pro reports incomplete", out)
	}
	cursor := out["next_cursor"].(string)
	request("GET", "tire-reports?before="+cursor, nil, 200)
	// Same JWT, no relogin: downgrade takes effect immediately.
	if _, e = admin.Exec(ctx, "update subscriptions set plan_id='starter',vehicle_limit=15,user_limit=3 where company_id=$1", company); e != nil {
		t.Fatal(e)
	}
	premiumDenied()
	out = request("GET", "tire-reports", nil, 200)
	if len(out["events"].([]any)) != 50 || out["next_cursor"] != "" {
		t.Fatal("Starter report limit bypassed")
	}
	out = request("GET", "tire-policy", nil, 200)
	if out["rotation_km"] != float64(10000) {
		t.Fatal("Starter retained premium custom rules")
	}
	if request("GET", "dashboard", nil, 200)["critical"] != float64(0) {
		t.Fatal("Starter dashboard retained Pro thresholds")
	}
	if request("GET", "tires", nil, 200)["data"].([]any)[0].(map[string]any)["status"] != "alerta" {
		t.Fatal("Starter inventory retained Pro thresholds")
	}
	// Expired Pro returns to Starter entitlements, even while trial is still active.
	if _, e = admin.Exec(ctx, "update subscriptions set plan_id='pro',paid_until=now()-interval '1 day' where company_id=$1", company); e != nil {
		t.Fatal(e)
	}
	premiumDenied()
	if _, e = admin.Exec(ctx, "update subscriptions set plan_id='pro',paid_until=now()+interval '1 day',suspended=true where company_id=$1", company); e != nil {
		t.Fatal(e)
	}
	out = request("GET", "entitlements", nil, 200)
	if out["features"].(map[string]any)["advanced_reports"] != false || out["can_write"] != false {
		t.Fatal("suspended subscription retained access")
	}
	request("POST", "tires/"+tire+"/movements", map[string]any{"action": "REMOVE"}, 402)
}
