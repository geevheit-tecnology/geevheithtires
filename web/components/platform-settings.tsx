"use client";
import { FormEvent, useEffect, useState } from "react";
import { LogOut, ShieldCheck } from "lucide-react";
import { api } from "./api";
import { PlanBenefits } from "./plan-access";
import { planCatalog } from "../lib/plan-catalog";
import type { AdminPlan, AdminIdentity } from "./platform-admin";

export function AdminPlans({
  plans,
  onChange,
}: {
  plans: AdminPlan[];
  onChange: () => Promise<void>;
}) {
  const starterTrial = plans.find((p) => p.id === "starter")?.trial_days ?? plans[0]?.trial_days ?? 14;
  const [globalTrialDays, setGlobalTrialDays] = useState(starterTrial);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");

  useEffect(() => {
    setGlobalTrialDays(starterTrial);
  }, [starterTrial]);

  async function saveGlobalTrial(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const nextDays = Number(f.get("global_trial_days") || globalTrialDays || 14);
    const customReason = String(f.get("trial_reason") || "").trim();
    const reason =
      customReason.length >= 5
        ? customReason
        : `Período de teste gratuito alterado para ${nextDays} dias em toda a plataforma`;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await api("platform/change", {
        target: "starter",
        operation: "set_trial_days",
        reason,
        payload: { days: nextDays },
      });
      setNotice(
        `Período de teste gratuito atualizado para ${nextDays} dias em todo o sistema (site público, cadastro, painel de clientes e empresas em avaliação).`,
      );
      await onChange();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function save(e: FormEvent<HTMLFormElement>, p: AdminPlan) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const nextName = String(f.get("name") || "").trim();
      const rawPrice = String(f.get("price") || "0").replace(",", ".");
      const nextPriceCents = Math.max(0, Math.round(Number(rawPrice) * 100));
      const nextTrialDays = Number(f.get("trial_days") || p.trial_days || 14);
      const isPublished = f.get("published") === "on";
      const customReason = String(f.get("reason") || "").trim();
      const reason =
        customReason.length >= 5
          ? customReason
          : `Atualização do plano ${nextName || p.name} (${nextTrialDays} dias de teste)`;
      const out = await api(
        `platform/plans/${p.id}`,
        {
          expected_version: p.version,
          name: nextName,
          price_cents: nextPriceCents,
          vehicle_limit: Number(f.get("vehicle_limit")),
          user_limit: Number(f.get("user_limit")),
          trial_days: nextTrialDays,
          published: isPublished,
          apply_limits: f.get("apply_limits") === "on",
          reason,
        },
        "PUT",
      );
      const savedName = out.name || nextName || p.name;
      const savedTrial = out.trial_days || nextTrialDays;
      setNotice(
        `Plano ${savedName} atualizado com sucesso (${savedTrial} dias de teste gratuito em todo o sistema). ${isPublished ? "Condições publicadas na plataforma." : "Plano salvo (marque 'Publicar na plataforma' para exibir o valor mensal no site)."}${out.clients_updated ? ` Limites sincronizados em ${out.clients_updated} cliente(s).` : ""}`,
      );
      await onChange();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <section className="surface management-section" style={{ marginBottom: 20 }}>
        <div className="admin-section-heading">
          <div>
            <span className="label">CONFIGURAÇÃO GLOBAL DE AVALIAÇÃO</span>
            <h2>Período de teste gratuito da plataforma ({starterTrial} dias atuais)</h2>
            <p>
              Ao alterar os dias de teste abaixo, o novo prazo passa a valer automaticamente em todo o sistema: site público, página de cadastro, termos, painel dos clientes e empresas em período de avaliação.
            </p>
          </div>
          <ShieldCheck size={22} />
        </div>
        <form onSubmit={(e) => void saveGlobalTrial(e)}>
          <fieldset className="plain-fields" disabled={busy}>
            <div className="form-grid">
              <label>
                Tempo de teste gratuito (em dias)
                <input
                  type="number"
                  name="global_trial_days"
                  min="1"
                  max="365"
                  value={globalTrialDays}
                  onChange={(e) => setGlobalTrialDays(Number(e.target.value))}
                  required
                />
              </label>
              <label>
                Observação para auditoria (opcional)
                <input
                  type="text"
                  name="trial_reason"
                  maxLength={500}
                  placeholder="Ex.: Campanha comercial ou ajuste padrão do teste"
                />
              </label>
            </div>
            <button className="button" disabled={busy}>
              {busy ? "Atualizando sistema…" : `Aplicar ${globalTrialDays || starterTrial} dias de teste em todo o sistema`}
            </button>
          </fieldset>
        </form>
      </section>

      <div className="info admin-publication-note">
        <ShieldCheck size={20} />
        <p>
          Edite abaixo o nome, o preço mensal (use 0 para &ldquo;Sob consulta&rdquo;), os limites de veículos/usuários e o tempo de teste de cada plano.
        </p>
      </div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="success">
          {notice}
        </p>
      )}
      <div className="admin-plan-grid">
        {plans.map((p) => {
          const spec = planCatalog.find((s) => s.id === p.id);
          return (
            <article
              className="surface admin-plan-card"
              key={`${p.id}-${p.version}-${p.price_cents}-${p.trial_days ?? starterTrial}-${p.published}`}
            >
              <div className="admin-plan-heading">
                <span className="label">
                  {p.id === "enterprise"
                    ? "CONDIÇÕES PERSONALIZÁVEIS"
                    : "PLANO DA PLATAFORMA"}
                </span>
                <h2>{p.name}</h2>
                <span
                  className={`admin-status ${p.published ? "positive" : "neutral"}`}
                >
                  {p.published
                    ? p.price_cents > 0
                      ? "Publicado no site"
                      : "Publicado (Sob consulta)"
                    : "Em rascunho"}
                </span>
                <p>{spec?.summary}</p>
              </div>
              <form onSubmit={(e) => void save(e, p)}>
                <fieldset className="plain-fields" disabled={busy}>
                  <label>
                    Nome do plano
                    <input
                      type="text"
                      name="name"
                      minLength={2}
                      maxLength={60}
                      defaultValue={p.name}
                      required
                    />
                  </label>
                  <div className="form-grid">
                    <label>
                      Valor mensal (R$) — 0 = Sob consulta
                      <input
                        type="number"
                        name="price"
                        step="0.01"
                        min="0"
                        max="1000000"
                        defaultValue={p.price_cents / 100}
                        onInput={(ev) => {
                          const val = Number((ev.currentTarget as HTMLInputElement).value);
                          const form = ev.currentTarget.form;
                          const pub = form?.elements.namedItem("published") as HTMLInputElement | null;
                          if (pub && val > 0 && !p.published) {
                            pub.checked = true;
                          }
                        }}
                        required
                      />
                    </label>
                    <label>
                      Tempo de teste grátis (dias)
                      <input
                        type="number"
                        name="trial_days"
                        min="1"
                        max="365"
                        defaultValue={p.trial_days ?? starterTrial}
                        required
                      />
                    </label>
                  </div>
                  <div className="form-grid">
                    <label>
                      Limite de veículos
                      <input
                        type="number"
                        name="vehicle_limit"
                        min="1"
                        max="100000"
                        defaultValue={p.vehicle_limit}
                        required
                      />
                    </label>
                    <label>
                      Limite de usuários
                      <input
                        type="number"
                        name="user_limit"
                        min="1"
                        max="10000"
                        defaultValue={p.user_limit}
                        required
                      />
                    </label>
                  </div>
                  <label className="checkbox">
                    <input
                      type="checkbox"
                      name="published"
                      defaultChecked={p.published}
                    />
                    <span>
                      Publicar plano e limites na plataforma
                      <small>
                        Exibe o plano no site e para os clientes (valores R$ 0 aparecem como Sob consulta).
                      </small>
                    </span>
                  </label>
                  <label className="checkbox">
                    <input type="checkbox" name="apply_limits" defaultChecked />
                    <span>
                      Atualizar limites dos clientes deste plano
                      <small>
                        Sincroniza os novos limites de veículos e usuários com os clientes atuais deste plano.
                      </small>
                    </span>
                  </label>
                  <label>
                    Observação / justificativa (opcional)
                    <input
                      type="text"
                      name="reason"
                      maxLength={500}
                      placeholder="Ex.: Ajuste comercial de preços e limites"
                    />
                  </label>
                  <button className="button" disabled={busy}>
                    {busy ? "Salvando…" : `Salvar plano ${p.name}`}
                  </button>
                </fieldset>
              </form>
              {spec && (
                <details className="admin-benefits">
                  <summary>Recursos e benefícios do {p.name}</summary>
                  <PlanBenefits
                    features={spec.features}
                    planned={spec.planned}
                  />
                </details>
              )}
            </article>
          );
        })}
      </div>
    </>
  );
}
export function AdminProfile({
  profile,
  onChange,
  onLogout,
}: {
  profile: AdminIdentity;
  onChange: () => Promise<void>;
  onLogout: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget,
      f = new FormData(form);
    if (f.get("new_password") !== f.get("confirmation")) {
      setError("A confirmação da nova senha não corresponde.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api(
        "platform/profile",
        {
          name: f.get("name"),
          current_password: f.get("current_password"),
          new_password: f.get("new_password") || "",
        },
        "PATCH",
      );
      form.reset();
      setNotice(
        "Perfil salvo. Sua sessão atual foi renovada. Se a senha mudou, as sessões anteriores foram encerradas.",
      );
      await onChange();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="admin-account-grid">
      <section className="surface management-section">
        <span className="label">SEU PERFIL</span>
        <h2>Dados e senha</h2>
        <p>{profile.email}</p>
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
        <form onSubmit={save}>
          <fieldset disabled={busy} className="plain-fields">
            <label>
              Seu nome
              <input
                name="name"
                defaultValue={profile.name}
                minLength={2}
                maxLength={100}
                required
                autoComplete="name"
              />
            </label>
            <label>
              Senha atual
              <input
                name="current_password"
                type="password"
                minLength={10}
                maxLength={128}
                required
                autoComplete="current-password"
              />
            </label>
            {!profile.managed_identity && (
              <>
                <label>
                  Nova senha
                  <input
                    name="new_password"
                    type="password"
                    minLength={10}
                    maxLength={128}
                    autoComplete="new-password"
                  />
                  <small>Deixe em branco para manter a senha atual.</small>
                </label>
                <label>
                  Confirmar nova senha
                  <input
                    name="confirmation"
                    type="password"
                    minLength={10}
                    maxLength={128}
                    autoComplete="new-password"
                  />
                </label>
              </>
            )}
            <button className="button">
              {busy ? "Salvando…" : "Salvar meus dados"}
            </button>
          </fieldset>
        </form>
      </section>
      <section className="admin-security-card">
        <ShieldCheck size={32} />
        <h2>Seu acesso protegido</h2>
        <p>
          A senha atual confirma que é você quem está alterando a conta. Ao
          trocar a senha, as sessões anteriores são revogadas e esta sessão é
          renovada.
        </p>
        <p>
          Seu acesso administrativo é separado das contas dos clientes. Alterar
          sua senha não muda senhas, planos ou dados das empresas.
        </p>
        <button
          className="button outline"
          onClick={() => void onLogout()}
          disabled={busy}
        >
          <LogOut size={18} />
          Sair desta sessão
        </button>
      </section>
    </div>
  );
}
