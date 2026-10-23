package main

import "net/http"

// vehicleConfigurations shares the persisted wheel positions with every client.
func (a *App) vehicleConfigurations(w http.ResponseWriter, r *http.Request) {
	t, err := a.tx(r)
	if err != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	data, err := rowsJSON(r.Context(), t, "select to_jsonb(a) from axle_templates a order by category, id")
	if err != nil {
		problem(w, 503, "Não foi possível carregar as configurações de veículos.")
		return
	}
	write(w, 200, map[string]any{"data": data})
}
