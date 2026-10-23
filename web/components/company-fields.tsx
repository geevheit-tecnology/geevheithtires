"use client";
import { useRef, useState } from "react";
import { api } from "./api";
import { validCNPJ } from "../lib/cnpj";

const states = ["AC", "AL", "AP", "AM", "BA", "CE", "DF", "ES", "GO", "MA", "MT", "MS", "MG", "PA", "PB", "PR", "PE", "PI", "RJ", "RN", "RS", "RO", "RR", "SC", "SP", "SE", "TO"];
type Address = { postal_code: string; street: string; number: string; complement: string; district: string; city: string; state: string };
type Registration = { cnpj: string; legal_name: string; trade_name: string; active: boolean; status: string; address: Address };
const empty: Address = { postal_code: "", street: "", number: "", complement: "", district: "", city: "", state: "" };

export function CompanyFields({ disabled }: { disabled: boolean }) {
  const [company, setCompany] = useState("");
  const [cnpj, setCNPJ] = useState("");
  const [address, setAddress] = useState<Address>(empty);
  const [registration, setRegistration] = useState<Registration | null>(null);
  const [cnpjError, setCNPJError] = useState("");
  const [cepError, setCEPError] = useState("");
  const [searchingCNPJ, setSearchingCNPJ] = useState(false);
  const [searchingCEP, setSearchingCEP] = useState(false);
  const cnpjRequest = useRef(0), cepRequest = useRef(0);
  const update = (key: keyof Address, value: string) => setAddress(current => ({ ...current, [key]: value }));

  async function lookupCNPJ() {
    const value = cnpj.toUpperCase().replace(/[./\s-]/g, "");
    if (!validCNPJ(value)) { setCNPJError("Informe um CNPJ válido."); return; }
    const request = ++cnpjRequest.current;
    setSearchingCNPJ(true); setCNPJError(""); setRegistration(null);
    try {
      const result: Registration = await api(`registration/cnpj/${value}`);
      if (request !== cnpjRequest.current) return;
      setRegistration(result);
      setCompany(result.trade_name || result.legal_name);
      ++cepRequest.current; setSearchingCEP(false); setCEPError("");
      setAddress({ ...empty, ...result.address });
    } catch (e) {
      if (request === cnpjRequest.current) setCNPJError((e as Error).message);
    } finally { if (request === cnpjRequest.current) setSearchingCNPJ(false); }
  }

  async function lookupCEP() {
    const value = address.postal_code.replace(/\D/g, "");
    if (!/^\d{8}$/.test(value)) { setCEPError("Informe um CEP com 8 dígitos."); return; }
    const request = ++cepRequest.current;
    setSearchingCEP(true); setCEPError("");
    try {
      const result: Address = await api(`registration/cep/${value}`);
      if (request !== cepRequest.current) return;
      // Preserve the user's building number and unit, which a CEP does not identify.
      setAddress(current => ({ ...current, postal_code: value, street: result.street, district: result.district, city: result.city, state: result.state }));
    } catch (e) {
      if (request === cepRequest.current) setCEPError((e as Error).message);
    } finally { if (request === cepRequest.current) setSearchingCEP(false); }
  }

  return <fieldset className="company-fields" disabled={disabled}>
    <legend>Dados da empresa</legend>
    <label htmlFor="company-cnpj">CNPJ</label>
    <div className="lookup-row">
      <input id="company-cnpj" name="cnpj" value={cnpj} onChange={e => {
        ++cnpjRequest.current; setSearchingCNPJ(false); setCNPJ(e.target.value); setRegistration(null); setCNPJError("");
      }} placeholder="00.000.000/0000-00" required inputMode="text" autoCapitalize="characters" maxLength={18} aria-describedby="cnpj-help" />
      <button type="button" className="lookup-button" onClick={lookupCNPJ} disabled={searchingCNPJ}>{searchingCNPJ ? "Consultando…" : "Consultar CNPJ"}</button>
    </div>
    <small id="cnpj-help">Consulte a razão social e a situação cadastral da empresa.</small>
    {cnpjError && <p className="lookup-error" role="alert">{cnpjError}</p>}
    {registration && <div className={registration.active ? "lookup-result" : "lookup-error"} role="status">
      <strong>{registration.legal_name}</strong><span>Situação cadastral: {registration.status}</span>
      {!registration.active && <span>O cadastro está disponível para empresas com situação ativa.</span>}
    </div>}
    <label>Nome da empresa<input name="company" value={company} onChange={e => setCompany(e.target.value)} autoComplete="organization" required minLength={2} maxLength={200} /></label>
    <label htmlFor="company-cep">CEP</label>
    <div className="lookup-row">
      <input id="company-cep" name="postal_code" value={address.postal_code} onChange={e => {
        ++cepRequest.current; setSearchingCEP(false); update("postal_code", e.target.value); setCEPError("");
      }} onBlur={() => { if (address.postal_code.replace(/\D/g, "").length === 8) void lookupCEP(); }} placeholder="00000-000" autoComplete="postal-code" inputMode="numeric" required maxLength={9} />
      <button type="button" className="lookup-button" onClick={lookupCEP} disabled={searchingCEP}>{searchingCEP ? "Buscando…" : "Buscar CEP"}</button>
    </div>
    {cepError && <p className="lookup-error" role="alert">{cepError} Você pode preencher o endereço manualmente.</p>}
    <label>Rua / logradouro<input name="street" value={address.street} onChange={e => update("street", e.target.value)} autoComplete="address-line1" required maxLength={200} /></label>
    <div className="address-pair">
      <label>Número<input name="number" value={address.number} onChange={e => update("number", e.target.value)} required maxLength={30} placeholder="Nº ou S/N" /></label>
      <label>Complemento<input name="complement" value={address.complement} onChange={e => update("complement", e.target.value)} autoComplete="address-line2" maxLength={200} /></label>
    </div>
    <label>Bairro<input name="district" value={address.district} onChange={e => update("district", e.target.value)} required maxLength={200} /></label>
    <div className="address-pair">
      <label>Cidade<input name="city" value={address.city} onChange={e => update("city", e.target.value)} autoComplete="address-level2" required maxLength={200} /></label>
      <label>Estado / UF<select name="state" value={address.state} onChange={e => update("state", e.target.value)} autoComplete="address-level1" required><option value="">Selecione</option>{states.map(state => <option key={state} value={state}>{state}</option>)}</select></label>
    </div>
  </fieldset>;
}
