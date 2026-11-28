package commercial

import "testing"

func TestCapabilitiesAndUnavailableModules(t *testing.T) {
	for _, plan := range Definition.Plans {
		caps := Capabilities(plan.ID)
		if !caps["core_operations"] || !caps["essential_reports"] {
			t.Fatal("core missing", plan.ID)
		}
		for _, key := range plan.Planned {
			if caps[key] {
				t.Fatal("unreleased module enabled", plan.ID, key)
			}
		}
	}
	if Capabilities("starter")["advanced_lifecycle"] || Capabilities("starter")["advanced_reports"] || Capabilities("starter")["brand_analytics"] {
		t.Fatal("Starter includes Pro privileges")
	}
	if Capabilities("unknown")["core_operations"] {
		t.Fatal("unknown tier granted privileges")
	}
}
