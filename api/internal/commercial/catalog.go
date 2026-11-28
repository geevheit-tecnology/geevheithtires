package commercial

import (
	_ "embed"
	"encoding/json"
)

//go:embed catalog.json
var source []byte

type Feature struct {
	Name      string `json:"name"`
	Benefit   string `json:"benefit"`
	Available bool   `json:"available"`
}
type Plan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Summary      string   `json:"summary"`
	VehicleLimit *int     `json:"vehicle_limit"`
	UserLimit    *int     `json:"user_limit"`
	Features     []string `json:"features"`
	Planned      []string `json:"planned"`
}
type Catalog struct {
	Features map[string]Feature `json:"features"`
	Plans    []Plan             `json:"plans"`
}

var Definition = func() Catalog {
	var c Catalog
	if err := json.Unmarshal(source, &c); err != nil {
		panic(err)
	}
	return c
}()

func Find(id string) (Plan, bool) {
	for _, p := range Definition.Plans {
		if p.ID == id {
			return p, true
		}
	}
	return Plan{}, false
}
func Capabilities(id string) map[string]bool {
	out := make(map[string]bool, len(Definition.Features))
	for key := range Definition.Features {
		out[key] = false
	}
	p, ok := Find(id)
	if !ok {
		return out
	}
	for _, key := range p.Features {
		out[key] = Definition.Features[key].Available
	}
	return out
}
