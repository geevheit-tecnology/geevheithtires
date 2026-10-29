package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (a *App) platformFinance(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	q := r.URL.Query()
	start, e1 := time.Parse("2006-01-02", q.Get("from"))
	end, e2 := time.Parse("2006-01-02", q.Get("to"))
	if e1 != nil || e2 != nil || end.Before(start) || end.Sub(start) > 366*24*time.Hour || len(q.Get("search")) > 150 || (q.Get("before") != "" && uuid.Validate(q.Get("before")) != nil) {
		problem(w, 422, "Informe um período válido de até 366 dias e confira os filtros.")
		return
	}
	var out json.RawMessage
	if e := a.db.QueryRow(r.Context(), "select platform_finance($1,$2,$3,$4,$5,$6)", claims(r).Subject, start, end, q.Get("status"), strings.TrimSpace(q.Get("search")), q.Get("before")).Scan(&out); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, out)
}

func (a *App) platformCreateReceivable(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	var d struct {
		ID      string `json:"id"`
		Company string `json:"company_id"`
		Plan    string `json:"plan_id"`
		Cents   int    `json:"price_cents"`
		Due     string `json:"due_date"`
		URL     string `json:"checkout_url"`
		Reason  string `json:"reason"`
	}
	if !decode(w, r, &d) {
		return
	}
	due, err := time.Parse("2006-01-02", d.Due)
	link, linkErr := url.Parse(d.URL)
	if uuid.Validate(d.ID) != nil || uuid.Validate(d.Company) != nil || d.Cents < 1 || d.Cents > 100000000 || err != nil || !validReason(d.Reason) || len(d.URL) > 2000 || (d.URL != "" && (linkErr != nil || link.Scheme != "https" || link.Hostname() == "" || link.User != nil)) {
		problem(w, 422, "Confira cliente, valor, vencimento, justificativa e link HTTPS opcional.")
		return
	}
	var invoice string
	if err = a.db.QueryRow(r.Context(), "select platform_create_receivable($1,$2,$3,$4,$5,$6,$7,$8)", claims(r).Subject, d.ID, d.Company, d.Plan, d.Cents, due, d.URL, strings.TrimSpace(d.Reason)).Scan(&invoice); err != nil {
		managementError(w, err)
		return
	}
	write(w, 201, map[string]string{"id": invoice})
}

func (a *App) platformSettleReceivable(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	target, ok := validPageID(w, r)
	if !ok {
		return
	}
	var d struct {
		Version   int64  `json:"expected_version"`
		Operation string `json:"operation"`
		Reference string `json:"reference"`
		PaidOn    string `json:"paid_on"`
		Reason    string `json:"reason"`
	}
	if !decode(w, r, &d) {
		return
	}
	var paidOn any
	if d.PaidOn != "" {
		parsed, err := time.Parse("2006-01-02", d.PaidOn)
		if err != nil {
			problem(w, 422, "Data do recebimento inválida.")
			return
		}
		paidOn = parsed
	}
	if d.Version < 1 || !validReason(d.Reason) || len(d.Reference) > 200 || (d.Operation != "pay" && d.Operation != "cancel" && d.Operation != "refund") || (d.Operation == "pay" && (len(strings.TrimSpace(d.Reference)) < 3 || paidOn == nil)) {
		problem(w, 422, "Confira a operação, a referência do pagamento e a justificativa.")
		return
	}
	var result json.RawMessage
	if err := a.db.QueryRow(r.Context(), "select platform_settle_receivable($1,$2,$3,$4,$5,$6,$7)", claims(r).Subject, target, d.Version, d.Operation, strings.TrimSpace(d.Reference), paidOn, strings.TrimSpace(d.Reason)).Scan(&result); err != nil {
		managementError(w, err)
		return
	}
	write(w, 200, result)
}

func (a *App) platformProfile(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	var d struct {
		Name     string `json:"name"`
		Current  string `json:"current_password"`
		Password string `json:"new_password"`
	}
	if !decode(w, r, &d) {
		return
	}
	d.Name = strings.TrimSpace(d.Name)
	if len(d.Name) < 2 || len(d.Name) > 100 || len(d.Current) < 10 || len(d.Current) > 128 || (d.Password != "" && (len(d.Password) < 10 || len(d.Password) > 128)) {
		problem(w, 422, "Informe seu nome, a senha atual e uma nova senha de 10 a 128 caracteres, se desejar alterá-la.")
		return
	}
	tx, err := a.tx(r)
	if err != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer tx.Rollback(r.Context())
	var hash, email string
	var subject *string
	err = tx.QueryRow(r.Context(), "select password_hash,email,neon_subject from users where id=$1 and company_id=$2 and deleted_at is null for update", claims(r).Subject, claims(r).Company).Scan(&hash, &email, &subject)
	if err != nil {
		problem(w, 401, "Conta indisponível.")
		return
	}
	valid := false
	if subject != nil {
		s, e := neonPassword(r.Context(), email, d.Current)
		valid = e == nil && s == *subject
	} else {
		valid = passwordOK(d.Current, hash)
	}
	if !valid {
		problem(w, 401, "Senha atual incorreta.")
		return
	}
	changed := d.Password != ""
	if changed && subject != nil {
		problem(w, 422, "Esta identidade usa Neon Auth. Altere a senha pelo fluxo de recuperação.")
		return
	}
	if changed {
		hash = passwordHash(d.Password)
	}
	_, err = tx.Exec(r.Context(), "update users set name=$2,password_hash=$3,session_version=session_version+case when $4 then 1 else 0 end,updated_at=now() where id=$1", claims(r).Subject, d.Name, hash, changed)
	if err == nil && changed {
		_, err = tx.Exec(r.Context(), "update refresh_tokens set revoked_at=now() where user_id=$1 and revoked_at is null", claims(r).Subject)
	}
	if err == nil && changed {
		_, err = tx.Exec(r.Context(), "update account_tokens set used_at=now() where user_id=$1 and used_at is null", claims(r).Subject)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "select platform_audit_profile($1,$2)", claims(r).Subject, changed)
	}
	if err != nil {
		problem(w, 500, "Não foi possível salvar seu perfil.")
		return
	}
	u := identity{ID: claims(r).Subject, Company: claims(r).Company, Name: d.Name, Email: email, Role: claims(r).Role}
	session, err := a.session(r.Context(), tx, u)
	if err != nil {
		problem(w, 500, "Não foi possível renovar sua sessão.")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		problem(w, 500, "Não foi possível salvar seu perfil.")
		return
	}
	session["message"] = "Perfil atualizado. Sua sessão atual foi renovada."
	write(w, 200, session)
}
