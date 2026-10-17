// Run against an isolated Neon branch. This creates disposable QA identities.
// Requires Playwright and an installed Chrome browser.
const { chromium } = require("playwright");
const assert = require("node:assert/strict");
const base = process.env.WEB_URL || "http://localhost:3029";
const backend = process.env.API_URL || "http://localhost:8099";
const password = "senha-qa-onboarding-12345";

function cnpj(seed) {
  let value = String(seed).slice(-12).padStart(12, "0");
  for (let n = 12; n < 14; n++) {
    let weight = n - 7, sum = 0;
    for (let i = 0; i < n; i++) {
      sum += Number(value[i]) * weight;
      if (--weight === 1) weight = 9;
    }
    const digit = 11 - sum % 11;
    value += digit >= 10 ? 0 : digit;
  }
  return value;
}

(async () => {
  const browser = await chromium.launch({ channel: "chrome", headless: true });
  try {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", e => errors.push(e.message));
    let signups = 0;
    page.on("request", r => { if (r.url().includes("/sign-up/email")) signups++; });
    const stamp = Date.now();
    const email = `qa-onboarding-${stamp}@example.com`;
    await page.goto(base + "/cadastro");
    await page.getByLabel("Seu nome").fill("Gestor QA");
    await page.getByLabel("Nome da empresa").fill("Frota QA incompleta");
    await page.getByLabel(/^CNPJ/).fill("00000000000000");
    await page.getByLabel("E-mail", { exact: true }).fill(email);
    await page.getByLabel(/^Senha/).fill(password);
    await page.getByRole("button", { name: "Criar conta e começar" }).click();
    await page.getByRole("alert").filter({ hasText: "Informe um CNPJ válido" }).waitFor();
    assert.equal(signups, 0, "invalid company must not create a Neon identity");
    console.log("PASS invalid CNPJ rejected before creating a Neon identity");

    // Reproduce an account created before its company was successfully saved.
    const seed = await page.request.post(base + "/api/auth/sign-up/email", {
      headers: { Origin: base }, data: { name: "Gestor QA", email, password },
    });
    assert.equal(seed.status(), 200, "test identity signup");
    await page.goto(base + "/login");
    await page.getByLabel("E-mail", { exact: true }).fill(email);
    await page.getByLabel(/^Senha/).fill(password);
    await page.getByRole("button", { name: "Entrar na plataforma" }).click();
    await page.getByRole("heading", { name: "Complete o cadastro da sua empresa." }).waitFor();
    assert.equal(await page.getByLabel("E-mail", { exact: true }).inputValue(), email);
    await page.getByLabel("Seu nome").fill("Gestor QA");
    await page.getByLabel("Nome da empresa").fill("Frota QA recuperada");
    await page.getByLabel(/^CNPJ/).fill(cnpj(stamp));
    await page.getByRole("button", { name: "Concluir cadastro e entrar" }).click();
    await page.waitForURL("**/app");
    await page.getByRole("heading", { name: "Olá, Gestor." }).waitFor();
    const me = await page.request.get(base + "/api/backend/me");
    assert.equal(me.status(), 200);
    const owner = await me.json();
    assert.equal(owner.role, "owner");
    assert.ok(owner.company_id);
    console.log("PASS unlinked Neon identity completes company onboarding and receives protected access");
    await page.reload();
    await page.getByRole("heading", { name: "Olá, Gestor." }).waitFor();
    await page.getByRole("button", { name: "Sair da conta" }).click();
    await page.waitForURL("**/login");
    assert.equal((await page.request.get(base + "/api/backend/me")).status(), 401);
    await page.getByLabel("E-mail", { exact: true }).fill(email);
    await page.getByLabel(/^Senha/).fill(password);
    await page.getByRole("button", { name: "Entrar na plataforma" }).click();
    await page.waitForURL("**/app");
    await page.getByRole("heading", { name: "Olá, Gestor." }).waitFor();
    console.log("PASS linked account login, reload and logout remain functional");

    await page.getByRole("button", { name: "Sair da conta" }).click();
    await page.waitForURL("**/login");
    const legacyEmail = `qa-legacy-${stamp}@example.com`;
    const legacy = await page.request.post(backend + "/api/v1/auth/register", {
      data: { name: "Gestor QA", email: legacyEmail, password, company: "Frota QA legada", cnpj: cnpj(stamp + 1) },
    });
    assert.ok(legacy.ok(), "legacy signup");
    await page.getByLabel("E-mail", { exact: true }).fill(legacyEmail);
    await page.getByLabel(/^Senha/).fill(password);
    await page.getByRole("button", { name: "Entrar na plataforma" }).click();
    await page.waitForURL("**/app").catch(async () => {
      throw new Error("Legacy login: " + (await page.locator(".error").allTextContents()).join(" "));
    });
    await page.getByRole("heading", { name: "Olá, Gestor." }).waitFor();
    console.log("PASS existing legacy authentication remains available");
    await page.getByRole("button", { name: "Sair da conta" }).click();
    await page.waitForURL("**/login");
    await page.goto(base + "/cadastro");
    await page.getByLabel("Seu nome").fill("Gestor QA");
    await page.getByLabel("Nome da empresa").fill("Frota QA nova");
    await page.getByLabel(/^CNPJ/).fill(cnpj(stamp + 2));
    await page.getByLabel("E-mail", { exact: true }).fill(`qa-new-${stamp}@example.com`);
    await page.getByLabel(/^Senha/).fill(password);
    await page.getByRole("button", { name: "Criar conta e começar" }).click();
    await page.waitForURL("**/app");
    await page.getByRole("heading", { name: "Olá, Gestor." }).waitFor();
    console.log("PASS fresh registration creates both identity and company");
    await page.getByRole("button", { name: "Sair da conta" }).click();
    await page.waitForURL("**/login");
    const pendingEmail = `qa-retry-${stamp}@example.com`;
    const pending = await page.request.post(base + "/api/auth/sign-up/email", {
      headers: { Origin: base }, data: { name: "Gestor QA", email: pendingEmail, password },
    });
    assert.equal(pending.status(), 200);
    await page.goto(base + "/cadastro");
    await page.getByLabel("Seu nome").fill("Gestor QA");
    await page.getByLabel("Nome da empresa").fill("Frota QA retomada");
    await page.getByLabel(/^CNPJ/).fill(cnpj(stamp + 3));
    await page.getByLabel("E-mail", { exact: true }).fill(pendingEmail);
    await page.getByLabel(/^Senha/).fill(password);
    await page.getByRole("button", { name: "Criar conta e começar" }).click();
    await page.waitForURL("**/app");
    await page.getByRole("heading", { name: "Olá, Gestor." }).waitFor();
    console.log("PASS registration resumes an existing Neon identity after proving its password");
    assert.deepEqual(errors, [], "browser exceptions");
  } finally { await browser.close(); }
})().catch(e => { console.error(e.message); process.exit(1); });
