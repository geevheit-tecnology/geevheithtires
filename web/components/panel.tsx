"use client";
import { useEffect, useState, useRef, FormEvent } from "react";
import { useRouter } from "next/navigation";
import {
  LayoutDashboard,
  Truck,
  CircleDot,
  Bell,
  Users,
  CreditCard,
  LogOut,
  Plus,
  ArrowUpRight,
  X,
  CheckCircle2,
  RefreshCw,
  Camera,
  Menu,
  Moon,
  Sun,
} from "lucide-react";
import { TireLoader } from "./tire-loader";
import { optimizeTirePhoto, OptimizedTirePhoto } from "../lib/image-optimize";
import { Brand } from "./brand";
import { api } from "./api";
import BillingPanel from "./billing-panel";
import TireInventory from "./tire-inventory";
import SupportCenter from "./support-center";
import AccountCenter from "./account-center";
import Link from "next/link";
import TireReports from "./tire-reports";
import {PlanAccessProvider, PlanAccess, lockedAccess} from "./plan-access";
import VehicleConfigurationFields, { VehicleConfiguration } from "./vehicle-configuration-fields";
import VehicleOdometer from "./vehicle-odometer";
import { neonClient, neonEnabled } from "./neon-client";
import { TireMap, Tire } from "./tire-map";
import DriversPanel, {VehicleDriver} from "./drivers-panel";
import {TireIdentityFields} from "./tire-identity-fields";
import {positionLabel} from "../lib/tire-position";
type Vehicle = {
  id: string;
  plate: string;
  model: string;
  template_id: string;
  current_km: number;
  driver_id?: string|null;
  driver_name?: string|null;
};
type User = { id: string; company_id: string; name: string; email: string; role: string };
type PendingInspection = { userId: string; companyId: string; tireId: string; tireCode: string; data: Record<string, unknown> };
const pendingKey = "geevheittire-pending-inspections-v1";
const snapshotKey = "geevheittire-offline-snapshot-v1";
type OfflineSnapshot = {
  user: User;
  dash: Dashboard;
  vehicles: Vehicle[];
  tires: Tire[];
  alerts: Alert[];
  configurations: VehicleConfiguration[];
  access: PlanAccess;
  savedAt: string;
};
type Alert = {
  id: string;
  message: string;
  severity: string;
  read: boolean;
  created_at: string;
};
type Dashboard = {
  vehicles: number;
  tires: number;
  critical: number;
  alerts: number;
  company: { name: string; status: string; trial_ends_at: string };
};
type MapData = { vehicle: Vehicle; positions: string[]; tires: Tire[] };
const nav = [
  { id: "dashboard", name: "Gestão de pneus", icon: LayoutDashboard },
  { id: "vehicles", name: "Veículos", icon: Truck },
  { id: "tires", name: "Pneus", icon: CircleDot },
  { id: "reports", name: "Relatórios de pneus", icon: LayoutDashboard },
  { id: "alerts", name: "Alertas", icon: Bell },
  { id: "users", name: "Equipe", icon: Users },
  { id: "drivers", name: "Motoristas", icon: Users },
  { id: "support", name:"Suporte e bugs", icon:Bell },
  { id: "account", name:"Minha conta", icon:Users },
  { id: "plan", name: "Plano e cobrança", icon: CreditCard },
];
export default function Panel() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [platformAdmin,setPlatformAdmin]=useState(false);
  const [supportDraft,setSupportDraft]=useState("");
  const [access,setAccess] = useState<PlanAccess>(lockedAccess);
  async function refreshPlan(){setAccess(await api("entitlements"));}
  const [tab, setTab] = useState("dashboard");
  const [dash, setDash] = useState<Dashboard | null>(null);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [configurations, setConfigurations] = useState<VehicleConfiguration[]>([]);
  const [tires, setTires] = useState<Tire[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [map, setMap] = useState<MapData | null>(null);
  const [dossierTireId, setDossierTireId] = useState<string>();
  const [selected, setSelected] = useState<Tire | null>(null);
  const [inspectionClient, setInspectionClient] = useState("");
  const [history, setHistory] = useState<
    {
      id: string;
      tread_mm: number;
      pressure_psi: number;
      inspected_at: string;
      recommendation: string;
    }[]
  >([]);
  const [modal, setModal] = useState("");
  const modalRef = useRef<HTMLElement>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [search, setSearch] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  useEffect(() => {
    if (!mobileMenuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMobileMenuOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [mobileMenuOpen]);
  const [pending, setPending] = useState<PendingInspection[]>([]);
  const [syncing, setSyncing] = useState(false);
  const [isOffline, setIsOffline] = useState(false);
  const [offlineSince, setOfflineSince] = useState("");
  const syncingRef = useRef(false);
  useEffect(() => {
    try { const saved = JSON.parse(localStorage.getItem(pendingKey) || "[]"); if (Array.isArray(saved)) setPending(saved); } catch {}
    if (typeof navigator !== "undefined" && !navigator.onLine) setIsOffline(true);
    const handleOffline = () => setIsOffline(true);
    const handleOnline = () => {
      setIsOffline(false);
      void load();
    };
    window.addEventListener("offline", handleOffline);
    window.addEventListener("online", handleOnline);
    return () => {
      window.removeEventListener("offline", handleOffline);
      window.removeEventListener("online", handleOnline);
    };
  }, []);
  function savePending(items: PendingInspection[]) {
    localStorage.setItem(pendingKey, JSON.stringify(items));
    setPending(items);
  }
  async function syncPending() {
    if (!user || syncingRef.current) return;
    syncingRef.current = true;
    setSyncing(true);
    try {
      // Confirm the current server identity before sending anything from this device.
      const current = await api("me");
      if (current.id !== user.id || current.company_id !== user.company_id) throw new Error("A conta atual mudou. Entre com a conta que registrou as inspeções.");
      let items: PendingInspection[] = JSON.parse(localStorage.getItem(pendingKey) || "[]");
      for (const item of items.filter(x => x.userId === current.id && x.companyId === current.company_id)) {
        await api(`tires/${item.tireId}/inspections`, item.data);
        // Re-read before removing so a newly queued inspection is not overwritten.
        items = (JSON.parse(localStorage.getItem(pendingKey) || "[]") as PendingInspection[])
          .filter(x => x.data.client_id !== item.data.client_id);
        savePending(items);
      }
      setNotice("Inspeções pendentes enviadas. Atualize os dados para consultar as recomendações.");
    } catch (e) { setError(`Sincronização pendente: ${(e as Error).message}`); }
    finally { syncingRef.current = false; setSyncing(false); }
  }
  useEffect(() => {
    if (!user || !pending.some(x => x.userId === user.id && x.companyId === user.company_id)) return;
    const online = () => { void syncPending(); };
    window.addEventListener("online", online);
    return () => window.removeEventListener("online", online);
  });
  const [theme, setTheme] = useState<"light" | "dark">("light");
  useEffect(() => {
    try { if (localStorage.getItem("geevheittire-theme") === "dark") setTheme("dark"); } catch {}
  }, []);
  function toggleTheme() {
    const next = theme === "light" ? "dark" : "light";
    setTheme(next);
    try { localStorage.setItem("geevheittire-theme", next); } catch {}
  }
  const [newTirePhoto, setNewTirePhoto] = useState<OptimizedTirePhoto | null>(null);
  const [photoBusy, setPhotoBusy] = useState(false);
  const manage = user?.role === "owner" || user?.role === "manager";
  useEffect(() => {
    if (!modal) return;
    const previous = document.activeElement as HTMLElement | null;
    const dialog = modalRef.current;
    const focusables = () =>
      Array.from(
        dialog?.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select:not(:disabled), a[href], [tabindex="0"]',
        ) || [],
      );
    focusables()[0]?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !busy) {
        event.preventDefault();
        setModal("");
      }
      if (event.key === "Tab") {
        const list = focusables();
        const first = list[0];
        const last = list[list.length - 1];
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previous?.focus();
    };
  }, [modal, busy]);
  async function all(path: string) {
    const data = [];
    let cursor = "";
    do {
      const out = await api(`${path}${cursor ? `?cursor=${cursor}` : ""}`);
      data.push(...out.data);
      cursor = out.next_cursor;
    } while (cursor);
    return data;
  }
  function restoreOfflineSnapshot(): boolean {
    try {
      const raw = localStorage.getItem(snapshotKey);
      if (!raw) return false;
      const snap = JSON.parse(raw) as OfflineSnapshot;
      if (!snap?.user?.id || !snap?.dash) return false;
      setUser(snap.user);
      setDash(snap.dash);
      setVehicles(Array.isArray(snap.vehicles) ? snap.vehicles : []);
      setTires(Array.isArray(snap.tires) ? snap.tires : []);
      setAlerts(Array.isArray(snap.alerts) ? snap.alerts : []);
      setConfigurations(Array.isArray(snap.configurations) ? snap.configurations : []);
      if (snap.access) setAccess(snap.access);
      setOfflineSince(snap.savedAt || "");
      setIsOffline(true);
      setLoaded(true);
      return true;
    } catch {
      return false;
    }
  }
  async function load() {
    setError("");
    if (typeof navigator !== "undefined" && !navigator.onLine) {
      if (restoreOfflineSnapshot()) return;
    }
    try {
      const u = await api("me");
      setUser(u);
      void api("platform/me").then(out=>setPlatformAdmin(out.admin===true)).catch(()=>setPlatformAdmin(false));
      const [d, v, t, a, configurations,rights] = await Promise.all([
        api("dashboard"),
        all("vehicles"),
        all("tires"),
        all("alerts"),
        api("vehicle-configurations"),
        api("entitlements"),
      ]);
      setAccess(rights);
      setConfigurations(configurations.data);
      setDash(d);
      setVehicles(v);
      setTires(t);
      setAlerts(a);
      setIsOffline(false);
      setOfflineSince("");
      try {
        const lightTires = (t as Tire[]).map(({ photo_data: _photo, ...rest }) => rest);
        const snap: OfflineSnapshot = {
          user: u,
          dash: d,
          vehicles: v,
          tires: lightTires,
          alerts: a,
          configurations: configurations.data,
          access: rights,
          savedAt: new Date().toISOString(),
        };
        localStorage.setItem(snapshotKey, JSON.stringify(snap));
      } catch {}
      if (u.role === "owner" || u.role === "manager")
        setUsers(await all("users"));
      setLoaded(true);
    } catch (e) {
      const msg = (e as Error).message || "";
      if (msg.includes("Sessão")) {
        router.replace("/login");
        return;
      }
      const networkError =
        e instanceof TypeError ||
        (typeof navigator !== "undefined" && !navigator.onLine);
      if (networkError && restoreOfflineSnapshot()) {
        return;
      }
      setError(msg);
    }
  }
  useEffect(() => {
    void load();
  }, []);
  useEffect(()=>{
    const timer=setInterval(()=>{if(document.visibilityState!=="visible"||(typeof navigator!=="undefined"&&!navigator.onLine))return;void Promise.all([api("dashboard"),all("alerts"),api("entitlements")]).then(([d,a,rights])=>{setDash(d);setAlerts(a);setAccess(rights);setIsOffline(false)}).catch(()=>{})},60000);
    return()=>clearInterval(timer);
  },[]);
  async function openMap(v: Vehicle) {
    setError("");
    const buildOfflineMap = () => {
      const cfg = configurations.find((c) => c.id === v.template_id);
      const positions = cfg?.positions || [];
      const vehicleTires = tires.filter((t) => t.vehicle_id === v.id);
      setMap({ vehicle: v, positions, tires: vehicleTires });
      setTab("vehicles");
    };
    if (isOffline || (typeof navigator !== "undefined" && !navigator.onLine)) {
      buildOfflineMap();
      return;
    }
    try {
      setMap(await api(`vehicles/${v.id}/tire-map`));
      setTab("vehicles");
    } catch (e) {
      if (e instanceof TypeError) {
        setIsOffline(true);
        buildOfflineMap();
        return;
      }
      setError((e as Error).message);
    }
  }
  async function select(t: Tire) {
    setInspectionClient(crypto.randomUUID());
    setSelected(t);
    setHistory([]);
    setModal("inspection");
    if (isOffline || (typeof navigator !== "undefined" && !navigator.onLine)) return;
    try {
      setHistory((await api(`tires/${t.id}/history`)).data);
    } catch (e) {
      if (e instanceof TypeError) {
        setIsOffline(true);
        return;
      }
      setError((e as Error).message);
    }
  }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const data: Record<string, unknown> = Object.fromEntries(
      new FormData(e.currentTarget),
    );
    let endpoint = modal;
    if (modal === "vehicles") data.current_km = Number(data.current_km);
    if (modal === "tires") {
      data.tread_mm = Number(data.tread_mm);
      data.acquisition_cost = Number(data.acquisition_cost);
      data.retread_count = Number(data.retread_count);
      data.manufacture_year = Number(data.manufacture_year);
      if (!data.vehicle_id) {
        delete data.vehicle_id;
        delete data.position;
      }
      if (data.pressure_psi !== undefined && data.pressure_psi !== "") {
        data.pressure_psi = Number(data.pressure_psi);
      } else {
        delete data.pressure_psi;
      }
      if (newTirePhoto) {
        data.photo_data = newTirePhoto.photoData;
        data.photo_thumb = newTirePhoto.photoThumb;
      }
    }
    if (modal === "users") endpoint = "users/invite";
    if (modal === "inspection") {
      endpoint = `tires/${selected?.id}/inspections`;
      data.tread_mm = Number(data.tread_mm);
      data.pressure_psi = Number(data.pressure_psi);
      data.structural_damage = data.structural_damage === "on";
      data.client_id = inspectionClient;
      data.inspected_at = new Date().toISOString();
      if(data.vehicle_km!==undefined && data.vehicle_km!=="") data.vehicle_km=Number(data.vehicle_km); else delete data.vehicle_km;
    }
    let inspectionAccepted = false;
    if (modal === "inspection" && (isOffline || (typeof navigator !== "undefined" && !navigator.onLine)) && user && selected) {
      try {
        const items: PendingInspection[] = JSON.parse(localStorage.getItem(pendingKey) || "[]");
        savePending([...items, { userId: user.id, companyId: user.company_id, tireId: selected.id, tireCode: selected.code, data }]);
        setModal("");
        setNotice("Inspeção salva neste aparelho (modo offline). Ela será enviada automaticamente quando o sinal voltar.");
      } catch {
        setError("Não foi possível guardar a inspeção neste aparelho. Mantenha o formulário aberto e tente novamente.");
      } finally {
        setBusy(false);
      }
      return;
    }
    try {
      const out = await api(endpoint, data);
      inspectionAccepted = modal === "inspection";
      setNotice(
        modal === "inspection"
          ? out.classification.recommendation
          : modal === "users"
            ? "Convite enviado por e-mail."
            : "Cadastro salvo.",
      );
      setModal("");
      await load();
      if (map) setMap(await api(`vehicles/${map.vehicle.id}/tire-map`));
    } catch (e) {
      if (modal === "inspection" && !inspectionAccepted && e instanceof TypeError && user && selected) {
        try {
          const items: PendingInspection[] = JSON.parse(localStorage.getItem(pendingKey) || "[]");
          savePending([...items, { userId: user.id, companyId: user.company_id, tireId: selected.id, tireCode: selected.code, data }]);
          setModal("");
          setNotice("Inspeção salva somente neste aparelho. Envie quando a conexão voltar; a classificação ainda não foi calculada.");
          return;
        } catch { setError("Não foi possível guardar a inspeção neste aparelho. Mantenha o formulário aberto e tente novamente."); return; }
      }
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function logout() {
    try {
      try { localStorage.removeItem(snapshotKey); } catch {}
      if (neonEnabled) {
        const out = await neonClient.signOut();
        if (out.error)
          throw new Error(out.error.message || "Falha ao sair da conta Neon.");
      }
      await api("auth/logout", {});
      router.push("/login");
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <PlanAccessProvider access={access} viewPlans={()=>setTab("plan")} refresh={refreshPlan}>
    <div className="panel" data-theme={theme}>
      {mobileMenuOpen && (
        <div
          className="sidebar-backdrop"
          aria-hidden="true"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}
      <aside id="panel-sidebar" className={mobileMenuOpen ? "sidebar open" : "sidebar"}>
        <div className="sidebar-top">
          <Brand />
          <button
            type="button"
            className="sidebar-close"
            aria-label="Fechar menu"
            onClick={() => setMobileMenuOpen(false)}
          >
            <X size={20} />
          </button>
        </div>
        <div className="company-badge">
          <span>{dash?.company.name?.slice(0, 1) || "G"}</span>
          <div>
            <b>{dash?.company.name || "Sua empresa"}</b>
            <small>Controle de pneus e frota</small>
          </div>
        </div>
        <nav>
          {nav
            .filter((n) => (!["users","drivers"].includes(n.id) || manage) && (n.id!=="reports" || manage || user?.role==="tire_tech"))
            .map((n) => (
              <button
                className={tab === n.id ? "active" : ""}
                key={n.id}
                onClick={() => {
                  setTab(n.id);
                  setSearch("");
                  setMobileMenuOpen(false);
                }}
              >
                <n.icon size={19} />
                {n.name}
                {n.id === "alerts" && !!dash?.alerts && (
                  <span className="count">{dash.alerts}</span>
                )}
              </button>
            ))}
          {platformAdmin&&<Link className="admin-entry" href="/admin" onClick={() => setMobileMenuOpen(false)}>Administração da plataforma →</Link>}
        </nav>
        <div className="sidebar-bottom">
          <span className="trial-label">
            {dash?.company.status === "trial"
              ? "Período de avaliação"
              : "Sua operação conectada"}
          </span>
          <p>
            Uma visão melhor.
            <br />
            Um caminho mais eficiente.
          </p>
          <button onClick={() => { setMobileMenuOpen(false); void logout(); }}>
            <LogOut size={17} />
            Sair da conta
          </button>
        </div>
      </aside>
      <div className="panel-body">
        <header className="panel-header">
          <div className="panel-header-left">
            <button
              type="button"
              className="icon-button mobile-menu-trigger"
              aria-label={mobileMenuOpen ? "Fechar menu" : "Abrir menu"}
              aria-expanded={mobileMenuOpen}
              aria-controls="panel-sidebar"
              onClick={() => setMobileMenuOpen((v) => !v)}
            >
              <Menu size={20} />
              {!!dash?.alerts && <i className="mobile-menu-dot" aria-hidden="true" />}
            </button>
            <span>
              <i className={isOffline ? "dot alerta" : "dot ok"} />
              GeevheitTire / {nav.find((n) => n.id === tab)?.name}{isOffline ? " (Offline)" : ""}
            </span>
          </div>
          <div className="panel-header-actions">
            <button type="button" className="icon-button theme-toggle" onClick={toggleTheme} aria-label={theme === "light" ? "Ativar tema escuro" : "Ativar tema claro"} title={theme === "light" ? "Ativar tema escuro" : "Ativar tema claro"}>
              {theme === "light" ? <Moon size={19} /> : <Sun size={19} />}
            </button>
            <div className="user-avatar">{user?.name?.slice(0, 1) || "G"}</div>
          </div>
        </header>
        <main className="panel-content">
          <div className="panel-title">
            <div>
              <span className="label">CONTROLE DE PNEUS • FROTA • MANUTENÇÃO</span>
              <h1>
                {tab === "dashboard"
                  ? `Olá, ${user?.name?.split(" ")[0] || "gestor"}.`
                  : nav.find((n) => n.id === tab)?.name}
              </h1>
              <p>
                {tab === "dashboard"
                  ? "Controle o estoque, acompanhe o desgaste e programe a manutenção."
                  : ({vehicles: "Cadastre a frota e acompanhe os pneus por eixo e posição.", tires: access.features.advanced_lifecycle?"Gerencie identificação, estoque, instalação, rodízio e recapagens.":"Organize o estoque e consulte dados, km e histórico de cada pneu.", reports:"Consulte ocorrências e os recursos de análise disponíveis no seu plano.", alerts: "Avalie desgaste, validade e ocorrências que exigem manutenção.", users: "Convide os profissionais e defina seus perfis de acesso.", drivers: "Cadastre os motoristas e vincule os responsáveis aos veículos.", plan: "Consulte os limites da operação e a situação da assinatura."}[tab] || "Acompanhe a gestão de pneus da sua empresa.")}
              </p>
            </div>
            <button
              className="icon-button"
              aria-label="Atualizar dados"
              onClick={() => void load()}
            >
              <RefreshCw size={19} />
            </button>
          </div>
          {isOffline && (
            <div className="info" role="status">
              <strong>Modo offline ativo.</strong>{" "}
              {offlineSince
                ? `Exibindo dados salvos em ${new Date(offlineSince).toLocaleString("pt-BR")}. `
                : "Sem conexão com a internet. "}
              Você pode consultar a frota e registrar inspeções normalmente; elas serão enviadas quando o sinal voltar.
            </div>
          )}
          {error && (
            <div role="alert" className="error">
              {error}<button className="table-link" onClick={()=>{setSupportDraft(`Erro observado: ${error}\nMódulo: ${tab}\nDescreva os passos para reproduzir:`);setTab("support");setError("")}}>Reportar este erro</button>
            </div>
          )}
          {notice && (
            <div role="status" className="success dismiss">
              {notice}
              <button aria-label="Fechar aviso" onClick={() => setNotice("")}>
                <X size={16} />
              </button>
            </div>
          )}
          {user && pending.some(x => x.userId === user.id && x.companyId === user.company_id) && <div className="info" role="status">{pending.filter(x => x.userId === user.id && x.companyId === user.company_id).length} inspeção(ões) aguardando envio neste aparelho. <button type="button" className="table-link" disabled={syncing} onClick={() => void syncPending()}>{syncing ? "Enviando…" : "Enviar agora"}</button></div>}
          {loaded&&<div className="plan-context"><span className="plan-level">{access.plan_name}</span><span>{access.status==="trial"?"Avaliação com recursos Starter":access.can_write?"Recursos do seu plano":"Acesso para consulta"} · até {access.vehicle_limit} veículos e {access.user_limit} usuários</span><button className="table-link" onClick={()=>setTab("plan")}>Ver benefícios</button></div>}
          {!loaded && !error && <TireLoader />}
          {tab === "dashboard" && (
            <>
              {manage && loaded && <section className="registration-actions" aria-label="Cadastros da gestão de pneus">
                <div><span className="label">CADASTROS DA OPERAÇÃO</span><h2>Do estoque à estrada</h2><p>Abra o cadastro, preencha os dados e salve para incluir na operação.</p></div>
                <div className="registration-buttons">
                  <button className="button" onClick={() => setModal("tires")}><CircleDot size={19}/>Cadastrar pneu<Plus size={17}/></button>
                  <button className="button outline" onClick={() => setModal("vehicles")}><Truck size={19}/>Cadastrar veículo<Plus size={17}/></button>
                  <button className="button outline" onClick={() => setTab("drivers")}><Users size={19}/>Cadastrar motorista<ArrowUpRight size={17}/></button>
                </div>
              </section>}
              <div className="stats">
                {[
                  {
                    label: "Veículos cadastrados",
                    value: dash?.vehicles || 0,
                    icon: Truck,
                  },
                  {
                    label: "Pneus monitorados",
                    value: dash?.tires || 0,
                    icon: CircleDot,
                  },
                  {
                    label: "Sulcos críticos",
                    value: dash?.critical || 0,
                    icon: Bell,
                  },
                  {
                    label: "Alertas não lidos",
                    value: dash?.alerts || 0,
                    icon: CheckCircle2,
                  },
                ].map((s, i) => (
                  <article
                    key={s.label}
                    className={i === 2 && s.value ? "stat critical" : "stat"}
                  >
                    <span>
                      {s.label}
                      <s.icon size={19} />
                    </span>
                    <strong>{s.value}</strong>
                    <small>
                      {i === 2
                        ? "Priorize a avaliação técnica"
                        : "Dados atuais da empresa"}
                    </small>
                  </article>
                ))}
              </div>
              {manage && (
                <section className="onboarding">
                  <div>
                    <span className="label">PRIMEIROS QUILÔMETROS</span>
                    <h2>Vamos colocar sua frota no mapa.</h2>
                    <p>Três passos para começar com os dados certos.</p>
                  </div>
                  <div className="steps">
                    {[
                      {
                        title: "Cadastre um veículo",
                        done: vehicles.length > 0,
                        action: "vehicles",
                      },
                      {
                        title: "Adicione seus pneus",
                        done: tires.length > 0,
                        action: "tires",
                      },
                      {
                        title: "Convide sua equipe",
                        done: users.length > 1,
                        action: "users",
                      },
                    ].map((s, i) => (
                      <button key={s.title} onClick={() => setModal(s.action)}>
                        <span className={s.done ? "step done" : "step"}>
                          {s.done ? <CheckCircle2 size={19} /> : i + 1}
                        </span>
                        <b>{s.title}</b>
                        <ArrowUpRight size={18} />
                      </button>
                    ))}
                  </div>
                </section>
              )}
              <div className="panel-columns">
                <section className="surface">
                  <div className="surface-title">
                    <h3>Sua frota</h3>
                    <button onClick={() => setTab("vehicles")}>
                      Ver todos →
                    </button>
                  </div>
                  {vehicles.length ? (
                    vehicles.slice(0, 5).map((v) => (
                      <button
                        className="vehicle-row"
                        key={v.id}
                        onClick={() => void openMap(v)}
                      >
                        <span className="vehicle-icon">
                          <Truck />
                        </span>
                        <span>
                          <b>{v.plate}</b>
                          <small>{v.model}</small>
                        </span>
                        <span>{v.current_km.toLocaleString("pt-BR")} km</span>
                        <ArrowUpRight size={17} />
                      </button>
                    ))
                  ) : (
                    <div className="empty-state">
                      <Truck />
                      <h3>Sua primeira rota começa aqui.</h3>
                      <p>
                        Cadastre um veículo para visualizar suas posições de
                        pneus.
                      </p>
                      {manage && (
                        <button
                          className="button small"
                          onClick={() => setModal("vehicles")}
                        >
                          Cadastrar veículo <Plus size={16} />
                        </button>
                      )}
                    </div>
                  )}
                </section>
                <section className="surface">
                  <div className="surface-title">
                    <h3>Atenção à operação</h3>
                    <Bell size={18} />
                  </div>
                  {alerts.filter((a) => !a.read).length ? (
                    alerts
                      .filter((a) => !a.read)
                      .slice(0, 4)
                      .map((a) => (
                        <div className="alert-row" key={a.id}>
                          <i className={`dot ${a.severity}`} />
                          <p>{a.message}</p>
                        </div>
                      ))
                  ) : (
                    <div className="empty-state">
                      <Shield />
                      <h3>Nenhum alerta pendente.</h3>
                      <p>
                        Os alertas são gerados automaticamente a partir das regras, inspeções, movimentações e quilometragens registradas.
                      </p>
                    </div>
                  )}
                </section>
              </div>
            </>
          )}
          {tab === "vehicles" && (
            <>
              <div className="list-toolbar">
                <input
                  placeholder="Buscar placa ou modelo"
                  aria-label="Buscar veículos"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                {manage && (
                  <button
                    className="button small"
                    onClick={() => setModal("vehicles")}
                  >
                    <Plus size={17} />
                    Novo veículo
                  </button>
                )}
              </div>
              <div className="fleet-grid">
                <section className="surface">
                  {vehicles
                    .filter((v) =>
                      `${v.plate} ${v.model}`
                        .toLowerCase()
                        .includes(search.toLowerCase()),
                    )
                    .map((v) => (
                      <button
                        className="vehicle-row"
                        key={v.id}
                        onClick={() => void openMap(v)}
                      >
                        <Truck />
                        <span>
                          <b>{v.plate}</b>
                          <small>{v.model}</small>
                        </span>
                        <span>{v.current_km.toLocaleString("pt-BR")} km</span>
                        <ArrowUpRight size={17} />
                      </button>
                    ))}
                  {!vehicles.length && (
                    <div className="empty-state">
                      Nenhum veículo cadastrado.
                    </div>
                  )}
                </section>
                <section className="surface">
                  {map ? (
                    <>
                      <div className="surface-title">
                        <div>
                          <h3>{map.vehicle.plate}</h3>
                          <small>{map.vehicle.model}</small>
                        </div>
                        <span className="pill">Mapa de pneus</span>
                      </div>
                      {(manage||user?.role==="tire_tech")&&<VehicleOdometer vehicle={map.vehicle} onChange={async()=>{await load();setMap(await api(`vehicles/${map.vehicle.id}/tire-map`))}}/>}
                      <VehicleDriver vehicle={map.vehicle} manage={manage} onSaved={async()=>{await load();setMap(await api(`vehicles/${map.vehicle.id}/tire-map`))}}/>
                      <TireMap
                        positions={map.positions}
                        tires={map.tires}
                        onSelect={t => setDossierTireId(t.id)}
                      />
                    </>
                  ) : (
                    <div className="empty-state">
                      <CircleDot />
                      <h3>Selecione um veículo.</h3>
                      <p>O mapa segue o template de eixos cadastrado.</p>
                    </div>
                  )}
                </section>
              </div>
            </>
          )}
          {dossierTireId && <TireInventory tires={tires} vehicles={vehicles} role={user?.role||""} onChange={async()=>{await load();if(map)setMap(await api(`vehicles/${map.vehicle.id}/tire-map`))}} onInspect={t=>void select(t)} tireId={dossierTireId} onClose={()=>setDossierTireId(undefined)}/>}
          {tab === "support"&&<SupportCenter draft={supportDraft}/>}
          {tab === "account"&&<AccountCenter/>}
          {tab === "reports" && (manage||user?.role==="tire_tech") && <TireReports onOpen={setDossierTireId}/>}
          {tab === "tires" && (
            <>
              <div className="list-toolbar">
                <p>Estoque, movimentações e histórico de cada pneu.</p>
                {manage && (
                  <button
                    className="button small"
                    onClick={() => setModal("tires")}
                  >
                    <Plus size={17} />
                    Novo pneu
                  </button>
                )}
              </div>
              <TireInventory tires={tires} vehicles={vehicles} role={user?.role||""} onChange={load} onInspect={t=>void select(t)}/>

            </>
          )}
          {tab === "alerts" && (
            <section className="surface">
              {alerts.map((a) => (
                <div className="alert-row" key={a.id}>
                  <i className={`dot ${a.severity}`} />
                  <div>
                    <b>{a.read ? "Lido" : "Não lido"}</b>
                    <p>{a.message}</p>
                    <small>
                      {new Date(a.created_at).toLocaleString("pt-BR")}
                    </small>
                  </div>
                  {!a.read && (
                    <button
                      className="table-link"
                      onClick={async () => {
                        try {
                          await api(`alerts/${a.id}/read`, {}, "PATCH");
                          await load();
                        } catch (e) {
                          setError((e as Error).message);
                        }
                      }}
                    >
                      Marcar como lido
                    </button>
                  )}
                </div>
              ))}
              {!alerts.length && (
                <div className="empty-state">
                  <Bell />
                  <h3>Nenhum alerta por enquanto.</h3>
                </div>
              )}
            </section>
          )}
          {tab === "users" && manage && (
            <>
              <div className="list-toolbar">
                <p>Cada pessoa tem seu próprio acesso.</p>
                <button
                  className="button small"
                  onClick={() => setModal("users")}
                >
                  <Plus size={17} />
                  Convidar pessoa
                </button>
              </div>
              <section className="surface">
                {users.map((u) => (
                  <div className="vehicle-row" key={u.id}>
                    <span className="user-avatar">{u.name[0]}</span>
                    <span>
                      <b>{u.name}</b>
                      <small>{u.email}</small>
                    </span>
                    <span className="pill">
                      {
                        (
                          {
                            owner: "Proprietário",
                            manager: "Gestor",
                            tire_tech: "Borracheiro",
                            driver: "Motorista",
                            inspector: "Inspetor",
                          } as Record<string, string>
                        )[u.role]
                      }
                    </span>
                  </div>
                ))}
              </section>
            </>
          )}
          {tab === "drivers" && manage && <DriversPanel/>}
          {tab === "plan" && <BillingPanel owner={user?.role === "owner"} />}
        </main>
      </div>
      {modal && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget && !busy) setModal("");
          }}
        >
          <section
            className="modal"
            ref={modalRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="modal-title"
          >
            <div className="surface-title">
              <h2 id="modal-title">
                {modal === "vehicles"
                  ? "Cadastrar veículo"
                  : modal === "tires"
                    ? "Cadastrar pneu"
                    : modal === "users"
                      ? "Convidar pessoa"
                      : `Inspeção · ${selected?.code}`}
              </h2>
              <button
                aria-label="Fechar formulário"
                className="icon-button"
                onClick={() => setModal("")}
              >
                <X />
              </button>
            </div>
            <form onSubmit={submit}>
              <p className="form-intro">{modal === "tires" ? "Preencha a identificação, as medidas e a condição de entrada. Ao final, escolha entre manter no estoque ou instalar em um veículo." : modal === "vehicles" ? "Informe a identificação, selecione o tipo e confira os eixos antes de salvar o veículo." : modal === "users" ? "Informe os dados e o perfil do profissional para enviar o convite de acesso." : "Registre as medições do pneu para atualizar sua condição e as recomendações de manutenção."}</p>
              {modal === "vehicles" && (
                <>
                  <label>
                    Placa
                    <input
                      name="plate"
                      required
                      placeholder="ABC1D23"
                      maxLength={8}
                    />
                  </label>
                  <label>
                    Marca e modelo
                    <input
                      name="model"
                      required
                      placeholder="Volvo FH 540"
                      minLength={2}
                    />
                  </label>
                  <VehicleConfigurationFields configurations={configurations} />
                  <label>
                    Odômetro atual (km)
                    <input
                      name="current_km"
                      type="number"
                      min="0"
                      defaultValue="0"
                      required
                    />
                  </label>
                </>
              )}
              {modal === "tires" && (
                <>
                  <fieldset className="registration-fields"><legend>Identificação e condição de entrada</legend>
                  <div className="form-grid">
                    <label>
                      Número de fogo
                      <input
                        name="code"
                        required
                        placeholder="P00123"
                        minLength={2}
                        maxLength={50}
                      />
                    </label>
                    <TireIdentityFields/>
                    <label>Recapagens anteriores à entrada<select name="retread_count" defaultValue="0" required>{Array.from({length:11},(_,i)=><option key={i} value={i}>{i===0?"Nenhuma · 1ª vida (banda original)":`${i} recapagem(ns) · ${i+1}ª vida`}</option>)}</select></label>
                    <label>
                      Marca
                      <input
                        name="brand"
                        required
                        placeholder="Michelin"
                        minLength={2}
                      />
                    </label>
                    <label>
                      Modelo
                      <input name="model" placeholder="X Multi" />
                    </label>
                    <label>
                      Medida
                      <input name="size" required defaultValue="295/80R22.5" />
                    </label>
                    <label>
                      Sulco inicial (mm)
                      <input
                        name="tread_mm"
                        type="number"
                        min="0.1"
                        max="40"
                        step="0.1"
                        defaultValue="15"
                        required
                      />
                    </label>
                    <label>
                      Custo de aquisição (R$)
                      <input
                        name="acquisition_cost"
                        type="number"
                        min="0"
                        step="0.01"
                        defaultValue="0"
                        required
                      />
                    </label>
                    <label>
                      Pressão recomendada (PSI)
                      <input
                        name="pressure_psi"
                        type="number"
                        min="0"
                        max="250"
                        step="0.5"
                        placeholder="Ex: 110"
                      />
                    </label>
                    <label>
                      Fornecedor de compra
                      <input
                        name="purchase_supplier"
                        maxLength={150}
                        placeholder="Ex: Distribuidora Sul"
                      />
                    </label>
                    <label>
                      Observações técnicas
                      <input
                        name="notes"
                        maxLength={500}
                        placeholder="Condição visual, lote, etc."
                      />
                    </label>
                  </div>
                  <div className="tire-photo-upload-box">
                    <label className="button outline small" style={{ width: "fit-content" }}>
                      <Camera size={16} />
                      {photoBusy
                        ? "Otimizando foto…"
                        : newTirePhoto
                          ? `Foto otimizada (${newTirePhoto.sizeKB} KB) · Trocar`
                          : "Tirar ou anexar foto do pneu (para 360° / AR)"}
                      <input
                        type="file"
                        accept="image/*"
                        capture="environment"
                        hidden
                        onChange={async (ev) => {
                          const f = ev.target.files?.[0];
                          ev.target.value = "";
                          if (!f) return;
                          setPhotoBusy(true);
                          setError("");
                          try {
                            setNewTirePhoto(await optimizeTirePhoto(f));
                          } catch (err) {
                            setError((err as Error).message);
                          } finally {
                            setPhotoBusy(false);
                          }
                        }}
                      />
                    </label>
                    {newTirePhoto && (
                      <div className="tire-photo-upload-preview">
                        <img src={newTirePhoto.photoThumb} alt="Prévia da foto do pneu" />
                        <button
                          type="button"
                          className="table-link"
                          onClick={() => setNewTirePhoto(null)}
                        >
                          Remover foto
                        </button>
                      </div>
                    )}
                  </div>
                  </fieldset>
                  <fieldset className="registration-fields"><legend>Destino do pneu</legend>
                  <label>
                    Instalar em veículo (opcional)
                    <select
                      name="vehicle_id"
                      onChange={(e) => {
                        const f = e.currentTarget.form;
                        const position = f?.elements.namedItem(
                          "position",
                        ) as HTMLSelectElement;
                        position.value = "";
                        position.disabled = !e.target.value;
                        position.required = !!e.target.value;
                        if (e.target.value) {
                          position.innerHTML =
                            '<option value="">Escolha a posição</option>';
                          const v = vehicles.find(
                            (v) => v.id === e.target.value,
                          );
                          const positions = configurations.find(c => c.id === v?.template_id)?.positions || [];
                          positions.forEach((p) => {
                            const o = document.createElement("option");
                            o.value = p;
                            o.text = positionLabel(p,positions);
                            position.add(o);
                          });
                        }
                      }}
                    >
                      <option value="">Manter em estoque</option>
                      {vehicles.map((v) => (
                        <option value={v.id} key={v.id}>
                          {v.plate} · {v.model}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    Posição
                    <select name="position" aria-label="Posição" disabled defaultValue="">
                      <option value="">Selecione um veículo</option>
                    </select>
                  </label>
                  </fieldset>
                </>
              )}
              {modal === "users" && (
                <>
                  <label>
                    Nome
                    <input name="name" required minLength={2} />
                  </label>
                  <label>
                    E-mail
                    <input name="email" type="email" required />
                  </label>
                  <label>
                    Perfil
                    <select name="role">
                      <option value="tire_tech">Borracheiro</option>
                      <option value="inspector">Inspetor</option>
                    </select>
                  </label>
                  <p className="caption">
                    O convite é enviado por e-mail para a pessoa definir sua
                    senha.
                  </p>
                </>
              )}
              {modal === "inspection" && (
                <>
                  <div className="form-grid">
                    <label>
                      Sulco (mm)
                      <input
                        name="tread_mm"
                        type="number"
                        min="0"
                        max="40"
                        step="0.1"
                        defaultValue={selected?.tread_mm}
                        required
                      />
                    </label>
                    <label>
                      Pressão (psi)
                      <input
                        name="pressure_psi"
                        type="number"
                        min="0"
                        max="200"
                        step="0.1"
                        defaultValue="100"
                        required
                      />
                    </label>
                  </div>
                  {selected?.vehicle_id&&<label>Odômetro atual do veículo (km)<input name="vehicle_km" type="number" min={vehicles.find(v=>v.id===selected.vehicle_id)?.current_km||0} defaultValue={vehicles.find(v=>v.id===selected.vehicle_id)?.current_km||0}/></label>}
                  <label className="checkbox">
                    <input name="structural_damage" type="checkbox" />
                    Dano estrutural identificado
                  </label>
                  <p className="info">
                    A classificação é calculada pelo servidor. Registre a
                    medição física do sulco.
                  </p>
                  {history.length > 0 && (
                    <div className="inspection-history">
                      <h3>Últimas inspeções</h3>
                      {history.slice(0, 4).map((i) => (
                        <p key={i.id}>
                          {new Date(i.inspected_at).toLocaleDateString("pt-BR")}{" "}
                          · {i.tread_mm} mm · {i.pressure_psi} psi
                        </p>
                      ))}
                    </div>
                  )}
                </>
              )}
              {error && (
                <div role="alert" className="error">
                  {error}
                </div>
              )}
              <button disabled={busy} className="button">
                {busy
                  ? "Salvando…"
                  : modal === "users"
                    ? "Enviar convite"
                    : "Salvar"}
                <ArrowUpRight size={17} />
              </button>
            </form>
          </section>
        </div>
      )}
    </div>
    </PlanAccessProvider>
  );
}
function Shield() {
  return <CheckCircle2 />;
}
