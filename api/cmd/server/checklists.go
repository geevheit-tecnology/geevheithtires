package main

import (
	"context"
	"encoding/json"
	"errors"
	"geevheittire/api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"sort"
	"time"
)

type inspectionInput struct {
	Tire        string     `json:"tire_id"`
	Client      string     `json:"client_id"`
	Tread       float64    `json:"tread_mm"`
	Pressure    float64    `json:"pressure_psi"`
	Damage      bool       `json:"structural_damage"`
	Temperature *float64   `json:"temperature_c,omitempty"`
	InspectedAt *time.Time `json:"inspected_at,omitempty"`
	VehicleKM   *int       `json:"vehicle_km,omitempty"`
}
type inspectionResult struct {
	ID             string                `json:"id"`
	Client         string                `json:"client_id"`
	Classification domain.Classification `json:"classification"`
	Replayed       bool                  `json:"replayed"`
	Applied        bool                  `json:"applied"`
}
type inspectionFailure struct {
	status int
	title  string
}

func (e *inspectionFailure) Error() string   { return e.title }
func failure(status int, title string) error { return &inspectionFailure{status, title} }
func inspectionError(w http.ResponseWriter, e error) {
	var f *inspectionFailure
	if errors.As(e, &f) {
		problem(w, f.status, f.title)
	} else {
		problem(w, 500, "Não foi possível salvar. Reenvie com o mesmo identificador.")
	}
}
func validInspection(d inspectionInput) error {
	if uuid.Validate(d.Tire) != nil || uuid.Validate(d.Client) != nil {
		return failure(422, "Identificador inválido.")
	}
	if _, e := domain.Classify(d.Tread, d.Damage); e != nil || d.Pressure < 0 || d.Pressure > 200 {
		return failure(422, "Medições inválidas.")
	}
	if d.Temperature != nil && (*d.Temperature < -50 || *d.Temperature > 200) {
		return failure(422, "Temperatura inválida.")
	}
	if d.VehicleKM != nil && *d.VehicleKM < 0 {
		return failure(422, "Odômetro inválido.")
	}
	if d.InspectedAt != nil && (d.InspectedAt.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) || d.InspectedAt.After(time.Now().Add(5*time.Minute))) {
		return failure(422, "Data da inspeção inválida. Verifique o relógio do aparelho.")
	}
	return nil
}
func (a *App) saveInspection(ctx context.Context, t pgx.Tx, c *Claims, d inspectionInput, vehicle string) (inspectionResult, error) {
	out := inspectionResult{Client: d.Client}
	if e := validInspection(d); e != nil {
		return out, e
	}
	if _, e := t.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", c.Company+":"+d.Client); e != nil {
		return out, e
	}
	var existingTire, status, recommendation string
	var level int
	e := t.QueryRow(ctx, "select id,tire_id,status,level,recommendation from inspections where company_id=$1 and client_id=$2", c.Company, d.Client).Scan(&out.ID, &existingTire, &status, &level, &recommendation)
	if e == nil {
		if existingTire != d.Tire {
			return out, failure(409, "Identificador já utilizado em outro pneu.")
		}
		out.Classification = domain.Classification{Status: status, Level: level, Recommendation: recommendation}
		out.Replayed = true
		return out, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	var latest *time.Time
	var assigned *string
	e = t.QueryRow(ctx, "select latest_inspected_at,vehicle_id from tires where id=$1 and company_id=$2 and deleted_at is null for update", d.Tire, c.Company).Scan(&latest, &assigned)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return out, failure(404, "Pneu não encontrado.")
		}
		return out, e
	}
	if vehicle != "" && (assigned == nil || *assigned != vehicle) {
		return out, failure(409, "O pneu mudou de veículo. Revise o checklist antes de reenviar.")
	}
	at := time.Now().UTC()
	if d.InspectedAt != nil {
		at = d.InspectedAt.UTC()
	}
	out.Applied = latest == nil || at.After(*latest)
	out.Classification, _ = domain.Classify(d.Tread, d.Damage)
	out.ID = id()
	_, e = t.Exec(ctx, `insert into inspections(id,company_id,tire_id,user_id,client_id,tread_mm,pressure_psi,structural_damage,status,level,recommendation,inspected_at,vehicle_id,vehicle_km,temperature_c) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, out.ID, c.Company, d.Tire, c.Subject, d.Client, d.Tread, d.Pressure, d.Damage, out.Classification.Status, out.Classification.Level, out.Classification.Recommendation, at, assigned, d.VehicleKM, d.Temperature)
	if e != nil {
		return out, e
	}
	outcome := "historical"
	if out.Applied {
		outcome = "applied"
		_, e = t.Exec(ctx, "update tires set tread_mm=$1,structural_damage=$2,latest_inspected_at=$3,updated_at=now() where id=$4 and company_id=$5", d.Tread, d.Damage, at, d.Tire, c.Company)
		if e != nil {
			return out, e
		}
		if assigned != nil && d.VehicleKM != nil {
			_, e = t.Exec(ctx, "update vehicles set current_km=greatest(current_km,$1),updated_at=now() where id=$2 and company_id=$3", *d.VehicleKM, *assigned, c.Company)
			if e != nil {
				return out, e
			}
		}
		pressure := d.Pressure < 90 || d.Pressure > 120
		hot := d.Temperature != nil && *d.Temperature >= 85
		if out.Classification.Status != "ok" || pressure || hot {
			severity := out.Classification.Status
			msg := out.Classification.Recommendation
			if pressure {
				msg += " Pressão fora da referência inicial (90–120 psi)."
				if severity == "ok" {
					severity = "alerta"
				}
			}
			if hot {
				severity = "critico"
				msg += " Temperatura elevada: verificar imediatamente."
			}
			_, e = t.Exec(ctx, "insert into alerts(id,company_id,tire_id,message,severity) values($1,$2,$3,$4,$5)", id(), c.Company, d.Tire, msg, severity)
			if e != nil {
				return out, e
			}
		}
	}
	_, e = t.Exec(ctx, "insert into sync_log(id,company_id,user_id,inspection_id,client_id,outcome,device_at) values($1,$2,$3,$4,$5,$6,$7)", id(), c.Company, c.Subject, out.ID, d.Client, outcome, at)
	if e != nil {
		return out, e
	}
	if e = a.audit(ctx, t, c, "inspection", out.ID, d); e != nil {
		return out, e
	}
	return out, nil
}
func (a *App) inspect(w http.ResponseWriter, r *http.Request) {
	if claims(r).Role == "driver" {
		problem(w, 403, "A inspeção técnica exige perfil autorizado.")
		return
	}
	var d inspectionInput
	if !decode(w, r, &d) {
		return
	}
	tid := chi.URLParam(r, "id")
	if d.Tire != "" && d.Tire != tid {
		problem(w, 422, "Pneu inconsistente.")
		return
	}
	d.Tire = tid
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	out, e := a.saveInspection(r.Context(), t, claims(r), d, "")
	if e != nil {
		inspectionError(w, e)
		return
	}
	if e = t.Commit(r.Context()); e != nil {
		inspectionError(w, e)
		return
	}
	status := 201
	if out.Replayed {
		status = 200
	}
	write(w, status, out)
}
func (a *App) checklist(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if c.Role == "driver" {
		problem(w, 403, "A inspeção técnica exige perfil autorizado.")
		return
	}
	var d struct {
		Client      string            `json:"client_id"`
		Vehicle     string            `json:"vehicle_id"`
		Inspections []inspectionInput `json:"inspections"`
	}
	if !decode(w, r, &d) {
		return
	}
	if uuid.Validate(d.Client) != nil || uuid.Validate(d.Vehicle) != nil || len(d.Inspections) == 0 || len(d.Inspections) > 80 {
		problem(w, 422, "Checklist inválido: informe veículo e de 1 a 80 pneus.")
		return
	}
	seen := map[string]bool{}
	for _, i := range d.Inspections {
		if e := validInspection(i); e != nil {
			inspectionError(w, e)
			return
		}
		if seen[i.Tire] || seen[i.Client] {
			problem(w, 422, "Pneu ou identificador repetido no checklist.")
			return
		}
		seen[i.Tire] = true
		seen[i.Client] = true
	}
	payload, _ := json.Marshal(d)
	hash := hashToken(string(payload))
	ctx := r.Context()
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(ctx)
	_, e = t.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", c.Company+":batch:"+d.Client)
	if e != nil {
		inspectionError(w, e)
		return
	}
	var storedHash, owner string
	var response []byte
	e = t.QueryRow(ctx, "select payload_hash,user_id,response from checklist_batches where company_id=$1 and client_id=$2", c.Company, d.Client).Scan(&storedHash, &owner, &response)
	if e == nil {
		if storedHash != hash || owner != c.Subject {
			problem(w, 409, "Identificador de checklist já utilizado com outros dados.")
			return
		}
		write(w, 200, json.RawMessage(response))
		return
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		inspectionError(w, e)
		return
	}
	var exists bool
	e = t.QueryRow(ctx, "select exists(select 1 from vehicles where id=$1 and company_id=$2 and deleted_at is null)", d.Vehicle, c.Company).Scan(&exists)
	if e != nil {
		inspectionError(w, e)
		return
	}
	if !exists {
		problem(w, 404, "Veículo não encontrado.")
		return
	}
	sort.Slice(d.Inspections, func(i, j int) bool { return d.Inspections[i].Tire < d.Inspections[j].Tire })
	results := []inspectionResult{}
	for _, i := range d.Inspections {
		out, err := a.saveInspection(ctx, t, c, i, d.Vehicle)
		if err != nil {
			inspectionError(w, err)
			return
		}
		results = append(results, out)
	}
	body := map[string]any{"client_id": d.Client, "data": results, "synced_at": time.Now().UTC()}
	response, _ = json.Marshal(body)
	_, e = t.Exec(ctx, "insert into checklist_batches(id,company_id,user_id,vehicle_id,client_id,payload_hash,response) values($1,$2,$3,$4,$5,$6,$7)", id(), c.Company, c.Subject, d.Vehicle, d.Client, hash, response)
	if e != nil {
		inspectionError(w, e)
		return
	}
	if e = t.Commit(ctx); e != nil {
		inspectionError(w, e)
		return
	}
	write(w, 201, json.RawMessage(response))
}
