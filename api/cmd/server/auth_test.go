package main

import "testing"

func TestPassword(t *testing.T) {
	h := passwordHash("uma-senha-segura")
	if h == passwordHash("uma-senha-segura") {
		t.Fatal("missing random salt")
	}
	if !passwordOK("uma-senha-segura", h) || passwordOK("outra-senha", h) || passwordOK("x", "invalid") {
		t.Fatal("password verification failed")
	}
}
func TestCNPJ(t *testing.T) {
	if !validCNPJ("11222333000181") || validCNPJ("11111111111111") || validCNPJ("11222333000182") || validCNPJ("") {
		t.Fatal("CNPJ validation failed")
	}
}

func TestAlphanumericCNPJ(t *testing.T) {
	for _, value := range []string{"12ABC34501DE35", "00000000E08G12"} {
		if !validCNPJ(value) {
			t.Fatalf("rejected valid alphanumeric CNPJ: %s", value)
		}
	}
	for _, value := range []string{"12ABC34501DE36", "12ABC34501DE3A", "12abc34501DE35", "12ÁBC34501DE35"} {
		if validCNPJ(value) {
			t.Fatalf("accepted invalid CNPJ: %s", value)
		}
	}
}
