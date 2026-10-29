package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type companyAddress struct {
	PostalCode string `json:"postal_code"`
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement"`
	District   string `json:"district"`
	City       string `json:"city"`
	State      string `json:"state"`
}
type companyRegistration struct {
	CNPJ       string         `json:"cnpj"`
	LegalName  string         `json:"legal_name"`
	TradeName  string         `json:"trade_name"`
	Status     string         `json:"status"`
	StatusCode int            `json:"status_code"`
	Active     bool           `json:"active"`
	Address    companyAddress `json:"address"`
	Source     string         `json:"source"`
	CheckedAt  time.Time      `json:"checked_at"`
}
type directoryError struct {
	status  int
	message string
}

func (e *directoryError) Error() string { return e.message }

type directoryEntry struct {
	value any
	until time.Time
}
type publicDirectory struct {
	client            *http.Client
	cnpjBase, cepBase string
	mu                sync.Mutex
	cache             map[string]directoryEntry
}

func newPublicDirectory() *publicDirectory {
	return &publicDirectory{
		client:   &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		cnpjBase: strings.TrimRight(env("CNPJ_API_BASE_URL", "https://brasilapi.com.br/api/cnpj/v1"), "/"),
		cepBase:  strings.TrimRight(env("CEP_API_BASE_URL", "https://viacep.com.br/ws"), "/"),
		cache:    make(map[string]directoryEntry),
	}
}
func (a *App) companyDirectory() *publicDirectory {
	if a.directory != nil {
		return a.directory
	}
	return newPublicDirectory()
}
func normalizedCNPJ(value string) string {
	return regexp.MustCompile(`[./\s-]`).ReplaceAllString(strings.ToUpper(value), "")
}
func digits(value string) string { return regexp.MustCompile(`[^0-9]`).ReplaceAllString(value, "") }
func (d *publicDirectory) cached(key string) (any, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.cache[key]
	return e.value, ok && time.Now().Before(e.until)
}
func (d *publicDirectory) remember(key string, value any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cache == nil {
		d.cache = make(map[string]directoryEntry)
	}
	if len(d.cache) >= 256 {
		for k := range d.cache {
			delete(d.cache, k)
			break
		}
	}
	d.cache[key] = directoryEntry{value, time.Now().Add(15 * time.Minute)}
}
func (d *publicDirectory) get(ctx context.Context, url string, out any) error {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GeevheitTire/1.0")
	resp, e := d.client.Do(req)
	if e != nil {
		return &directoryError{503, "Serviço de consulta indisponível. Tente novamente em instantes."}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return &directoryError{404, "Cadastro não encontrado na base de consulta."}
	}
	if resp.StatusCode != 200 {
		return &directoryError{503, "Serviço de consulta indisponível. Tente novamente em instantes."}
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out) != nil {
		return &directoryError{503, "O serviço de consulta retornou dados incompletos. Tente novamente."}
	}
	return nil
}
func (d *publicDirectory) cnpj(ctx context.Context, value string) (companyRegistration, error) {
	value = normalizedCNPJ(value)
	if !validCNPJ(value) {
		return companyRegistration{}, &directoryError{422, "Informe um CNPJ válido."}
	}
	if !regexp.MustCompile(`^[0-9]{14}$`).MatchString(value) {
		return companyRegistration{}, &directoryError{503, "A base de consulta ainda não oferece este formato de CNPJ. Entre em contato para validação."}
	}
	if v, ok := d.cached("cnpj:" + value); ok {
		return v.(companyRegistration), nil
	}
	var raw struct {
		CNPJ       string `json:"cnpj"`
		LegalName  string `json:"razao_social"`
		TradeName  string `json:"nome_fantasia"`
		Code       int    `json:"situacao_cadastral"`
		Status     string `json:"descricao_situacao_cadastral"`
		CEP        string `json:"cep"`
		Street     string `json:"logradouro"`
		StreetType string `json:"descricao_tipo_de_logradouro"`
		Number     string `json:"numero"`
		Complement string `json:"complemento"`
		District   string `json:"bairro"`
		City       string `json:"municipio"`
		State      string `json:"uf"`
	}
	if e := d.get(ctx, d.cnpjBase+"/"+value, &raw); e != nil {
		return companyRegistration{}, e
	}
	if normalizedCNPJ(raw.CNPJ) != value || strings.TrimSpace(raw.LegalName) == "" || raw.Code < 1 {
		return companyRegistration{}, &directoryError{503, "O serviço retornou um cadastro incompleto. Tente novamente."}
	}
	out := companyRegistration{CNPJ: value, LegalName: raw.LegalName, TradeName: raw.TradeName, Status: raw.Status, StatusCode: raw.Code, Active: raw.Code == 2, Source: "BrasilAPI", CheckedAt: time.Now().UTC(), Address: companyAddress{PostalCode: digits(raw.CEP), Street: strings.TrimSpace(raw.StreetType + " " + raw.Street), Number: raw.Number, Complement: raw.Complement, District: raw.District, City: raw.City, State: raw.State}}
	d.remember("cnpj:"+value, out)
	return out, nil
}
func (d *publicDirectory) cep(ctx context.Context, value string) (companyAddress, error) {
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(value) {
		return companyAddress{}, &directoryError{422, "Informe um CEP com 8 dígitos."}
	}
	if v, ok := d.cached("cep:" + value); ok {
		return v.(companyAddress), nil
	}
	var raw struct {
		Error    any    `json:"erro"`
		CEP      string `json:"cep"`
		Street   string `json:"logradouro"`
		District string `json:"bairro"`
		City     string `json:"localidade"`
		State    string `json:"uf"`
	}
	if e := d.get(ctx, d.cepBase+"/"+value+"/json/", &raw); e != nil {
		return companyAddress{}, e
	}
	if raw.Error == true || raw.Error == "true" {
		return companyAddress{}, &directoryError{404, "CEP não encontrado."}
	}
	if digits(raw.CEP) != value || raw.City == "" || !validState(raw.State) {
		return companyAddress{}, &directoryError{503, "A consulta retornou um endereço incompleto. Tente novamente."}
	}
	out := companyAddress{PostalCode: value, Street: raw.Street, District: raw.District, City: raw.City, State: raw.State}
	d.remember("cep:"+value, out)
	return out, nil
}
func directoryProblem(w http.ResponseWriter, e error) {
	var d *directoryError
	if errors.As(e, &d) {
		problem(w, d.status, d.message)
	} else {
		problem(w, 503, "Não foi possível consultar o cadastro. Tente novamente.")
	}
}
func (a *App) lookupCNPJ(w http.ResponseWriter, r *http.Request) {
	out, e := a.companyDirectory().cnpj(r.Context(), chi.URLParam(r, "cnpj"))
	if e != nil {
		directoryProblem(w, e)
		return
	}
	write(w, 200, out)
}
func (a *App) lookupCEP(w http.ResponseWriter, r *http.Request) {
	out, e := a.companyDirectory().cep(r.Context(), chi.URLParam(r, "cep"))
	if e != nil {
		directoryProblem(w, e)
		return
	}
	write(w, 200, out)
}
func validState(value string) bool {
	return strings.Contains("|AC|AL|AP|AM|BA|CE|DF|ES|GO|MA|MT|MS|MG|PA|PB|PR|PE|PI|RJ|RN|RS|RO|RR|SC|SP|SE|TO|", "|"+value+"|") && len(value) == 2
}
func checkedAddress(value *companyAddress) error {
	if value == nil {
		return nil
	}
	value.PostalCode = strings.ReplaceAll(strings.TrimSpace(value.PostalCode), "-", "")
	value.State = strings.ToUpper(strings.TrimSpace(value.State))
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(value.PostalCode) || !validState(value.State) {
		return &directoryError{422, "Confira CEP e estado do endereço."}
	}
	for _, s := range []string{value.Street, value.Number, value.Complement, value.District, value.City} {
		if len(s) > 200 {
			return &directoryError{422, "O endereço contém um campo muito longo."}
		}
	}
	return nil
}
func (a *App) verifiedCompany(ctx context.Context, value string) (companyRegistration, error) {
	out, e := a.companyDirectory().cnpj(ctx, value)
	if e != nil {
		return out, e
	}
	if !out.Active {
		return out, &directoryError{422, "O CNPJ não está com situação cadastral ativa. Confira os dados da empresa."}
	}
	return out, nil
}
func insertCompany(ctx context.Context, t pgx.Tx, companyID, name, cnpj string, address *companyAddress, registration companyRegistration) error {
	if address == nil {
		address = &registration.Address
	}
	data, e := json.Marshal(address)
	if e != nil {
		return e
	}
	_, e = t.Exec(ctx, "insert into companies(id,name,cnpj,legal_name,cnpj_registration_status,cnpj_verified_at,cnpj_verification_source,address) values($1,$2,$3,$4,$5,$6,$7,$8)", companyID, strings.TrimSpace(name), cnpj, registration.LegalName, registration.StatusCode, registration.CheckedAt, registration.Source, data)
	return e
}
func (a *App) companyProfile(w http.ResponseWriter, r *http.Request) {
	t, e := a.tx(r)
	if e != nil {
		problem(w, 503, "Banco indisponível.")
		return
	}
	defer t.Rollback(r.Context())
	var out struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		CNPJ       string          `json:"cnpj"`
		LegalName  string          `json:"legal_name"`
		Address    json.RawMessage `json:"address"`
		Status     int             `json:"cnpj_status"`
		VerifiedAt *time.Time      `json:"cnpj_verified_at"`
		Source     string          `json:"source"`
	}
	e = t.QueryRow(r.Context(), "select id,name,cnpj,coalesce(legal_name,''),address,coalesce(cnpj_registration_status,0),cnpj_verified_at,coalesce(cnpj_verification_source,'') from companies where id=$1 and deleted_at is null", claims(r).Company).Scan(&out.ID, &out.Name, &out.CNPJ, &out.LegalName, &out.Address, &out.Status, &out.VerifiedAt, &out.Source)
	if e != nil {
		problem(w, 404, "Empresa não encontrada.")
		return
	}
	write(w, 200, out)
}
