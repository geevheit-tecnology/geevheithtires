"use client";
import { FormEvent, useEffect, useState, useRef } from "react";
import { api } from "./api";
import { Tire } from "./tire-map";
import {TireIdentityEditor,roadNames} from "./tire-identity-fields";
import {positionLabel} from "../lib/tire-position";
type Vehicle = { id: string; plate: string; current_km: number };
type Asset = Tire & {
  vehicle_id?: string;
  version: number;
  stock_location: string;
  tracked_km: number;
  total_tracked_km: number;
  retread_count: number;
  acquisition_cost: number;
  movement_cost_cents: number;
  plate: string | null;
  current_vehicle_km: number | null;
  driver_name?: string|null;
  initial_retread_count?: number;
};
type Movement = {
  id: string;
  action: string;
  created_at: string;
  from_state: string | null;
  to_state: string;
  from_plate: string | null;
  to_plate: string | null;
  from_position: string | null;
  to_position: string | null;
  source_km: number | null;
  destination_km: number | null;
  tracked_km: number;
  stock_location: string;
  reason: string;
  supplier: string;
  cost_cents: number;
  actor_name: string | null;
  tread_mm: number;
  life_number?: number|null;
  road_use?: string|null;
  from_driver_name?: string|null;
  to_driver_name?: string|null;
};
type Details = { tire: Asset; movements: Movement[]; next_cursor: string };
type Policy = {
  warning_mm: number;
  critical_mm: number;
  rotation_km: number;
  inspection_days: number;
  minimum_stock: number;
  max_retreads: number;
};
const stateNames: Record<string, string> = {
  EM_ESTOQUE: "Em estoque",
  INSTALADO: "Instalado",
  EM_RECAPAGEM: "Em recapagem",
  DESCARTADO: "Descartado",
};
const actions: Record<string, string> = {
  REGISTER: "Cadastro inicial",
  BASELINE: "Situação ao iniciar rastreamento",
  INSTALL: "Instalar no veículo",
  STOCK_TRANSFER: "Transferir local no estoque",
  REMOVE: "Retirar para estoque",
  TRANSFER: "Transferir de veículo",
  ROTATE: "Rodízio / troca de posições",
  RECAP_SEND: "Enviar para recapagem",
  RECAP_RETURN: "Retornar da recapagem",
  DISCARD: "Confirmar descarte",
};
const number = (v: number) => Number(v || 0).toLocaleString("pt-BR");
const currency = (v: number) =>
  Number(v).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
function TirePolicy({
  manage,
  onChange,
}: {
  manage: boolean;
  onChange: () => Promise<void>;
}) {
  const [p, setP] = useState<Policy | null>(null),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    api("tire-policy")
      .then(setP)
      .catch((e) => setMessage(e.message));
  }, []);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    try {
      const body = Object.fromEntries(
        [...new FormData(e.currentTarget)].map(([k, v]) => [k, Number(v)]),
      );
      setP(await api("tire-policy", body, "PATCH"));
      setMessage("Regras salvas e alertas recalculados.");
      await onChange();
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <details className="surface lifecycle-policy">
      <summary>Regras dos alertas e estoque mínimo</summary>
      <p>
        Referências operacionais da sua empresa. Ajuste conforme o fabricante, a
        aplicação e a avaliação técnica da frota. Os avisos não executam trocas
        nem descartes.
      </p>
      {message && <p role="status">{message}</p>}
      {p && (
        <form onSubmit={submit}>
          <div className="form-grid">
            {(
              [
                ["warning_mm", "Aviso de sulco (mm)", 1, 20, 0.1],
                ["critical_mm", "Intervenção de sulco (mm)", 1, 19.9, 0.1],
                ["rotation_km", "Intervalo de rodízio (km)", 100, 500000, 100],
                ["inspection_days", "Intervalo de inspeção (dias)", 1, 365, 1],
                ["minimum_stock", "Estoque mínimo por medida", 0, 10000, 1],
                ["max_retreads", "Máximo de recapagens", 0, 10, 1],
              ] as const
            ).map(([key, label, min, max, step]) => (
              <label key={key}>
                {label}
                <input
                  type="number"
                  name={key}
                  min={min}
                  max={max}
                  step={step}
                  defaultValue={p[key]}
                  disabled={!manage}
                  required
                />
              </label>
            ))}
          </div>
          {manage && (
            <button className="button small" disabled={busy}>
              {busy ? "Recalculando…" : "Salvar regras"}
            </button>
          )}
        </form>
      )}
    </details>
  );
}
export default function TireInventory({
  tires,
  vehicles,
  role,
  onChange,
  onInspect,
}: {
  tires: Tire[];
  vehicles: Vehicle[];
  role: string;
  onChange: () => Promise<void>;
  onInspect: (t: Tire) => void;
}) {
  const [filter, setFilter] = useState(""),
    [search, setSearch] = useState(""),
    [details, setDetails] = useState<Details | null>(null),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [loading, setLoading] = useState(false),
    [busy, setBusy] = useState(false);
  const [action, setAction] = useState(""),
    [destination, setDestination] = useState(""),
    [map, setMap] = useState<{
      positions: string[];
      tires: (Tire & { version: number })[];
    } | null>(null),
    [position, setPosition] = useState(""),
    [client, setClient] = useState("");
  const detailRequest = useRef(0);
  const move = role === "owner" || role === "manager" || role === "tire_tech",
    manage = role === "owner" || role === "manager";
  async function open(tid: string) {
    const request = ++detailRequest.current;
    setLoading(true);
    setError("");
    try {
      const fresh = await api(`tires/${tid}/lifecycle`);
      if (request === detailRequest.current) setDetails(fresh);
    } catch (e) {
      if (request === detailRequest.current) setError((e as Error).message);
    } finally {
      if (request === detailRequest.current) setLoading(false);
    }
  }
  useEffect(() => {
    setDetails((previous) => {
      if (!previous) return previous;
      const updated = tires.find((t) => t.id === previous.tire.id);
      if (!updated || (updated.version || 0) < previous.tire.version)
        return previous;
      return {
        ...previous,
        tire: {
          ...previous.tire,
          ...updated,
          version: updated.version ?? previous.tire.version,
          stock_location:
            updated.stock_location ?? previous.tire.stock_location,
        },
      };
    });
  }, [tires]);
  useEffect(() => {
    let active = true;
    setMap(null);
    setPosition("");
    if (destination)
      api(`vehicles/${destination}/tire-map`)
        .then((m) => {
          if (active) setMap(m);
        })
        .catch((e) => {
          if (active) setError(e.message);
        });
    return () => {
      active = false;
    };
  }, [destination]);
  const asset = details?.tire,
    occupied = map?.tires.find(
      (t) => t.position === position && t.id !== asset?.id,
    );
  async function start(value: string) {
    setAction("");
    setError("");
    setNotice("");
    if (!asset || !value) return;
    const request = ++detailRequest.current;
    setLoading(true);
    try {
      const fresh: Details = await api(`tires/${asset.id}/lifecycle`);
      if (request !== detailRequest.current) return;
      setDetails(fresh);
      setClient(crypto.randomUUID());
      setDestination(value === "ROTATE" ? fresh.tire.vehicle_id || "" : "");
      setPosition("");
      setAction(value);
    } catch (e) {
      if (request === detailRequest.current) setError((e as Error).message);
    } finally {
      if (request === detailRequest.current) setLoading(false);
    }
  }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!asset) return;
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    const body: Record<string, unknown> = {
      client_id: client,
      action,
      expected_version: asset.version,
      reason: f.get("reason"),
      cost_cents: Math.round(Number(f.get("cost") || 0) * 100),
    };
    if (asset.state === "INSTALADO")
      body.source_km = Number(f.get("source_km"));
    if (["INSTALL", "TRANSFER", "ROTATE"].includes(action)) {
      body.vehicle_id = destination;
      body.position = position;
      body.vehicle_km = Number(f.get("vehicle_km"));
      if (occupied) {
        body.swap_tire_id = occupied.id;
        body.swap_version = occupied.version;
      }
    }
    if (f.has("stock_location")) body.stock_location = f.get("stock_location");
    if (f.has("supplier")) body.supplier = f.get("supplier");
    if (action === "RECAP_RETURN")
      body.new_tread_mm = Number(f.get("new_tread_mm"));
    try {
      await api(`tires/${asset.id}/movements`, body);
      setAction("");
      setNotice(
        "Movimentação concluída. Histórico, estoque e alertas atualizados.",
      );
      await onChange();
      await open(asset.id);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function more() {
    if (!details?.next_cursor) return;
    try {
      const next = await api(
        `tires/${details.tire.id}/lifecycle?before=${details.next_cursor}`,
      );
      setDetails({
        ...next,
        movements: [...details.movements, ...next.movements],
      });
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <div className="tire-inventory">
      <div className="inventory-summary">
        <button
          className={!filter ? "active" : ""}
          onClick={() => setFilter("")}
        >
          <strong>{tires.length}</strong>Todos os pneus
        </button>
        {Object.entries(stateNames).map(([value, label]) => (
          <button
            key={value}
            className={filter === value ? "active" : ""}
            onClick={() => setFilter(value)}
          >
            <strong>{tires.filter((t) => t.state === value).length}</strong>
            {label}
          </button>
        ))}
      </div>
      <TirePolicy manage={manage} onChange={onChange} />
      <label className="inventory-search">
        Buscar número de fogo, série, DOT, marca ou medida
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Digite para localizar um pneu"
        />
      </label>
      {notice && (
        <p className="info" role="status">
          {notice}
        </p>
      )}
      {error && (
        <div className="error" role="alert">
          {error}
        </div>
      )}
      <section className="surface table-wrap">
        <table>
          <thead>
            <tr>
              <th>Pneu</th>
              <th>Marca / medida</th>
              <th>Sulco</th>
              <th>Localização</th>
              <th>Estado</th>
              <th>Gestão</th>
            </tr>
          </thead>
          <tbody>
            {tires
              .filter(
                (t) =>
                  (!filter || t.state === filter) &&
                  `${t.code} ${t.serial_number||""} ${t.dot_code||""} ${t.brand} ${t.size}`
                    .toLowerCase()
                    .includes(search.toLowerCase()),
              )
              .map((t) => (
                <tr key={t.id}>
                  <td>
                    <b>{t.code}</b>
                    <small>Série: {t.serial_number||"Não informada"}</small>
                  </td>
                  <td>
                    {t.brand}
                    <small>{t.size}</small>
                  </td>
                  <td>
                    <span className={`pill ${t.status}`}>{t.tread_mm} mm</span>
                  </td>
                  <td>
                    {t.vehicle_id
                      ? `${vehicles.find((v) => v.id === t.vehicle_id)?.plate || "Veículo"} · ${t.position}`
                      : t.state === "EM_ESTOQUE"
                        ? t.stock_location || "Principal"
                        : "—"}
                  </td>
                  <td>{stateNames[t.state || ""] || t.state}</td>
                  <td>
                    <button
                      className="table-link"
                      disabled={busy || loading}
                      onClick={() => {
                        setAction("");
                        void open(t.id);
                      }}
                    >
                      Ciclo de vida →
                    </button>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
        {!tires.length && (
          <p className="empty-state">
            Cadastre um pneu para iniciar o controle do estoque.
          </p>
        )}
      </section>
      {loading && !details && <p role="status">Carregando histórico…</p>}
      {asset && (
        <section className="surface lifecycle-details">
          <div className="surface-title">
            <div>
              <span className="label">DOSSIÊ DO PNEU</span>
              <h2>
                {asset.code} · {asset.brand}
              </h2>
            </div>
            <button
              className="table-link"
              disabled={busy}
              onClick={() => {
                setDetails(null);
                setAction("");
              }}
            >
              Fechar
            </button>
          </div>
          <p>
            {stateNames[asset.state || ""]} ·{" "}
            {asset.plate
              ? `${asset.plate} / ${asset.position}`
              : asset.stock_location}
          </p>
          <p><strong>{asset.retread_count+1}ª vida</strong> · {asset.retread_count===0?"Banda original":`${asset.retread_count} recapagem(ns)`} · {roadNames[asset.road_use||""]||"Via não informada"}</p>
          <ol className="lifecycle-stages" aria-label="Etapas do ciclo do pneu">{Object.entries(stateNames).map(([state,name])=><li key={state} aria-current={asset.state===state?"step":undefined}>{name}</li>)}</ol>
          <div className="lifecycle-facts">
            <p><strong>Número de fogo</strong><br/>{asset.code}</p>
            <p><strong>Número de série</strong><br/>{asset.serial_number||"Não informado"}</p>
            <p><strong>DOT</strong><br/>{asset.dot_code||"Não informado"}</p>
            <p><strong>Validade operacional</strong><br/>{asset.expires_on?new Date(asset.expires_on.slice(0,10)+"T00:00:00Z").toLocaleDateString("pt-BR",{timeZone:"UTC"}):"Não informada"}</p>
            <p><strong>Posição e lado</strong><br/>{asset.position?positionLabel(asset.position):"Fora do veículo"}</p>
            <p><strong>Motorista do veículo</strong><br/>{asset.driver_name||"Não vinculado"}</p>
          </div>
          {manage&&<TireIdentityEditor tire={asset} onSaved={async()=>{await onChange();await open(asset.id)}}/>}
          <div className="lifecycle-facts">
            <p>
              <strong>{number(asset.total_tracked_km)} km</strong>
              <br />
              Percurso rastreado
            </p>
            <p>
              <strong>{asset.retread_count}</strong>
              <br />
              Recapagens totais, incluindo as anteriores à entrada
            </p>
            <p>
              <strong>
                {currency(
                  Number(asset.acquisition_cost) +
                    asset.movement_cost_cents / 100,
                )}
              </strong>
              <br />
              Aquisição + serviços
            </p>
            <p>
              <strong>{asset.tread_mm} mm</strong>
              <br />
              Sulco registrado
            </p>
          </div>
          <p className="caption">
            A quilometragem usa os odômetros informados a partir do início do
            rastreamento. Não estimamos percursos anteriores.
          </p>
          <div className="lifecycle-actions">
            {role !== "driver" &&
              asset.state !== "DESCARTADO" &&
              asset.state !== "EM_RECAPAGEM" && (
                <button
                  className="button small"
                  onClick={() => {
                    setAction("");
                    onInspect(asset);
                  }}
                >
                  Registrar inspeção
                </button>
              )}
            {move && asset.state !== "DESCARTADO" && (
              <label>
                Movimentar
                <select
                  aria-label="Movimentar"
                  value={action}
                  disabled={busy || loading}
                  onChange={(e) => void start(e.target.value)}
                >
                  <option value="">Selecione uma ação</option>
                  {(asset.state === "INSTALADO"
                    ? ["REMOVE", "ROTATE", "TRANSFER"]
                    : asset.state === "EM_RECAPAGEM"
                      ? ["RECAP_RETURN", "DISCARD"]
                      : ["INSTALL", "STOCK_TRANSFER", "RECAP_SEND", "DISCARD"]
                  ).map((a) => (
                    <option value={a} key={a}>
                      {actions[a]}
                    </option>
                  ))}
                </select>
              </label>
            )}
          </div>
          {action && (
            <form className="lifecycle-form" onSubmit={submit}>
              <h3>{actions[action]}</h3>
              <p>
                Confirme uma operação física realizada pela equipe. O registro
                será permanente no histórico.
              </p>
              <div className="form-grid">
                {asset.state === "INSTALADO" && (
                  <label>
                    Odômetro atual de {asset.plate} (km)
                    <input
                      key={`source-${asset.version}`}
                      name="source_km"
                      type="number"
                      min={asset.current_vehicle_km || 0}
                      defaultValue={asset.current_vehicle_km || 0}
                      required
                    />
                  </label>
                )}
                {["INSTALL", "TRANSFER", "ROTATE"].includes(action) && (
                  <>
                    <label>
                      Veículo de destino
                      <select
                        aria-label="Veículo de destino"
                        value={destination}
                        required
                        disabled={action === "ROTATE"}
                        onChange={(e) => setDestination(e.target.value)}
                      >
                        <option value="">Selecione o veículo</option>
                        {vehicles
                          .filter(
                            (v) =>
                              action !== "TRANSFER" ||
                              v.id !== asset.vehicle_id,
                          )
                          .map((v) => (
                            <option key={v.id} value={v.id}>
                              {v.plate}
                            </option>
                          ))}
                      </select>
                    </label>
                    <label>
                      Posição de destino
                      <select
                        aria-label="Posição de destino"
                        value={position}
                        required
                        onChange={(e) => setPosition(e.target.value)}
                      >
                        <option value="">Selecione a posição</option>
                        {map?.positions
                          .filter(
                            (p) =>
                              destination !== asset.vehicle_id ||
                              p !== asset.position,
                          )
                          .map((p) => {
                            const tire = map.tires.find(
                              (t) => t.position === p,
                            );
                            return (
                              <option key={p} value={p}>
                                {positionLabel(p,map.positions)}{" "}
                                {tire ? `· ${tire.code} (ocupada)` : "· livre"}
                              </option>
                            );
                          })}
                      </select>
                    </label>
                    <label>
                      Odômetro no destino (km)
                      <input
                        key={destination}
                        name="vehicle_km"
                        type="number"
                        min={
                          vehicles.find((v) => v.id === destination)
                            ?.current_km || 0
                        }
                        defaultValue={
                          vehicles.find((v) => v.id === destination)
                            ?.current_km || 0
                        }
                        required
                      />
                    </label>
                  </>
                )}
                {["REMOVE", "RECAP_RETURN", "STOCK_TRANSFER"].includes(
                  action,
                ) && (
                  <label>
                    Local no estoque
                    <input
                      name="stock_location"
                      maxLength={100}
                      defaultValue={asset.stock_location || "Principal"}
                      required
                    />
                  </label>
                )}
                {action === "RECAP_SEND" && (
                  <label>
                    Fornecedor da recapagem
                    <input name="supplier" maxLength={150} required />
                  </label>
                )}
                {action === "RECAP_RETURN" && (
                  <>
                    <label>
                      Sulco após recapagem (mm)
                      <input
                        name="new_tread_mm"
                        type="number"
                        min="1"
                        max="40"
                        step="0.1"
                        required
                      />
                    </label>
                    <label>
                      Fornecedor / documento do serviço
                      <input name="supplier" maxLength={150} />
                    </label>
                  </>
                )}
                <label>
                  Custo do serviço (R$)
                  <input
                    name="cost"
                    type="number"
                    min="0"
                    max="1000000"
                    step="0.01"
                    defaultValue="0"
                    required
                  />
                </label>
              </div>
              {occupied && (
                <div className="info">
                  A posição está ocupada por <strong>{occupied.code}</strong>.{" "}
                  {action === "ROTATE"
                    ? `Os dois pneus trocarão de posição no mesmo registro.`
                    : "Retire o pneu da posição antes de instalar ou transferir."}
                </div>
              )}
              {occupied && action === "ROTATE" && (
                <label className="checkbox">
                  <input type="checkbox" required />
                  Confirmo o rodízio entre {asset.code} e {occupied.code}.
                </label>
              )}
              {action === "DISCARD" && (
                <label className="checkbox">
                  <input type="checkbox" required />
                  Confirmo a avaliação técnica e o descarte definitivo. O pneu
                  não poderá voltar à operação.
                </label>
              )}
              <label>
                Motivo / observações
                <input
                  name="reason"
                  minLength={5}
                  maxLength={500}
                  required
                  placeholder="Descreva a operação e a avaliação realizada"
                />
              </label>
              <button
                className="button small"
                disabled={busy || (!!occupied && action !== "ROTATE")}
              >
                {busy ? "Registrando…" : "Confirmar movimentação"}
              </button>
              <button
                className="table-link"
                type="button"
                disabled={busy}
                onClick={() => setAction("")}
              >
                Cancelar
              </button>
            </form>
          )}
          <h3>Histórico de movimentações</h3>
          <ol className="lifecycle-timeline">
            {details.movements.map((m) => (
              <li key={m.id}>
                <div>
                  <strong>{actions[m.action] || m.action}</strong>
                  <time>{new Date(m.created_at).toLocaleString("pt-BR")}</time>
                </div>
                <p>
                  {m.from_plate
                    ? `${m.from_plate} / ${m.from_position}`
                    : stateNames[m.from_state || ""] || "Entrada"}{" "}
                  →{" "}
                  {m.to_plate
                    ? `${m.to_plate} / ${m.to_position}`
                    : stateNames[m.to_state]}
                  {m.to_state === "EM_ESTOQUE" ? ` · ${m.stock_location}` : ""}
                </p>
                <p>{m.reason}</p>
                {(m.life_number||m.road_use||m.from_driver_name||m.to_driver_name)&&<p>{m.life_number?`${m.life_number}ª vida · `:""}{m.road_use?`${roadNames[m.road_use]||m.road_use}`:""}{m.from_driver_name?` · motorista na saída: ${m.from_driver_name}`:""}{m.to_driver_name?` · motorista na entrada: ${m.to_driver_name}`:""}</p>}
                <small>
                  {m.actor_name || "Sistema"} · {number(m.tracked_km)} km
                  rastreados · sulco {m.tread_mm} mm
                  {m.source_km !== null
                    ? ` · odômetro de saída ${number(m.source_km)} km`
                    : ""}
                  {m.destination_km !== null
                    ? ` · odômetro de entrada ${number(m.destination_km)} km`
                    : ""}
                  {m.supplier ? ` · ${m.supplier}` : ""}
                  {m.cost_cents
                    ? ` · custo ${currency(m.cost_cents / 100)}`
                    : ""}
                </small>
              </li>
            ))}
          </ol>
          {details.next_cursor && (
            <button className="table-link" onClick={more}>
              Carregar movimentações anteriores
            </button>
          )}
        </section>
      )}
    </div>
  );
}
