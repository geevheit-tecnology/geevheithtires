"use client";
import { FormEvent, useState } from "react";
import { api } from "./api";

export type TireIdentity = { code: string; serial_number?: string | null; dot_code?: string | null; expires_on?: string | null; road_use?: string; version?: number };
export const roadNames: Record<string,string> = { ASFALTO:"Asfalto", TERRA:"Terra", MISTO:"Uso misto", NAO_INFORMADO:"Não informado" };
export function TireIdentityFields({ tire, includeCode=false }: { tire?: TireIdentity; includeCode?: boolean }) {
 return <>
  {includeCode&&<label>Número de fogo<input name="code" defaultValue={tire?.code} required minLength={2} maxLength={50} placeholder="P00123"/><small>Identificação única do pneu na sua frota.</small></label>}
  <label>Número de série de fábrica<input name="serial_number" defaultValue={tire?.serial_number||""} required minLength={2} maxLength={80}/></label>
  <label>DOT<input name="dot_code" defaultValue={tire?.dot_code||""} required minLength={4} maxLength={40} placeholder="Ex.: 1025 ou DOT … 1025"/><small>Informe o código completo ou os quatro dígitos finais: semana e ano de fabricação.</small></label>
  <label>Validade operacional<input name="expires_on" type="date" defaultValue={tire?.expires_on?.slice(0,10)||""} required/><small>Data limite definida pela empresa conforme a orientação do fabricante. Não é calculada automaticamente pelo DOT.</small></label>
  <label>Tipo de via<select name="road_use" defaultValue={tire?.road_use==="NAO_INFORMADO"?"":tire?.road_use||""} required><option value="">Selecione</option>{["ASFALTO","TERRA","MISTO"].map(v=><option key={v} value={v}>{roadNames[v]}</option>)}</select></label>
 </>;
}
export function TireIdentityEditor({ tire, onSaved }: { tire: TireIdentity & {id:string}; onSaved:()=>Promise<void> }) {
 const [busy,setBusy]=useState(false),[error,setError]=useState("");
 async function save(e:FormEvent<HTMLFormElement>) { e.preventDefault();setBusy(true);setError("");try { await api(`tires/${tire.id}/identity`,{...Object.fromEntries(new FormData(e.currentTarget)),expected_version:tire.version},"PATCH");await onSaved(); } catch(e) {setError((e as Error).message)} finally {setBusy(false)} }
 return <details className="lifecycle-policy"><summary>Editar identificação, validade e tipo de via</summary><form onSubmit={save} key={tire.version}><fieldset disabled={busy} className="identity-fields"><TireIdentityFields tire={tire} includeCode/>{error&&<p className="error" role="alert">{error}</p>}<button className="button primary" disabled={busy}>{busy?"Salvando…":"Salvar identificação"}</button></fieldset></form></details>;
}
