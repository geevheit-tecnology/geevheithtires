package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var neonKeys = struct {
	sync.Mutex
	url  string
	at   time.Time
	keys map[string]ed25519.PublicKey
}{}

func verifyNeon(ctx context.Context, raw string) (jwt.MapClaims, error) {
	base, e := url.Parse(os.Getenv("NEON_AUTH_BASE_URL"))
	if e != nil || base.Scheme != "https" || base.Host == "" {
		return nil, errors.New("Neon Auth não configurado")
	}
	issuer := base.Scheme + "://" + base.Host
	jwks := os.Getenv("NEON_AUTH_JWKS_URL")
	if jwks == "" {
		jwks = strings.TrimRight(base.String(), "/") + "/.well-known/jwks.json"
	}
	parsed, e := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		neonKeys.Lock()
		defer neonKeys.Unlock()
		if neonKeys.url != jwks || time.Since(neonKeys.at) > 5*time.Minute || neonKeys.keys[kid] == nil {
			req, e := http.NewRequestWithContext(ctx, "GET", jwks, nil)
			if e != nil {
				return nil, e
			}
			resp, e := (&http.Client{Timeout: 10 * time.Second}).Do(req)
			if e != nil {
				return nil, e
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				return nil, errors.New("JWKS indisponível")
			}
			var doc struct {
				Keys []struct {
					Kid string `json:"kid"`
					Kty string `json:"kty"`
					Crv string `json:"crv"`
					X   string `json:"x"`
				} `json:"keys"`
			}
			if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); e != nil {
				return nil, e
			}
			keys := map[string]ed25519.PublicKey{}
			for _, k := range doc.Keys {
				b, err := base64.RawURLEncoding.DecodeString(k.X)
				if err == nil && k.Kty == "OKP" && k.Crv == "Ed25519" && len(b) == ed25519.PublicKeySize {
					keys[k.Kid] = ed25519.PublicKey(b)
				}
			}
			neonKeys.keys = keys
			neonKeys.at = time.Now()
			neonKeys.url = jwks
		}
		key := neonKeys.keys[kid]
		if key == nil {
			return nil, errors.New("chave desconhecida")
		}
		return key, nil
	}, jwt.WithIssuer(issuer), jwt.WithAudience(issuer), jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithExpirationRequired())
	if e != nil || !parsed.Valid {
		return nil, errors.New("sessão Neon inválida")
	}
	c, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("sessão Neon inválida")
	}
	if banned, _ := c["banned"].(bool); banned {
		return nil, errors.New("conta desativada")
	}
	return c, nil
}

// Native clients still use the common Go endpoint. Verify managed credentials at
// Neon instead of storing a second password hash for a Neon-managed identity.
func neonPassword(ctx context.Context, email, password string) (string, error) {
	base := strings.TrimRight(os.Getenv("NEON_AUTH_BASE_URL"), "/")
	if !strings.HasPrefix(base, "https://") {
		return "", errors.New("Neon Auth não configurado")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 15 * time.Second, Jar: jar, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	data, _ := json.Marshal(map[string]any{"email": email, "password": password, "rememberMe": false})
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/sign-in/email", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", env("APP_URL", "http://localhost:3017"))
	resp, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	var out struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || out.User.ID == "" {
		return "", errors.New("credenciais inválidas")
	}
	logout, _ := http.NewRequestWithContext(ctx, "POST", base+"/sign-out", strings.NewReader("{}"))
	logout.Header.Set("Content-Type", "application/json")
	logout.Header.Set("Origin", env("APP_URL", "http://localhost:3017"))
	if rr, err := client.Do(logout); err == nil {
		rr.Body.Close()
	}
	return out.User.ID, nil
}
func (a *App) neonSession(w http.ResponseWriter, r *http.Request) {
	var d credentials
	if !decode(w, r, &d) {
		return
	}
	c, e := verifyNeon(r.Context(), d.Token)
	if e != nil {
		problem(w, 401, "Sessão Neon inválida ou expirada.")
		return
	}
	subject, _ := c["sub"].(string)
	email, _ := c["email"].(string)
	name, _ := c["name"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if subject == "" || !validEmail(email) {
		problem(w, 401, "Identidade Neon incompleta.")
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
	e = t.QueryRow(ctx, "select id,company_id,name,email,role from auth_find_neon_user($1)", subject).Scan(&u.ID, &u.Company, &u.Name, &u.Email, &u.Role)
	if errors.Is(e, pgx.ErrNoRows) {
		d.CNPJ = regexp.MustCompile(`[./\s-]`).ReplaceAllString(strings.ToUpper(d.CNPJ), "")
		if len(strings.TrimSpace(d.Company)) < 2 || !validCNPJ(d.CNPJ) {
			problem(w, 422, "Complete o cadastro da empresa para vincular sua conta Neon.")
			return
		}
		if len(name) < 2 {
			name = d.Name
		}
		if len(name) < 2 {
			name = "Gestor"
		}
		u = identity{id(), id(), name, email, "owner"}
		_, e = t.Exec(ctx, "select set_config('app.company_id',$1,true)", u.Company)
		if e == nil {
			_, e = t.Exec(ctx, "insert into companies(id,name,cnpj) values($1,$2,$3)", u.Company, d.Company, d.CNPJ)
		}
		if e == nil {
			_, e = t.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role,neon_subject,email_verified_at) values($1,$2,$3,$4,$5,'owner',$6,case when $7 then now() else null end)", u.ID, u.Company, u.Name, u.Email, passwordHash(randomToken()), subject, c["emailVerified"] == true)
		}
		if e != nil {
			problem(w, 409, "Empresa ou e-mail já cadastrado. Use o acesso existente; contas não são vinculadas apenas por e-mail.")
			return
		}
		if a.audit(ctx, t, &Claims{Company: u.Company, RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID}}, "company", u.Company, map[string]string{"provider": "neon", "name": d.Company}) != nil {
			problem(w, 500, "Falha ao registrar cadastro.")
			return
		}
	} else if e != nil {
		problem(w, 500, "Falha ao localizar conta.")
		return
	}
	out, e := a.session(ctx, t, u)
	if e != nil || t.Commit(ctx) != nil {
		problem(w, 500, "Falha ao iniciar sessão.")
		return
	}
	write(w, 200, out)
}
