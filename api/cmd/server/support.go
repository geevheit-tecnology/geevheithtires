package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

func oneOf(v string, values ...string) bool {
	for _, s := range values {
		if v == s {
			return true
		}
	}
	return false
}
func (a *App) createSupportTicket(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Subject     string `json:"subject"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Module      string `json:"module"`
		Path        string `json:"page_path"`
	}
	if !decode(w, r, &d) {
		return
	}
	d.Subject = strings.TrimSpace(d.Subject)
	d.Description = strings.TrimSpace(d.Description)
	d.Module = strings.TrimSpace(d.Module)
	if len(d.Subject) < 5 || len(d.Subject) > 160 || len(d.Description) < 10 || len(d.Description) > 5000 || len(d.Module) < 2 || len(d.Module) > 80 || !oneOf(d.Category, "bug", "error", "question", "suggestion") || len(d.Path) > 200 || strings.ContainsAny(d.Path, "?#") || (d.Path != "" && !strings.HasPrefix(d.Path, "/")) {
		problem(w, 422, "Confira assunto, módulo e descrição. Não inclua senhas, tokens ou dados de pagamento.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	c := claims(r)
	tid := id()
	if _, e = t.Exec(r.Context(), "insert into support_tickets(id,company_id,created_by,subject,description,category,module,page_path) values($1,$2,$3,$4,$5,$6,$7,$8)", tid, c.Company, c.Subject, d.Subject, d.Description, d.Category, d.Module, d.Path); e != nil {
		problem(w, 500, "Não foi possível abrir o chamado.")
		return
	}
	if e = a.audit(r.Context(), t, c, "support_ticket", tid, map[string]string{"subject": d.Subject}); e != nil {
		problem(w, 500, "Não foi possível registrar o chamado.")
		return
	}
	if e = t.Commit(r.Context()); e != nil {
		problem(w, 500, "Não foi possível salvar o chamado.")
		return
	}
	write(w, 201, map[string]string{"id": tid})
}
func (a *App) supportTickets(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("before")
	if cursor != "" && uuid.Validate(cursor) != nil {
		problem(w, 422, "Cursor inválido.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	out, e := rowsJSON(r.Context(), t, "select to_jsonb(t) from support_tickets t where company_id=$1 and ($2='' or id<nullif($2,'')::uuid) order by id desc limit 51", claims(r).Company, cursor)
	if e != nil {
		problem(w, 500, "Não foi possível carregar os chamados.")
		return
	}
	next := ""
	if len(out) > 50 {
		out = out[:50]
		next = out[49]["id"].(string)
	}
	write(w, 200, map[string]any{"data": out, "next_cursor": next})
}
func (a *App) supportTicketDetail(w http.ResponseWriter, r *http.Request) {
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	var ticket json.RawMessage
	if e = t.QueryRow(r.Context(), "select to_jsonb(t) from support_tickets t where company_id=$1 and id=$2", claims(r).Company, tid).Scan(&ticket); e != nil {
		problem(w, 404, "Chamado não encontrado.")
		return
	}
	messages, e := rowsJSON(r.Context(), t, "select to_jsonb(m)||jsonb_build_object('author_name',u.name) from support_messages m left join users u on u.id=m.author_id where m.company_id=$1 and m.ticket_id=$2 and not m.internal order by m.created_at,m.id", claims(r).Company, tid)
	if e != nil {
		problem(w, 500, "Não foi possível carregar as respostas.")
		return
	}
	write(w, 200, map[string]any{"ticket": ticket, "messages": messages})
}
func (a *App) replySupportTicket(w http.ResponseWriter, r *http.Request) {
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	var d struct {
		Body string `json:"body"`
	}
	if !decode(w, r, &d) {
		return
	}
	d.Body = strings.TrimSpace(d.Body)
	if len(d.Body) < 2 || len(d.Body) > 5000 {
		problem(w, 422, "Escreva uma mensagem entre 2 e 5000 caracteres.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	var state string
	if e = t.QueryRow(r.Context(), "select status from support_tickets where company_id=$1 and id=$2 for update", claims(r).Company, tid).Scan(&state); e != nil {
		problem(w, 404, "Chamado não encontrado.")
		return
	}
	if state == "closed" {
		problem(w, 409, "Este chamado está encerrado. Abra um novo chamado se precisar.")
		return
	}
	_, e = t.Exec(r.Context(), "insert into support_messages(id,company_id,ticket_id,author_id,body) values($1,$2,$3,$4,$5)", id(), claims(r).Company, tid, claims(r).Subject, d.Body)
	if e != nil {
		problem(w, 500, "Não foi possível salvar a resposta.")
		return
	}
	if e = t.Commit(r.Context()); e != nil {
		problem(w, 500, "Não foi possível concluir a resposta.")
		return
	}
	write(w, 201, map[string]bool{"ok": true})
}
func (a *App) platformTickets(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	q := r.URL.Query()
	if len(q.Get("search")) > 150 || (q.Get("before") != "" && uuid.Validate(q.Get("before")) != nil) || (q.Get("status") != "" && !oneOf(q.Get("status"), "open", "triage", "in_progress", "resolved", "closed")) || (q.Get("priority") != "" && !oneOf(q.Get("priority"), "low", "normal", "high", "critical")) {
		problem(w, 422, "Filtros inválidos.")
		return
	}
	var out json.RawMessage
	if e := a.db.QueryRow(r.Context(), "select platform_tickets($1,$2,$3,$4,$5)", claims(r).Subject, q.Get("search"), q.Get("status"), q.Get("priority"), q.Get("before")).Scan(&out); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, out)
}
func (a *App) platformTicketDetail(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	var out json.RawMessage
	if e := a.db.QueryRow(r.Context(), "select platform_ticket_detail($1,$2)", claims(r).Subject, tid).Scan(&out); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, out)
}
func (a *App) platformUpdateTicket(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	tid, ok := validPageID(w, r)
	if !ok {
		return
	}
	var d struct {
		Version  int64  `json:"expected_version"`
		Status   string `json:"status"`
		Priority string `json:"priority"`
		Reason   string `json:"reason"`
		Reply    string `json:"reply"`
		Internal bool   `json:"internal"`
	}
	if !decode(w, r, &d) {
		return
	}
	if d.Version < 1 || !validReason(d.Reason) || !oneOf(d.Status, "open", "triage", "in_progress", "resolved", "closed") || !oneOf(d.Priority, "low", "normal", "high", "critical") || len(d.Reply) > 5000 {
		problem(w, 422, "Confira situação, prioridade e justificativa.")
		return
	}
	if _, e := a.db.Exec(r.Context(), "select platform_update_ticket($1,$2,$3,$4,$5,$6,$7,$8)", claims(r).Subject, tid, d.Version, d.Status, d.Priority, d.Reason, strings.TrimSpace(d.Reply), d.Internal); e != nil {
		managementError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
