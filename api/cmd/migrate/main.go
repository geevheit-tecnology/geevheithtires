package main

import (
	"context"
	"fmt"
	"geevheittire/api/migrations"
	"github.com/jackc/pgx/v5"
	"os"
)

func main() {
	ctx := context.Background()
	conn, e := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		panic(e)
	}
	defer conn.Close(ctx)
	if _, e = conn.Exec(ctx, "select pg_advisory_lock(8246701)"); e != nil {
		panic(e)
	}
	defer conn.Exec(ctx, "select pg_advisory_unlock(8246701)")
	if _, e = conn.Exec(ctx, "create table if not exists schema_migrations(version text primary key,applied_at timestamptz not null default now())"); e != nil {
		panic(e)
	}
	entries, _ := migrations.Files.ReadDir(".")
	for _, entry := range entries {
		n := entry.Name()
		if len(n) < 7 || n[len(n)-7:] != ".up.sql" {
			continue
		}
		var applied bool
		if e = conn.QueryRow(ctx, "select exists(select 1 from schema_migrations where version=$1)", n).Scan(&applied); e != nil {
			panic(e)
		}
		if applied {
			continue
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			panic(e)
		}
		sql, e := migrations.Files.ReadFile(n)
		if e != nil {
			panic(e)
		}
		if _, e = tx.Exec(ctx, string(sql)); e != nil {
			tx.Rollback(ctx)
			panic(e)
		}
		if _, e = tx.Exec(ctx, "insert into schema_migrations(version) values($1)", n); e != nil {
			tx.Rollback(ctx)
			panic(e)
		}
		if e = tx.Commit(ctx); e != nil {
			panic(e)
		}
		fmt.Println("Applied", n)
	}
}
