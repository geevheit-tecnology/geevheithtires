package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type pixConfig struct {
	Key          string `json:"pix_key"`
	ReceiverName string `json:"receiver_name"`
	ReceiverCity string `json:"receiver_city"`
	BankCode     string `json:"bank_code"`
	BankName     string `json:"bank_name"`
	Agency       string `json:"agency"`
	Account      string `json:"account"`
}

func pixSettings() pixConfig {
	return pixConfig{
		Key:          env("PIX_KEY", "getuliomoreira.engsoftware@gmail.com"),
		ReceiverName: env("PIX_RECEIVER_NAME", "GETULIO MOREIRA"),
		ReceiverCity: env("PIX_RECEIVER_CITY", "BRASIL"),
		BankCode:     env("PIX_BANK_CODE", "380"),
		BankName:     env("PIX_BANK_NAME", "PicPay Serviços S.A."),
		Agency:       env("PIX_BANK_AGENCY", "0001"),
		Account:      env("PIX_BANK_ACCOUNT", "140174949-6"),
	}
}

func emvField(tag, value string) string {
	return fmt.Sprintf("%s%02d%s", tag, len(value), value)
}

func crc16CCITT(data string) string {
	var crc uint16 = 0xFFFF
	for i := 0; i < len(data); i++ {
		crc ^= uint16(data[i]) << 8
		for j := 0; j < 8; j++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return fmt.Sprintf("%04X", crc)
}

func sanitizeEMVASCII(s string, max int) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			b.WriteRune(r)
		}
		if b.Len() >= max {
			break
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "GEEVHEITTIRE"
	}
	return out
}

func sanitizeTxID(ref string) string {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	var b strings.Builder
	for _, r := range ref {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() >= 25 {
			break
		}
	}
	if b.Len() == 0 {
		return "***"
	}
	return b.String()
}

// buildPixPayload generates a standard Banco Central BR Code (PIX Copia e Cola) with CRC16-CCITT.
func buildPixPayload(cfg pixConfig, cents int, txid string) string {
	merchantAccount := emvField("00", "br.gov.bcb.pix") + emvField("01", strings.TrimSpace(cfg.Key))
	additionalData := emvField("05", sanitizeTxID(txid))
	amount := fmt.Sprintf("%.2f", float64(cents)/100.0)
	payload := emvField("00", "01") +
		emvField("26", merchantAccount) +
		emvField("52", "0000") +
		emvField("53", "986") +
		emvField("54", amount) +
		emvField("58", "BR") +
		emvField("59", sanitizeEMVASCII(cfg.ReceiverName, 25)) +
		emvField("60", sanitizeEMVASCII(cfg.ReceiverCity, 15)) +
		emvField("62", additionalData) +
		"6304"
	return payload + crc16CCITT(payload)
}

func newPixReference() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "PIX-GT" + strings.ToUpper(hex.EncodeToString(b))
}

func formatBRL(cents int) string {
	reais := cents / 100
	rem := cents % 100
	return fmt.Sprintf("R$ %d,%02d", reais, rem)
}

func (a *App) createPixCharge(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Plan string `json:"plan_id"`
	}
	if !decode(w, r, &d) {
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)

	c := claims(r)
	var owner bool
	var userName string
	e = t.QueryRow(ctx, "select role='owner',name from users where id=$1 and deleted_at is null", c.Subject).Scan(&owner, &userName)
	if e != nil || !owner {
		problem(w, 403, "A geração de PIX para contratação exige o proprietário da empresa.")
		return
	}

	var suspended bool
	var companyName, companyCNPJ string
	var currentVehicles, currentUsers int
	e = t.QueryRow(ctx, `select s.suspended,c.name,c.cnpj,(select count(*) from vehicles where company_id=$1 and deleted_at is null),(select count(*) from users where company_id=$1 and deleted_at is null) from companies c join subscriptions s on s.company_id=c.id where c.id=$1 and c.deleted_at is null`, c.Company).Scan(&suspended, &companyName, &companyCNPJ, &currentVehicles, &currentUsers)
	if e != nil || suspended {
		problem(w, 403, "Entre em contato com a administração para regularizar a empresa.")
		return
	}

	var planName string
	var cents, vehicles, users int
	if e = t.QueryRow(ctx, "select name,price_cents,vehicle_limit,user_limit from billing_plans where id=$1 and published", d.Plan).Scan(&planName, &cents, &vehicles, &users); e != nil {
		problem(w, 422, "Plano indisponível ou não publicado.")
		return
	}
	if cents <= 0 {
		problem(w, 422, "O valor deste plano ainda não foi definido.")
		return
	}
	if currentVehicles > vehicles || currentUsers > users {
		problem(w, 422, "A utilização atual excede os limites deste plano.")
		return
	}

	if _, e = t.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,7))", c.Company); e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}

	var invoiceID, oldPlan, reference, due, source string
	var oldCents int
	err := t.QueryRow(ctx, `select id,plan_id,price_cents,due_date::text,source,reference from billing_invoices where company_id=$1 and status in ('creating','pending','overdue')`, c.Company).Scan(&invoiceID, &oldPlan, &oldCents, &due, &source, &reference)
	created := false
	if err == nil {
		if oldPlan != d.Plan {
			problem(w, 409, "Já existe uma cobrança aberta para outro plano. Regularize ou aguarde a administração antes de trocar de plano.")
			return
		}
		cents = oldCents
		if strings.TrimSpace(reference) == "" {
			reference = newPixReference()
			if _, e = t.Exec(ctx, "update billing_invoices set reference=$1 where id=$2 and company_id=$3", reference, invoiceID, c.Company); e != nil {
				problem(w, 503, "Não foi possível atualizar o identificador do PIX.")
				return
			}
		}
	} else {
		invoiceID = id()
		due = time.Now().AddDate(0, 0, 3).Format("2006-01-02")
		reference = newPixReference()
		_, e = t.Exec(ctx, `insert into billing_invoices(id,company_id,plan_id,price_cents,vehicle_limit,user_limit,due_date,status,environment,source,reference) values($1,$2,$3,$4,$5,$6,$7,'pending','production','manual',$8)`, invoiceID, c.Company, d.Plan, cents, vehicles, users, due, reference)
		if e != nil {
			problem(w, 503, "Não foi possível gerar a cobrança PIX.")
			return
		}
		created = true
	}

	if created {
		subject := fmt.Sprintf("PIX gerado · %s · %s", companyName, reference)
		if len(subject) > 160 {
			subject = subject[:160]
		}
		desc := fmt.Sprintf(
			"Nova cobrança PIX manual gerada pelo proprietário %s.\nEmpresa: %s (CNPJ %s)\nPlano: %s (%s)\nValor: %s\nVencimento: %s\nIdentificador: %s\n\nConfira o recebimento no extrato do PicPay e confirme a baixa na aba Financeiro do painel administrativo para liberar 30 dias de acesso.",
			userName, companyName, companyCNPJ, planName, d.Plan, formatBRL(cents), due, reference,
		)
		_, _ = t.Exec(ctx, "insert into support_tickets(id,company_id,created_by,subject,description,category,module,page_path) values($1,$2,$3,$4,$5,'question','Financeiro · PIX','/app')", id(), c.Company, c.Subject, subject, desc)
		_ = a.audit(ctx, t, c, "pix_charge_created", invoiceID, map[string]any{"plan_id": d.Plan, "price_cents": cents, "reference": reference})
	}

	if e = t.Commit(ctx); e != nil {
		problem(w, 503, "Não foi possível salvar a cobrança PIX.")
		return
	}

	cfg := pixSettings()
	txid := sanitizeTxID(reference)
	write(w, 200, map[string]any{
		"invoice_id":    invoiceID,
		"plan_id":       d.Plan,
		"plan_name":     planName,
		"price_cents":   cents,
		"due_date":      due,
		"reference":     reference,
		"txid":          txid,
		"pix_payload":   buildPixPayload(cfg, cents, txid),
		"pix_key":       cfg.Key,
		"receiver_name": cfg.ReceiverName,
		"bank_code":     cfg.BankCode,
		"bank_name":     cfg.BankName,
		"agency":        cfg.Agency,
		"account":       cfg.Account,
	})
}

func (a *App) notifyPixPaid(w http.ResponseWriter, r *http.Request) {
	var d struct {
		InvoiceID string `json:"invoice_id"`
		Note      string `json:"note"`
	}
	if !decode(w, r, &d) {
		return
	}
	d.Note = strings.TrimSpace(d.Note)
	if len(d.Note) > 400 {
		problem(w, 422, "A observação deve ter no máximo 400 caracteres.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)

	c := claims(r)
	var owner bool
	var userName string
	if e = t.QueryRow(ctx, "select role='owner',name from users where id=$1 and deleted_at is null", c.Subject).Scan(&owner, &userName); e != nil || !owner {
		problem(w, 403, "Apenas o proprietário da empresa pode avisar o pagamento.")
		return
	}

	var invoiceID, planID, reference, due string
	var cents int
	if strings.TrimSpace(d.InvoiceID) != "" {
		e = t.QueryRow(ctx, `select id,plan_id,price_cents,due_date::text,reference from billing_invoices where id=$1 and company_id=$2 and status in ('pending','overdue')`, d.InvoiceID, c.Company).Scan(&invoiceID, &planID, &cents, &due, &reference)
	} else {
		e = t.QueryRow(ctx, `select id,plan_id,price_cents,due_date::text,reference from billing_invoices where company_id=$1 and status in ('pending','overdue') order by created_at desc limit 1`, c.Company).Scan(&invoiceID, &planID, &cents, &due, &reference)
	}
	if e != nil {
		problem(w, 404, "Nenhuma cobrança PIX pendente encontrada.")
		return
	}

	var companyName, companyCNPJ string
	_ = t.QueryRow(ctx, "select name,cnpj from companies where id=$1", c.Company).Scan(&companyName, &companyCNPJ)

	subject := fmt.Sprintf("PIX pago · Conferir %s · %s", reference, companyName)
	if len(subject) > 160 {
		subject = subject[:160]
	}
	noteLine := "Sem observação adicional."
	if d.Note != "" {
		noteLine = "Informação do comprovante: " + d.Note
	}
	desc := fmt.Sprintf(
		"O proprietário %s informou que realizou o pagamento via PIX.\nEmpresa: %s (CNPJ %s)\nPlano: %s\nValor: %s\nIdentificador: %s\nVencimento: %s\n%s\n\nConferir no PicPay (ag. 0001 / cc 140174949-6) e confirmar o recebimento na aba Financeiro do /admin para liberar o acesso.",
		userName, companyName, companyCNPJ, planID, formatBRL(cents), reference, due, noteLine,
	)

	var existingTicket string
	err := t.QueryRow(ctx, "select id from support_tickets where company_id=$1 and module='Financeiro · PIX' and subject like $2 and status<>'closed' order by id desc limit 1", c.Company, "%"+reference+"%").Scan(&existingTicket)
	if err == nil && existingTicket != "" {
		_, e = t.Exec(ctx, "insert into support_messages(id,ticket_id,author_id,body,internal_note) values($1,$2,$3,$4,false)", id(), existingTicket, c.Subject, desc)
	} else {
		_, e = t.Exec(ctx, "insert into support_tickets(id,company_id,created_by,subject,description,category,module,page_path) values($1,$2,$3,$4,$5,'question','Financeiro · PIX','/app')", id(), c.Company, c.Subject, subject, desc)
	}
	if e != nil {
		problem(w, 500, "Não foi possível enviar o aviso de pagamento.")
		return
	}
	_ = a.audit(ctx, t, c, "pix_payment_notified", invoiceID, map[string]any{"reference": reference, "note": d.Note})
	if e = t.Commit(ctx); e != nil {
		problem(w, 500, "Não foi possível registrar o aviso de pagamento.")
		return
	}
	write(w, 200, map[string]any{"ok": true, "reference": reference})
}
