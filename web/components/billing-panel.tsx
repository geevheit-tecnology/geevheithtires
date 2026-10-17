"use client";
import { useEffect, useState } from "react";
import { api } from "./api";
export type Plan = { id: string; name: string; price_cents: number; vehicle_limit: number; user_limit: number; published: boolean };
type Invoice = {id:string;plan_id:string;price_cents:number;status:string;checkout_url:string|null;due_date:string;environment:string};
type Billing = {status:string;trial_ends_at:string;paid_until:string|null;vehicle_limit:number;user_limit:number;vehicles:number;users:number;payments_enabled:boolean;environment:string;plans:Plan[];invoices:Invoice[]};
export const money=(cents:number)=>new Intl.NumberFormat("pt-BR",{style:"currency",currency:"BRL"}).format(cents/100);
export const date=(v:string|null)=>v?new Date(v).toLocaleDateString("pt-BR",{timeZone:"UTC"}):"—";
const states:Record<string,string>={trial:"Em avaliação",active:"Assinatura ativa",expired:"Avaliação ou assinatura encerrada",suspended:"Empresa suspensa",creating:"Em conciliação",pending:"Aguardando pagamento",paid:"Pago",overdue:"Vencido",refunded:"Estornado ou contestado",cancelled:"Cancelado"};
export default function BillingPanel({owner}:{owner:boolean}){
 const [data,setData]=useState<Billing|null>(null),[error,setError]=useState(""),[busy,setBusy]=useState("");
 async function load(){try{setData(await api("billing"));setError("")}catch(e){setError((e as Error).message)}}
 useEffect(()=>{void load()},[]);
 async function checkout(plan:string){setBusy(plan);setError("");try{const res=await api("billing/checkout",{plan_id:plan});window.location.assign(res.checkout_url)}catch(e){setError((e as Error).message);void load()}finally{setBusy("")}}
 return <section className="surface plan-status"><span className="label">PLANO E COBRANÇA</span><h2>{data?states[data.status]:"Sua assinatura"}</h2>
 <p role="alert">{error}</p>{!data?<button className="button secondary" onClick={load}>Atualizar</button>:<>
 <p>Avaliação até {date(data.trial_ends_at)}{data.paid_until?` · Acesso pago até ${date(data.paid_until)}`:""}.</p>
 <p><strong>{data.vehicles} / {data.vehicle_limit}</strong> veículos · <strong>{data.users} / {data.user_limit}</strong> usuários, incluindo convites.</p>
 {["expired","suspended"].includes(data.status)&&<div className="info">Seu acesso está disponível para consulta. Entre em contato com a administração para regularizar a empresa.</div>}
 {!data.payments_enabled&&<div className="info">Os pagamentos ainda não foram ativados. Nenhuma cobrança será gerada. Você pode consultar sua avaliação e seus limites aqui.</div>}
 {data.payments_enabled&&<p>{data.environment==="sandbox"?"Ambiente de teste: pagamentos sem valor comercial. ":""}Mensalidade por link Asaas, sem renovação automática. O pagamento confirmado libera um mês de acesso. Não solicitamos dados de cartão neste sistema.</p>}
 <div className="commercial-grid">{data.plans.map(p=><article className="surface" key={p.id}><h3>{p.name}</h3><h2>{money(p.price_cents)}<small> / mês</small></h2><p>Até {p.vehicle_limit} veículos e {p.user_limit} usuários.</p><button className="button primary" disabled={!owner||!data.payments_enabled||!!busy||data.status==="suspended"} onClick={()=>checkout(p.id)}>{busy===p.id?"Preparando…":!data.payments_enabled?"Contratação em breve":"Pagar mensalidade"}</button></article>)}</div>
 {!owner&&<p>A contratação é exclusiva do proprietário da empresa.</p>}
 <h3>Histórico de mensalidades</h3>{data.invoices.length===0?<p>Nenhuma cobrança gerada.</p>:<div className="commercial-scroll"><table className="commercial-table"><thead><tr><th>Plano</th><th>Valor</th><th>Vencimento</th><th>Situação</th><th>Pagamento</th></tr></thead><tbody>{data.invoices.map(i=><tr key={i.id}><td>{i.plan_id}{i.environment==="sandbox"?" (teste)":""}</td><td>{money(i.price_cents)}</td><td>{date(i.due_date)}</td><td>{states[i.status]}</td><td>{i.checkout_url&&["pending","overdue"].includes(i.status)?<a href={i.checkout_url} target="_blank" rel="noopener noreferrer">Abrir fatura</a>:"—"}</td></tr>)}</tbody></table></div>}
 <button className="button secondary" onClick={load}>Atualizar situação</button></>}
 </section>
}
