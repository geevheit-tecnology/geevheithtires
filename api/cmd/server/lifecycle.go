package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type tirePolicy struct {
	Warning        float64 `json:"warning_mm"`
	Critical       float64 `json:"critical_mm"`
	Rotation       int64   `json:"rotation_km"`
	InspectionDays int     `json:"inspection_days"`
	MinimumStock   int     `json:"minimum_stock"`
	MaxRetreads    int     `json:"max_retreads"`
}

func policy(ctx context.Context, t pgx.Tx, company string) (tirePolicy, error) {
	p := tirePolicy{5, 3, 10000, 30, 2, 2}
	e := t.QueryRow(ctx, "select warning_mm,critical_mm,rotation_km,inspection_days,minimum_stock,max_retreads from tire_policies where company_id=$1", company).Scan(&p.Warning, &p.Critical, &p.Rotation, &p.InspectionDays, &p.MinimumStock, &p.MaxRetreads)
	if errors.Is(e, pgx.ErrNoRows) {
		return p, nil
	}
	return p, e
}
func fleetLock(ctx context.Context, t pgx.Tx, company string) error {
	_, e := t.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,8))", company)
	return e
}
func canMove(c *Claims) bool { return canManage(c) || c.Role == "tire_tech" }
func (a *App) getTirePolicy(w http.ResponseWriter, r *http.Request) {
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	p, e := policy(r.Context(), t, claims(r).Company)
	if e != nil {
		problem(w, 503, "Não foi possível carregar as regras.")
		return
	}
	write(w, 200, p)
}
func (a *App) setTirePolicy(w http.ResponseWriter, r *http.Request) {
	if !canManage(claims(r)) {
		problem(w, 403, "Acesso restrito à gestão.")
		return
	}
	var p tirePolicy
	if !decode(w, r, &p) {
		return
	}
	if p.Critical < 1 || p.Warning <= p.Critical || p.Warning > 20 || p.Rotation < 100 || p.Rotation > 500000 || p.InspectionDays < 1 || p.InspectionDays > 365 || p.MinimumStock < 0 || p.MinimumStock > 10000 || p.MaxRetreads < 0 || p.MaxRetreads > 10 {
		problem(w, 422, "Confira os limites das regras de manutenção.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	ctx := r.Context()
	c := claims(r)
	if e = fleetLock(ctx, t, c.Company); e == nil {
		_, e = t.Exec(ctx, `insert into tire_policies(company_id,warning_mm,critical_mm,rotation_km,inspection_days,minimum_stock,max_retreads) values($1,$2,$3,$4,$5,$6,$7) on conflict(company_id) do update set warning_mm=excluded.warning_mm,critical_mm=excluded.critical_mm,rotation_km=excluded.rotation_km,inspection_days=excluded.inspection_days,minimum_stock=excluded.minimum_stock,max_retreads=excluded.max_retreads`, c.Company, p.Warning, p.Critical, p.Rotation, p.InspectionDays, p.MinimumStock, p.MaxRetreads)
	}
	if e == nil {
		e = a.audit(ctx, t, c, "tire_policy", c.Company, p)
	}
	if e == nil {
		e = a.refreshMaintenance(ctx, t, c.Company)
	}
	if e == nil {
		e = t.Commit(ctx)
	}
	if e != nil {
		problem(w, 500, "Não foi possível salvar as regras.")
		return
	}
	write(w, 200, p)
}

type movementInput struct {
	Client        string   `json:"client_id"`
	Action        string   `json:"action"`
	Version       int64    `json:"expected_version"`
	Vehicle       string   `json:"vehicle_id"`
	Position      string   `json:"position"`
	SourceKM      *int     `json:"source_km"`
	DestinationKM *int     `json:"vehicle_km"`
	Swap          string   `json:"swap_tire_id"`
	SwapVersion   int64    `json:"swap_version"`
	Reason        string   `json:"reason"`
	Location      string   `json:"stock_location"`
	Supplier      string   `json:"supplier"`
	Cost          int      `json:"cost_cents"`
	Tread         *float64 `json:"new_tread_mm"`
}
type tireSnapshot struct {
	ID, Code, State, Location string
	Vehicle, Position         *string
	Mounted                   *int
	KM                        int64
	Rotation                  int64
	Version                   int64
	Retreads                  int
	Tread                     float64
	Damage                    bool
}

func loadTire(ctx context.Context, t pgx.Tx, company, tid string) (tireSnapshot, error) {
	var s tireSnapshot
	e := t.QueryRow(ctx, `select id,code,state,stock_location,vehicle_id,position,mounted_at_km,tracked_km,rotation_baseline_km,version,retread_count,tread_mm,structural_damage from tires where company_id=$1 and id=$2 and deleted_at is null for update`, company, tid).Scan(&s.ID, &s.Code, &s.State, &s.Location, &s.Vehicle, &s.Position, &s.Mounted, &s.KM, &s.Rotation, &s.Version, &s.Retreads, &s.Tread, &s.Damage)
	return s, e
}
func vehicleOdo(ctx context.Context, t pgx.Tx, company, vid string, km *int) (int, error) {
	var current int
	e := t.QueryRow(ctx, "select current_km from vehicles where company_id=$1 and id=$2 and deleted_at is null for update", company, vid).Scan(&current)
	if errors.Is(e, pgx.ErrNoRows) {
		return 0, failure(404, "Veículo não encontrado.")
	}
	if e != nil {
		return 0, e
	}
	if km == nil || *km < current || *km > 2147483647 {
		return 0, failure(422, "Informe o odômetro atual, igual ou maior que o já registrado.")
	}
	_, e = t.Exec(ctx, "update vehicles set current_km=$3,updated_at=now() where company_id=$1 and id=$2", company, vid, *km)
	return *km, e
}
func event(ctx context.Context, t pgx.Tx, c *Claims, before, after tireSnapshot, client, action, reason, supplier string, cost int, source, dest *int) error {
	var actor any
	if c.Subject != "" {
		actor = c.Subject
	}
	var operation any
	if client != "" {
		operation = client
	}
	_, e := t.Exec(ctx, `insert into tire_movements(id,company_id,tire_id,actor_id,client_id,action,from_state,to_state,from_vehicle_id,from_position,to_vehicle_id,to_position,source_km,destination_km,tracked_km,stock_location,reason,supplier,cost_cents,tread_mm) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, id(), c.Company, after.ID, actor, operation, action, before.State, after.State, before.Vehicle, before.Position, after.Vehicle, after.Position, source, dest, after.KM, after.Location, reason, supplier, cost, after.Tread)
	return e
}
func saveTire(ctx context.Context, t pgx.Tx, company string, s tireSnapshot) error {
	_, e := t.Exec(ctx, `update tires set state=$3,vehicle_id=$4,position=$5,mounted_at_km=$6,tracked_km=$7,rotation_baseline_km=$8,retread_count=$9,tread_mm=$10,stock_location=$11,version=version+1,updated_at=now() where company_id=$1 and id=$2`, company, s.ID, s.State, s.Vehicle, s.Position, s.Mounted, s.KM, s.Rotation, s.Retreads, s.Tread, s.Location)
	return e
}
func (a *App) moveTire(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if !canMove(c) {
		problem(w, 403, "A movimentação exige gestor ou borracheiro.")
		return
	}
	tid := chi.URLParam(r, "id")
	var d movementInput
	if !decode(w, r, &d) {
		return
	}
	d.Reason = strings.TrimSpace(d.Reason)
	d.Location = strings.TrimSpace(d.Location)
	d.Supplier = strings.TrimSpace(d.Supplier)
	if uuid.Validate(tid) != nil || uuid.Validate(d.Client) != nil || d.Version < 1 || len(d.Reason) < 5 || len(d.Reason) > 500 || len(d.Location) > 100 || len(d.Supplier) > 150 || d.Cost < 0 || d.Cost > 100000000 {
		problem(w, 422, "Confira identificadores, versão, motivo (5 a 500 caracteres) e custo.")
		return
	}
	payload, _ := json.Marshal(struct {
		Tire string
		Data movementInput
	}{tid, d})
	hash := hashToken(string(payload))
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	ctx := r.Context()
	if e = fleetLock(ctx, t, c.Company); e != nil {
		inspectionError(w, e)
		return
	}
	var storedHash, actor string
	var response json.RawMessage
	e = t.QueryRow(ctx, "select payload_hash,actor_id,response from tire_operations where company_id=$1 and client_id=$2", c.Company, d.Client).Scan(&storedHash, &actor, &response)
	if e == nil {
		if storedHash != hash || actor != c.Subject {
			problem(w, 409, "Identificador já utilizado em outra movimentação.")
			return
		}
		write(w, 200, response)
		return
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		inspectionError(w, e)
		return
	}
	before, e := loadTire(ctx, t, c.Company, tid)
	if errors.Is(e, pgx.ErrNoRows) {
		problem(w, 404, "Pneu não encontrado.")
		return
	}
	if e != nil {
		inspectionError(w, e)
		return
	}
	if before.Version != d.Version {
		problem(w, 409, "O pneu mudou. Atualize o cadastro antes de movimentar.")
		return
	}
	if before.State == "DESCARTADO" {
		problem(w, 409, "Pneu descartado não pode voltar à operação.")
		return
	}
	p, e := policy(ctx, t, c.Company)
	if e != nil {
		inspectionError(w, e)
		return
	}
	after := before
	var source, dest *int
	if before.Vehicle != nil {
		n, err := vehicleOdo(ctx, t, c.Company, *before.Vehicle, d.SourceKM)
		if err != nil {
			inspectionError(w, err)
			return
		}
		source = &n
		if before.Mounted == nil || n < *before.Mounted {
			problem(w, 422, "Odômetro incompatível com a instalação.")
			return
		}
		after.KM += int64(n - *before.Mounted)
	}
	installedAction := d.Action == "INSTALL" || d.Action == "TRANSFER" || d.Action == "ROTATE"
	if installedAction {
		if d.Action == "INSTALL" && before.State != "EM_ESTOQUE" || d.Action != "INSTALL" && before.State != "INSTALADO" {
			problem(w, 409, "Ação incompatível com o estado atual.")
			return
		}
		if before.Damage || before.Tread < p.Critical {
			problem(w, 422, "Pneu com dano ou sulco crítico não pode ser instalado. Encaminhe para avaliação técnica.")
			return
		}
		if uuid.Validate(d.Vehicle) != nil || d.Position == "" {
			problem(w, 422, "Informe veículo e posição de destino.")
			return
		}
		if before.Vehicle != nil && *before.Vehicle == d.Vehicle && before.Position != nil && *before.Position == d.Position {
			problem(w, 422, "O pneu já está nessa posição.")
			return
		}
		if d.Action == "TRANSFER" && before.Vehicle != nil && *before.Vehicle == d.Vehicle {
			problem(w, 422, "Para trocar posições no mesmo veículo, use rodízio.")
			return
		}
		if d.Action == "ROTATE" && (before.Vehicle == nil || *before.Vehicle != d.Vehicle) {
			problem(w, 422, "O rodízio deve ocorrer no mesmo veículo. Para outro veículo, use transferência.")
			return
		}
		if before.Vehicle != nil && *before.Vehicle == d.Vehicle && d.SourceKM != nil && d.DestinationKM != nil && *d.SourceKM != *d.DestinationKM {
			problem(w, 422, "Use a mesma quilometragem na origem e no destino deste veículo.")
			return
		}
		var valid bool
		if e = t.QueryRow(ctx, `select exists(select 1 from vehicles v join axle_templates a on a.id=v.template_id where v.company_id=$1 and v.id=$2 and v.deleted_at is null and a.positions ? $3)`, c.Company, d.Vehicle, d.Position).Scan(&valid); e != nil || !valid {
			problem(w, 422, "Posição inválida para o veículo.")
			return
		}
		n, err := vehicleOdo(ctx, t, c.Company, d.Vehicle, d.DestinationKM)
		if err != nil {
			inspectionError(w, err)
			return
		}
		dest = &n
		var occupied string
		err = t.QueryRow(ctx, "select id from tires where company_id=$1 and vehicle_id=$2 and position=$3 and deleted_at is null", c.Company, d.Vehicle, d.Position).Scan(&occupied)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			inspectionError(w, err)
			return
		}
		if occupied != "" {
			if d.Action != "ROTATE" || d.Swap != occupied || d.SwapVersion < 1 {
				problem(w, 409, "Posição ocupada. No rodízio, confirme a troca entre os dois pneus.")
				return
			}
			peer, err := loadTire(ctx, t, c.Company, occupied)
			if err != nil {
				inspectionError(w, err)
				return
			}
			if peer.Version != d.SwapVersion || peer.Damage || peer.Tread < p.Critical || peer.Mounted == nil || *dest < *peer.Mounted {
				problem(w, 409, "O outro pneu mudou ou precisa de avaliação técnica. Atualize o mapa.")
				return
			}
			peerAfter := peer
			peerAfter.KM += int64(*dest - *peer.Mounted)
			peerAfter.Vehicle = before.Vehicle
			peerAfter.Position = before.Position
			peerAfter.Mounted = source
			peerAfter.Rotation = peerAfter.KM
			// Temporarily release the source slot; both final assignments and both events commit together.
			_, e = t.Exec(ctx, "update tires set state='EM_ESTOQUE',vehicle_id=null,position=null,mounted_at_km=null where company_id=$1 and id=$2", c.Company, tid)
			if e == nil {
				e = saveTire(ctx, t, c.Company, peerAfter)
			}
			if e == nil {
				e = event(ctx, t, c, peer, peerAfter, d.Client, "ROTATE", d.Reason, "", 0, dest, source)
			}
			if e != nil {
				inspectionError(w, e)
				return
			}
		} else if d.Swap != "" {
			problem(w, 409, "O pneu de destino mudou. Atualize o mapa.")
			return
		}
		after.State = "INSTALADO"
		after.Vehicle = &d.Vehicle
		after.Position = &d.Position
		after.Mounted = dest
		after.Rotation = after.KM
	} else {
		if d.Vehicle != "" || d.Position != "" || d.Swap != "" {
			problem(w, 422, "Esta ação não recebe posição de destino.")
			return
		}
		after.Vehicle = nil
		after.Position = nil
		after.Mounted = nil
		switch d.Action {
		case "REMOVE":
			if before.State != "INSTALADO" {
				problem(w, 409, "Somente pneus instalados podem ser retirados.")
				return
			}
			after.State = "EM_ESTOQUE"
		case "STOCK_TRANSFER":
			if before.State != "EM_ESTOQUE" || d.Location == before.Location {
				problem(w, 422, "Selecione um local de estoque diferente para um pneu em estoque.")
				return
			}
			after.State = "EM_ESTOQUE"
		case "RECAP_SEND":
			if before.State != "EM_ESTOQUE" || before.Damage || before.Retreads >= p.MaxRetreads {
				problem(w, 422, "Retire o pneu para estoque e confirme a elegibilidade técnica para recapagem.")
				return
			}
			if d.Supplier == "" {
				problem(w, 422, "Informe o fornecedor da recapagem.")
				return
			}
			after.State = "EM_RECAPAGEM"
		case "RECAP_RETURN":
			if before.State != "EM_RECAPAGEM" || d.Tread == nil || *d.Tread < p.Warning || *d.Tread > 40 || before.Damage {
				problem(w, 422, "Confirme o retorno da recapagem com sulco adequado e sem dano estrutural.")
				return
			}
			after.State = "EM_ESTOQUE"
			after.Tread = *d.Tread
			after.Retreads++
			after.Rotation = after.KM
		case "DISCARD":
			if before.State == "INSTALADO" {
				problem(w, 422, "Retire o pneu do veículo antes de confirmar o descarte.")
				return
			}
			after.State = "DESCARTADO"
		default:
			problem(w, 422, "Movimentação desconhecida.")
			return
		}
		if after.State == "EM_ESTOQUE" && d.Location == "" {
			problem(w, 422, "Informe o local de armazenamento.")
			return
		}
		if d.Location != "" {
			after.Location = d.Location
		}
	}
	if d.Action != "RECAP_RETURN" && d.Tread != nil {
		problem(w, 422, "O novo sulco só é informado no retorno da recapagem; para medição, use inspeção.")
		return
	}
	if d.Action == "RECAP_RETURN" {
		_, e = t.Exec(ctx, "update tires set measurement_cycle_at=now(),latest_inspected_at=null where company_id=$1 and id=$2", c.Company, tid)
		if e != nil {
			inspectionError(w, e)
			return
		}
	}
	if e = saveTire(ctx, t, c.Company, after); e == nil {
		e = event(ctx, t, c, before, after, d.Client, d.Action, d.Reason, d.Supplier, d.Cost, source, dest)
	}
	if e == nil {
		e = a.audit(ctx, t, c, "tire_movement", tid, d)
	}
	if e == nil {
		e = a.refreshMaintenance(ctx, t, c.Company)
	}
	out := map[string]any{"id": tid, "version": before.Version + 1, "state": after.State, "client_id": d.Client}
	raw, _ := json.Marshal(out)
	if e == nil {
		_, e = t.Exec(ctx, "insert into tire_operations(company_id,client_id,actor_id,payload_hash,response) values($1,$2,$3,$4,$5)", c.Company, d.Client, c.Subject, hash, raw)
	}
	if e == nil {
		e = t.Commit(ctx)
	}
	if e != nil {
		inspectionError(w, e)
		return
	}
	write(w, 201, out)
}
func (a *App) tireLifecycle(w http.ResponseWriter, r *http.Request) {
	tid := chi.URLParam(r, "id")
	if uuid.Validate(tid) != nil {
		problem(w, 404, "Pneu não encontrado.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	ctx := r.Context()
	var summary json.RawMessage
	e = t.QueryRow(ctx, `select to_jsonb(t)||jsonb_build_object('plate',v.plate,'current_vehicle_km',v.current_km,'total_tracked_km',t.tracked_km+case when t.vehicle_id is not null then greatest(0,v.current_km-t.mounted_at_km) else 0 end,'movement_cost_cents',coalesce((select sum(cost_cents) from tire_movements m where m.company_id=t.company_id and m.tire_id=t.id),0)) from tires t left join vehicles v on v.company_id=t.company_id and v.id=t.vehicle_id where t.company_id=$1 and t.id=$2 and t.deleted_at is null`, claims(r).Company, tid).Scan(&summary)
	if e != nil {
		problem(w, 404, "Pneu não encontrado.")
		return
	}
	cursor := r.URL.Query().Get("before")
	if cursor != "" && uuid.Validate(cursor) != nil {
		problem(w, 422, "Cursor inválido.")
		return
	}
	movements, e := rowsJSON(ctx, t, `select to_jsonb(m)||jsonb_build_object('actor_name',u.name,'from_plate',fv.plate,'to_plate',tv.plate) from tire_movements m left join users u on u.company_id=m.company_id and u.id=m.actor_id left join vehicles fv on fv.company_id=m.company_id and fv.id=m.from_vehicle_id left join vehicles tv on tv.company_id=m.company_id and tv.id=m.to_vehicle_id where m.company_id=$1 and m.tire_id=$2 and ($3='' or (m.created_at,m.id)<(select created_at,id from tire_movements where company_id=$1 and tire_id=$2 and id=nullif($3,'')::uuid)) order by m.created_at desc,m.id desc limit 51`, claims(r).Company, tid, cursor)
	if e != nil {
		problem(w, 500, "Não foi possível carregar o ciclo de vida.")
		return
	}
	next := ""
	if len(movements) > 50 {
		movements = movements[:50]
		next = movements[49]["id"].(string)
	}
	write(w, 200, map[string]any{"tire": summary, "movements": movements, "next_cursor": next})
}
func (a *App) updateOdometer(w http.ResponseWriter, r *http.Request) {
	if !canMove(claims(r)) {
		problem(w, 403, "Acesso restrito à operação autorizada.")
		return
	}
	vid := chi.URLParam(r, "id")
	if uuid.Validate(vid) != nil {
		problem(w, 404, "Veículo não encontrado.")
		return
	}
	var d struct {
		KM *int `json:"current_km"`
	}
	if !decode(w, r, &d) {
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	ctx := r.Context()
	if e = fleetLock(ctx, t, claims(r).Company); e == nil {
		_, e = vehicleOdo(ctx, t, claims(r).Company, vid, d.KM)
	}
	if e == nil {
		e = a.audit(ctx, t, claims(r), "vehicle_odometer", vid, d)
	}
	if e == nil {
		e = a.refreshMaintenance(ctx, t, claims(r).Company)
	}
	if e == nil {
		e = t.Commit(ctx)
	}
	if e != nil {
		inspectionError(w, e)
		return
	}
	write(w, 200, map[string]any{"current_km": d.KM})
}
