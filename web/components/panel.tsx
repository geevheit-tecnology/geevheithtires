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
} from "lucide-react";
import { Brand } from "./brand";
import { api } from "./api";
import BillingPanel from "./billing-panel";
import TireInventory from "./tire-inventory";
import VehicleOdometer from "./vehicle-odometer";
import { neonClient, neonEnabled } from "./neon-client";
import { TireMap, Tire } from "./tire-map";
type Vehicle = {
  id: string;
  plate: string;
  model: string;
  template_id: string;
  current_km: number;
};
type User = { id: string; name: string; email: string; role: string };
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
  { id: "dashboard", name: "Visão geral", icon: LayoutDashboard },
  { id: "vehicles", name: "Veículos", icon: Truck },
  { id: "tires", name: "Pneus", icon: CircleDot },
  { id: "alerts", name: "Alertas", icon: Bell },
  { id: "users", name: "Equipe", icon: Users },
  { id: "plan", name: "Plano e cobrança", icon: CreditCard },
];
export default function Panel() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [tab, setTab] = useState("dashboard");
  const [dash, setDash] = useState<Dashboard | null>(null);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [tires, setTires] = useState<Tire[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [map, setMap] = useState<MapData | null>(null);
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
  async function load() {
    setError("");
    try {
      const u = await api("me");
      setUser(u);
      const [d, v, t, a] = await Promise.all([
        api("dashboard"),
        all("vehicles"),
        all("tires"),
        all("alerts"),
      ]);
      setDash(d);
      setVehicles(v);
      setTires(t);
      setAlerts(a);
      if (u.role === "owner" || u.role === "manager")
        setUsers(await all("users"));
      setLoaded(true);
    } catch (e) {
      setError((e as Error).message);
      if ((e as Error).message.includes("Sessão")) router.replace("/login");
    }
  }
  useEffect(() => {
    void load();
  }, []);
  useEffect(()=>{
    const timer=setInterval(()=>{if(document.visibilityState!=="visible")return;void Promise.all([api("dashboard"),all("alerts")]).then(([d,a])=>{setDash(d);setAlerts(a)}).catch(()=>{})},60000);
    return()=>clearInterval(timer);
  },[]);
  async function openMap(v: Vehicle) {
    setError("");
    try {
      setMap(await api(`vehicles/${v.id}/tire-map`));
      setTab("vehicles");
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function select(t: Tire) {
    setInspectionClient(crypto.randomUUID());
    setSelected(t);
    setModal("inspection");
    try {
      setHistory((await api(`tires/${t.id}/history`)).data);
    } catch (e) {
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
      if (!data.vehicle_id) {
        delete data.vehicle_id;
        delete data.position;
      }
    }
    if (modal === "users") endpoint = "users/invite";
    if (modal === "inspection") {
      endpoint = `tires/${selected?.id}/inspections`;
      data.tread_mm = Number(data.tread_mm);
      data.pressure_psi = Number(data.pressure_psi);
      data.structural_damage = data.structural_damage === "on";
      data.client_id = inspectionClient;
      if(data.vehicle_km!==undefined && data.vehicle_km!=="") data.vehicle_km=Number(data.vehicle_km); else delete data.vehicle_km;
    }
    try {
      const out = await api(endpoint, data);
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
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function logout() {
    try {
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
    <div className="panel">
      <aside className="sidebar">
        <Brand />
        <div className="company-badge">
          <span>{dash?.company.name?.slice(0, 1) || "G"}</span>
          <div>
            <b>{dash?.company.name || "Sua empresa"}</b>
            <small>Gestão de frota</small>
          </div>
        </div>
        <nav>
          {nav
            .filter((n) => n.id !== "users" || manage)
            .map((n) => (
              <button
                className={tab === n.id ? "active" : ""}
                key={n.id}
                onClick={() => {
                  setTab(n.id);
                  setSearch("");
                }}
              >
                <n.icon size={19} />
                {n.name}
                {n.id === "alerts" && !!dash?.alerts && (
                  <span className="count">{dash.alerts}</span>
                )}
              </button>
            ))}
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
          <button onClick={logout}>
            <LogOut size={17} />
            Sair da conta
          </button>
        </div>
      </aside>
      <div className="panel-body">
        <header className="panel-header">
          <span>
            <i className="dot ok" />
            GeevheitTire / {nav.find((n) => n.id === tab)?.name}
          </span>
          <div className="user-avatar">{user?.name?.slice(0, 1) || "G"}</div>
        </header>
        <main className="panel-content">
          <div className="panel-title">
            <div>
              <span className="label">SUA OPERAÇÃO EM MOVIMENTO</span>
              <h1>
                {tab === "dashboard"
                  ? `Olá, ${user?.name?.split(" ")[0] || "gestor"}.`
                  : nav.find((n) => n.id === tab)?.name}
              </h1>
              <p>
                {tab === "dashboard"
                  ? "Acompanhe o que precisa da sua atenção hoje."
                  : "Informações da sua empresa, sempre no mesmo lugar."}
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
          {error && (
            <div role="alert" className="error">
              {error}
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
          {!loaded && !error && <p>Carregando sua operação…</p>}
          {tab === "dashboard" && (
            <>
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
                      <TireMap
                        positions={map.positions}
                        tires={map.tires}
                        onSelect={select}
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
                  <label>
                    Configuração
                    <select name="template_id">
                      <option value="tractor-6x2">Cavalo mecânico 6x2</option>
                      <option value="trailer-3">Carreta 3 eixos</option>
                    </select>
                  </label>
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
                  <div className="form-grid">
                    <label>
                      Código
                      <input
                        name="code"
                        required
                        placeholder="P00123"
                        minLength={2}
                      />
                    </label>
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
                  </div>
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
                          const positions =
                            v?.template_id === "tractor-6x2"
                              ? [
                                  "1EE",
                                  "1DE",
                                  "2EE",
                                  "2EI",
                                  "2DI",
                                  "2DE",
                                  "3EE",
                                  "3EI",
                                  "3DI",
                                  "3DE",
                                  "STEP1",
                                ]
                              : [
                                  "1EE",
                                  "1EI",
                                  "1DI",
                                  "1DE",
                                  "2EE",
                                  "2EI",
                                  "2DI",
                                  "2DE",
                                  "3EE",
                                  "3EI",
                                  "3DI",
                                  "3DE",
                                  "STEP1",
                                  "STEP2",
                                ];
                          positions.forEach((p) => {
                            const o = document.createElement("option");
                            o.value = p;
                            o.text = p;
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
                    <select name="position" disabled defaultValue="">
                      <option value="">Selecione um veículo</option>
                    </select>
                  </label>
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
  );
}
function Shield() {
  return <CheckCircle2 />;
}
