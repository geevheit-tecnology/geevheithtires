"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import { planCatalog, provisionalPrice } from "../lib/plan-catalog";
import { PlanBenefits, usePlanAccess } from "./plan-access";
import QRCode from "qrcode";
import { api } from "./api";

export type Plan = {
  id: string;
  name: string;
  price_cents: number;
  vehicle_limit: number;
  user_limit: number;
  published: boolean;
};

type PixCharge = {
  invoice_id: string;
  plan_id: string;
  plan_name?: string;
  price_cents: number;
  due_date: string;
  reference: string;
  txid: string;
  pix_payload: string;
  payment_notified?: boolean;
  pix_key: string;
  receiver_name: string;
  bank_code: string;
  bank_name: string;
  agency: string;
  account: string;
};

type Invoice = {
  id: string;
  plan_id: string;
  price_cents: number;
  status: string;
  checkout_url: string | null;
  due_date: string;
  environment: string;
  source?: string;
  reference?: string;
  paid_at?: string | null;
};

type Billing = {
  status: string;
  trial_ends_at: string;
  paid_until: string | null;
  vehicle_limit: number;
  user_limit: number;
  vehicles: number;
  users: number;
  payments_enabled: boolean;
  pix_enabled?: boolean;
  active_pix?: PixCharge | null;
  environment: string;
  plans: Plan[];
  invoices: Invoice[];
};

export const money = (cents: number) =>
  new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(
    cents / 100,
  );

export const date = (v: string | null) =>
  v ? new Date(v).toLocaleDateString("pt-BR", { timeZone: "UTC" }) : "—";

const states: Record<string, string> = {
  trial: "Em avaliação",
  active: "Assinatura ativa",
  expired: "Avaliação ou assinatura encerrada",
  suspended: "Empresa suspensa",
  creating: "Em conciliação",
  pending: "Aguardando pagamento",
  paid: "Pago",
  overdue: "Vencido",
  refunded: "Estornado ou contestado",
  cancelled: "Cancelado",
};

export default function BillingPanel({ owner }: { owner: boolean }) {
  const { access, refresh } = usePlanAccess();
  const [data, setData] = useState<Billing | null>(null);
  const [pix, setPix] = useState<PixCharge | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [copied, setCopied] = useState("");
  const [busy, setBusy] = useState("");
  const [notifying, setNotifying] = useState(false);

  const paymentLock = useRef(false);
  const [qr, setQr] = useState("");
  const [reported, setReported] = useState("");
  const payload = pix?.pix_payload;
  const paymentReported = !!pix && (pix.payment_notified || reported === pix.invoice_id);
  const pixBox = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let active = true;
    setQr("");
    if (payload) void QRCode.toDataURL(payload, { width: 240, margin: 4 })
      .then(url => { if (active) setQr(url); })
      .catch(() => { if (active) setError("Falha ao gerar o QR Code. Use o Pix Copia e Cola."); });
    return () => { active = false; };
  }, [payload]);

  async function sharePix(charge: PixCharge) {
    const text = `Pix GeevheitTire · ${charge.plan_name || data?.plans.find(plan => plan.id === charge.plan_id)?.name || charge.plan_id} · ${money(charge.price_cents)}
Chave: ${charge.pix_key}
Pix Copia e Cola: ${charge.pix_payload}`;
    if (!navigator.share) {
      await copyText("share", text);
      return;
    }
    try {
      await navigator.share({ title: "Pagamento Pix GeevheitTire", text });
    } catch (e) {
      if ((e as Error).name !== "AbortError") setError("Falha ao compartilhar. Copie os dados do Pix.");
    }
  }

  async function load() {
    try {
      const res: Billing = await api("billing");
      setData(res);
      if (res.active_pix) setPix(res.active_pix);
      else setPix(null);
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    }
  }

  useEffect(() => {
    const refreshOnFocus = () => {
      if (document.visibilityState === "visible" && !paymentLock.current) void load();
    };
    refreshOnFocus();
    const timer = setInterval(refreshOnFocus, 15000);
    window.addEventListener("focus", refreshOnFocus);
    return () => {
      clearInterval(timer);
      window.removeEventListener("focus", refreshOnFocus);
    };
  }, []);

  async function copyText(label: string, value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      setTimeout(() => setCopied(""), 2500);
    } catch {
      setError("Não foi possível copiar automaticamente. Selecione e copie o texto manualmente.");
    }
  }

  async function startPayment(planId: string, checkout = false) {
    if (paymentLock.current) return;
    if (!checkout && pix?.plan_id === planId) {
      pixBox.current?.scrollIntoView({ block: "center" });
      return;
    }
    paymentLock.current = true;
    setBusy(planId);
    setError("");
    setNotice("");
    try {
      if (checkout || (!data?.pix_enabled && data?.payments_enabled)) {
        const res = await api("billing/checkout", { plan_id: planId });
        window.location.assign(res.checkout_url);
        return;
      }
      const charge: PixCharge = await api("billing/pix", { plan_id: planId });
      setPix(charge);
      setNotice(
        `PIX gerado com identificador ${charge.reference}. Faça o pagamento e clique em "Já fiz o PIX" para agilizar a conferência e liberação do acesso.`,
      );
      requestAnimationFrame(() => pixBox.current?.scrollIntoView({ block: "center" }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      paymentLock.current = false;
      setBusy("");
    }
  }

  async function notifyPaid(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!pix || paymentLock.current || paymentReported) return;
    paymentLock.current = true;
    const form = e.currentTarget;
    const note = String(new FormData(form).get("note") || "").trim();
    setNotifying(true);
    setError("");
    setNotice("");
    try {
      await api("billing/pix/notify", { invoice_id: pix.invoice_id, note });
      setNotice(
        `Administração notificada sobre o pagamento ${pix.reference}. Assim que o recebimento for conferido no PicPay, seu acesso será liberado automaticamente.`,
      );
      setReported(pix.invoice_id);
      form.reset();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      paymentLock.current = false;
      setNotifying(false);
    }
  }

  return (
    <section className="surface plan-status">
      <span className="label">PLANO E COBRANÇA</span>
      <h2>{data ? states[data.status] : "Sua assinatura"}</h2>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="success" role="status">
          {notice}
        </p>
      )}
      {!data ? (
        <button className="button secondary" onClick={() => void load()}>
          Atualizar
        </button>
      ) : (
        <>
          <p>
            <strong>Plano em uso: {access.plan_name}.</strong>{" "}
            {data.status === "trial"
              ? "A avaliação libera os recursos iniciais do plano."
              : "Confira abaixo os benefícios disponíveis para sua operação."}
          </p>
          <p>
            Avaliação até {date(data.trial_ends_at)}
            {data.paid_until ? ` · Acesso pago até ${date(data.paid_until)}` : ""}.
          </p>
          <p>
            <strong>
              {data.vehicles} / {data.vehicle_limit}
            </strong>{" "}
            veículos ·{" "}
            <strong>
              {data.users} / {data.user_limit}
            </strong>{" "}
            usuários, incluindo convites.
          </p>
          {["expired", "suspended"].includes(data.status) && (
            <div className="info">
              Seu acesso está disponível para consulta. Gere o PIX abaixo ou entre em contato com a administração para regularizar a empresa.
            </div>
          )}
          {data.pix_enabled && (
            <div className="info">
              <strong>Pagamento via PIX manual:</strong> clique em{" "}
              <b>Gerar PIX para pagar</b> no plano desejado, faça o PIX usando a
              chave ou o código Copia e Cola e clique em{" "}
              <b>Já fiz o PIX</b>. A administração confere o identificador e
              libera 30 dias de acesso.
            </div>
          )}

          {pix && (
            <div ref={pixBox} className="pix-box" role="region" aria-label="Dados para pagamento via PIX">
              <div className="pix-box-head">
                <div>
                  <span className="label">COBRANÇA PIX EM ABERTO</span>
                  <h3>
                    {money(pix.price_cents)} · Vence em {date(pix.due_date)}
                  </h3>
                </div>
                <span className="pill alerta">Identificador: {pix.reference}</span>
              </div>

              {qr && <img className="pix-qr" src={qr} width={240} height={240} alt="QR Code para pagar por Pix" />}
              <button type="button" className="button small" onClick={() => void sharePix(pix)}>
                {copied === "share" ? "Dados copiados" : "Compartilhar dados do Pix"}
              </button>
              <div className="pix-grid">
                <div className="pix-field">
                  <small>Chave PIX (E-mail)</small>
                  <strong>{pix.pix_key}</strong>
                  <button
                    type="button"
                    className="button outline small"
                    onClick={() => void copyText("chave", pix.pix_key)}
                  >
                    {copied === "chave" ? "Chave copiada!" : "Copiar chave PIX"}
                  </button>
                </div>
                <div className="pix-field">
                  <small>Identificador para conferência</small>
                  <strong>{pix.reference}</strong>
                  <button
                    type="button"
                    className="button outline small"
                    onClick={() => void copyText("ref", pix.reference)}
                  >
                    {copied === "ref" ? "Identificador copiado!" : "Copiar identificador"}
                  </button>
                </div>
              </div>

              <div className="pix-field">
                <small>PIX Copia e Cola (já com valor e identificador)</small>
                <code className="pix-payload">{pix.pix_payload}</code>
                <button
                  type="button"
                  className="button small"
                  onClick={() => void copyText("payload", pix.pix_payload)}
                >
                  {copied === "payload" ? "Código PIX copiado!" : "Copiar PIX Copia e Cola"}
                </button>
              </div>

              <p className="pix-bank-info">
                <strong>Dados bancários:</strong> Banco {pix.bank_code} ·{" "}
                {pix.bank_name} · Agência {pix.agency} · Conta corrente{" "}
                {pix.account} · Favorecido: {pix.receiver_name}
              </p>

              {owner && (
                <form className="pix-notify-form" onSubmit={(e) => void notifyPaid(e)}>
                  <label>
                    Já realizou o PIX? Informe uma observação ou código do comprovante (opcional)
                    <input
                      type="text"
                      name="note"
                      maxLength={400}
                      placeholder="Ex: Pago às 14:30 via PicPay / Banco do Brasil"
                    />
                  </label>
                  <button className="button" disabled={notifying || !!busy || paymentReported}>
                    {notifying
                      ? "Avisando administração…"
                      : paymentReported ? "Pagamento informado" : "Já fiz o PIX — avisar administração"}
                  </button>
                </form>
              )}
            </div>
          )}

          <p className="pricing-note">
            Os planos publicados são definidos pela administração. Mensalidades
            registradas mantêm o valor e os limites originais.
          </p>

          <div className="commercial-grid">
            {planCatalog.map((p) => {
              const offer = data.plans.find((o) => o.id === p.id);
              const displayName = offer?.name || p.name;
              const isCurrentPix = pix?.plan_id === p.id;
              return (
                <article
                  className={`surface commercial-plan ${access.plan_id === p.id ? "current-plan" : ""}`}
                  key={p.id}
                >
                  <span className="plan-level">
                    {access.plan_id === p.id ? "SEU PLANO" : displayName}
                  </span>
                  <h3>{displayName}</h3>
                  <p>{p.summary}</p>
                  <h2>
                    {offer ? money(offer.price_cents) : provisionalPrice}
                    <small> / mês</small>
                  </h2>
                  <p>
                    {p.vehicle_limit
                      ? `Até ${offer?.vehicle_limit || p.vehicle_limit} veículos e ${offer?.user_limit || p.user_limit} usuários.`
                      : "Limites definidos em contrato; contratação sob consulta."}
                  </p>
                  <PlanBenefits features={p.features} planned={p.planned} />
                  <button
                    className="button primary"
                    disabled={
                      !offer ||
                      !owner ||
                      !!busy || notifying ||
                      data.status === "suspended"
                    }
                    onClick={() => void startPayment(p.id)}
                  >
                    {busy === p.id
                      ? "Gerando PIX…"
                      : !offer
                        ? "Sob consulta"
                        : data.payments_enabled && !data.pix_enabled
                          ? "Pagar mensalidade"
                          : isCurrentPix
                            ? "Ver dados do PIX gerado"
                            : "Gerar PIX para pagar"}
                  </button>
                  {offer && owner && data.payments_enabled && data.pix_enabled && (
                    <button type="button" className="button outline" disabled={!!busy || notifying || !!pix || data.status === "suspended"}
                      onClick={() => void startPayment(p.id, true)}>Pagar pelo gateway</button>
                  )}
                </article>
              );
            })}
          </div>

          {!owner && (
            <p>A contratação e geração de PIX são exclusivas do proprietário da empresa.</p>
          )}

          <h3>Histórico de mensalidades</h3>
          {data.invoices.length === 0 ? (
            <p>Nenhuma cobrança gerada.</p>
          ) : (
            <div className="commercial-scroll">
              <table className="commercial-table">
                <thead>
                  <tr>
                    <th>Plano</th>
                    <th>Valor</th>
                    <th>Vencimento</th>
                    <th>Situação</th>
                    <th>Identificador / Pagamento</th>
                  </tr>
                </thead>
                <tbody>
                  {data.invoices.map((i) => (
                    <tr key={i.id}>
                      <td>
                        {i.plan_id}
                        {i.environment === "sandbox" ? " (teste)" : ""}
                      </td>
                      <td>{money(i.price_cents)}</td>
                      <td>{date(i.due_date)}</td>
                      <td>{states[i.status]}</td>
                      <td>
                        {i.reference && (
                          <small>
                            <strong>{i.reference}</strong>
                          </small>
                        )}
                        {i.source === "manual" && <small>PIX / Registro manual</small>}
                        {i.paid_at && <small>Recebido em {date(i.paid_at)}</small>}
                        {i.checkout_url &&
                        ["pending", "overdue"].includes(i.status) ? (
                          <a
                            href={i.checkout_url}
                            target="_blank"
                            rel="noopener noreferrer"
                          >
                            Abrir fatura
                          </a>
                        ) : null}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <button className="button secondary" onClick={() => void load()}>
            Atualizar situação
          </button>
        </>
      )}
    </section>
  );
}
