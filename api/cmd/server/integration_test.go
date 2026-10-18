package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestTenantFlow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", url)
	ctx := context.Background()
	db, e := newPool(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	admin, e := pgx.Connect(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(ctx)
	a := &App{db: db, secret: []byte("test-secret-with-at-least-32-characters"), url: "http://localhost:3000", directory: fakeDirectory(t)}
	r := chi.NewRouter()
	r.Post("/register", a.register)
	r.Post("/login", a.login)
	r.Post("/refresh", a.refresh)
	r.Post("/logout", a.logout)
	r.Post("/accept", a.accept)
	r.Group(func(r chi.Router) {
		r.Use(a.auth)
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			tx, e := a.tx(r)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			var n int
			e = tx.QueryRow(ctx, "select count(*) from users").Scan(&n)
			if e != nil {
				t.Fatal(e)
			}
			write(w, 200, map[string]int{"users": n})
		})
		r.Get("/dashboard", a.dashboard)
		r.Get("/company", a.companyProfile)
		r.Get("/{resource:vehicles|tires|users|alerts}", a.list)
		r.Post("/vehicles", a.createVehicle)
		r.Post("/tires", a.createTire)
		r.Get("/vehicles/{id}/tire-map", a.tireMap)
		r.Post("/tires/{id}/inspections", a.inspect)
		r.Post("/checklists", a.checklist)
		r.Get("/tires/{id}/history", a.history)
	})
	request := func(method, path, token string, body any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: expected %d got %d: %s", method, path, want, w.Code, w.Body.String())
		}
		var out map[string]any
		if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	cnpj := func() string {
		u := uuid.NewString()
		raw := ""
		for _, ch := range u {
			if ch >= '0' && ch <= '9' {
				raw += string(ch)
			}
		}
		for len(raw) < 12 {
			raw += "1"
		}
		raw = raw[:12]
		for n := 12; n < 14; n++ {
			sum := 0
			weight := n - 7
			for i := 0; i < n; i++ {
				sum += int(raw[i]-'0') * weight
				weight--
				if weight == 1 {
					weight = 9
				}
			}
			digit := 11 - sum%11
			if digit >= 10 {
				digit = 0
			}
			raw += string(rune('0' + digit))
		}
		return raw
	}
	emails := []string{"test-" + uuid.NewString() + "@example.com", "test-" + uuid.NewString() + "@example.com"}
	companies := []string{}
	defer func() {
		for _, cid := range companies {
			for _, table := range []string{"tire_operations", "tire_movements", "tire_policies", "checklist_batches", "sync_log", "audit_log", "alerts", "inspections", "tires", "vehicles", "refresh_tokens", "account_tokens", "users", "billing_invoices", "subscriptions"} {
				if _, e := admin.Exec(ctx, "delete from "+table+" where company_id=$1", cid); e != nil {
					t.Error(e)
				}
			}
			if _, e := admin.Exec(ctx, "delete from companies where id=$1", cid); e != nil {
				t.Error(e)
			}
		}
	}()
	tokens := []string{}
	sessions := []map[string]any{}
	for i, email := range emails {
		street := "Rua A"
		if i == 1 {
			street = "Rua B"
		}
		s := request("POST", "/register", "", map[string]any{"name": "Gestor teste", "email": email, "password": "senha-segura-123", "company": "Frota de teste", "cnpj": cnpj(), "address": companyAddress{PostalCode: "01001-000", Street: street, Number: "10", District: "Sé", City: "São Paulo", State: "sp"}}, 201)
		sessions = append(sessions, s)
		companies = append(companies, s["user"].(map[string]any)["company_id"].(string))
		tokens = append(tokens, s["access_token"].(string))
	}
	request("GET", "/company", "", nil, 401)
	profile := request("GET", "/company?company_id="+companies[1], tokens[0], nil, 200)
	if profile["id"] != companies[0] || profile["legal_name"] != "Empresa QA LTDA" || profile["cnpj_verified_at"] == nil || profile["source"] != "BrasilAPI" || profile["address"].(map[string]any)["street"] != "Rua A" || profile["address"].(map[string]any)["state"] != "SP" {
		t.Fatal("company verification/address persistence or tenant isolation failed")
	}
	request("POST", "/login", "", map[string]string{"email": emails[0], "password": "incorreta"}, 401)
	request("POST", "/login", "", map[string]string{"email": emails[0], "password": "senha-segura-123"}, 200)
	vehicle := request("POST", "/vehicles", tokens[0], map[string]any{"plate": "ABC1D23", "model": "Volvo FH", "template_id": "tractor-6x2", "current_km": 125000}, 201)["id"].(string)
	request("POST", "/vehicles", tokens[0], map[string]any{"plate": "ABC1D23", "model": "Volvo FH", "template_id": "tractor-6x2", "current_km": 0}, 409)
	request("GET", "/vehicles/"+vehicle+"/tire-map", tokens[1], nil, 404)
	list := request("GET", "/vehicles", tokens[1], nil, 200)
	if len(list["data"].([]any)) != 0 {
		t.Fatal("cross-tenant vehicle leak")
	}
	tireBody := map[string]any{"code": "P00123", "brand": "Michelin", "model": "X", "size": "295/80R22.5", "tread_mm": 8, "acquisition_cost": 2500, "vehicle_id": vehicle, "position": "1EE"}
	request("POST", "/tires", tokens[1], tireBody, 422)
	tire := request("POST", "/tires", tokens[0], tireBody, 201)["id"].(string)
	tireBody["code"] = "P00124"
	request("POST", "/tires", tokens[0], tireBody, 409)
	request("GET", "/tires/"+tire+"/history", tokens[1], nil, 200)
	client := id()
	inspection := map[string]any{"client_id": client, "tread_mm": 8, "pressure_psi": 85, "structural_damage": true}
	request("POST", "/tires/"+tire+"/inspections", tokens[1], inspection, 404)
	createdInspection := request("POST", "/tires/"+tire+"/inspections", tokens[0], inspection, 201)
	replayedInspection := request("POST", "/tires/"+tire+"/inspections", tokens[0], inspection, 200)
	if createdInspection["id"] != replayedInspection["id"] || createdInspection["classification"].(map[string]any)["recommendation"] != replayedInspection["classification"].(map[string]any)["recommendation"] {
		t.Fatal("inspection replay changed the response contract")
	}
	// Independent rules: structural wear, disposal evaluation, pressure and low stock.
	d := request("GET", "/dashboard", tokens[0], nil, 200)
	if d["critical"].(float64) != 1 || d["alerts"].(float64) != 4 {
		t.Fatalf("unexpected dashboard %+v", d)
	}
	m := request("GET", "/vehicles/"+vehicle+"/tire-map", tokens[0], nil, 200)
	if m["tires"].([]any)[0].(map[string]any)["status"] != "critico" {
		t.Fatal("structural damage missing in map")
	}
	raw := sessions[0]["refresh_token"].(string)
	renewed := request("POST", "/refresh", "", map[string]string{"refresh_token": raw}, 200)
	request("POST", "/refresh", "", map[string]string{"refresh_token": raw}, 401)
	request("POST", "/logout", "", map[string]string{"refresh_token": renewed["refresh_token"].(string)}, 200)
	request("POST", "/refresh", "", map[string]string{"refresh_token": renewed["refresh_token"].(string)}, 401)
	// Unscoped runtime reads must return nothing, including private authentication tables.
	conn, e := db.Acquire(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Release()
	for _, table := range []string{"companies", "users", "refresh_tokens", "account_tokens", "vehicles", "tires", "inspections", "alerts", "audit_log"} {
		var n int
		if e = conn.QueryRow(ctx, "select count(*) from "+table).Scan(&n); e != nil || n != 0 {
			t.Fatalf("unscoped table %s leaked %d rows (%v)", table, n, e)
		}
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "select set_config('app.company_id',$1,true)", companies[1]); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = tx.QueryRow(ctx, "select count(*) from users").Scan(&count); e != nil || count != 1 {
		t.Fatalf("identity RLS failed %d %v", count, e)
	}
	_, e = tx.Exec(ctx, "insert into vehicles(id,company_id,plate,model,template_id) values($1,$2,'ZZZ1Z99','Illegal','tractor-6x2')", id(), companies[0])
	if e == nil {
		t.Fatal("RLS accepted foreign tenant write")
	}
	// Verification and reset tokens are single-use; reset revokes sessions.
	uid := sessions[0]["user"].(map[string]any)["id"].(string)
	for _, kind := range []string{"verify", "reset"} {
		rawToken := randomToken()
		_, e = admin.Exec(ctx, "insert into account_tokens(id,company_id,user_id,token_hash,kind,expires_at) values($1,$2,$3,$4,$5,now()+interval '1 hour')", id(), companies[0], uid, hashToken(rawToken), kind)
		if e != nil {
			t.Fatal(e)
		}
		body := map[string]string{"token": rawToken}
		if kind == "reset" {
			body["password"] = "uma-nova-senha-123"
		}
		request("POST", "/accept", "", body, 200)
		request("POST", "/accept", "", body, 422)
	}
	request("POST", "/login", "", map[string]string{"email": emails[0], "password": "senha-segura-123"}, 401)
	request("POST", "/login", "", map[string]string{"email": emails[0], "password": "uma-nova-senha-123"}, 200)

}
