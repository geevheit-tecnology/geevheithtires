package server

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var photoDataPattern = regexp.MustCompile(`^data:image/(webp|jpeg);base64,[A-Za-z0-9+/=]+$`)

func validPhotoDataURL(s string, maxLen int) bool {
	if s == "" {
		return true
	}
	if len(s) > maxLen {
		return false
	}
	return photoDataPattern.MatchString(s)
}

func validOptionalPressure(p *float64) bool {
	if p == nil {
		return true
	}
	return *p >= 0 && *p <= 250
}

func cleanOptionalText(s string, maxLen int) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return "", false
	}
	return s, true
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (a *App) updateTireMedia(w http.ResponseWriter, r *http.Request) {
	if !canMove(claims(r)) {
		problem(w, 403, "Acesso restrito à operação autorizada.")
		return
	}
	tid := chi.URLParam(r, "id")
	if uuid.Validate(tid) != nil {
		problem(w, 404, "Pneu não encontrado.")
		return
	}
	var d struct {
		Version          int64    `json:"expected_version"`
		PhotoData        *string  `json:"photo_data"`
		PhotoThumb       *string  `json:"photo_thumb"`
		PressurePSI      *float64 `json:"pressure_psi"`
		PurchaseSupplier *string  `json:"purchase_supplier"`
		Notes            *string  `json:"notes"`
	}
	if !decode(w, r, &d) {
		return
	}
	if d.Version < 1 || !validOptionalPressure(d.PressurePSI) {
		problem(w, 422, "Confira a versão e a pressão informada (0 a 250 PSI).")
		return
	}
	if d.PhotoData != nil && !validPhotoDataURL(*d.PhotoData, 200000) {
		problem(w, 422, "Foto inválida ou acima do limite otimizado (máx. 150 KB em WebP/JPEG).")
		return
	}
	if d.PhotoThumb != nil && !validPhotoDataURL(*d.PhotoThumb, 20000) {
		problem(w, 422, "Miniatura da foto inválida ou muito grande.")
		return
	}
	var supplier *string
	if d.PurchaseSupplier != nil {
		clean, ok := cleanOptionalText(*d.PurchaseSupplier, 150)
		if !ok {
			problem(w, 422, "Fornecedor deve ter no máximo 150 caracteres.")
			return
		}
		supplier = &clean
	}
	var notes *string
	if d.Notes != nil {
		clean, ok := cleanOptionalText(*d.Notes, 500)
		if !ok {
			problem(w, 422, "Observações devem ter no máximo 500 caracteres.")
			return
		}
		notes = &clean
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

	var photoDataVal any
	if d.PhotoData != nil {
		photoDataVal = nullableString(*d.PhotoData)
	}
	var photoThumbVal any
	if d.PhotoThumb != nil {
		photoThumbVal = nullableString(*d.PhotoThumb)
	}
	var supplierVal any
	if supplier != nil {
		supplierVal = nullableString(*supplier)
	}
	var notesVal any
	if notes != nil {
		notesVal = nullableString(*notes)
	}

	result, e := t.Exec(
		r.Context(),
		`update tires set
			photo_data = case when $4::boolean then $5 else photo_data end,
			photo_thumb = case when $6::boolean then $7 else photo_thumb end,
			pressure_psi = case when $8::boolean then $9 else pressure_psi end,
			purchase_supplier = case when $10::boolean then $11 else purchase_supplier end,
			notes = case when $12::boolean then $13 else notes end,
			version = version + 1,
			updated_at = now()
		where company_id = $1 and id = $2 and version = $3 and deleted_at is null`,
		claims(r).Company,
		tid,
		d.Version,
		d.PhotoData != nil, photoDataVal,
		d.PhotoThumb != nil, photoThumbVal,
		d.PressurePSI != nil, d.PressurePSI,
		supplier != nil, supplierVal,
		notes != nil, notesVal,
	)
	if e != nil {
		problem(w, 422, "Não foi possível atualizar a foto ou os campos do pneu.")
		return
	}
	if result.RowsAffected() != 1 {
		problem(w, 409, "Pneu não encontrado ou alterado. Atualize antes de salvar.")
		return
	}
	if e = a.audit(r.Context(), t, claims(r), "tire_media", tid, map[string]any{
		"has_photo":         d.PhotoData != nil && *d.PhotoData != "",
		"pressure_psi":      d.PressurePSI,
		"purchase_supplier": supplier,
	}); e != nil {
		inspectionError(w, e)
		return
	}
	if e = t.Commit(r.Context()); e != nil {
		inspectionError(w, e)
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
