"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { api } from "./api";
import { date, money, Plan } from "./billing-panel";

type Client = {
  id: string; name: string; cnpj: string; owner_email: string; status: string;
  plan_id: string | null; vehicles: number; users: number; vehicle_limit: number;
  user_limit: number; trial_ends_at: string; paid_until: string | null; pending_pix?: boolean;
};
type Invoice = {
  id: string; plan_id: string; price_cents: number; status: string; due_date: string;
  paid_at: string | null; source?: string; environment: string; version?: number;
  reference?: string; payment_notified?: boolean;
};
type Detail = {
  company: { id: string; name: string; cnpj: string; deleted_at: string | null };
  subscription: { plan_id: string | null; paid_until: string | null; updated_at: string };
  users: { id: string; name: string; email: string; role: string; deleted_at: string | null }[];
  invoices: Invoice[];
};

const states: Record<string, string> = {
  active: "Ativa", trial: "Em avaliação", suspended: "Suspensa", expired: "Vencida", closed: "Encerrada",
};
const invoiceStates: Record<string, string> = {
  creating: "Em geração", pending: "Pendente", paid: "Pago", overdue: "Vencido", cancelled: "Cancelado", refunded: "Estornado",
};

export default function PlatformClients({ plans, onChange, pendingOnly }: {
  plans: Plan[]; onChange: () => Promise<void>; pendingOnly: boolean;
}) {
  const initialQuery = pendingOnly ? "status=pending_pix" : "";
  const [clients, setClients] = useState<Client[]>([]);
  const [next, setNext] = useState("");
  const [query, setQuery] = useState(initialQuery);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [payment, setPayment] = useState<Invoice | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const paymentForm = useRef<HTMLFormElement>(null);
  const planName = (id: string | null) => plans.find(p => p.id === id)?.name || id || "Avaliação Starter";

  async function load(q = query, append = false) {
    const out = await api(`platform/clients?${q}${append ? `&before=${next}` : ""}`);
    setClients(old => append ? [...old, ...out.data] : out.data);
    setNext(out.next_cursor);
  }

  async function run(work: () => Promise<void>) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await work();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }

  useEffect(() => { void run(() => load(initialQuery)); }, [initialQuery]);
  const detailOpen = !!detail;
  useEffect(() => {
    if (detailOpen) dialog.current?.showModal();
    else dialog.current?.close();
  }, [detailOpen]);
  useEffect(() => {
    if (payment) paymentForm.current?.scrollIntoView({ block: "nearest" });
  }, [payment]);
  useEffect(() => {
    if (error || notice) dialog.current?.scrollTo({ top: 0 });
  }, [error, notice]);

  function open(id: string) {
    void run(async () => {
      setPayment(null);
      setDetail(await api(`platform/clients/${id}`));
    });
  }

  function filter(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const q = new URLSearchParams(Object.fromEntries(new FormData(e.currentTarget)) as Record<string, string>).toString();
    void run(async () => {
      await load(q);
      setQuery(q);
    });
  }

  function saveAccess(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!detail) return;
    const form = new FormData(e.currentTarget);
    const id = detail.company.id;
    void run(async () => {
      await api(`platform/clients/${id}/access`, {
        plan_id: form.get("plan_id"), days: Number(form.get("days")), reason: form.get("reason"),
        expected_updated_at: detail.subscription.updated_at,
      });
      setNotice("Plano e vigência atualizados.");
      await refreshClient(id);
    });
  }

  async function refreshClient(id: string) {
    await onChange();
    setDetail(await api(`platform/clients/${id}`));
    await load();
  }

  function change(id: string, operation: string) {
    if (lock.current) return;
    const reason = window.prompt("Justificativa desta alteração (mínimo 5 caracteres):");
    if (!reason) return;
    void run(async () => {
      await api("platform/change", { target: id, operation, reason, payload: operation === "trial" ? { days: 14 } : {} });
      setNotice("Alteração registrada na auditoria.");
      await load();
      await onChange();
    });
  }

  function closeCompany(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!detail) return;
    const form = Object.fromEntries(new FormData(e.currentTarget));
    void run(async () => {
      await api(`platform/clients/${detail.company.id}/close`, form);
      setDetail(null);
      setNotice("Cliente encerrado. Histórico arquivado.");
      await load();
      await onChange();
    });
  }

  function approve(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!detail || !payment) return;
    const form = new FormData(e.currentTarget);
    const id = detail.company.id;
    void run(async () => {
      await api(`platform/finance/${payment.id}`, {
        operation: "pay", expected_version: payment.version, reference: form.get("reference"),
        paid_on: form.get("paid_on"), reason: form.get("reason"),
      }, "PATCH");
      setPayment(null);
      setNotice("Recebimento confirmado e um mês de acesso aplicado ao cliente.");
      await refreshClient(id);
    });
  }

  function closeDetail() {
    if (lock.current) return;
    setDetail(null);
    setPayment(null);
  }

  const feedback = <>
    {error && <p className="error" role="alert">{error}</p>}
    {notice && <p className="success" role="status">{notice}</p>}
  </>;

  return <>
    <section className="surface management-section">
      <h2>Clientes e empresas</h2>
      <form className="report-filters" onSubmit={filter}>
        <label>Buscar clientes<input name="search" maxLength={150} placeholder="Empresa, CNPJ ou e-mail" /></label>
        <label>Situação<select name="status" defaultValue={pendingOnly ? "pending_pix" : ""}>
          <option value="">Todas as empresas em operação</option>
          <option value="pending_pix">Pix aguardando conferência</option>
          {Object.entries(states).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select></label>
        <button className="button" disabled={busy}>Buscar clientes</button>
      </form>
      {!detail && feedback}
      <div className="table-wrap">
        <table>
          <thead><tr><th>Empresa</th><th>CNPJ</th><th>Contato</th><th>Situação</th><th>Plano</th><th>Uso</th><th>Vigência</th><th>Ações</th></tr></thead>
          <tbody>{clients.map(client => <tr key={client.id}>
            <td><button type="button" className="table-link" disabled={busy} onClick={() => open(client.id)}>{client.name}</button></td>
            <td>{client.cnpj}</td><td>{client.owner_email}</td>
            <td>{states[client.status]}{client.pending_pix && <small>Pix aguardando conferência</small>}</td>
            <td>{planName(client.plan_id)}</td>
            <td>{client.vehicles}/{client.vehicle_limit} veículos · {client.users}/{client.user_limit} usuários</td>
            <td>{date(client.paid_until || client.trial_ends_at)}</td>
            <td><div className="commercial-actions">
              <button className="table-link" disabled={busy} onClick={() => open(client.id)}>
                {client.pending_pix ? "Conferir pagamento" : client.status === "closed" ? "Ver ficha" : "Editar / excluir"}
              </button>
              {client.status !== "closed" && <>
                <button className="table-link" disabled={busy} onClick={() => change(client.id, client.status === "suspended" ? "restore" : "suspend")}>
                  {client.status === "suspended" ? "Restaurar" : "Suspender"}
                </button>
                <button className="table-link" disabled={busy} onClick={() => change(client.id, "trial")}>Prorrogar 14 dias</button>
              </>}
            </div></td>
          </tr>)}</tbody>
        </table>
      </div>
      {!clients.length && !busy && <p>Nenhuma empresa encontrada.</p>}
      {next && <button className="table-link" disabled={busy} onClick={() => void run(() => load(query, true))}>Mais clientes</button>}
    </section>
    <dialog ref={dialog} className="tire-dossier-dialog client-detail" aria-label="Ficha do cliente" onCancel={e => { e.preventDefault(); closeDetail(); }}>
      {detail && <>
        <div className="dossier-toolbar"><h2>{detail.company.name}</h2><button className="table-link" disabled={busy} onClick={closeDetail}>Fechar ficha</button></div>
        {feedback}
        <p>CNPJ {detail.company.cnpj} · {planName(detail.subscription.plan_id)} · Vigência paga até {date(detail.subscription.paid_until)}</p>
        <h3>Usuários</h3>
        {detail.users.map(user => <p key={user.id}>{user.name} · {user.email} · {user.role}{user.deleted_at ? " · encerrado" : ""}</p>)}
        <h3>Histórico de pagamentos</h3>
        {!detail.invoices.length && <p>Nenhuma fatura registrada.</p>}
        {!!detail.invoices.length && <div className="table-wrap"><table>
          <thead><tr><th>Plano</th><th>Valor</th><th>Vencimento</th><th>Pagamento</th><th>Situação</th><th>Referência</th><th>Ações</th></tr></thead>
          <tbody>{detail.invoices.map(invoice => {
            const openInvoice = ["pending", "overdue"].includes(invoice.status);
            return <tr key={invoice.id}>
              <td>{planName(invoice.plan_id)}</td><td>{money(invoice.price_cents)}</td>
              <td>{date(invoice.due_date)}</td><td>{date(invoice.paid_at)}</td>
              <td>{invoice.payment_notified && openInvoice ? "Pix informado" : invoiceStates[invoice.status] || invoice.status}</td>
              <td>{invoice.reference || "—"}</td>
              <td>{!detail.company.deleted_at && openInvoice && invoice.source === "manual" && invoice.environment === "production" &&
                <button className="table-link" disabled={busy} onClick={() => { setError(""); setNotice(""); setPayment(invoice); }}>Aprovar pagamento</button>}</td>
            </tr>;
          })}</tbody>
        </table></div>}
        {payment && <form ref={paymentForm} key={payment.id} className="surface management-section" onSubmit={approve}>
          <h3>Confirmar pagamento de {money(payment.price_cents)}</h3>
          <p>Confira o recebimento no extrato antes de confirmar.</p>
          <fieldset disabled={busy} className="plain-fields">
            <label>Referência do recebimento<input name="reference" required minLength={3} maxLength={200} defaultValue={payment.reference} /></label>
            <label>Data do pagamento<input name="paid_on" type="date" required max={new Intl.DateTimeFormat("sv-SE", { timeZone: "America/Sao_Paulo" }).format(new Date())} /></label>
            <label>Justificativa<textarea name="reason" required minLength={5} maxLength={500} /></label>
            <button className="button">Confirmar recebimento</button>{" "}
            <button className="table-link" type="button" onClick={() => setPayment(null)}>Cancelar</button>
          </fieldset>
        </form>}
        {!detail.company.deleted_at && <>
          <h3>Editar plano e vigência</h3>
          <form key={detail.subscription.updated_at} onSubmit={saveAccess}>
            <fieldset disabled={busy} className="plain-fields">
              <div className="form-grid">
                <label>Plano do cliente<select name="plan_id" defaultValue={detail.subscription.plan_id || "starter"}>
                  {plans.map(plan => <option key={plan.id} value={plan.id}>{plan.name}</option>)}
                </select></label>
                <label>Dias de acesso a partir de hoje<input name="days" type="number" min={1} max={366} defaultValue={30} required /></label>
              </div>
              <label>Justificativa da vigência<textarea name="reason" minLength={5} maxLength={500} required /></label>
              <button className="button">Salvar acesso ao plano</button>
            </fieldset>
          </form>
          <details>
            <summary>Excluir cliente da operação</summary>
            <p>Revoga o acesso dos usuários e arquiva o histórico. Regularize as faturas em aberto primeiro.</p>
            <form onSubmit={closeCompany}>
              <fieldset disabled={busy} className="plain-fields">
                <label>Confirme o CNPJ da empresa<input name="confirmation" required autoComplete="off" /></label>
                <label>Justificativa do encerramento<textarea name="reason" minLength={5} maxLength={500} required /></label>
                <label className="checkbox"><input type="checkbox" required />Confirmo o encerramento desta empresa.</label>
                <button className="button danger">Encerrar este cliente</button>
              </fieldset>
            </form>
          </details>
        </>}
      </>}
    </dialog>
  </>;
}
