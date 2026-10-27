package main

import (
	"context"
	"github.com/jackc/pgx/v5"
	"os"
	"strings"
	"testing"
)

func TestComparativeCPKAlertRequiresPeersAndResolves(t *testing.T) {
	database := os.Getenv("TEST_DATABASE_URL")
	if database == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, e := pgx.Connect(ctx, database)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close(ctx)
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	company := id()
	if _, e = tx.Exec(ctx, "select set_config('app.company_id',$1,true)", company); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "insert into companies(id,name,cnpj) values($1,'CPK QA',$2)", company, "cpk-"+company); e != nil {
		t.Fatal(e)
	}
	a := &App{}
	expensive := ""
	for i := 0; i < 4; i++ {
		tire := id()
		cost := 1000
		if i == 0 {
			cost = 2000
			expensive = tire
		}
		if _, e = tx.Exec(ctx, `insert into tires(id,company_id,code,serial_number,dot_code,expires_on,road_use,brand,size,tread_mm,acquisition_cost,tracked_km) values($1,$2,$3,$3,'1025','2035-01-01','MISTO','QA','295/80 R22.5',9,$4,10000)`, tire, company, tire, cost); e != nil {
			t.Fatal(e)
		}
		if e = a.refreshMaintenance(ctx, tx, company); e != nil {
			t.Fatal(e)
		}
		var count int
		if e = tx.QueryRow(ctx, "select count(*) from alerts where company_id=$1 and kind='cpk_high' and resolved_at is null", company).Scan(&count); e != nil {
			t.Fatal(e)
		}
		want := 0
		if i == 3 {
			want = 1
		}
		if count != want {
			t.Fatalf("%d peers: want %d alerts got %d", i, want, count)
		}
	}
	if _, e = tx.Exec(ctx, "update tires set acquisition_cost=1000 where id=$1", expensive); e != nil {
		t.Fatal(e)
	}
	if e = a.refreshMaintenance(ctx, tx, company); e != nil {
		t.Fatal(e)
	}
	var resolved bool
	if e = tx.QueryRow(ctx, "select resolved_at is not null from alerts where company_id=$1 and tire_id=$2 and kind='cpk_high'", company, expensive).Scan(&resolved); e != nil || !resolved {
		t.Fatal("CPK alert not reconciled", e)
	}
	// Rotation warning starts 10% before the configured interval and advances when due.
	vehicle, tire := id(), id()
	if _, e = tx.Exec(ctx, "insert into vehicles(id,company_id,plate,model,template_id,current_km) values($1,$2,'QAX1A23','QA','tractor-6x2',9000)", vehicle, company); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, `insert into tires(id,company_id,code,serial_number,dot_code,expires_on,road_use,brand,size,tread_mm,acquisition_cost,state,vehicle_id,position,mounted_at_km) values($1,$2,$4,$4,'1025','2035-01-01','MISTO','QA','295/80 R22.5',9,1000,'INSTALADO',$3,'1EE',0)`, tire, company, vehicle, tire); e != nil {
		t.Fatal(e)
	}
	if e = a.refreshMaintenance(ctx, tx, company); e != nil {
		t.Fatal(e)
	}
	var message string
	if e = tx.QueryRow(ctx, "select message from alerts where company_id=$1 and tire_id=$2 and kind='rotation' and resolved_at is null", company, tire).Scan(&message); e != nil || !strings.Contains(message, "faltam 1000 km") {
		t.Fatal("missing early rotation warning", message, e)
	}
	if _, e = tx.Exec(ctx, "update vehicles set current_km=10000 where id=$1", vehicle); e != nil {
		t.Fatal(e)
	}
	if e = a.refreshMaintenance(ctx, tx, company); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRow(ctx, "select message from alerts where company_id=$1 and tire_id=$2 and kind='rotation' and resolved_at is null", company, tire).Scan(&message); e != nil || !strings.Contains(message, "rodízio previsto") {
		t.Fatal("missing due rotation warning", message, e)
	}

}
