package main

import (
	"encoding/json"
	"errors"
	"geevheittire/api/internal/commercial"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"strings"
	"time"
)

func managementError(w http.ResponseWriter, e error) {
	var p *pgconn.PgError
	if errors.As(e, &p) {
		switch p.Code {
		case "40001":
			problem(w, 409, "O registro mudou. Atualize a tela antes de salvar.")
			return
		case "P0002":
			problem(w, 404, "Registro não encontrado ou já encerrado.")
			return
		case "42501":
			problem(w, 403, "Operação não autorizada para esta conta.")
			return
		case "22023", "23514":
			messages := map[string]string{"Usage exceeds plan limits": "O uso atual excede os limites do plano escolhido.", "Resolve open invoices before closing": "Regularize ou cancele as faturas em aberto antes de encerrar a empresa.", "Administrative account cannot be closed here": "Uma empresa com administradores da plataforma não pode ser encerrada por este fluxo.", "Confirm company CNPJ": "Digite o CNPJ da empresa para confirmar.", "Confirmation required": "A confirmação informada não corresponde ao cadastro."}
			message := messages[p.Message]
			if message == "" {
				message = "Confira os limites, a vigência e a justificativa. Planos sem valor definido não podem ser publicados."
			}
			problem(w, 422, message)
			return
		}
	}
	problem(w, 500, "Não foi possível concluir a operação administrativa.")
}
func validPageID(w http.ResponseWriter, r *http.Request) (string, bool) {
	v := chi.URLParam(r, "id")
	if uuid.Validate(v) != nil {
		problem(w, 404, "Registro não encontrado.")
		return "", false
	}
	return v, true
}
func validReason(s string) bool { return len(strings.TrimSpace(s)) >= 5 && len(s) <= 500 }
func (a *App) platformClients(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	q := r.URL.Query()
	cursor := q.Get("before")
	if cursor != "" && uuid.Validate(cursor) != nil {
		problem(w, 422, "Cursor inválido.")
		return
	}
	var out json.RawMessage
	if e := a.db.QueryRow(r.Context(), "select platform_clients($1,$2,$3,$4)", claims(r).Subject, strings.TrimSpace(q.Get("search")), q.Get("status"), cursor).Scan(&out); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, out)
}
func (a *App) platformClientDetail(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	var out json.RawMessage
	if e := a.db.QueryRow(r.Context(), "select platform_client_detail($1,$2)", claims(r).Subject, tid).Scan(&out); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, out)
}
func (a *App) platformConfigurePlan(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	plan := chi.URLParam(r, "id")
	if _, ok := commercial.Find(plan); !ok {
		problem(w, 422, "Plano desconhecido.")
		return
	}
	var d struct {
		Version   int64  `json:"expected_version"`
		Price     int    `json:"price_cents"`
		Vehicles  int    `json:"vehicle_limit"`
		Users     int    `json:"user_limit"`
		Published bool   `json:"published"`
		Reason    string `json:"reason"`
	}
	if !decode(w, r, &d) {
		return
	}
	if d.Version < 1 || !validReason(d.Reason) || d.Price < 0 || d.Price > 100000000 || d.Vehicles < 1 || d.Vehicles > 100000 || d.Users < 1 || d.Users > 10000 || (d.Published && d.Price == 0) {
		problem(w, 422, "Informe limites válidos e justificativa. Para publicar, defina um valor maior que zero.")
		return
	}
	_, e := a.db.Exec(r.Context(), "select platform_configure_plan($1,$2,$3,$4,$5,$6,$7,$8)", claims(r).Subject, plan, d.Version, d.Price, d.Vehicles, d.Users, d.Published, d.Reason)
	if e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *App) platformClientAccess(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	var d struct {
		Plan    string    `json:"plan_id"`
		Days    int       `json:"days"`
		Updated time.Time `json:"expected_updated_at"`
		Reason  string    `json:"reason"`
	}
	if !decode(w, r, &d) {
		return
	}
	if _, ok := commercial.Find(d.Plan); !ok || d.Days < 1 || d.Days > 366 || d.Updated.IsZero() || !validReason(d.Reason) {
		problem(w, 422, "Confira plano, prazo e justificativa.")
		return
	}
	if _, e := a.db.Exec(r.Context(), "select platform_client_access($1,$2,$3,$4,$5,$6)", claims(r).Subject, tid, d.Plan, d.Days, d.Updated, d.Reason); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *App) platformCloseClient(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	var d struct {
		Confirmation string `json:"confirmation"`
		Reason       string `json:"reason"`
	}
	if !decode(w, r, &d) {
		return
	}
	if !validReason(d.Reason) || len(d.Confirmation) > 100 {
		problem(w, 422, "Informe confirmação e justificativa.")
		return
	}
	if _, e := a.db.Exec(r.Context(), "select platform_close_client($1,$2,$3,$4)", claims(r).Subject, tid, d.Confirmation, d.Reason); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *App) publicPlans(w http.ResponseWriter, r *http.Request) {
	rows, e := a.db.Query(r.Context(), "select id,vehicle_limit,user_limit,price_cents from billing_plans where published")
	if e != nil {
		problem(w, 503, "Não foi possível consultar os planos.")
		return
	}
	defer rows.Close()
	offers := map[string]map[string]int{}
	for rows.Next() {
		var id string
		var vehicles, users, price int
		if e = rows.Scan(&id, &vehicles, &users, &price); e != nil {
			problem(w, 503, "Não foi possível consultar os planos.")
			return
		}
		offers[id] = map[string]int{"vehicle_limit": vehicles, "user_limit": users, "price_cents": price}
	}
	if rows.Err() != nil {
		problem(w, 503, "Não foi possível consultar os planos.")
		return
	}
	out := []map[string]any{}
	for _, p := range commercial.Definition.Plans {
		raw, _ := json.Marshal(p)
		var item map[string]any
		_ = json.Unmarshal(raw, &item)
		item["display_price_cents"] = 0
		if offer, ok := offers[p.ID]; ok {
			item["vehicle_limit"] = offer["vehicle_limit"]
			item["user_limit"] = offer["user_limit"]
			if asaasFromEnv() != nil {
				item["display_price_cents"] = offer["price_cents"]
			}
		}
		out = append(out, item)
	}
	write(w, 200, map[string]any{"plans": out})
}
