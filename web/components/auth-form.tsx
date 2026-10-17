"use client";
import { useState, FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, ShieldCheck } from "lucide-react";
import { Brand } from "./brand";
import { api } from "./api";
import { neonClient, neonEnabled } from "./neon-client";
export function AuthForm({
  mode,
}: {
  mode: "login" | "register" | "recover" | "accept";
}) {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const verify = mode === "accept" && params.get("kind") === "verify";
  const title =
    mode === "register"
      ? "Sua frota merece uma gestão melhor."
      : mode === "recover"
        ? "Vamos recuperar seu acesso."
        : mode === "accept"
          ? verify
            ? "Confirme seu e-mail."
            : "Defina sua senha."
          : "Bom ter você de volta.";
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const form = new FormData(e.currentTarget);
    const data = Object.fromEntries(form);
    try {
      if (neonEnabled && mode === "recover") {
        const result = await neonClient.requestPasswordReset({
          email: String(data.email),
          redirectTo: `${window.location.origin}/aceitar-convite?kind=neon-reset`,
        });
        if (result.error) throw new Error(result.error.message || "Não foi possível solicitar a recuperação.");
        setMessage("Se o e-mail estiver cadastrado, você receberá instruções de recuperação.");
        return;
      }
      if (neonEnabled && mode === "accept" && params.get("kind") === "neon-reset") {
        const result = await neonClient.resetPassword({newPassword: String(data.password), token: params.get("token") || ""});
        if (result.error) throw new Error(result.error.message || "Link inválido ou expirado.");
        setMessage("Senha atualizada. Entre novamente.");
        return;
      }
      let managedSession: unknown;
      if (neonEnabled && (mode === "login" || mode === "register")) {
        let result =
          mode === "register"
            ? await neonClient.signUp.email({
                name: String(data.name),
                email: String(data.email),
                password: String(data.password),
              })
            : await neonClient.signIn.email({
                email: String(data.email),
                password: String(data.password),
              });
        // A failed company submission may leave an authenticated Neon identity.
        // Prove the same credentials again before retrying company onboarding.
        if (result.error && mode === "register") {
          result = await neonClient.signIn.email({email: String(data.email), password: String(data.password)});
        }
        if (result.error) {
          if (mode === "register")
            throw new Error(
              result.error.message || "Não foi possível criar a conta Neon.",
            );
        } else {
          const token = await neonClient.token();
          if (token.error || !token.data?.token)
            throw new Error(
              "Confirme seu e-mail ou entre novamente para continuar.",
            );
          managedSession = await api("auth/neon", {
            token: token.data.token,
            ...(mode === "register"
              ? { name: data.name, company: data.company, cnpj: data.cnpj }
              : {}),
          });
        }
      }
      const out =
        managedSession ||
        (await api(
          `auth/${mode === "register" ? "register" : mode === "accept" ? "accept" : mode}`,
          mode === "accept" ? { ...data, token: params.get("token") } : data,
        ));
      if (mode === "recover" || mode === "accept") {
        setMessage(out.message);
      } else {
        router.push("/app");
        router.refresh();
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-layout">
      <aside>
        <Brand />
        <div>
          <span className="eyebrow">CONTROLE EM CADA QUILÔMETRO</span>
          <h1>
            O próximo passo
            <br />
            da sua frota
            <br />
            <em>começa aqui.</em>
          </h1>
          <p>Uma conta. Toda a operação conectada.</p>
        </div>
        <span>
          <ShieldCheck size={18} />
          Acesso individual. Dados da sua empresa.
        </span>
      </aside>
      <main className="auth-main">
        <Link className="back" href="/">
          ← Voltar ao site
        </Link>
        <div className="auth-box">
          <span className="label">GEEVHEITTIRE</span>
          <h2>{title}</h2>
          <p>
            {mode === "register"
              ? "Comece seu período de avaliação de 14 dias, sem cartão."
              : "Use seu acesso à plataforma."}
          </p>
          {message ? (
            <div className="success" role="status">
              {message}
              <Link href="/login">Ir para o login →</Link>
            </div>
          ) : (
            <form onSubmit={submit}>
              {mode === "register" && (
                <>
                  <label>
                    Seu nome
                    <input
                      name="name"
                      autoComplete="name"
                      required
                      minLength={2}
                    />
                  </label>
                  <label>
                    Nome da empresa
                    <input
                      name="company"
                      autoComplete="organization"
                      required
                      minLength={2}
                    />
                  </label>
                  <label>
                    CNPJ
                    <input
                      name="cnpj"
                      placeholder="00.000.000/0000-00"
                      required
                      inputMode="text"
                      autoCapitalize="characters"
                      maxLength={18}
                    />
                  </label>
                </>
              )}
              {mode !== "accept" && (
                <label>
                  E-mail
                  <input
                    name="email"
                    type="email"
                    autoComplete="email"
                    required
                  />
                </label>
              )}
              {mode !== "recover" && !verify && (
                <label>
                  Senha
                  <input
                    name="password"
                    type="password"
                    autoComplete={
                      mode === "login" ? "current-password" : "new-password"
                    }
                    minLength={mode === "login" ? 1 : 10}
                    maxLength={128}
                    required
                  />
                  {mode !== "login" && <small>Pelo menos 10 caracteres.</small>}
                </label>
              )}
              {error && (
                <div className="error" role="alert">
                  {error}
                </div>
              )}
              <button className="button" disabled={busy}>
                {busy
                  ? "Aguarde…"
                  : mode === "register"
                    ? "Criar conta e começar"
                    : mode === "recover"
                      ? "Enviar instruções"
                      : mode === "accept"
                        ? verify
                          ? "Confirmar e-mail"
                          : "Salvar senha"
                        : "Entrar na plataforma"}
                <ArrowRight size={18} />
              </button>
            </form>
          )}
          {mode === "login" && (
            <>
              <Link className="auth-link" href="/recuperar-senha">
                Esqueci minha senha
              </Link>
              <p className="auth-switch">
                Ainda não tem conta? <Link href="/cadastro">Comece aqui</Link>
              </p>
            </>
          )}
          {mode === "register" && (
            <p className="auth-switch">
              Já tem uma conta? <Link href="/login">Entrar</Link>
            </p>
          )}
        </div>
      </main>
    </div>
  );
}
