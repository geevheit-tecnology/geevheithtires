package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"time"
)

type maintenanceAlert struct{ key, kind, tire, message, severity string }

// refreshMaintenance is called inside the same transaction as the triggering change.
// The company lock provides consistent stock counts and serializes alert reconciliation.
func (a *App) refreshMaintenance(ctx context.Context, t pgx.Tx, company string) error {
	if e := fleetLock(ctx, t, company); e != nil {
		return e
	}
	p, e := policy(ctx, t, company)
	if e != nil {
		return e
	}
	rows, e := t.Query(ctx, `select t.id,t.code,t.size,t.state,t.tread_mm,t.structural_damage,t.retread_count,
 t.tracked_km+case when t.vehicle_id is not null and t.position not like 'STEP%' then greatest(0,v.current_km-t.mounted_at_km) else 0 end-t.rotation_baseline_km,
 coalesce(t.latest_inspected_at,t.measurement_cycle_at,t.created_at),i.pressure_psi,i.temperature_c
 from tires t left join vehicles v on v.company_id=t.company_id and v.id=t.vehicle_id
 left join lateral(select pressure_psi,temperature_c from inspections where company_id=t.company_id and tire_id=t.id and inspected_at>=t.measurement_cycle_at order by inspected_at desc,id desc limit 1) i on true
 where t.company_id=$1 and t.deleted_at is null `, company)
	if e != nil {
		return e
	}
	desired := []maintenanceAlert{}
	stock := map[string]int{}
	now := time.Now()
	for rows.Next() {
		var tid, code, size, state string
		var tread float64
		var damage bool
		var retreads int
		var sinceRotation int64
		var last time.Time
		var pressure, temperature *float64
		if e = rows.Scan(&tid, &code, &size, &state, &tread, &damage, &retreads, &sinceRotation, &last, &pressure, &temperature); e != nil {
			rows.Close()
			return e
		}
		if _, ok := stock[size]; !ok {
			stock[size] = 0
		}
		if state == "EM_ESTOQUE" && !damage && tread >= p.Critical {
			stock[size]++
		}
		add := func(kind, msg, severity string) {
			desired = append(desired, maintenanceAlert{kind + ":" + tid, kind, tid, code + ": " + msg, severity})
		}
		if state == "EM_RECAPAGEM" || state == "DESCARTADO" {
			continue
		}
		if damage {
			add("wear", "dano estrutural registrado. Retirar de operação e solicitar avaliação técnica imediata.", "critico")
			add("disposal", "avaliar a carcaça e a necessidade de descarte. O sistema não autoriza descarte automático.", "critico")
		} else if tread < p.Critical {
			add("wear", fmt.Sprintf("sulco %.1f mm abaixo do limite operacional configurado (%.1f mm). Providenciar retirada e avaliação.", tread, p.Critical), "critico")
		} else if tread < p.Warning {
			add("wear", fmt.Sprintf("sulco %.1f mm próximo da intervenção (aviso em %.1f mm). Planejar troca.", tread, p.Warning), "alerta")
		}
		if !damage && tread < p.Warning {
			if retreads < p.MaxRetreads {
				add("retread", "avaliar elegibilidade da carcaça para recapagem; depende de confirmação técnica.", "alerta")
			} else {
				add("disposal", "limite de recapagens configurado atingido e sulco reduzido. Avaliar retirada e destinação técnica.", "alerta")
			}
		}
		if state == "INSTALADO" && sinceRotation >= p.Rotation {
			add("rotation", fmt.Sprintf("rodízio previsto após %d km desde a instalação/último rodízio (intervalo %d km).", sinceRotation, p.Rotation), "alerta")
		}
		if now.Sub(last) >= time.Duration(p.InspectionDays)*24*time.Hour {
			add("inspection_due", fmt.Sprintf("inspeção vencida; periodicidade configurada de %d dias. Registrar novas medições.", p.InspectionDays), "alerta")
		}
		if pressure != nil && (*pressure < 90 || *pressure > 120) {
			add("pressure", fmt.Sprintf("pressão %.1f psi fora da referência inicial de 90–120 psi. Conferir a especificação do fabricante e corrigir.", *pressure), "alerta")
		}
		if temperature != nil && *temperature >= 85 {
			add("temperature", fmt.Sprintf("temperatura %.1f °C elevada. Solicitar verificação imediata.", *temperature), "critico")
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for size, n := range stock {
		if n < p.MinimumStock {
			desired = append(desired, maintenanceAlert{"stock:" + size, "stock_low", "", fmt.Sprintf("Estoque baixo de %s: %d pneu(s) disponível(is), mínimo configurado %d. Planejar reposição.", size, n, p.MinimumStock), "alerta"})
		}
	}
	active := make([]string, 0, len(desired))
	for _, item := range desired {
		active = append(active, item.key)
		var tid any
		if item.tire != "" {
			tid = item.tire
		}
		_, e = t.Exec(ctx, `insert into alerts(id,company_id,tire_id,message,severity,kind,rule_key) values($1,$2,$3,$4,$5,$6,$7)
  on conflict(company_id,rule_key) where resolved_at is null and rule_key is not null do update set message=excluded.message,severity=excluded.severity,updated_at=now(),read=case when alerts.severity<>excluded.severity then false else alerts.read end`, id(), company, tid, item.message, item.severity, item.kind, item.key)
		if e != nil {
			return e
		}
	}
	_, e = t.Exec(ctx, "update alerts set resolved_at=now(),read=true,updated_at=now() where company_id=$1 and resolved_at is null and rule_key is not null and not(rule_key=any($2::text[]))", company, active)
	return e
}
func (a *App) maintenanceSweep(ctx context.Context) {
	rows, e := a.db.Query(ctx, "select id from maintenance_companies()")
	if e != nil {
		slog.Warn("maintenance tenant lookup failed")
		return
	}
	companies := []string{}
	for rows.Next() {
		var company string
		if e = rows.Scan(&company); e != nil {
			break
		}
		companies = append(companies, company)
	}
	rowErr := rows.Err()
	rows.Close()
	if e != nil || rowErr != nil {
		return
	}
	for _, company := range companies {
		if ctx.Err() != nil {
			return
		}
		func() {
			work, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()
			t, err := a.db.Begin(work)
			if err != nil {
				return
			}
			defer t.Rollback(work)
			_, err = t.Exec(work, "select set_config('app.company_id',$1,true)", company)
			if err == nil {
				err = a.refreshMaintenance(work, t, company)
			}
			if err == nil {
				err = t.Commit(work)
			}
			if err != nil {
				slog.Warn("maintenance reconciliation failed", "company_id", company)
			}
		}()
	}
}
func (a *App) maintenanceWorker(ctx context.Context) {
	a.maintenanceSweep(ctx)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.maintenanceSweep(ctx)
		}
	}
}
