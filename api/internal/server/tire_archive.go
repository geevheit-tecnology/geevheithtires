package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (a *App) archiveTire(w http.ResponseWriter, r *http.Request) {
	c := claims(r)
	if !canManage(c) {
		problem(w, 403, "A exclusão de pneus exige acesso de gestão.")
		return
	}
	tid := chi.URLParam(r, "id")
	if uuid.Validate(tid) != nil {
		problem(w, 404, "Pneu não encontrado.")
		return
	}
	var input struct {
		Version int64 `json:"expected_version"`
	}
	if !decode(w, r, &input) {
		return
	}
	ctx := r.Context()
	tx, err := a.tx(r)
	if err != nil {
		inspectionError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if err = fleetLock(ctx, tx, c.Company); err != nil {
		inspectionError(w, err)
		return
	}
	result, err := tx.Exec(ctx, `update tires set deleted_at=now(), updated_at=now(), version=version+1
		where company_id=$1 and id=$2 and version=$3 and deleted_at is null and vehicle_id is null`, c.Company, tid, input.Version)
	if err != nil {
		inspectionError(w, err)
		return
	}
	if result.RowsAffected() != 1 {
		problem(w, 409, "Retire o pneu do veículo ou atualize os dados antes de excluir.")
		return
	}
	if err = a.audit(ctx, tx, c, "tire_archived", tid, map[string]any{"version": input.Version}); err == nil {
		err = a.refreshMaintenance(ctx, tx, c.Company)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		inspectionError(w, err)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
