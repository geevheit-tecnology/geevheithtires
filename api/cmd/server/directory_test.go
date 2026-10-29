package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func fakeDirectory(t *testing.T) *publicDirectory {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/cnpj/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"cnpj": strings.TrimPrefix(r.URL.Path, "/cnpj/"), "razao_social": "Empresa QA LTDA", "nome_fantasia": "Frota QA", "situacao_cadastral": 2, "descricao_situacao_cadastral": "ATIVA", "cep": "01001000", "logradouro": "Praça da Sé", "numero": "10", "bairro": "Sé", "municipio": "São Paulo", "uf": "SP"})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{"cep": "01001-000", "logradouro": "Praça da Sé", "bairro": "Sé", "localidade": "São Paulo", "uf": "SP"})
		}
	}))
	t.Cleanup(server.Close)
	d := newPublicDirectory()
	d.client = server.Client()
	d.cnpjBase = server.URL + "/cnpj"
	d.cepBase = server.URL + "/cep"
	return d
}
func TestDirectoryLookupAndCache(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"cnpj":"00000000000191","razao_social":"Empresa de teste","situacao_cadastral":2,"descricao_situacao_cadastral":"ATIVA","qsa":[{"nome_socio":"not exposed"}]}`)
	}))
	defer server.Close()
	d := newPublicDirectory()
	d.client = server.Client()
	d.cnpjBase = server.URL
	for i := 0; i < 2; i++ {
		out, e := d.cnpj(context.Background(), "00.000.000/0001-91")
		if e != nil || !out.Active || out.LegalName != "Empresa de teste" || out.CheckedAt.IsZero() {
			t.Fatalf("lookup: %+v %v", out, e)
		}
		payload, _ := json.Marshal(out)
		if bytes.Contains(payload, []byte("nome_socio")) {
			t.Fatal("unnecessary personal data exposed")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("expected cached lookup, got %d requests", calls.Load())
	}
	if _, e := d.cnpj(context.Background(), "00000000000000"); e == nil {
		t.Fatal("invalid CNPJ accepted")
	}
	if calls.Load() != 1 {
		t.Fatal("invalid CNPJ reached provider")
	}
}
func TestDirectoryCEPAndMissingRecords(t *testing.T) {
	d := fakeDirectory(t)
	out, e := d.cep(context.Background(), "01001000")
	if e != nil || out.City != "São Paulo" || out.State != "SP" {
		t.Fatalf("CEP: %+v %v", out, e)
	}
	for _, value := range []string{"01001-000", "123", "1234567A"} {
		if _, e := d.cep(context.Background(), value); e == nil {
			t.Fatal("invalid CEP accepted")
		}
	}
	for _, body := range []string{`{"erro":true}`, `{"erro":"true"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		missing := newPublicDirectory()
		missing.client = server.Client()
		missing.cepBase = server.URL
		_, e := missing.cep(context.Background(), "99999999")
		server.Close()
		if e == nil {
			t.Fatal("missing CEP accepted")
		}
	}
}
func TestRegistrationRequiresActiveRegistryEvenWithoutBrowser(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   int
	}{
		{"inactive", 200, `{"cnpj":"00000000000191","razao_social":"Empresa QA","situacao_cadastral":8,"descricao_situacao_cadastral":"BAIXADA"}`, 422},
		{"missing", 404, `{}`, 404},
		{"provider unavailable", 503, `{}`, 503},
		{"provider rate limit", 429, `{}`, 503},
		{"wrong company", 200, `{"cnpj":"99999999999999","razao_social":"Outro cadastro","situacao_cadastral":2}`, 503},
		{"malformed", 200, `not-json`, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			d := newPublicDirectory()
			d.client = server.Client()
			d.cnpjBase = server.URL
			// No database configured: rejection must happen before opening a transaction.
			a := &App{directory: d}
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"name":"Gestor QA","email":"qa@example.com","password":"password-qa-12345","company":"Empresa QA","cnpj":"00000000000191"}`))
			w := httptest.NewRecorder()
			a.register(w, req)
			if w.Code != tc.want {
				t.Fatalf("expected %d got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}
func TestAddressValidation(t *testing.T) {
	valid := &companyAddress{PostalCode: "01001-000", State: "sp", Number: "S/N"}
	if e := checkedAddress(valid); e != nil || valid.PostalCode != "01001000" || valid.State != "SP" {
		t.Fatalf("address normalization: %+v %v", valid, e)
	}
	for _, address := range []*companyAddress{{PostalCode: "123", State: "SP"}, {PostalCode: "01001000", State: "XX"}, {PostalCode: "01001000", State: "SP", Street: strings.Repeat("x", 201)}} {
		if checkedAddress(address) == nil {
			t.Fatal("invalid address accepted")
		}
	}
}
