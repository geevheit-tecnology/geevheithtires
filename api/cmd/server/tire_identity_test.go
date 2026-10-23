package main

import (
	"testing"
	"time"
)

func TestTireIdentityValidation(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	valid := tireIdentityInput{Code: " f001 ", Serial: " s001 ", DOT: "DOT AB 1025", Expiry: "2030-10-23", Road: "MISTO"}
	if !valid.validate(now) || valid.Code != "F001" || valid.Serial != "S001" || valid.DOT != "AB1025" {
		t.Fatal("valid identity was not normalized")
	}
	for _, dot := range []string{"0025", "5425", "5325", "0127", "5226", "ABCD", ""} {
		d := valid
		d.DOT = dot
		if d.validate(now) {
			t.Errorf("accepted invalid/future DOT %q", dot)
		}
	}
	for _, expiry := range []string{"", "2030-02-30", "2020-01-01"} {
		d := valid
		d.Expiry = expiry
		if d.validate(now) {
			t.Errorf("accepted invalid expiry %q", expiry)
		}
	}
	d := valid
	d.Road = "OUTRO"
	if d.validate(now) {
		t.Fatal("accepted invalid road")
	}
	d = valid
	d.Serial = ""
	if d.validate(now) {
		t.Fatal("accepted missing serial")
	}
	d = valid
	d.Expiry = "2026-01-01"
	if !d.validate(now) {
		t.Fatal("expired intake for stock must remain possible")
	}
}
