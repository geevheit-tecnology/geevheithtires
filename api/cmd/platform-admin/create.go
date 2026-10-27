package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/argon2"
	"net/mail"
	"net/url"
	"os"
	"strings"
)

func createAdministrator(ctx context.Context, tx pgx.Tx, email string) (string, string, error) {
	email = strings.ToLower(email)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", "", errors.New("invalid email")
	}
	base, err := url.Parse(os.Getenv("APP_URL"))
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Hostname() != "localhost") {
		return "", "", errors.New("APP_URL must be HTTPS or localhost")
	}
	var exists bool
	if err = tx.QueryRow(ctx, "select exists(select 1 from users where lower(email)=lower($1))", email).Scan(&exists); err != nil {
		return "", "", err
	}
	if exists {
		return "", "", errors.New("email already registered; existing account was not changed")
	}
	company, user := uuid.NewString(), uuid.NewString()
	if _, err = tx.Exec(ctx, "select set_config('app.company_id',$1,true)", company); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'Administração GeevheitTire',$2)", company, "internal-"+company); err != nil {
		return "", "", err
	}
	secret, salt, token := make([]byte, 32), make([]byte, 16), make([]byte, 32)
	for _, b := range [][]byte{secret, salt, token} {
		if _, err = rand.Read(b); err != nil {
			return "", "", err
		}
	}
	hash := base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(argon2.IDKey(secret, salt, 2, 64*1024, 2, 32))
	if _, err = tx.Exec(ctx, "insert into users(id,company_id,name,email,password_hash,role) values($1,$2,'Administrador GeevheitTire',$3,$4,'owner')", user, company, email, hash); err != nil {
		return "", "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(token)
	digest := sha256.Sum256([]byte(raw))
	if _, err = tx.Exec(ctx, "insert into account_tokens(id,company_id,user_id,token_hash,kind,expires_at) values($1,$2,$3,$4,'invite',now()+interval '24 hours')", uuid.NewString(), company, user, hex.EncodeToString(digest[:])); err != nil {
		return "", "", err
	}
	base.Path = "/aceitar-convite"
	base.RawQuery = url.Values{"token": {raw}, "kind": {"invite"}, "next": {"/admin"}}.Encode()
	return user, base.String(), nil
}
