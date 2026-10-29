package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type tireIdentityInput struct {
	ManufactureYear int    `json:"manufacture_year"`
	Code            string `json:"code"`
	DOT             string `json:"dot_code"`
	Serial          string `json:"serial_number"`
	Expiry          string `json:"expires_on"`
	Road            string `json:"road_use"`
}

func (d *tireIdentityInput) validate(now time.Time) bool {
	d.Code = strings.ToUpper(strings.TrimSpace(d.Code))
	d.Serial = strings.ToUpper(strings.TrimSpace(d.Serial))
	d.DOT = strings.ToUpper(strings.Join(strings.Fields(d.DOT), ""))
	d.DOT = strings.TrimPrefix(d.DOT, "DOT")
	if len(d.Code) < 2 || len(d.Code) > 50 || len(d.Serial) < 2 || len(d.Serial) > 80 || !regexp.MustCompile(`^[A-Z0-9]{0,28}[0-9]{4}$`).MatchString(d.DOT) {
		return false
	}
	tail := d.DOT[len(d.DOT)-4:]
	week, _ := strconv.Atoi(tail[:2])
	yy, _ := strconv.Atoi(tail[2:])
	year := 2000 + yy
	if d.ManufactureYear != 0 && d.ManufactureYear != year {
		return false
	}
	d.ManufactureYear = year
	if week < 1 || week > 53 || year > now.Year() {
		return false
	}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	start = start.AddDate(0, 0, (7-int(start.Weekday()))%7+(week-1)*7)
	if start.Year() != year || start.After(now.UTC()) {
		return false
	}
	expiry, err := time.Parse("2006-01-02", d.Expiry)
	if err != nil || expiry.Before(start) {
		return false
	}
	return d.Road == "ASFALTO" || d.Road == "TERRA" || d.Road == "MISTO"
}

func (a *App) updateTireIdentity(w http.ResponseWriter, r *http.Request) {
	if !canManage(claims(r)) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	var d struct {
		tireIdentityInput
		Version int64 `json:"expected_version"`
	}
	if !decode(w, r, &d) {
		return
	}
	tid := chi.URLParam(r, "id")
	if uuid.Validate(tid) != nil || d.Version < 1 || !d.validate(time.Now()) {
		problem(w, 422, "Confira número de fogo, número de série, DOT (semana e ano), validade e tipo de via.")
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
	result, e := t.Exec(r.Context(), `update tires set code=$3,dot_code=$4,expires_on=$5,road_use=$6,serial_number=$8,version=version+1,updated_at=now() where company_id=$1 and id=$2 and version=$7 and deleted_at is null`, claims(r).Company, tid, d.Code, d.DOT, d.Expiry, d.Road, d.Version, d.Serial)
	if e != nil {
		problem(w, 409, "Número de fogo já cadastrado ou dados inválidos.")
		return
	}
	if result.RowsAffected() != 1 {
		problem(w, 409, "Pneu não encontrado ou alterado. Atualize antes de salvar.")
		return
	}
	e = a.audit(r.Context(), t, claims(r), "tire_identity", tid, d)
	if e == nil {
		e = a.refreshMaintenance(r.Context(), t, claims(r).Company)
	}
	if e == nil {
		e = t.Commit(r.Context())
	}
	if e != nil {
		inspectionError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
