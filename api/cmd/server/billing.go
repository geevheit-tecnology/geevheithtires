package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func commercialError(w http.ResponseWriter, err error) bool {
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "P0001" {
		problem(w, 402, p.Message)
		return true
	}
	return false
}
func (a *App) billingGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || strings.HasPrefix(r.URL.Path, "/api/v1/billing") || strings.HasPrefix(r.URL.Path, "/api/v1/platform") {
			next.ServeHTTP(w, r)
			return
		}
		t, e := a.tx(r)
		if e != nil {
			problem(w, 503, "Banco indisponível.")
			return
		}
		defer t.Rollback(r.Context())
		var allowed bool
		e = t.QueryRow(r.Context(), `select not s.suspended and (s.paid_until>now() or c.trial_ends_at>now()) from subscriptions s join companies c on c.id=s.company_id where c.id=$1 and c.deleted_at is null`, claims(r).Company).Scan(&allowed)
		if e != nil {
			problem(w, 503, "Não foi possível consultar a assinatura.")
			return
		}
		if !allowed {
			problem(w, 402, "Acesso somente para consulta. Abra Plano e cobrança para regularizar sua assinatura.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) platformOK(w http.ResponseWriter, r *http.Request) bool {
	var ok bool
	e := a.db.QueryRow(r.Context(), "select platform_allowed($1)", claims(r).Subject).Scan(&ok)
	if e != nil {
		problem(w, 503, "Não foi possível verificar seu acesso.")
		return false
	}
	if !ok {
		problem(w, 403, "Acesso exclusivo ao administrador da plataforma.")
		return false
	}
	return true
}
func (a *App) platformMe(w http.ResponseWriter, r *http.Request) {
	if a.platformOK(w, r) {
		write(w, 200, map[string]bool{"admin": true})
	}
}
func (a *App) platformDashboard(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	var out json.RawMessage
	if e := a.db.QueryRow(r.Context(), "select platform_snapshot($1)", claims(r).Subject).Scan(&out); e != nil {
		problem(w, 503, "Não foi possível carregar a administração.")
		return
	}
	write(w, 200, out)
}
func (a *App) platformChange(w http.ResponseWriter, r *http.Request) {
	if !a.platformOK(w, r) {
		return
	}
	var d struct {
		Target    string          `json:"target"`
		Operation string          `json:"operation"`
		Reason    string          `json:"reason"`
		Payload   json.RawMessage `json:"payload"`
	}
	if !decode(w, r, &d) {
		return
	}
	if len(d.Payload) == 0 {
		d.Payload = json.RawMessage(`{}`)
	}
	_, e := a.db.Exec(r.Context(), "select platform_change($1,$2,$3,$4,$5)", claims(r).Subject, d.Target, d.Operation, d.Payload, d.Reason)
	if e != nil {
		problem(w, 422, "Operação inválida. Informe uma justificativa de 5 a 500 caracteres e confira os dados.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *App) billingStatus(w http.ResponseWriter, r *http.Request) {
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	var out map[string]any
	var raw []byte
	e = t.QueryRow(r.Context(), `select jsonb_build_object('trial_ends_at',c.trial_ends_at,'paid_until',s.paid_until,'plan_id',s.plan_id,'suspended',s.suspended,
 'status',case when s.suspended then 'suspended' when s.paid_until>now() then 'active' when c.trial_ends_at>now() then 'trial' else 'expired' end,
 'vehicle_limit',case when s.paid_until>now() then s.vehicle_limit else 15 end,'user_limit',case when s.paid_until>now() then s.user_limit else 3 end,
 'vehicles',(select count(*) from vehicles where company_id=c.id and deleted_at is null),'users',(select count(*) from users where company_id=c.id and deleted_at is null),
 'plans',coalesce((select jsonb_agg(to_jsonb(p) order by price_cents) from billing_plans p where published),'[]'),
 'invoices',coalesce((select jsonb_agg(to_jsonb(x)) from (select id,plan_id,price_cents,status,checkout_url,due_date,paid_at,environment from billing_invoices where company_id=c.id order by created_at desc limit 24) x),'[]'))
 from companies c join subscriptions s on s.company_id=c.id where c.id=$1 and c.deleted_at is null`, claims(r).Company).Scan(&raw)
	if e != nil {
		problem(w, 503, "Não foi possível carregar a assinatura.")
		return
	}
	_ = json.Unmarshal(raw, &out)
	out["payments_enabled"] = asaasFromEnv() != nil
	out["environment"] = os.Getenv("ASAAS_ENVIRONMENT")
	write(w, 200, out)
}

type asaasClient struct {
	base, key, mode string
	client          *http.Client
}

func asaasFromEnv() *asaasClient {
	mode := os.Getenv("ASAAS_ENVIRONMENT")
	key := os.Getenv("ASAAS_API_KEY")
	if key == "" || len(os.Getenv("ASAAS_WEBHOOK_TOKEN")) < 32 {
		return nil
	}
	base := "https://api-sandbox.asaas.com/v3"
	if mode == "production" {
		if os.Getenv("ASAAS_LIVE_ENABLED") != "true" {
			return nil
		}
		base = "https://api.asaas.com/v3"
	} else if mode != "sandbox" {
		return nil
	}
	return &asaasClient{base: base, key: key, mode: mode, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *asaasClient) call(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	r, e := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if e != nil {
		return e
	}
	r.Header.Set("access_token", c.key)
	r.Header.Set("User-Agent", "GeevheitTire/1.0")
	r.Header.Set("Content-Type", "application/json")
	resp, e := c.client.Do(r)
	if e != nil {
		return errors.New("serviço de pagamento indisponível")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("pagamento: HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

type asaasPayment struct {
	ID                string      `json:"id"`
	Customer          string      `json:"customer"`
	Status            string      `json:"status"`
	Value             json.Number `json:"value"`
	InvoiceURL        string      `json:"invoiceUrl"`
	ExternalReference string      `json:"externalReference"`
}

func paymentCents(value json.Number) (int, error) {
	// Decimal parsing avoids accepting truncated or fractional cents from a provider.
	s := value.String()
	if !regexp.MustCompile(`^[0-9]{1,8}(\.[0-9]{1,2})?$`).MatchString(s) {
		return 0, errors.New("invalid value")
	}
	var whole, frac int
	parts := strings.Split(s, ".")
	_, _ = fmt.Sscanf(parts[0], "%d", &whole)
	if len(parts) == 2 {
		f := parts[1]
		if len(f) == 1 {
			f += "0"
		}
		_, _ = fmt.Sscanf(f, "%d", &frac)
	}
	return whole*100 + frac, nil
}
func validCheckoutURL(raw, mode string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := u.Hostname()
	return (host == "www.asaas.com" || host == "asaas.com") && mode == "production" || (host == "sandbox.asaas.com" || host == "www.asaas.com") && mode == "sandbox"
}
func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	c := asaasFromEnv()
	if c == nil {
		problem(w, 503, "Pagamentos ainda não foram ativados pelo administrador.")
		return
	}
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
	var owner bool
	e = t.QueryRow(ctx, "select role='owner' from users where id=$1 and deleted_at is null", claims(r).Subject).Scan(&owner)
	if e != nil || !owner {
		problem(w, 403, "A contratação exige o proprietário da empresa.")
		return
	}
	var suspended bool
	var currentVehicles, currentUsers int
	if e = t.QueryRow(ctx, `select suspended,(select count(*) from vehicles where company_id=$1 and deleted_at is null),(select count(*) from users where company_id=$1 and deleted_at is null) from subscriptions where company_id=$1`, claims(r).Company).Scan(&suspended, &currentVehicles, &currentUsers); e != nil || suspended {
		problem(w, 403, "Entre em contato com a administração para regularizar a empresa.")
		return
	}
	var cents, vehicles, users int
	if e = t.QueryRow(ctx, "select price_cents,vehicle_limit,user_limit from billing_plans where id=$1 and published", d.Plan).Scan(&cents, &vehicles, &users); e != nil {
		problem(w, 422, "Plano indisponível.")
		return
	}
	if currentVehicles > vehicles || currentUsers > users {
		problem(w, 422, "A utilização atual excede os limites deste plano.")
		return
	}
	// One durable order per company. An ambiguous provider timeout never creates a second charge.
	_, e = t.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,7))", claims(r).Company)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	invoiceID := id()
	due := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
	_, e = t.Exec(ctx, `insert into billing_invoices(id,company_id,plan_id,price_cents,vehicle_limit,user_limit,due_date,environment) values($1,$2,$3,$4,$5,$6,$7,$8) on conflict (company_id) where status in ('creating','pending','overdue') do nothing`, invoiceID, claims(r).Company, d.Plan, cents, vehicles, users, due, c.mode)
	if e != nil {
		problem(w, 503, "Não foi possível preparar a cobrança.")
		return
	}
	var oldPlan, mode string
	var attempted *time.Time
	var checkout *string
	e = t.QueryRow(ctx, `select id,plan_id,price_cents,due_date::text,environment,checkout_url,provider_attempted_at from billing_invoices where company_id=$1 and status in ('creating','pending','overdue')`, claims(r).Company).Scan(&invoiceID, &oldPlan, &cents, &due, &mode, &checkout, &attempted)
	if e != nil || mode != c.mode || oldPlan != d.Plan {
		problem(w, 409, "Já existe uma cobrança aberta. Regularize-a antes de trocar de plano.")
		return
	}
	if e = t.Commit(ctx); e != nil {
		problem(w, 503, "Não foi possível preparar a cobrança.")
		return
	}
	if checkout != nil {
		write(w, 200, map[string]string{"checkout_url": *checkout})
		return
	}
	// Serialize provider creation for the same durable invoice, including retries.
	t, e = a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	e = t.QueryRow(ctx, "select provider_attempted_at,checkout_url from billing_invoices where id=$1 for update", invoiceID).Scan(&attempted, &checkout)
	if e != nil {
		problem(w, 503, "Cobrança indisponível.")
		return
	}
	if checkout != nil {
		write(w, 200, map[string]string{"checkout_url": *checkout})
		return
	}
	var payments struct {
		Data []asaasPayment `json:"data"`
	}
	if c.call(ctx, "GET", "/payments?externalReference="+url.QueryEscape(invoiceID), nil, &payments) != nil {
		problem(w, 503, "Não foi possível consultar o pagamento. Tente novamente.")
		return
	}
	var payment asaasPayment
	if len(payments.Data) > 0 {
		payment = payments.Data[0]
	} else {
		if attempted != nil {
			problem(w, 503, "Cobrança em conciliação. Aguarde ou contate o suporte; não será gerada outra cobrança.")
			return
		}
		var name, cnpj, email string
		if t.QueryRow(ctx, `select c.name,c.cnpj,u.email from companies c join users u on u.company_id=c.id and u.id=$2 where c.id=$1`, claims(r).Company, claims(r).Subject).Scan(&name, &cnpj, &email) != nil {
			problem(w, 503, "Cadastro indisponível.")
			return
		}
		var customers struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		var customer struct {
			ID string `json:"id"`
		}
		if c.call(ctx, "GET", "/customers?cpfCnpj="+url.QueryEscape(cnpj), nil, &customers) != nil {
			problem(w, 503, "Não foi possível consultar o cadastro de cobrança.")
			return
		}
		if len(customers.Data) > 0 {
			customer.ID = customers.Data[0].ID
		} else if c.call(ctx, "POST", "/customers", map[string]any{"name": name, "cpfCnpj": cnpj, "email": email, "notificationDisabled": true, "externalReference": claims(r).Company}, &customer) != nil {
			problem(w, 503, "Não foi possível preparar o cadastro de cobrança.")
			return
		}
		if customer.ID == "" {
			problem(w, 503, "Cadastro de cobrança inválido.")
			return
		}
		// Commit intent before issuing a non-idempotent remote POST. Retries reconcile by reference only.
		if _, e = t.Exec(ctx, "update billing_invoices set provider_attempted_at=now(),customer_id=$2 where id=$1", invoiceID, customer.ID); e != nil || t.Commit(ctx) != nil {
			problem(w, 503, "Não foi possível preparar o pagamento.")
			return
		}
		if c.call(ctx, "POST", "/payments", map[string]any{"customer": customer.ID, "billingType": "UNDEFINED", "value": float64(cents) / 100, "dueDate": due, "externalReference": invoiceID, "description": "GeevheitTire — mensalidade " + oldPlan}, &payment) != nil {
			problem(w, 503, "Confirmação da cobrança pendente. Tente novamente para consultar a mesma cobrança.")
			return
		}
	}
	got, e := paymentCents(payment.Value)
	if e != nil || got != cents || payment.ExternalReference != invoiceID || payment.ID == "" || !validCheckoutURL(payment.InvoiceURL, c.mode) {
		problem(w, 503, "O pagamento precisa de conciliação pelo suporte.")
		return
	}
	// Release a transaction held only for reconciliation, then store provider mapping tenant-scoped.
	_ = t.Rollback(ctx)
	t, e = a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	_, e = t.Exec(ctx, "update billing_invoices set provider_id=$2,checkout_url=$3,status='pending' where id=$1 and status='creating'", invoiceID, payment.ID, payment.InvoiceURL)
	if e != nil || t.Commit(ctx) != nil {
		problem(w, 503, "Pagamento criado; tente novamente para recuperar o link.")
		return
	}
	write(w, 200, map[string]string{"checkout_url": payment.InvoiceURL})
}
func (a *App) asaasWebhook(w http.ResponseWriter, r *http.Request) {
	c := asaasFromEnv()
	secret := os.Getenv("ASAAS_WEBHOOK_TOKEN")
	if c == nil {
		problem(w, 503, "Integração desativada.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("asaas-access-token")), []byte(secret)) != 1 {
		problem(w, 401, "Notificação não autorizada.")
		return
	}
	var d struct {
		ID      string       `json:"id"`
		Event   string       `json:"event"`
		Payment asaasPayment `json:"payment"`
	}
	// Provider payloads contain additional fields. Bound the body, but allow schema additions.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if json.NewDecoder(r.Body).Decode(&d) != nil || len(d.ID) < 1 || len(d.ID) > 200 || !regexp.MustCompile(`^pay_[A-Za-z0-9]+$`).MatchString(d.Payment.ID) {
		problem(w, 400, "Notificação inválida.")
		return
	}
	var exists bool
	if a.db.QueryRow(r.Context(), "select exists(select 1 from billing_lookup($1))", d.Payment.ID).Scan(&exists) != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	// Creation webhooks can race the mapping transaction; return retryable status, never lose payment.
	if !exists {
		problem(w, 503, "Pagamento ainda não conciliado.")
		return
	}
	var payment asaasPayment
	if c.call(r.Context(), "GET", "/payments/"+d.Payment.ID, nil, &payment) != nil {
		problem(w, 503, "Não foi possível confirmar o pagamento.")
		return
	}
	cents, e := paymentCents(payment.Value)
	if e != nil || payment.ID != d.Payment.ID || cents <= 0 || cents > math.MaxInt32 {
		problem(w, 422, "Valor inválido.")
		return
	}
	_, e = a.db.Exec(r.Context(), "select billing_settle($1,$2,$3,$4,$5)", d.ID, payment.ID, payment.Status, cents, c.mode)
	if e != nil {
		problem(w, 503, "Não foi possível conciliar o pagamento.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
