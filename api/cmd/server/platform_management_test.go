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

func TestPlatformSupportAndAccountClosure(t *testing.T) {
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
	a := &App{db: db, secret: []byte("platform-test-at-least-32-character-secret")}
	companies, users, tokens := []string{}, []string{}, []string{}
	password := "Temporary-QA-Password-2026"
	for i := 0; i < 3; i++ {
		company, user := id(), id()
		companies = append(companies, company)
		users = append(users, user)
		if _, e = owner.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'Management QA',$2)", company, "qa-"+company); e != nil {
			t.Fatal(e)
		}
		if _, e = owner.Exec(ctx, "select set_config('app.company_id',$1,false)", company); e != nil {
			t.Fatal(e)
		}
		if _, e = owner.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,'Management QA',$3,$4,'owner')", user, company, user+"@qa.invalid", passwordHash(password)); e != nil {
			t.Fatal(e)
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{company, "owner", jwt.RegisteredClaims{Subject: user, Issuer: "geevheittire", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString(a.secret)
		tokens = append(tokens, token)
	}
	defer func() {
		for _, company := range companies {
			for _, table := range []string{"support_messages", "support_tickets"} {
				if _, err := owner.Exec(ctx, "delete from "+table+" where company_id=$1", company); err != nil {
					t.Error(err)
				}
			}
		}
		for _, company := range companies {
			for _, table := range []string{"support_messages", "support_tickets", "platform_audit", "platform_admins", "audit_log", "refresh_tokens", "account_tokens", "users", "subscriptions", "companies"} {
				var query string
				switch table {
				case "platform_audit":
					query = "delete from platform_audit where actor_id in(select id from users where company_id=$1)"
				case "platform_admins":
					query = "delete from platform_admins where user_id in(select id from users where company_id=$1)"
				case "companies":
					query = "delete from companies where id=$1"
				default:
					query = "delete from " + table + " where company_id=$1"
				}
				if _, err := owner.Exec(ctx, query, company); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	if _, e = owner.Exec(ctx, "insert into platform_admins(user_id) values($1)", users[0]); e != nil {
		t.Fatal(e)
	}
	r := chi.NewRouter()
	r.Use(a.auth)
	r.Get("/platform/clients", a.platformClients)
	r.Get("/platform/clients/{id}", a.platformClientDetail)
	r.Post("/platform/clients/{id}/access", a.platformClientAccess)
	r.Get("/platform/tickets", a.platformTickets)
	r.Get("/platform/tickets/{id}", a.platformTicketDetail)
	r.Patch("/platform/tickets/{id}", a.platformUpdateTicket)
	r.Post("/support/tickets", a.createSupportTicket)
	r.Get("/support/tickets/{id}", a.supportTicketDetail)
	r.Get("/account", a.accountStatus)
	r.Post("/account/close", a.closeAccount)
	request := func(who int, method, path string, body any, want int) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+tokens[who])
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s want %d got %d %s", method, path, want, rec.Code, rec.Body.String())
		}
		out := map[string]any{}
		if e = json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	request(1, "GET", "/platform/clients", nil, 403)
	request(0, "GET", "/platform/clients?search=Management", nil, 200)
	detail := request(0, "GET", "/platform/clients/"+companies[1], nil, 200)
	if bytes.Contains(mustJSON(detail), []byte("password_hash")) {
		t.Fatal("password hash exposed")
	}
	subscription := detail["subscription"].(map[string]any)
	grant := map[string]any{"plan_id": "pro", "days": 30, "expected_updated_at": subscription["updated_at"], "reason": "QA manual access grant"}
	request(0, "POST", "/platform/clients/"+companies[1]+"/access", grant, 200)
	request(0, "POST", "/platform/clients/"+companies[1]+"/access", grant, 409)
	ticket := request(1, "POST", "/support/tickets", map[string]any{"subject": "Erro ao abrir ficha", "description": "A ficha não abre quando clico no pneu.", "category": "bug", "module": "Estoque", "page_path": "/app"}, 201)["id"].(string)
	request(2, "GET", "/support/tickets/"+ticket, nil, 404)
	request(1, "GET", "/platform/tickets", nil, 403)
	request(0, "GET", "/platform/tickets", nil, 200)
	update := map[string]any{"expected_version": 1, "status": "in_progress", "priority": "high", "reason": "QA internal investigation", "reply": "Nota reservada da equipe", "internal": true}
	request(0, "PATCH", "/platform/tickets/"+ticket, update, 200)
	request(0, "PATCH", "/platform/tickets/"+ticket, update, 409)
	visible := request(1, "GET", "/support/tickets/"+ticket, nil, 200)
	if len(visible["messages"].([]any)) != 0 {
		t.Fatal("internal note leaked")
	}
	private := request(0, "GET", "/platform/tickets/"+ticket, nil, 200)
	if len(private["messages"].([]any)) != 1 {
		t.Fatal("admin note missing")
	}
	request(1, "POST", "/account/close", map[string]any{"password": "Incorrect-Password", "confirmation": "qa-" + companies[1]}, 401)
	request(1, "GET", "/account", nil, 200)
	request(0, "POST", "/account/close", map[string]any{"password": password, "confirmation": "qa-" + companies[0]}, 403)
	request(1, "POST", "/account/close", map[string]any{"password": password, "confirmation": "qa-" + companies[1]}, 200)
	request(1, "GET", "/account", nil, 401)
	request(2, "GET", "/account", nil, 200)
	var archived bool
	if e = owner.QueryRow(ctx, "select deleted_at is not null from companies where id=$1", companies[1]).Scan(&archived); e != nil || !archived {
		t.Fatal("company not archived", e)
	}
}
func mustJSON(v any) []byte { raw, _ := json.Marshal(v); return raw }
