package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type removalReport struct {
	Cause       string `json:"cause"`
	Carcass     string `json:"carcass"`
	Assessment  string `json:"assessment"`
	Responsible string `json:"responsible"`
}

func (d *movementInput) validateReport() bool {
	if d.Action != "REMOVE" && d.Action != "DISCARD" {
		return d.Report == nil
	}
	if d.Report == nil {
		return false
	}
	r := d.Report
	r.Assessment = strings.TrimSpace(r.Assessment)
	r.Responsible = strings.TrimSpace(r.Responsible)
	validCause := r.Cause == "DESGASTE" || r.Cause == "ESTOURO" || r.Cause == "CORTE" || r.Cause == "IMPACTO" || r.Cause == "SEPARACAO" || r.Cause == "PREVENTIVA" || r.Cause == "OUTRO"
	validCarcass := r.Carcass == "PRESERVADA" || r.Carcass == "PERDIDA"
	return validCause && validCarcass && len(r.Assessment) >= 10 && len(r.Assessment) <= 3000 && len(r.Responsible) >= 2 && len(r.Responsible) <= 150 && (d.Action != "DISCARD" || r.Carcass == "PERDIDA")
}

func (a *App) tireReports(w http.ResponseWriter, r *http.Request) {
	if !canMove(claims(r)) {
		problem(w, 403, "Relatórios restritos à gestão e ao borracheiro.")
		return
	}
	access, e := a.resolveEntitlements(r)
	if e != nil {
		problem(w, 503, "Não foi possível verificar os recursos do plano.")
		return
	}
	q := r.URL.Query()
	if !access.Features["advanced_reports"] && (q.Get("from") != "" || q.Get("to") != "" || q.Get("before") != "") {
		featureProblem(w, "advanced_reports")
		return
	}
	start, end := q.Get("from"), q.Get("to")
	for _, v := range []string{start, end} {
		if v != "" {
			if _, e := time.Parse("2006-01-02", v); e != nil {
				problem(w, 422, "Período inválido.")
				return
			}
		}
	}
	if start != "" && end != "" && start > end {
		problem(w, 422, "A data inicial deve preceder a final.")
		return
	}
	kind, cursor := q.Get("kind"), q.Get("before")
	if kind != "" && kind != "DISCARD" && kind != "RECAP_SEND" && kind != "RECAP_RETURN" && kind != "REMOVE" {
		problem(w, 422, "Tipo de relatório inválido.")
		return
	}
	if cursor != "" && uuid.Validate(cursor) != nil {
		problem(w, 422, "Cursor inválido.")
		return
	}
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	company := claims(r).Company
	// Comparison is a current fleet snapshot, grouped by equivalent application.
	brands := []map[string]any{}
	ranking := []map[string]any{}
	if access.Features["brand_analytics"] {
		brands, e = rowsJSON(r.Context(), t, `with assets as (
 select t.*,coalesce((select count(*) from tire_movements m where m.company_id=t.company_id and m.tire_id=t.id and m.removal_report->>'carcass'='PERDIDA'),0) as losses,coalesce((select count(*) from tire_movements m where m.company_id=t.company_id and m.tire_id=t.id and m.removal_report->>'cause'='ESTOURO'),0) as bursts,t.tracked_km+case when t.state='INSTALADO' and t.position not like 'STEP%' then greatest(0,v.current_km-t.mounted_at_km) else 0 end as km,
 coalesce((select sum(m.cost_cents) from tire_movements m where m.company_id=t.company_id and m.tire_id=t.id),0)/100.0+t.acquisition_cost as cost
 from tires t left join vehicles v on v.company_id=t.company_id and v.id=t.vehicle_id where t.company_id=$1 and t.deleted_at is null
 ) select jsonb_build_object('brand',upper(trim(brand)),'model',upper(trim(model)),'size',upper(trim(size)),'road_use',road_use,'tires',count(*),'closed',count(*) filter(where state='DESCARTADO'),'closed_samples',count(*) filter(where state='DESCARTADO' and km>0),'carcass_losses',coalesce(sum(losses),0),'bursts',coalesce(sum(bursts),0),'tracked_km',sum(km),'closed_avg_km',avg(km) filter(where state='DESCARTADO' and km>0),'cost_per_km',sum(cost) filter(where km>0)/nullif(sum(km) filter(where km>0),0),'cost_samples',count(*) filter(where km>0),'retreads',sum(retread_count-initial_retread_count),'unknown_entry',count(*) filter(where initial_retread_count>0)) from assets group by upper(trim(brand)),upper(trim(model)),upper(trim(size)),road_use order by upper(trim(size)),road_use,upper(trim(brand))`, company)
		if e != nil {
			problem(w, 500, "Não foi possível comparar as marcas.")
			return
		}
	}
	if access.Features["brand_analytics"] {
		ranking, e = rowsJSON(r.Context(), t, `with assets as (
        select t.id,t.code,t.brand,t.model,t.size,t.road_use,t.state,t.initial_retread_count,
        t.tracked_km+case when t.state='INSTALADO' and t.position not like 'STEP%' then greatest(0,v.current_km-t.mounted_at_km) else 0 end as km,
        t.acquisition_cost+coalesce((select sum(m.cost_cents)/100.0 from tire_movements m where m.company_id=t.company_id and m.tire_id=t.id),0) as cost
        from tires t left join vehicles v on v.company_id=t.company_id and v.id=t.vehicle_id where t.company_id=$1 and t.deleted_at is null
      ) select to_jsonb(a)||jsonb_build_object('cost_per_km',cost/nullif(km,0),'rank',dense_rank() over(partition by upper(trim(size)),road_use order by cost/km)) from assets a where km>=1000 and cost>0 and initial_retread_count=0 order by upper(trim(size)),road_use,cost/km,code limit 100`, company)
		if e != nil {
			problem(w, 500, "Não foi possível carregar o ranking de pneus.")
			return
		}
	}

	events, e := rowsJSON(r.Context(), t, `select to_jsonb(m)||jsonb_build_object('code',t.code,'serial_number',t.serial_number,'brand',t.brand,'size',t.size,'actor_name',u.name,'plate',v.plate) from tire_movements m join tires t on t.company_id=m.company_id and t.id=m.tire_id left join users u on u.company_id=m.company_id and u.id=m.actor_id left join vehicles v on v.company_id=m.company_id and v.id=m.from_vehicle_id where m.company_id=$1 and m.action in ('REMOVE','DISCARD','RECAP_SEND','RECAP_RETURN') and ($2='' or m.created_at>=nullif($2,'')::date) and ($3='' or m.created_at<(nullif($3,'')::date+interval '1 day')) and ($4='' or case when $4='DISCARD' then m.to_state='DESCARTADO' else m.action=$4 end) and ($5='' or (m.created_at,m.id)<(select created_at,id from tire_movements where company_id=$1 and id=nullif($5,'')::uuid)) order by m.created_at desc,m.id desc limit 51`, company, start, end, kind, cursor)
	if e != nil {
		problem(w, 500, "Não foi possível carregar os relatórios.")
		return
	}
	next := ""
	if len(events) > 50 {
		events = events[:50]
		if access.Features["advanced_reports"] {
			next = events[49]["id"].(string)
		}
	}
	write(w, 200, map[string]any{"brands": brands, "ranking": ranking, "events": events, "next_cursor": next, "limited": !access.Features["advanced_reports"], "can_compare_brands": access.Features["brand_analytics"]})
}
