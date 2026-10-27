package main

import (
	"context"
	"encoding/json"
	"geevheittire/api/internal/commercial"
	"net/http"
	"strings"
)

type entitlementKey struct{}
type entitlements struct {
	PlanID       string          `json:"plan_id"`
	PlanName     string          `json:"plan_name"`
	Status       string          `json:"status"`
	CanWrite     bool            `json:"can_write"`
	VehicleLimit int             `json:"vehicle_limit"`
	UserLimit    int             `json:"user_limit"`
	Features     map[string]bool `json:"features"`
}

// Entitlements are resolved from the subscription, never a client-supplied tier or JWT feature.
func (a *App) resolveEntitlements(r *http.Request) (entitlements, error) {
	if cached, ok := r.Context().Value(entitlementKey{}).(entitlements); ok {
		return cached, nil
	}
	out := entitlements{PlanID: "starter", PlanName: "Starter", VehicleLimit: 15, UserLimit: 3}
	t, e := a.tx(r)
	if e != nil {
		return out, e
	}
	defer t.Rollback(r.Context())
	var paid, trial, suspended bool
	var plan string
	var vehicles, users int
	e = t.QueryRow(r.Context(), `select coalesce(s.plan_id,''),coalesce(s.paid_until>now(),false),c.trial_ends_at>now(),s.suspended,s.vehicle_limit,s.user_limit from companies c join subscriptions s on s.company_id=c.id where c.id=$1 and c.deleted_at is null`, claims(r).Company).Scan(&plan, &paid, &trial, &suspended, &vehicles, &users)
	if e != nil {
		return out, e
	}
	out.Status = "expired"
	if paid {
		out.Status = "active"
		out.VehicleLimit = vehicles
		out.UserLimit = users
		if p, ok := commercial.Find(plan); ok {
			out.PlanID = p.ID
			out.PlanName = p.Name
		}
	}
	if !paid && trial {
		out.Status = "trial"
	}
	out.CanWrite = (paid || trial) && !suspended
	if suspended {
		out.Status = "suspended"
		out.PlanID = "starter"
		out.PlanName = "Starter"
	}
	out.Features = commercial.Capabilities(out.PlanID)
	return out, nil
}
func (a *App) entitlementContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Billing and platform administration must remain available even if an account needs attention.
		if r.URL.Path == "/api/v1/billing" || r.URL.Path == "/api/v1/billing/checkout" || strings.HasPrefix(r.URL.Path, "/api/v1/platform") || strings.HasPrefix(r.URL.Path, "/api/v1/support/") || r.URL.Path == "/api/v1/account/close" {
			next.ServeHTTP(w, r)
			return
		}
		access, e := a.resolveEntitlements(r)
		if e != nil {
			problem(w, 503, "Não foi possível verificar os recursos do plano.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), entitlementKey{}, access)))
	})
}
func featureProblem(w http.ResponseWriter, feature string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": "Este recurso exige o plano Pro ou Enterprise. Consulte Plano e cobrança.", "status": 403, "code": "feature_unavailable", "feature": feature, "required_plan": "pro"})
}
func (a *App) requireFeature(w http.ResponseWriter, r *http.Request, key string) bool {
	access, e := a.resolveEntitlements(r)
	if e != nil {
		problem(w, 503, "Não foi possível verificar os recursos do plano.")
		return false
	}
	if !access.Features[key] {
		featureProblem(w, key)
		return false
	}
	return true
}
func (a *App) entitlementStatus(w http.ResponseWriter, r *http.Request) {
	access, e := a.resolveEntitlements(r)
	if e != nil {
		problem(w, 503, "Não foi possível consultar o plano.")
		return
	}
	write(w, 200, access)
}
