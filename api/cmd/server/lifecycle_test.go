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

func TestTireLifecycleAndAutomation(t *testing.T) {
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
	a := &App{db: db, secret: []byte("lifecycle-test-secret-at-least-32-characters")}
	companies, users := []string{id(), id()}, []string{id(), id()}
	for i, c := range companies {
		if _, e = admin.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'Lifecycle QA',$2)", c, "lifecycle-"+c); e != nil {
			t.Fatal(e)
		}
		if _, e = admin.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,'Lifecycle QA',$3,'unused','owner')", users[i], c, c+"@lifecycle.invalid"); e != nil {
			t.Fatal(e)
		}
	}
	defer func() {
		for _, c := range companies {
			_, _ = admin.Exec(ctx, "select set_config('app.company_id',$1,false)", c)
			for _, table := range []string{"tire_operations", "tire_movements", "tire_policies", "checklist_batches", "sync_log", "audit_log", "alerts", "inspections", "tires", "vehicles", "drivers", "refresh_tokens", "account_tokens", "billing_invoices", "subscriptions", "users"} {
				if _, err := admin.Exec(ctx, "delete from "+table+" where company_id=$1", c); err != nil {
					t.Error(err)
				}
			}
			if _, err := admin.Exec(ctx, "delete from companies where id=$1", c); err != nil {
				t.Error(err)
			}
		}
	}()
	token := func(who int, role string) string {
		raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{Company: companies[who], Role: role, RegisteredClaims: jwt.RegisteredClaims{Subject: users[who], Issuer: "geevheittire", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString(a.secret)
		return raw
	}
	tokens := []string{token(0, "owner"), token(1, "owner"), token(0, "driver"), token(0, "inspector")}
	r := chi.NewRouter()
	r.Use(a.auth)
	r.Use(a.billingGuard)
	r.Post("/api/v1/vehicles", a.createVehicle)
	r.Post("/api/v1/tires", a.createTire)
	r.Get("/api/v1/tires/{id}/lifecycle", a.tireLifecycle)
	r.Post("/api/v1/tires/{id}/movements", a.moveTire)
	r.Post("/api/v1/tires/{id}/inspections", a.inspect)
	r.Get("/api/v1/vehicles/{id}/tire-map", a.tireMap)
	r.Patch("/api/v1/vehicles/{id}/odometer", a.updateOdometer)
	r.Patch("/api/v1/tire-policy", a.setTirePolicy)
	r.Patch("/api/v1/tires/{id}/identity", a.updateTireIdentity)
	r.Post("/api/v1/drivers", a.saveDriver)
	r.Patch("/api/v1/drivers/{id}", a.saveDriver)
	r.Get("/api/v1/{resource:drivers|tires}", a.list)
	r.Patch("/api/v1/vehicles/{id}/driver", a.assignDriver)
	request := func(method, path string, who int, body any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/api/v1/"+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+tokens[who])
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: want %d got %d %s", method, path, want, w.Code, w.Body.String())
		}
		var out map[string]any
		if e = json.Unmarshal(w.Body.Bytes(), &out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	snapshot := func(tid string) map[string]any {
		return request("GET", "tires/"+tid+"/lifecycle", 0, nil, 200)["tire"].(map[string]any)
	}
	vehicle := func(plate string, who int) string {
		return request("POST", "vehicles", who, map[string]any{"plate": plate, "model": "QA truck", "template_id": "tractor-6x2", "current_km": 1000}, 201)["id"].(string)
	}
	v1, v2, foreign := vehicle("AAA1B23", 0), vehicle("BBB1C23", 0), vehicle("CCC1D23", 1)
	driver := request("POST", "drivers", 0, map[string]any{"name": "Motorista QA"}, 201)["id"].(string)
	foreignDriver := request("POST", "drivers", 1, map[string]any{"name": "Motorista de outra empresa"}, 201)["id"].(string)
	request("PATCH", "vehicles/"+v1+"/driver", 0, map[string]any{"driver_id": foreignDriver}, 422)
	request("PATCH", "vehicles/"+foreign+"/driver", 0, map[string]any{"driver_id": driver}, 404)
	request("PATCH", "vehicles/"+v1+"/driver", 2, map[string]any{"driver_id": driver}, 403)
	request("GET", "drivers", 2, nil, 403)
	if len(request("GET", "drivers", 0, nil, 200)["data"].([]any)) != 1 {
		t.Fatal("cross-tenant driver leak")
	}
	request("PATCH", "vehicles/"+v1+"/driver", 0, map[string]any{"driver_id": driver}, 200)
	request("PATCH", "drivers/"+driver, 1, map[string]any{"name": "Outra empresa não pode alterar"}, 404)
	request("POST", "drivers", 0, map[string]any{"name": "CNH inválida", "license_number": "123"}, 422)
	expiredDriver := request("POST", "drivers", 0, map[string]any{"name": "CNH vencida QA", "license_number": "12345678901", "license_expires_on": "2020-01-01"}, 201)["id"].(string)
	request("PATCH", "vehicles/"+v2+"/driver", 0, map[string]any{"driver_id": expiredDriver}, 422)
	request("PATCH", "drivers/"+driver, 0, map[string]any{"name": "Motorista QA", "active": false}, 409)
	create := func(code string, installed bool) string {
		body := map[string]any{"code": code, "serial_number": "SERIAL-" + code, "dot_code": "1025", "expires_on": "2035-12-31", "road_use": "MISTO", "brand": "QA Brand", "size": "295/80 R22.5", "tread_mm": 8, "acquisition_cost": 1000}
		if installed {
			body["vehicle_id"] = v1
			body["position"] = "1EE"
		}
		return request("POST", "tires", 0, body, 201)["id"].(string)
	}
	first, second := create("QA-STOCK", false), create("QA-INSTALLED", true)
	if snapshot(second)["driver_name"] != "Motorista QA" || snapshot(first)["serial_number"] != "SERIAL-QA-STOCK" {
		t.Fatal("missing tire identity or vehicle driver")
	}
	registration := request("GET", "tires/"+second+"/lifecycle", 0, nil, 200)["movements"].([]any)[0].(map[string]any)
	if registration["to_driver_name"] != "Motorista QA" || registration["life_number"] != float64(1) {
		t.Fatal("missing immutable driver/life snapshot")
	}
	move := func(tid, action string, extra map[string]any, want int) map[string]any {
		body := map[string]any{"client_id": id(), "action": action, "expected_version": snapshot(tid)["version"], "reason": "QA physical operation confirmed"}
		for k, v := range extra {
			body[k] = v
		}
		request("POST", "tires/"+tid+"/movements", 0, body, want)
		return body
	}
	move(first, "STOCK_TRANSFER", map[string]any{"stock_location": "Galpão de reserva"}, 201)
	firstInstall := move(first, "INSTALL", map[string]any{"vehicle_id": v1, "position": "1DE", "vehicle_km": 1000}, 201)
	request("POST", "tires/"+first+"/movements", 0, firstInstall, 200)
	firstInstall["reason"] = "Changed content using same operation key"
	request("POST", "tires/"+first+"/movements", 0, firstInstall, 409)
	if n := len(request("GET", "tires/"+first+"/lifecycle", 0, nil, 200)["movements"].([]any)); n != 3 {
		t.Fatalf("replay created duplicate event: %d", n)
	}
	request("GET", "tires/"+first+"/lifecycle", 1, nil, 404)
	request("POST", "tires/"+first+"/movements", 2, map[string]any{}, 403)
	request("POST", "tires/"+first+"/movements", 3, map[string]any{}, 403)
	move(first, "TRANSFER", map[string]any{"source_km": 1000, "vehicle_id": foreign, "position": "2EE", "vehicle_km": 1000}, 422)
	move(first, "ROTATE", map[string]any{"source_km": 1100, "vehicle_id": v1, "position": "1EE", "vehicle_km": 1100}, 409)
	move(first, "ROTATE", map[string]any{"source_km": 1100, "vehicle_id": v1, "position": "1EE", "vehicle_km": 1100, "swap_tire_id": second, "swap_version": snapshot(second)["version"]}, 201)
	s1, s2 := snapshot(first), snapshot(second)
	if s1["position"] != "1EE" || s2["position"] != "1DE" || s1["tracked_km"] != float64(100) || s2["tracked_km"] != float64(100) {
		t.Fatalf("swap mismatch: %v %v", s1, s2)
	}
	move(second, "TRANSFER", map[string]any{"source_km": 1100, "vehicle_id": v2, "position": "2EE", "vehicle_km": 1000}, 201)
	move(second, "TRANSFER", map[string]any{"source_km": 1000, "vehicle_id": v1, "position": "1DE", "vehicle_km": 1100}, 201)
	request("PATCH", "vehicles/"+v1+"/odometer", 0, map[string]any{"current_km": 12100}, 200)
	request("PATCH", "vehicles/"+v1+"/odometer", 0, map[string]any{"current_km": 12000}, 422)
	count := func(kind string, active bool) int {
		t.Helper()
		var n int
		query := "select count(*) from alerts where company_id=$1 and kind=$2"
		if active {
			query += " and resolved_at is null"
		}
		if err := admin.QueryRow(ctx, query, companies[0], kind).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// Switch the operator connection's scope before querying tenant-protected tables.
	_, _ = admin.Exec(ctx, "select set_config('app.company_id',$1,false)", companies[0])
	if count("rotation", true) != 2 || count("stock_low", true) != 1 {
		t.Fatal("automatic rotation/stock alerts missing")
	}
	a.maintenanceSweep(ctx)
	if count("rotation", true) != 2 || count("stock_low", true) != 1 {
		t.Fatal("sweep duplicated active alerts")
	}
	move(first, "REMOVE", map[string]any{"source_km": 12100, "stock_location": "Galpão A"}, 201)
	if snapshot(first)["tracked_km"] != float64(11100) {
		t.Fatal("tracked mileage not settled")
	}
	inspect := func(tread float64, extra map[string]any) map[string]any {
		body := map[string]any{"client_id": id(), "tread_mm": tread, "pressure_psi": 100, "structural_damage": false}
		for k, v := range extra {
			body[k] = v
		}
		return request("POST", "tires/"+first+"/inspections", 0, body, 201)
	}
	inspect(4.5, nil)
	if count("wear", true) != 1 || count("retread", true) != 1 {
		t.Fatal("wear/retread warning missing")
	}
	move(first, "INSTALL", map[string]any{"expected_version": 1, "vehicle_id": v2, "position": "1EE", "vehicle_km": 1000}, 409)
	move(first, "RECAP_SEND", map[string]any{"supplier": "Recap QA"}, 201)
	recapStarted := time.Now().Add(-time.Minute)
	move(first, "RECAP_RETURN", map[string]any{"new_tread_mm": 9, "stock_location": "Galpão B", "supplier": "Recap QA", "cost_cents": 30000}, 201)
	if count("wear", true) != 0 || count("retread", true) != 0 {
		t.Fatal("resolved maintenance alerts remained active")
	}
	s1 = snapshot(first)
	if s1["retread_count"] != float64(1) || s1["tread_mm"] != float64(9) || s1["movement_cost_cents"] != float64(30000) {
		t.Fatalf("recap return failed: %v", s1)
	}
	if out := inspect(2, map[string]any{"inspected_at": recapStarted}); out["applied"] != false {
		t.Fatal("old inspection overwrote new recap cycle")
	}
	if snapshot(first)["tread_mm"] != float64(9) {
		t.Fatal("new tread lost")
	}
	request("PATCH", "tire-policy", 0, map[string]any{"warning_mm": 5, "critical_mm": 3, "rotation_km": 10000, "inspection_days": 30, "minimum_stock": 2, "max_retreads": 1}, 200)
	inspect(2.5, nil)
	if count("disposal", true) != 1 {
		t.Fatal("disposal evaluation alert missing")
	}
	move(first, "INSTALL", map[string]any{"vehicle_id": v2, "position": "1EE", "vehicle_km": 1000}, 422)
	move(first, "RECAP_SEND", map[string]any{"supplier": "Recap QA"}, 422)
	move(first, "DISCARD", nil, 201)
	move(first, "INSTALL", map[string]any{"vehicle_id": v2, "position": "1EE", "vehicle_km": 1000}, 409)
	if count("wear", true) != 0 || count("disposal", true) != 0 {
		t.Fatal("discarded tire kept maintenance alerts")
	}
	// Two independent tires race for one slot: one succeeds and the other remains in stock.
	x, y := create("QA-RACE-X", false), create("QA-RACE-Y", false)
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for _, tid := range []string{x, y} {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			body := map[string]any{"client_id": id(), "action": "INSTALL", "expected_version": 1, "vehicle_id": v2, "position": "1EE", "vehicle_km": 1000, "reason": "QA concurrent installation"}
			b, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/api/v1/tires/"+tid+"/movements", bytes.NewReader(b))
			req.Header.Set("Authorization", "Bearer "+tokens[0])
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			results <- w.Code
		}(tid)
	}
	wg.Wait()
	close(results)
	codes := map[int]int{}
	for code := range results {
		codes[code]++
	}
	if codes[201] != 1 || codes[409] != 1 {
		t.Fatal(codes)
	}
	// A spare is carried by the vehicle but does not roll. Swaps account each tire separately.
	spare := create("QA-SPARE", false)
	move(spare, "INSTALL", map[string]any{"vehicle_id": v2, "position": "STEP1", "vehicle_km": 1000}, 201)
	request("PATCH", "vehicles/"+v2+"/odometer", 0, map[string]any{"current_km": 25000}, 200)
	if snapshot(spare)["total_tracked_km"] != float64(0) {
		t.Fatal("spare accumulated road distance")
	}
	move(spare, "ROTATE", map[string]any{"source_km": 25000, "vehicle_id": v2, "position": "2EE", "vehicle_km": 25000}, 201)
	request("PATCH", "vehicles/"+v2+"/odometer", 0, map[string]any{"current_km": 26000}, 200)
	if snapshot(spare)["total_tracked_km"] != float64(1000) {
		t.Fatal("former spare did not accumulate road distance")
	}
	move(spare, "ROTATE", map[string]any{"source_km": 26000, "vehicle_id": v2, "position": "STEP1", "vehicle_km": 26000}, 201)
	request("PATCH", "vehicles/"+v2+"/odometer", 0, map[string]any{"current_km": 27000}, 200)
	if snapshot(spare)["total_tracked_km"] != float64(1000) {
		t.Fatal("tire kept accumulating while in spare position")
	}
	peer := x
	if snapshot(y)["state"] == "INSTALADO" {
		peer = y
	}
	move(spare, "ROTATE", map[string]any{"source_km": 27000, "vehicle_id": v2, "position": "1EE", "vehicle_km": 27000, "swap_tire_id": peer, "swap_version": snapshot(peer)["version"]}, 201)
	request("PATCH", "vehicles/"+v2+"/odometer", 0, map[string]any{"current_km": 28000}, 200)
	if snapshot(spare)["total_tracked_km"] != float64(2000) || snapshot(peer)["total_tracked_km"] != float64(26000) {
		t.Fatal("spare/road swap mixed tracked mileage")
	}
	// Movements are append-only for the runtime role.
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tx.Exec(ctx, "select set_config('app.company_id',$1,true)", companies[0])
	_, err = tx.Exec(ctx, "update tire_movements set reason='tampered' where company_id=$1", companies[0])
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("runtime can alter movement history")
	}
	// A timer sweep generates overdue-inspection warnings without a UI action.
	_, e = admin.Exec(ctx, "update tires set measurement_cycle_at=now()-interval '40 days',latest_inspected_at=null where company_id=$1 and id=$2", companies[0], y)
	if e != nil {
		t.Fatal(e)
	}
	a.maintenanceSweep(ctx)
	if count("inspection_due", true) < 1 {
		t.Fatal("scheduled inspection warning missing")
	}
	// Unknown legacy identity stays unknown until explicitly completed; stale/foreign edits fail.
	legacy := create("QA-LEGACY", false)
	_, e = admin.Exec(ctx, "update tires set dot_code=null,serial_number=null,expires_on=null,road_use='NAO_INFORMADO' where company_id=$1 and id=$2", companies[0], legacy)
	if e != nil {
		t.Fatal(e)
	}
	move(legacy, "INSTALL", map[string]any{"vehicle_id": v1, "position": "2DE", "vehicle_km": 12100}, 422)
	identity := map[string]any{"code": "QA-LEGACY", "serial_number": "SERIAL-LEGACY", "dot_code": "1025", "expires_on": "2026-01-01", "road_use": "TERRA", "expected_version": snapshot(legacy)["version"]}
	request("PATCH", "tires/"+legacy+"/identity", 1, identity, 409)
	request("PATCH", "tires/"+legacy+"/identity", 2, identity, 403)
	request("PATCH", "tires/"+legacy+"/identity", 0, identity, 200)
	request("PATCH", "tires/"+legacy+"/identity", 0, identity, 409)
	move(legacy, "INSTALL", map[string]any{"vehicle_id": v1, "position": "2DE", "vehicle_km": 12100}, 422)
	if count("expiry", true) < 1 {
		t.Fatal("expired tire alert missing")
	}
	for _,item:=range request("GET","tires",0,nil,200)["data"].([]any) {row:=item.(map[string]any);if row["id"]==legacy && row["status"]!="critico" {t.Fatal("expired tire shown as healthy")}}
	identity["expires_on"] = "2035-12-31"
	identity["expected_version"] = snapshot(legacy)["version"]
	request("PATCH", "tires/"+legacy+"/identity", 0, identity, 200)
	move(legacy, "INSTALL", map[string]any{"vehicle_id": v1, "position": "2DE", "vehicle_km": 12100}, 201)
	if count("expiry", true) != 0 {
		t.Fatal("fixed expiry alert not resolved")
	}
	// Reassigning or renaming the driver must not rewrite old movement snapshots.
	request("PATCH", "drivers/"+driver, 0, map[string]any{"name": "Motorista QA atualizado", "active": true}, 200)
	oldRegistration := request("GET", "tires/"+second+"/lifecycle", 0, nil, 200)["movements"].([]any)
	for _, item := range oldRegistration {
		m := item.(map[string]any)
		if m["action"] == "REGISTER" && m["to_driver_name"] != "Motorista QA" {
			t.Fatal("driver history was rewritten")
		}
	}
	request("PATCH", "vehicles/"+v1+"/driver", 0, map[string]any{"driver_id": ""}, 200)
	request("PATCH", "drivers/"+driver, 0, map[string]any{"name": "Motorista QA atualizado", "active": false}, 200)
	request("PATCH", "vehicles/"+v1+"/driver", 0, map[string]any{"driver_id": driver}, 422)
}
