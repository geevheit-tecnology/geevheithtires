package main

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type driverInput struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	License string `json:"license_number"`
	Expiry  string `json:"license_expires_on"`
	Active  *bool  `json:"active"`
}

func (a *App) saveDriver(w http.ResponseWriter, r *http.Request) {
	if !canManage(claims(r)) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	var d driverInput
	if !decode(w, r, &d) {
		return
	}
	d.Name = strings.TrimSpace(d.Name)
	d.Phone = strings.TrimSpace(d.Phone)
	d.License = strings.TrimSpace(d.License)
	var expiry any
	if d.Expiry != "" {
		date, err := time.Parse("2006-01-02", d.Expiry)
		if err != nil {
			problem(w, 422, "Informe uma data válida para a CNH.")
			return
		}
		expiry = date
	}
	if len(d.Name) < 2 || len(d.Name) > 150 || len(d.Phone) > 30 || (d.License != "" && (!regexp.MustCompile(`^[0-9]{11}$`).MatchString(d.License) || expiry == nil)) {
		problem(w, 422, "Informe nome e, se preenchida, CNH com 11 dígitos e validade.")
		return
	}
	tid := chi.URLParam(r, "id")
	creating := tid == ""
	if creating {
		tid = id()
	} else if uuid.Validate(tid) != nil {
		problem(w, 404, "Motorista não encontrado.")
		return
	}
	active := true
	if d.Active != nil {
		active = *d.Active
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	if e = fleetLock(r.Context(), t, claims(r).Company); e != nil {
		inspectionError(w, e)
		return
	}
	if !active {
		var assigned bool
		if e = t.QueryRow(r.Context(), "select exists(select 1 from vehicles where company_id=$1 and driver_id=$2 and deleted_at is null)", claims(r).Company, tid).Scan(&assigned); e != nil {
			inspectionError(w, e)
			return
		}
		if assigned {
			problem(w, 409, "Retire o vínculo com os veículos antes de inativar o motorista.")
			return
		}
	}
	if creating {
		_, e = t.Exec(r.Context(), `insert into drivers(id,company_id,name,phone,license_number,license_expires_on,active) values($1,$2,$3,$4,$5,$6,$7)`, tid, claims(r).Company, d.Name, d.Phone, d.License, expiry, active)
	} else {
		result, err := t.Exec(r.Context(), `update drivers set name=$3,phone=$4,license_number=$5,license_expires_on=$6,active=$7,updated_at=now() where company_id=$1 and id=$2`, claims(r).Company, tid, d.Name, d.Phone, d.License, expiry, active)
		e = err
		if e == nil && result.RowsAffected() == 0 {
			problem(w, 404, "Motorista não encontrado.")
			return
		}
	}
	if e != nil {
		problem(w, 409, "CNH já cadastrada ou dados inválidos.")
		return
	}
	// Do not copy CNH or phone into the general audit log.
	e = a.audit(r.Context(), t, claims(r), "driver", tid, map[string]any{"active": active})
	if e == nil {
		e = t.Commit(r.Context())
	}
	if e != nil {
		inspectionError(w, e)
		return
	}
	status := 200
	if creating {
		status = 201
	}
	write(w, status, map[string]string{"id": tid})
}

func (a *App) assignDriver(w http.ResponseWriter, r *http.Request) {
	if !canManage(claims(r)) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	var d struct {
		Driver string `json:"driver_id"`
	}
	if !decode(w, r, &d) {
		return
	}
	vid := chi.URLParam(r, "id")
	if uuid.Validate(vid) != nil || (d.Driver != "" && uuid.Validate(d.Driver) != nil) {
		problem(w, 422, "Veículo ou motorista inválido.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	if e = fleetLock(r.Context(), t, claims(r).Company); e != nil {
		inspectionError(w, e)
		return
	}
	var driver any
	if d.Driver != "" {
		var valid bool
		e = t.QueryRow(r.Context(), `select exists(select 1 from drivers where company_id=$1 and id=$2 and active and (license_expires_on is null or license_expires_on>=current_date))`, claims(r).Company, d.Driver).Scan(&valid)
		if e != nil {
			inspectionError(w, e)
			return
		}
		if !valid {
			problem(w, 422, "Selecione um motorista ativo da empresa, com CNH dentro da validade quando informada.")
			return
		}
		driver = d.Driver
	}
	result, e := t.Exec(r.Context(), "update vehicles set driver_id=$3,updated_at=now() where company_id=$1 and id=$2 and deleted_at is null", claims(r).Company, vid, driver)
	if e != nil {
		inspectionError(w, e)
		return
	}
	if result.RowsAffected() != 1 {
		problem(w, 404, "Veículo não encontrado.")
		return
	}
	e = a.audit(r.Context(), t, claims(r), "vehicle_driver", vid, d)
	if e == nil {
		e = t.Commit(r.Context())
	}
	if e != nil {
		inspectionError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
