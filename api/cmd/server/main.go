package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

type App struct {
	db           *pgxpool.Pool
	secret       []byte
	url          string
	mailKey      string
	directory    *publicDirectory
	limiter      sync.Map
	limiterSweep atomic.Int64
}
type Claims struct {
	Company string `json:"company_id"`
	Role    string `json:"role"`
	Version int    `json:"sv,omitempty"`
	jwt.RegisteredClaims
}
type identity struct {
	ID      string `json:"id"`
	Company string `json:"company_id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Role    string `json:"role"`
}
type credentials struct {
	Name     string          `json:"name"`
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Company  string          `json:"company"`
	CNPJ     string          `json:"cnpj"`
	Role     string          `json:"role"`
	Token    string          `json:"token"`
	Address  *companyAddress `json:"address,omitempty"`
}
type rateWindow struct {
	sync.Mutex
	at    time.Time
	count int
}

func id() string { return uuid.Must(uuid.NewV7()).String() }
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": title, "status": status})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		problem(w, 400, "Dados inválidos.")
		return false
	}
	return true
}
func hashToken(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func passwordHash(p string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	h := argon2.IDKey([]byte(p), salt, 2, 64*1024, 2, 32)
	return base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(h)
}
func passwordOK(p, h string) bool {
	parts := strings.Split(h, ".")
	if len(parts) != 2 {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[0])
	if e != nil {
		return false
	}
	want, e := base64.RawStdEncoding.DecodeString(parts[1])
	if e != nil {
		return false
	}
	got := argon2.IDKey([]byte(p), salt, 2, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(want, got) == 1
}
func validEmail(e string) bool {
	return len(e) <= 254 && regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(e)
}
func validCNPJ(s string) bool {
	if len(s) != 14 || strings.Trim(s, s[:1]) == "" {
		return false
	}
	for n := 12; n < 14; n++ {
		sum := 0
		weight := n - 7
		for i := 0; i < n; i++ {
			if !((s[i] >= '0' && s[i] <= '9') || (i < 12 && s[i] >= 'A' && s[i] <= 'Z')) {
				return false
			}
			sum += int(s[i]-'0') * weight
			weight--
			if weight == 1 {
				weight = 9
			}
		}
		digit := 11 - sum%11
		if digit >= 10 {
			digit = 0
		}
		if int(s[n]-'0') != digit {
			return false
		}
	}
	return true
}
func (a *App) rate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			key = r.RemoteAddr
		}
		if c, ok := r.Context().Value(claimsKey{}).(*Claims); ok {
			key = "user:" + c.Subject
		} else if a.db != nil {
			if !a.publicQuota(w, r, key) {
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		now := time.Now()
		prev := a.limiterSweep.Load()
		if now.Unix()-prev >= 60 && a.limiterSweep.CompareAndSwap(prev, now.Unix()) {
			a.limiter.Range(func(k, v any) bool {
				b := v.(*rateWindow)
				b.Lock()
				if now.Sub(b.at) > 2*time.Minute {
					a.limiter.Delete(k)
				}
				b.Unlock()
				return true
			})
		}
		v, _ := a.limiter.LoadOrStore(key, &rateWindow{at: now})
		b := v.(*rateWindow)
		b.Lock()
		if time.Since(b.at) > time.Minute {
			b.at = time.Now()
			b.count = 0
		}
		b.count++
		ok := b.count <= 30
		b.Unlock()
		if !ok {
			w.Header().Set("Retry-After", "60")
			problem(w, 429, "Muitas tentativas. Aguarde um minuto.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		c := &Claims{}
		t, e := jwt.ParseWithClaims(raw, c, func(t *jwt.Token) (any, error) { return a.secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("geevheittire"), jwt.WithExpirationRequired())
		if e != nil || !t.Valid {
			problem(w, 401, "Sessão expirada. Entre novamente.")
			return
		}
		if uuid.Validate(c.Company) != nil || uuid.Validate(c.Subject) != nil {
			problem(w, 401, "Sessão inválida.")
			return
		}
		var active bool
		if err := a.db.QueryRow(r.Context(), "select auth_account_active($1,$2) and auth_session_version($1)=$3", c.Subject, c.Company, c.Version).Scan(&active); err != nil {
			problem(w, 503, "Não foi possível verificar a conta.")
			return
		}
		if !active {
			problem(w, 401, "Conta encerrada ou indisponível. Entre com uma conta ativa.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsKey{}, c)))
	})
}

type claimsKey struct{}

func claims(r *http.Request) *Claims { return r.Context().Value(claimsKey{}).(*Claims) }
func canManage(c *Claims) bool       { return c.Role == "owner" || c.Role == "manager" }
func (a *App) tx(r *http.Request) (pgx.Tx, error) {
	t, e := a.db.Begin(r.Context())
	if e != nil {
		return nil, e
	}
	_, e = t.Exec(r.Context(), "select set_config('app.company_id',$1,true)", claims(r).Company)
	if e != nil {
		_ = t.Rollback(r.Context())
		return nil, e
	}
	return t, nil
}
func (a *App) session(ctx context.Context, t pgx.Tx, u identity) (map[string]any, error) {
	if _, e := t.Exec(ctx, "select set_config('app.company_id',$1,true)", u.Company); e != nil {
		return nil, e
	}
	now := time.Now()
	var version int
	if e := t.QueryRow(ctx, "select session_version from users where id=$1", u.ID).Scan(&version); e != nil {
		return nil, e
	}
	c := Claims{Company: u.Company, Role: u.Role, Version: version, RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID, Issuer: "geevheittire", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute))}}
	access, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(a.secret)
	if e != nil {
		return nil, e
	}
	refresh := randomToken()
	_, e = t.Exec(ctx, "insert into refresh_tokens(id,company_id,user_id,token_hash,expires_at) values($1,$2,$3,$4,$5)", id(), u.Company, u.ID, hashToken(refresh), now.Add(30*24*time.Hour))
	return map[string]any{"access_token": access, "refresh_token": refresh, "user": u}, e
}
func (a *App) mail(ctx context.Context, email, subject, link string) error {
	if a.mailKey == "" {
		return errors.New("envio de e-mail não configurado")
	}
	payload, _ := json.Marshal(map[string]any{"from": env("EMAIL_FROM", ""), "to": []string{email}, "subject": subject, "text": subject + "\n\n" + link + "\n\nSe você não solicitou, ignore esta mensagem."})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+a.mailKey)
	req.Header.Set("Content-Type", "application/json")
	resp, e := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return errors.New("falha no envio de e-mail")
	}
	return nil
}
func (a *App) accountToken(ctx context.Context, t pgx.Tx, u identity, kind string) (string, error) {
	if _, e := t.Exec(ctx, "select set_config('app.company_id',$1,true)", u.Company); e != nil {
		return "", e
	}
	raw := randomToken()
	_, e := t.Exec(ctx, "insert into account_tokens(id,company_id,user_id,token_hash,kind,expires_at) values($1,$2,$3,$4,$5,now()+interval '24 hours')", id(), u.Company, u.ID, hashToken(raw), kind)
	return raw, e
}
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var d credentials
	if !decode(w, r, &d) {
		return
	}
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	d.CNPJ = regexp.MustCompile(`[./\s-]`).ReplaceAllString(strings.ToUpper(d.CNPJ), "")
	if !validEmail(d.Email) || len(d.Password) < 10 || len(d.Password) > 128 || len(strings.TrimSpace(d.Name)) < 2 || len(strings.TrimSpace(d.Company)) < 2 || !validCNPJ(d.CNPJ) {
		problem(w, 422, "Informe nome, empresa, e-mail, CNPJ válido e senha com 10 a 128 caracteres.")
		return
	}
	ctx := r.Context()
	if e := checkedAddress(d.Address); e != nil {
		directoryProblem(w, e)
		return
	}
	registration, e := a.verifiedCompany(ctx, d.CNPJ)
	if e != nil {
		directoryProblem(w, e)
		return
	}
	t, e := a.db.Begin(ctx)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	u := identity{id(), id(), d.Name, d.Email, "owner"}
	if _, e = t.Exec(ctx, "select set_config('app.company_id',$1,true)", u.Company); e != nil {
		problem(w, 500, "Falha ao iniciar empresa.")
		return
	}
	e = insertCompany(ctx, t, u.Company, d.Company, d.CNPJ, d.Address, registration)
	if e == nil {
		_, e = t.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,$3,$4,$5,'owner')", u.ID, u.Company, u.Name, u.Email, passwordHash(d.Password))
	}
	if e != nil {
		problem(w, 409, "Não foi possível cadastrar. Verifique os dados ou entre com sua conta.")
		return
	}
	if a.audit(ctx, t, &Claims{Company: u.Company, RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID}}, "company", u.Company, map[string]string{"name": d.Company}) != nil {
		problem(w, 500, "Falha ao registrar auditoria.")
		return
	}
	raw, e := a.accountToken(ctx, t, u, "verify")
	if e != nil {
		problem(w, 500, "Falha ao criar verificação.")
		return
	}
	out, e := a.session(ctx, t, u)
	if e != nil || t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao criar sessão.")
		return
	}
	e = a.mail(ctx, u.Email, "Verifique seu e-mail — GeevheitTire", a.url+"/aceitar-convite?kind=verify&token="+raw)
	out["email_sent"] = e == nil
	write(w, 201, out)
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var d credentials
	if !decode(w, r, &d) {
		return
	}
	if len(d.Password) > 128 {
		problem(w, 401, "E-mail ou senha inválidos.")
		return
	}
	var u identity
	var h string
	var neonSubject *string
	e := a.db.QueryRow(r.Context(), "select id,company_id,name,email,role,password_hash,neon_subject from auth_find_user($1)", strings.ToLower(strings.TrimSpace(d.Email))).Scan(&u.ID, &u.Company, &u.Name, &u.Email, &u.Role, &h, &neonSubject)
	valid := false
	if e == nil {
		if neonSubject != nil {
			providerID, err := neonPassword(r.Context(), u.Email, d.Password)
			valid = err == nil && providerID == *neonSubject
		} else {
			valid = passwordOK(d.Password, h)
		}
	}
	if !valid {
		problem(w, 401, "E-mail ou senha inválidos.")
		return
	}
	t, e := a.db.Begin(r.Context())
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	out, e := a.session(r.Context(), t, u)
	if e != nil || t.Commit(r.Context()) != nil {
		problem(w, 500, "Falha ao criar sessão.")
		return
	}
	write(w, 200, out)
}
func (a *App) refresh(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Token string `json:"refresh_token"`
	}
	if !decode(w, r, &d) {
		return
	}
	ctx := r.Context()
	t, e := a.db.Begin(ctx)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	var u identity
	e = t.QueryRow(ctx, `select id,company_id,name,email,role from auth_refresh($1)`, hashToken(d.Token)).Scan(&u.ID, &u.Company, &u.Name, &u.Email, &u.Role)
	if e != nil {
		problem(w, 401, "Sessão expirada.")
		return
	}
	_, e = t.Exec(ctx, "select auth_logout($1)", hashToken(d.Token))
	if e != nil {
		problem(w, 500, "Falha ao renovar sessão.")
		return
	}
	out, e := a.session(ctx, t, u)
	if e != nil || t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao renovar sessão.")
		return
	}
	write(w, 200, out)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	var d struct {
		Token string `json:"refresh_token"`
	}
	if !decode(w, r, &d) {
		return
	}
	_, e := a.db.Exec(r.Context(), "select auth_logout($1)", hashToken(d.Token))
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func (a *App) recover(w http.ResponseWriter, r *http.Request) {
	var d credentials
	if !decode(w, r, &d) {
		return
	}
	ctx := r.Context()
	var u identity
	e := a.db.QueryRow(ctx, "select id,company_id,name,email,role from auth_find_user($1)", strings.ToLower(strings.TrimSpace(d.Email))).Scan(&u.ID, &u.Company, &u.Name, &u.Email, &u.Role)
	if e == nil {
		t, err := a.db.Begin(ctx)
		if err == nil {
			defer t.Rollback(ctx)
			raw, err := a.accountToken(ctx, t, u, "reset")
			if err == nil && t.Commit(ctx) == nil {
				if err = a.mail(ctx, u.Email, "Redefina sua senha — GeevheitTire", a.url+"/aceitar-convite?kind=reset&token="+raw); err != nil {
					slog.Warn("recovery delivery unavailable")
				}
			}
		}
	}
	write(w, 200, map[string]string{"message": "Se houver uma conta, enviaremos as instruções por e-mail."})
}
func (a *App) accept(w http.ResponseWriter, r *http.Request) {
	var d credentials
	if !decode(w, r, &d) {
		return
	}
	ctx := r.Context()
	t, e := a.db.Begin(ctx)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	var uid, kind, company string
	e = t.QueryRow(ctx, "select user_id,kind,company_id from auth_account_token($1)", hashToken(d.Token)).Scan(&uid, &kind, &company)
	if e != nil {
		problem(w, 422, "Link inválido ou expirado.")
		return
	}
	if _, e = t.Exec(ctx, "select set_config('app.company_id',$1,true)", company); e != nil {
		problem(w, 500, "Falha ao validar conta.")
		return
	}
	if kind != "verify" {
		if len(d.Password) < 10 || len(d.Password) > 128 {
			problem(w, 422, "A senha deve ter de 10 a 128 caracteres.")
			return
		}
		_, e = t.Exec(ctx, "update users set password_hash=$1,email_verified_at=coalesce(email_verified_at,now()),session_version=session_version+1,updated_at=now() where id=$2", passwordHash(d.Password), uid)
		if e == nil {
			_, e = t.Exec(ctx, "update refresh_tokens set revoked_at=now() where user_id=$1", uid)
		}
	} else {
		_, e = t.Exec(ctx, "update users set email_verified_at=now() where id=$1", uid)
	}
	if e == nil {
		_, e = t.Exec(ctx, "update account_tokens set used_at=now() where token_hash=$1", hashToken(d.Token))
	}
	if e != nil || t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao confirmar conta.")
		return
	}
	write(w, 200, map[string]string{"message": "Conta atualizada. Você já pode entrar."})
}
func (a *App) invite(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if !canManage(c) {
		problem(w, 403, "Seu perfil não pode convidar usuários.")
		return
	}
	if a.mailKey == "" {
		problem(w, 503, "O envio de convites aguarda a configuração do e-mail.")
		return
	}
	var d credentials
	if !decode(w, r, &d) {
		return
	}
	d.Email = strings.ToLower(strings.TrimSpace(d.Email))
	if !validEmail(d.Email) || len(d.Name) < 2 || (d.Role != "tire_tech" && d.Role != "inspector") {
		problem(w, 422, "Informe nome, e-mail e um perfil permitido.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	u := identity{id(), c.Company, d.Name, d.Email, d.Role}
	_, e = t.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,$3,$4,$5,$6)", u.ID, u.Company, u.Name, u.Email, passwordHash(randomToken()), u.Role)
	if e != nil {
		if commercialError(w, e) {
			return
		}
		problem(w, 409, "E-mail já cadastrado.")
		return
	}
	if a.audit(ctx, t, c, "user", u.ID, u) != nil {
		problem(w, 500, "Falha ao registrar auditoria.")
		return
	}
	raw, e := a.accountToken(ctx, t, u, "invite")
	if e != nil {
		problem(w, 500, "Falha ao gerar convite.")
		return
	}
	if e = a.mail(ctx, u.Email, "Convite para GeevheitTire", a.url+"/aceitar-convite?token="+raw); e != nil {
		problem(w, 502, "Não foi possível enviar o convite. Tente novamente.")
		return
	}
	if t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao salvar convite.")
		return
	}
	write(w, 201, u)
}
func rowsJSON(ctx context.Context, t pgx.Tx, q string, args ...any) ([]map[string]any, error) {
	rows, e := t.Query(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		var obj map[string]any
		if e = json.Unmarshal(b, &obj); e != nil {
			return nil, e
		}
		out = append(out, obj)
	}
	return out, rows.Err()
}
func (a *App) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	resource := chi.URLParam(r, "resource")
	queries := map[string]string{
		"vehicles": "select to_jsonb(v)||jsonb_build_object('driver_name',(select name from drivers d where d.company_id=v.company_id and d.id=v.driver_id)) from vehicles v where company_id=$1 and deleted_at is null and ($2='' or id::text>$2) order by id limit 51",
		"drivers":  "select to_jsonb(d) from drivers d where company_id=$1 and ($2='' or id::text>$2) order by id limit 51",
		"tires":    "select to_jsonb(v)||jsonb_build_object('status',case when expires_on<current_date or structural_damage or tread_mm<$3 then 'critico' when tread_mm<$4 then 'alerta' else 'ok' end) from tires v where company_id=$1 and deleted_at is null and ($2='' or id::text>$2) order by id limit 51",
		"alerts":   "select to_jsonb(v) from alerts v where company_id=$1 and resolved_at is null and ($2='' or id::text>$2) order by id limit 51",
		"users":    "select jsonb_build_object('id',id,'name',name,'email',email,'role',role,'email_verified',email_verified_at is not null) from users where company_id=$1 and deleted_at is null and ($2='' or id::text>$2) order by id limit 51",
	}
	q, ok := queries[resource]
	if !ok {
		problem(w, 404, "Recurso não encontrado.")
		return
	}
	if (resource == "users" || resource == "drivers") && !canManage(claims(r)) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	args := []any{claims(r).Company, r.URL.Query().Get("cursor")}
	if resource == "tires" {
		p, err := policy(ctx, t, claims(r).Company)
		if err != nil {
			problem(w, 503, "Não foi possível consultar as regras.")
			return
		}
		args = append(args, p.Critical, p.Warning)
	}
	out, e := rowsJSON(ctx, t, q, args...)
	if e != nil {
		problem(w, 500, "Não foi possível carregar os registros.")
		return
	}
	next := ""
	if len(out) > 50 {
		out = out[:50]
		next = out[49]["id"].(string)
	}
	write(w, 200, map[string]any{"data": out, "next_cursor": next})
}
func (a *App) audit(ctx context.Context, t pgx.Tx, c *Claims, entity, eid string, data any) error {
	b, e := json.Marshal(data)
	if e != nil {
		return e
	}
	_, e = t.Exec(ctx, "insert into audit_log(id,company_id,user_id,entity,entity_id,action,after_data) values($1,$2,$3,$4,$5,'create',$6)", id(), c.Company, c.Subject, entity, eid, b)
	return e
}
func (a *App) createVehicle(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if !canManage(c) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	var d struct {
		Plate    string `json:"plate"`
		Model    string `json:"model"`
		Template string `json:"template_id"`
		KM       int    `json:"current_km"`
	}
	if !decode(w, r, &d) {
		return
	}
	d.Plate = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(d.Plate), "-", ""))
	if !regexp.MustCompile(`^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$`).MatchString(d.Plate) || d.KM < 0 || len(d.Model) < 2 || d.Template == "" {
		problem(w, 422, "Verifique placa, modelo, odômetro e configuração de eixos.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	var validTemplate bool
	if e = t.QueryRow(ctx, "select exists(select 1 from axle_templates where id=$1)", d.Template).Scan(&validTemplate); e != nil {
		problem(w, 503, "Não foi possível validar a configuração de eixos.")
		return
	}
	if !validTemplate {
		problem(w, 422, "Selecione uma configuração de eixos válida.")
		return
	}
	eid := id()
	_, e = t.Exec(ctx, "insert into vehicles(id,company_id,plate,model,template_id,current_km) values($1,$2,$3,$4,$5,$6)", eid, c.Company, d.Plate, d.Model, d.Template, d.KM)
	if e != nil {
		if commercialError(w, e) {
			return
		}
		problem(w, 409, "Placa já cadastrada ou dados inválidos.")
		return
	}
	if a.audit(ctx, t, c, "vehicle", eid, d) != nil || t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao salvar veículo.")
		return
	}
	write(w, 201, map[string]string{"id": eid})
}
func (a *App) createTire(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if !canManage(c) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	var d struct {
		tireIdentityInput
		Retreads int     `json:"retread_count"`
		Brand    string  `json:"brand"`
		Model    string  `json:"model"`
		Size     string  `json:"size"`
		Tread    float64 `json:"tread_mm"`
		Cost     float64 `json:"acquisition_cost"`
		Vehicle  string  `json:"vehicle_id"`
		Position string  `json:"position"`
	}
	if !decode(w, r, &d) {
		return
	}
	if !d.validate(time.Now()) || d.Retreads < 0 || d.Retreads > 10 || len(d.Brand) < 2 || len(d.Brand) > 150 || len(d.Model) > 150 || len(d.Size) < 3 || len(d.Size) > 50 || d.Tread <= 0 || d.Tread > 40 || d.Cost < 0 || d.Cost > 1000000 || (d.Vehicle == "") != (d.Position == "") {
		problem(w, 422, "Verifique número de fogo, número de série, DOT, validade, tipo de via, recapagens, marca, medida, sulco, custo e instalação.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	if e = fleetLock(ctx, t, c.Company); e != nil {
		inspectionError(w, e)
		return
	}
	var mounted any
	var vid any
	var pos any
	state := "EM_ESTOQUE"
	if d.Vehicle != "" {
		if d.Expiry < time.Now().UTC().Format("2006-01-02") {
			problem(w, 422, "Pneu com validade vencida deve permanecer fora de operação.")
			return
		}
		if uuid.Validate(d.Vehicle) != nil {
			problem(w, 422, "Veículo inválido.")
			return
		}
		var ok bool
		e = t.QueryRow(ctx, "select exists(select 1 from vehicles v join axle_templates a on a.id=v.template_id where v.id=$1 and v.company_id=$2 and v.deleted_at is null and a.positions ? $3)", d.Vehicle, c.Company, d.Position).Scan(&ok)
		if e != nil || !ok {
			problem(w, 422, "Veículo ou posição inválida.")
			return
		}
		vid = d.Vehicle
		pos = d.Position
		state = "INSTALADO"
		var km int
		if e = t.QueryRow(ctx, "select current_km from vehicles where id=$1 and company_id=$2", d.Vehicle, c.Company).Scan(&km); e != nil {
			inspectionError(w, e)
			return
		}
		mounted = km
		p, err := policy(ctx, t, c.Company)
		if err != nil {
			inspectionError(w, err)
			return
		}
		if d.Tread < p.Critical {
			problem(w, 422, "Pneu com sulco crítico deve permanecer fora de operação para avaliação.")
			return
		}
	}
	eid := id()
	_, e = t.Exec(ctx, "insert into tires(id,company_id,code,brand,model,size,tread_mm,acquisition_cost,vehicle_id,position,state,mounted_at_km,dot_code,expires_on,road_use,retread_count,initial_retread_count,serial_number) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16,$17)", eid, c.Company, d.Code, d.Brand, d.Model, d.Size, d.Tread, d.Cost, vid, pos, state, mounted, d.DOT, d.Expiry, d.Road, d.Retreads, d.Serial)
	if e != nil {
		problem(w, 409, "Número de fogo já cadastrado ou posição ocupada.")
		return
	}
	snapshot, err := loadTire(ctx, t, c.Company, eid)
	if err == nil {
		err = event(ctx, t, c, tireSnapshot{}, snapshot, "", "REGISTER", "Cadastro inicial do pneu.", "", 0, nil, snapshot.Mounted)
	}
	if err == nil {
		err = a.refreshMaintenance(ctx, t, c.Company)
	}
	if err != nil || a.audit(ctx, t, c, "tire", eid, d) != nil || t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao salvar pneu.")
		return
	}
	write(w, 201, map[string]string{"id": eid})
}
func (a *App) tireMap(w http.ResponseWriter, r *http.Request) {
	vid := chi.URLParam(r, "id")
	if uuid.Validate(vid) != nil {
		problem(w, 404, "Veículo não encontrado.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	var vehicle, positions []byte
	e = t.QueryRow(ctx, "select to_jsonb(v)||jsonb_build_object('driver_name',(select name from drivers d where d.company_id=v.company_id and d.id=v.driver_id)),a.positions from vehicles v join axle_templates a on a.id=v.template_id where v.id=$1 and v.company_id=$2 and v.deleted_at is null", vid, claims(r).Company).Scan(&vehicle, &positions)
	if e != nil {
		problem(w, 404, "Veículo não encontrado.")
		return
	}
	p, err := policy(ctx, t, claims(r).Company)
	if err != nil {
		problem(w, 503, "Não foi possível consultar as regras.")
		return
	}
	tires, e := rowsJSON(ctx, t, "select to_jsonb(t)||jsonb_build_object('status',case when expires_on<current_date or structural_damage or tread_mm<$3 then 'critico' when tread_mm<$4 then 'alerta' else 'ok' end) from tires t where vehicle_id=$1 and company_id=$2 and deleted_at is null", vid, claims(r).Company, p.Critical, p.Warning)
	if e != nil {
		problem(w, 500, "Falha ao carregar mapa.")
		return
	}
	write(w, 200, map[string]any{"vehicle": json.RawMessage(vehicle), "positions": json.RawMessage(positions), "tires": tires})
}
func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	p, err := policy(ctx, t, claims(r).Company)
	if err != nil {
		problem(w, 503, "Não foi possível consultar as regras.")
		return
	}
	var result []byte
	e = t.QueryRow(ctx, `select jsonb_build_object('vehicles',(select count(*) from vehicles where company_id=$1 and deleted_at is null),'tires',(select count(*) from tires where company_id=$1 and deleted_at is null and state<>'DESCARTADO'),'critical',(select count(*) from tires where company_id=$1 and (expires_on<current_date or structural_damage or tread_mm<$2) and state in ('INSTALADO','EM_ESTOQUE') and deleted_at is null),'alerts',(select count(*) from alerts where company_id=$1 and not read and resolved_at is null),'company',(select jsonb_build_object('name',name,'status',status,'trial_ends_at',trial_ends_at) from companies where id=$1))`, claims(r).Company, p.Critical).Scan(&result)
	if e != nil {
		problem(w, 500, "Falha ao carregar indicadores.")
		return
	}
	write(w, 200, json.RawMessage(result))
}
func (a *App) history(w http.ResponseWriter, r *http.Request) {
	tid := chi.URLParam(r, "id")
	if uuid.Validate(tid) != nil {
		problem(w, 404, "Pneu não encontrado.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	out, e := rowsJSON(ctx, t, "select to_jsonb(i) from inspections i where tire_id=$1 and company_id=$2 order by inspected_at desc limit 100", tid, claims(r).Company)
	if e != nil {
		problem(w, 500, "Falha ao carregar histórico.")
		return
	}
	write(w, 200, map[string]any{"data": out})
}
func (a *App) readAlert(w http.ResponseWriter, r *http.Request) {
	aid := chi.URLParam(r, "id")
	if uuid.Validate(aid) != nil {
		problem(w, 404, "Alerta não encontrado.")
		return
	}
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	tag, e := t.Exec(ctx, "update alerts set read=true where id=$1 and company_id=$2", aid, claims(r).Company)
	if e != nil || tag.RowsAffected() != 1 {
		problem(w, 404, "Alerta não encontrado.")
		return
	}
	if t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao atualizar alerta.")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		slog.Error("JWT_SECRET precisa de pelo menos 32 caracteres")
		os.Exit(1)
	}
	db, e := newPool(ctx)
	if e != nil {
		slog.Error("invalid database configuration")
		os.Exit(1)
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		slog.Error("database unavailable", "error", e)
		os.Exit(1)
	}
	a := &App{db: db, secret: []byte(secret), url: env("APP_URL", "http://localhost:3000"), mailKey: os.Getenv("RESEND_API_KEY"), directory: newPublicDirectory()}
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	go a.maintenanceWorker(workerCtx)
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, requestObservation, middleware.Timeout(30*time.Second))
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if a.db.Ping(r.Context()) != nil {
			problem(w, 503, "Banco indisponível.")
			return
		}
		write(w, 200, map[string]string{"status": "ok", "service": "geevheittire", "tire_lifecycle_revision": "3", "admin_revision": "2", "release": "2026-10-29-admin-control"})
	})
	r.Post("/api/v1/webhooks/asaas", a.asaasWebhook)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/plans", a.publicPlans)
		r.Group(func(r chi.Router) {
			r.Use(a.rate, passwordWorkLimit)
			r.Post("/auth/register", a.register)
			r.Post("/auth/login", a.login)
			r.Post("/auth/neon", a.neonSession)
			r.Post("/auth/refresh", a.refresh)
			r.Post("/auth/logout", a.logout)
			r.Post("/auth/recover", a.recover)
			r.Post("/auth/accept", a.accept)
			r.Get("/registration/cnpj/{cnpj}", a.lookupCNPJ)
			r.Get("/registration/cep/{cep}", a.lookupCEP)
		})
		r.Group(func(r chi.Router) {
			r.Use(a.auth)
			r.Use(a.billingGuard)
			r.Use(a.entitlementContext)
			r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
				c := claims(r)
				var u identity
				tx, err := a.tx(r)
				if err != nil {
					problem(w, 503, "Banco indisponível.")
					return
				}
				defer tx.Rollback(r.Context())
				e := tx.QueryRow(r.Context(), "select id,company_id,name,email,role from users where id=$1 and company_id=$2 and deleted_at is null", c.Subject, c.Company).Scan(&u.ID, &u.Company, &u.Name, &u.Email, &u.Role)
				if e != nil {
					problem(w, 401, "Conta indisponível.")
					return
				}
				write(w, 200, u)
			})
			r.Get("/billing", a.billingStatus)
			r.Get("/entitlements", a.entitlementStatus)
			r.Post("/billing/checkout", a.checkout)
			r.Get("/platform/me", a.platformMe)
			r.Get("/platform", a.platformDashboard)
			r.Post("/platform/change", a.platformChange)
			r.Get("/platform/clients", a.platformClients)
			r.Get("/platform/clients/{id}", a.platformClientDetail)
			r.Post("/platform/clients/{id}/access", a.platformClientAccess)
			r.Post("/platform/clients/{id}/close", a.platformCloseClient)
			r.Put("/platform/plans/{id}", a.platformConfigurePlan)
			r.Get("/platform/finance", a.platformFinance)
			r.Post("/platform/finance", a.platformCreateReceivable)
			r.Patch("/platform/finance/{id}", a.platformSettleReceivable)
			r.With(a.rate, passwordWorkLimit).Patch("/platform/profile", a.platformProfile)
			r.Get("/platform/tickets", a.platformTickets)
			r.Get("/platform/tickets/{id}", a.platformTicketDetail)
			r.Patch("/platform/tickets/{id}", a.platformUpdateTicket)
			r.Get("/support/tickets", a.supportTickets)
			r.Get("/support/tickets/{id}", a.supportTicketDetail)
			r.With(a.rate).Post("/support/tickets", a.createSupportTicket)
			r.With(a.rate).Post("/support/tickets/{id}/messages", a.replySupportTicket)
			r.Get("/account", a.accountStatus)
			r.With(a.rate).Post("/account/close", a.closeAccount)
			r.Get("/dashboard", a.dashboard)
			r.Get("/company", a.companyProfile)
			r.Get("/{resource:vehicles|tires|alerts|users|drivers}", a.list)
			r.Post("/drivers", a.saveDriver)
			r.Patch("/drivers/{id}", a.saveDriver)
			r.Patch("/vehicles/{id}/driver", a.assignDriver)
			r.Get("/vehicle-configurations", a.vehicleConfigurations)
			r.Post("/vehicles", a.createVehicle)
			r.Post("/tires", a.createTire)
			r.Patch("/tires/{id}/identity", a.updateTireIdentity)
			r.Post("/tires/{id}/movements", a.moveTire)
			r.Get("/tires/{id}/lifecycle", a.tireLifecycle)
			r.Get("/tire-reports", a.tireReports)
			r.Get("/tire-policy", a.getTirePolicy)
			r.Patch("/tire-policy", a.setTirePolicy)
			r.Patch("/vehicles/{id}/odometer", a.updateOdometer)
			r.Post("/users/invite", a.invite)
			r.Get("/vehicles/{id}/tire-map", a.tireMap)
			r.Post("/tires/{id}/inspections", a.inspect)
			r.Post("/checklists", a.checklist)
			r.Get("/tires/{id}/history", a.history)
			r.Patch("/alerts/{id}/read", a.readAlert)
		})
	})
	port := env("PORT", "8080")
	if _, e = strconv.Atoi(port); e != nil {
		panic(e)
	}
	server := &http.Server{Addr: ":" + port, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("GeevheitTire API listening", "port", port)
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			slog.Error("server failed", "error", e)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	fmt.Println("API encerrada")
}

func newPool(ctx context.Context) (*pgxpool.Pool, error) {
	cfg, e := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if e != nil {
		return nil, e
	}
	max, e := strconv.Atoi(env("DB_MAX_CONNS", "20"))
	if e != nil || max < 2 || max > 100 {
		return nil, fmt.Errorf("DB_MAX_CONNS must be between 2 and 100")
	}
	cfg.MaxConns = int32(max)
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnLifetimeJitter = 5 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 10 * time.Second
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "20000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "8000"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "20000"
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, e := c.Exec(ctx, "SET ROLE geevheit_runtime")
		return e
	}
	return pgxpool.NewWithConfig(ctx, cfg)
}
