package main

import "testing"

func TestRemovalReportValidation(t *testing.T) {
	for _, tc := range []struct {
		action, carcass, cause, assessment string
		valid                              bool
	}{
		{"REMOVE", "PRESERVADA", "PREVENTIVA", "Avaliação da carcaça preservada.", true},
		{"REMOVE", "PERDIDA", "ESTOURO", "Estouro com perda da carcaça.", true},
		{"DISCARD", "PERDIDA", "CORTE", "Corte profundo na estrutura.", true},
		{"DISCARD", "PRESERVADA", "CORTE", "Corte profundo na estrutura.", false},
		{"REMOVE", "PERDIDA", "INVALIDO", "Dano na estrutura da carcaça.", false},
		{"REMOVE", "PERDIDA", "ESTOURO", "curto", false},
		{"INSTALL", "PERDIDA", "ESTOURO", "Dano na estrutura da carcaça.", false},
	} {
		d := movementInput{Action: tc.action, Report: &removalReport{Cause: tc.cause, Carcass: tc.carcass, Assessment: tc.assessment, Responsible: "Técnico QA"}}
		if d.validateReport() != tc.valid {
			t.Errorf("%+v", tc)
		}
	}
	for _, action := range []string{"REMOVE", "DISCARD"} {
		d := movementInput{Action: action}
		if d.validateReport() {
			t.Errorf("%s accepted missing report", action)
		}
	}
}
