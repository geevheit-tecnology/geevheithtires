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
