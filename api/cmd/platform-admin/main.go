// platform-admin is an operator-only tool. It requires the direct migration-owner
// connection; the web runtime cannot grant platform membership.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"strings"
	"time"
)

func main() {
	email := flag.String("email", "", "Existing account email")
	reason := flag.String("reason", "", "Audit justification (5-500 characters)")
	revoke := flag.Bool("revoke", false, "Revoke access immediately")
	flag.Parse()
	if *email == "" || len(strings.TrimSpace(*reason)) < 5 || len(*reason) > 500 {
		fmt.Fprintln(os.Stderr, "Use -email and -reason (5-500 characters)")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, e := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		fmt.Fprintln(os.Stderr, "Database connection failed")
		os.Exit(1)
	}
	defer db.Close(ctx)
	tx, e := db.Begin(ctx)
	if e != nil {
		panic(e)
	}
	defer tx.Rollback(ctx)
	var userID, name string
	e = tx.QueryRow(ctx, "select id,name from users where lower(email)=lower($1) and deleted_at is null", strings.TrimSpace(*email)).Scan(&userID, &name)
	if e != nil {
		fmt.Fprintln(os.Stderr, "Existing account not found. Complete registration first.")
		os.Exit(1)
	}
	action := "grant_admin"
	if *revoke {
		_, e = tx.Exec(ctx, "delete from platform_admins where user_id=$1", userID)
		action = "revoke_admin"
	} else {
		_, e = tx.Exec(ctx, "insert into platform_admins(user_id) values($1) on conflict do nothing", userID)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "Membership change failed; use the migration-owner connection")
		os.Exit(1)
	}
	_, e = tx.Exec(ctx, "insert into platform_audit(actor_id,action,target,reason) values($1,$2,$1::text,$3)", userID, action, *reason)
	if e != nil || tx.Commit(ctx) != nil {
		fmt.Fprintln(os.Stderr, "Audit commit failed")
		os.Exit(1)
	}
	fmt.Printf("%s: %s (%s)\n", action, name, *email)
}
