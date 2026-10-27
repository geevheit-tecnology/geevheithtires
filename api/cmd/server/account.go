package main

import (
	"net/http"
	"strings"
)

func (a *App) accountStatus(w http.ResponseWriter, r *http.Request) {
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	var name, cnpj string
	var protected, pending bool
	e = t.QueryRow(r.Context(), `select c.name,c.cnpj,platform_allowed($2),exists(select 1 from billing_invoices where company_id=c.id and status in ('creating','pending','overdue')) from companies c where c.id=$1 and c.deleted_at is null`, claims(r).Company, claims(r).Subject).Scan(&name, &cnpj, &protected, &pending)
	if e != nil {
		problem(w, 503, "Não foi possível consultar a conta.")
		return
	}
	write(w, 200, map[string]any{"company_name": name, "cnpj": cnpj, "owner": claims(r).Role == "owner", "protected_admin": protected, "open_invoices": pending})
}
func (a *App) closeAccount(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Password     string `json:"password"`
		Confirmation string `json:"confirmation"`
	}
	if !decode(w, r, &d) {
		return
	}
	if len(d.Password) < 10 || len(d.Password) > 128 || len(d.Confirmation) > 100 {
		problem(w, 422, "Informe a senha e a confirmação de encerramento.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	c := claims(r)
	var hash, email string
	var neon *string
	e = t.QueryRow(r.Context(), "select password_hash,email,neon_subject from users where id=$1 and company_id=$2 and deleted_at is null", c.Subject, c.Company).Scan(&hash, &email, &neon)
	if e != nil {
		problem(w, 401, "Conta indisponível.")
		return
	}
	valid := false
	if neon != nil {
		subject, err := neonPassword(r.Context(), email, d.Password)
		valid = err == nil && subject == *neon
	} else {
		valid = passwordOK(d.Password, hash)
	}
	if !valid {
		problem(w, 401, "Senha atual incorreta.")
		return
	}
	var mode string
	if e = t.QueryRow(r.Context(), "select account_close($1,$2)", c.Subject, strings.TrimSpace(d.Confirmation)).Scan(&mode); e != nil {
		managementError(w, e)
		return
	}
	if e = a.audit(r.Context(), t, c, "account_closure", c.Subject, map[string]string{"scope": mode}); e != nil {
		problem(w, 500, "Não foi possível registrar o encerramento.")
		return
	}
	if e = t.Commit(r.Context()); e != nil {
		problem(w, 500, "Não foi possível concluir o encerramento.")
		return
	}
	write(w, 200, map[string]any{"closed": true, "scope": mode, "history_archived": true})
}
