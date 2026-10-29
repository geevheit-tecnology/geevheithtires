"use client";
import {useCallback,useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";
import {ArrowUpRight,BarChart3,Bell,CreditCard,FileText,LayoutDashboard,LogOut,RefreshCw,Settings,ShieldCheck,Users,Wallet} from "lucide-react";
import {Brand} from "./brand";
import {api,ApiError} from "./api";
import {Plan,money,date} from "./billing-panel";
import PlatformClients from "./platform-clients";
import PlatformTickets from "./platform-tickets";
import PlatformFinance from "./platform-finance";
import {AdminPlans,AdminProfile} from "./platform-settings";
export type AdminPlan=Plan&{version:number};
export type AdminIdentity={name:string;email:string;managed_identity:boolean};
type Audit={id:number;action:string;target:string;reason:string;created_at:string};
type Snapshot={plans:AdminPlan[];profile:AdminIdentity;metrics:{companies:number;active:number;trials:number;received_cents:number};operations:{expired:number;suspended:number;expiring:number;open_tickets:number;critical_tickets:number};audit:Audit[]};
const tabs=[{id:"overview",name:"Visão geral",icon:LayoutDashboard},{id:"clients",name:"Clientes",icon:Users},{id:"plans",name:"Planos e preços",icon:CreditCard},{id:"finance",name:"Financeiro",icon:Wallet},{id:"tickets",name:"Chamados e bugs",icon:Bell},{id:"audit",name:"Auditoria",icon:FileText},{id:"profile",name:"Minha conta",icon:Settings}];
const descriptions:Record<string,string>={overview:"Acompanhe a operação, identifique pendências e controle o crescimento da plataforma.",clients:"Gerencie empresas, responsáveis, planos, limites e períodos de acesso.",plans:"Defina preços e limites e publique as condições que seus clientes consultam.",finance:"Controle mensalidades, recebimentos e vencimentos com histórico por empresa.",tickets:"Acompanhe o atendimento e mantenha seus clientes informados.",audit:"Confira quando e por que cada ação foi realizada.",profile:"Atualize seu nome e sua senha sem depender de intervenção técnica."};
const actionNames:Record<string,string>={configure_plan:"Plano atualizado",sync_plan_limits:"Limites sincronizados com clientes",grant_access:"Vigência concedida",suspend:"Cliente suspenso",restore:"Cliente restaurado",trial:"Avaliação prorrogada",create_receivable:"Mensalidade registrada",receivable_pay:"Recebimento confirmado",receivable_cancel:"Mensalidade cancelada",receivable_refund:"Recebimento estornado",update_profile:"Perfil atualizado",reset_admin_password:"Senha administrativa redefinida",grant_admin:"Acesso administrativo concedido",close_company:"Empresa encerrada"};
export default function PlatformAdmin(){
 const router=useRouter();
 const [data,setData]=useState<Snapshot|null>(null),[error,setError]=useState(""),[tab,setTab]=useState("overview"),[updated,setUpdated]=useState("");
 const load=useCallback(async()=>{try{setData(await api("platform"));setUpdated(new Date().toLocaleTimeString("pt-BR",{hour:"2-digit",minute:"2-digit"}));setError("")}catch(e){setError((e as Error).message);if(e instanceof ApiError&&e.status===401)router.replace('/admin/login')}},[router]);
 useEffect(()=>{void load();const hash=window.location.hash.slice(1);if(tabs.some(t=>t.id===hash))setTab(hash)},[load]);
 useEffect(()=>{const refresh=()=>{if(document.visibilityState==="visible")void load()};const timer=setInterval(refresh,15000);window.addEventListener("focus",refresh);return()=>{clearInterval(timer);window.removeEventListener("focus",refresh)}},[load]);
 function navigate(id:string){setTab(id);setError("");window.history.replaceState(null,"",`#${id}`);window.scrollTo({top:0,behavior:"smooth"})}
 async function logout(){try{await api("auth/logout",{});router.replace('/admin/login')}catch(e){setError((e as Error).message)}}
 const selected=tabs.find(t=>t.id===tab)!;
 return <div className="admin-shell commercial-admin">
  <aside className="admin-sidebar"><Brand/><div className="admin-workspace"><ShieldCheck size={24}/><div><strong>Controle da plataforma</strong><small>Acesso administrativo</small></div></div><nav aria-label="Seções da administração">{tabs.map(t=><button key={t.id} className={tab===t.id?"active":""} aria-current={tab===t.id?"page":undefined} onClick={()=>navigate(t.id)}><t.icon size={19}/><span>{t.name}</span>{t.id==="tickets"&&!!data?.operations.open_tickets&&<b className="admin-nav-count">{data.operations.open_tickets}</b>}</button>)}</nav><div className="admin-sidebar-footer"><Link href="/" target="_blank" rel="noopener noreferrer">Ver site público <ArrowUpRight size={16}/></Link><button onClick={()=>void logout()}><LogOut size={18}/>Sair da administração</button></div></aside>
  <div className="admin-body"><header className="admin-topbar"><span><ShieldCheck size={17}/>GeevheitTire <span className="admin-breadcrumb">/ Administração</span></span><button className="admin-profile-button" onClick={()=>navigate("profile")}><span className="user-avatar">{data?.profile.name.slice(0,1)||"A"}</span><span>{data?.profile.name||"Administrador"}<small>Minha conta</small></span></button></header>
  <main className="admin-main"><div className="admin-heading"><div><span className="label">CENTRO DE CONTROLE</span><h1>{selected.name}</h1><p>{descriptions[tab]}</p></div><div className="admin-refresh"><button className="button outline" onClick={()=>void load()}><RefreshCw size={17}/>Atualizar</button>{updated&&<small>Atualizado às {updated}</small>}</div></div>
  {error&&<div className="error" role="alert">{error}{!data&&<Link href="/admin/login"> Entrar como administrador</Link>}</div>}{!data&&!error&&<p role="status">Verificando acesso administrativo…</p>}
  {data&&<>
   {tab==="overview"&&<>
    <div className="admin-metrics">{[{label:"Empresas cadastradas",value:data.metrics.companies,icon:Users,note:"Cadastros ativos na plataforma"},{label:"Assinaturas ativas",value:data.metrics.active,icon:ShieldCheck,note:`${data.metrics.trials} empresa(s) em avaliação`},{label:"Recebido acumulado",value:money(data.metrics.received_cents),icon:Wallet,note:"Recebimentos confirmados em produção"},{label:"Atendimentos pendentes",value:data.operations.open_tickets,icon:Bell,note:`${data.operations.critical_tickets} chamado(s) crítico(s)`}].map(m=><article className="admin-metric" key={m.label}><div><span>{m.label}</span><m.icon size={20}/></div><strong>{m.value}</strong><small>{m.note}</small></article>)}</div>
    <section className="admin-welcome"><div><span className="label">DA DECISÃO À OPERAÇÃO</span><h2>O controle está nas suas mãos.</h2><p>Atualize condições comerciais, acompanhe os recebimentos e gerencie o acesso de cada empresa em um único lugar.</p></div><button className="button" onClick={()=>navigate("plans")}>Configurar planos <ArrowUpRight size={18}/></button></section>
    <div className="admin-overview-grid"><section className="surface management-section"><div className="admin-section-heading"><div><span className="label">ATENÇÃO À CARTEIRA</span><h2>O que precisa de você</h2></div><Users size={22}/></div><div className="admin-attention-list">{[["Acessos vencidos",data.operations.expired,"Regularize a vigência dos clientes.","clients"],["Vencimentos em até 7 dias",data.operations.expiring,"Antecipe a próxima mensalidade.","clients"],["Empresas suspensas",data.operations.suspended,"Revise as contas suspensas.","clients"],["Chamados críticos",data.operations.critical_tickets,"Priorize a resposta da sua equipe.","tickets"]].map(([name,n,help,destination])=><button key={String(name)} onClick={()=>navigate(String(destination))}><span><strong>{name}</strong><small>{help}</small></span><b>{n}</b><ArrowUpRight size={18}/></button>)}</div></section>
    <section className="surface management-section"><div className="admin-section-heading"><div><span className="label">ROTINA ADMINISTRATIVA</span><h2>Ações rápidas</h2></div><BarChart3 size={22}/></div><div className="admin-quick-actions">{[{id:"clients",name:"Gerenciar clientes",text:"Planos, limites e vigência",icon:Users},{id:"finance",name:"Acompanhar financeiro",text:"Mensalidades e recebimentos reais",icon:Wallet},{id:"plans",name:"Editar preços dos planos",text:"Starter, Pro e Enterprise",icon:CreditCard},{id:"profile",name:"Alterar minha senha",text:"Dados e segurança da sua conta",icon:Settings}].map(a=><button key={a.id} onClick={()=>navigate(a.id)}><a.icon size={22}/><span><strong>{a.name}</strong><small>{a.text}</small></span><ArrowUpRight size={17}/></button>)}</div></section></div>
    <section className="surface management-section"><div className="admin-section-heading"><h2>Últimas alterações</h2><button className="table-link" onClick={()=>navigate("audit")}>Ver auditoria</button></div><AuditList rows={data.audit.slice(0,5)}/></section>
   </>}
   {tab==="clients"&&<PlatformClients plans={data.plans} onChange={load}/>}
   {tab==="finance"&&<PlatformFinance plans={data.plans} onChange={load}/>}
   {tab==="tickets"&&<PlatformTickets/>}
   {tab==="plans"&&<AdminPlans plans={data.plans} onChange={load}/>}
   {tab==="profile"&&<AdminProfile profile={data.profile} onChange={load} onLogout={logout}/>}
   {tab==="audit"&&<section className="surface management-section"><div className="admin-section-heading"><h2>Auditoria administrativa</h2><span className="admin-status neutral">Últimas 50 alterações</span></div><AuditList rows={data.audit}/></section>}
  </>}
  <footer className="admin-footer"><span><ShieldCheck size={15}/>Alterações protegidas e auditadas</span><span>GeevheitTire · Administração</span></footer></main></div>
 </div>
}
function AuditList({rows}:{rows:Audit[]}){return rows.length?<ol className="admin-audit-list">{rows.map(a=><li key={a.id}><span className="admin-audit-dot"/><div><strong>{actionNames[a.action]||a.action.replaceAll("_"," ")}</strong><p>{a.reason}</p><small>{date(a.created_at)} · {new Date(a.created_at).toLocaleTimeString("pt-BR",{hour:"2-digit",minute:"2-digit"})} · {a.target}</small></div></li>)}</ol>:<div className="empty-state"><FileText/><h3>Nenhuma alteração registrada</h3><p>As ações administrativas aparecerão aqui com data e justificativa.</p></div>}
