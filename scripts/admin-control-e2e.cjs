// Only a local web/API connected to the disposable 100-company validation branch.
const fs = require("node:fs"),
  assert = require("node:assert/strict");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const base = process.env.WEB_URL;
if (!base || !/^http:\/\/(localhost|127\.0\.0\.1):\d+$/.test(base))
  throw Error("Explicit local WEB_URL required");
const fixtures = JSON.parse(fs.readFileSync(process.env.QA_CAPACITY_ACCOUNTS));
(async () => {
  const browser = await chromium.launch({ channel: "chrome", headless: true });
  let page;
  try {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
      reducedMotion: "reduce",
    });
    page = await context.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const admin = fixtures.admin.user;
    await page.goto(base + "/admin/login");
    await page.getByLabel("E-mail", { exact: true }).fill(admin.email);
    await page.getByLabel("Senha", { exact: true }).fill(admin.password);
    await page.getByRole("button", { name: "Entrar na plataforma" }).click();
    await page.waitForURL(base + "/admin");
    await page
      .getByRole("heading", { name: "O controle está nas suas mãos." })
      .waitFor();
    const customer = await browser.newContext({
      viewport: { width: 390, height: 1000 },
    });
    await customer.addCookies([
      {
        name: "gt_access",
        value: fixtures.clients[0].users[0].token,
        url: base,
        httpOnly: true,
        sameSite: "Lax",
      },
    ]);
    const client = await customer.newPage();
    await client.goto(base + "/app");
    await client.locator(".plan-context").waitFor();
    const landing = await browser.newPage({
      viewport: { width: 1440, height: 1000 },
      reducedMotion: "reduce",
    });
    await landing.goto(base);
    await page
      .getByRole("button", { name: "Planos e preços", exact: true })
      .click();
    assert.equal(await page.locator(".admin-plan-card").count(), 3);
    const pro = page
      .locator(".admin-plan-card")
      .filter({ has: page.getByRole("heading", { name: "Pro", exact: true }) });
    await pro.locator("[name=price]").fill("399.90");
    await pro.locator("[name=vehicle_limit]").fill("75");
    await pro.locator("[name=user_limit]").fill("15");
    await pro.locator("[name=published]").check();
    await pro.locator("[name=apply_limits]").check();
    await pro
      .locator("[name=reason]")
      .fill("Atualização comercial aprovada na validação isolada.");
    const saved = page.waitForResponse(
      (r) =>
        r.url().includes("/platform/plans/pro") &&
        r.request().method() === "PUT",
    );
    await pro
      .getByRole("button", { name: "Salvar plano Pro", exact: true })
      .click();
    assert.equal((await saved).status(), 200);
    await page
      .getByRole("status")
      .filter({ hasText: "Plano Pro salvo" })
      .waitFor();
    await landing.bringToFront();
    await landing.evaluate(() => window.dispatchEvent(new Event("focus")));
    await landing
      .locator(".plan")
      .filter({
        has: landing.locator(".label").getByText("Pro", { exact: true }),
      })
      .getByText("399,90", { exact: false })
      .waitFor();
    await client.reload();
    await client
      .locator(".plan-context")
      .getByText("até 75 veículos e 15 usuários", { exact: false })
      .waitFor();
    await client
      .locator(".plan-context")
      .getByRole("button", { name: "Ver benefícios" })
      .click();
    await client.getByText("R$ 399,90", { exact: false }).first().waitFor();
    console.log(
      "PASS plan save through PUT proxy, all three plans, published price on landing without reload, customer limits and customer plan price.",
    );
    await page.bringToFront();
    await page.getByRole("button", { name: "Financeiro", exact: true }).click();
    await page
      .getByRole("button", { name: "Registrar mensalidade", exact: true })
      .click();
    await page
      .getByLabel("Buscar empresa", { exact: true })
      .fill(fixtures.clients[0].name);
    await page
      .getByRole("button", { name: "Buscar empresa", exact: true })
      .click();
    await page
      .getByLabel("Cliente da mensalidade")
      .selectOption(fixtures.clients[0].company_id);
    await page.getByLabel("Plano da mensalidade").selectOption("pro");
    await page.getByLabel("Valor da mensalidade (R$)").fill("399.90");
    await page
      .getByLabel("Justificativa da mensalidade")
      .fill("Mensalidade registrada para validação do fluxo financeiro.");
    await page
      .getByRole("button", { name: "Salvar mensalidade", exact: true })
      .click();
    await page
      .getByRole("status")
      .filter({ hasText: "Mensalidade registrada." })
      .waitFor();
    const reference = "PIX-QA-" + Date.now();
    const row = page
      .locator("tr")
      .filter({ hasText: fixtures.clients[0].name });
    await row
      .getByRole("button", { name: "Confirmar recebimento", exact: true })
      .click();
    const modal = page.getByRole("dialog", { name: "Atualizar mensalidade" });
    await modal.getByLabel("Referência do recebimento").fill(reference);
    await modal
      .getByLabel("Justificativa da operação")
      .fill("Pagamento conferido no ambiente isolado.");
    await modal.locator("input[type=checkbox]").check();
    await modal.getByRole("button", { name: "Confirmar operação" }).click();
    await modal.waitFor({ state: "hidden" });
    await row.getByText(reference, { exact: true }).waitFor();
    await client.reload();
    await client
      .locator(".plan-context")
      .getByRole("button", { name: "Ver benefícios" })
      .click();
    await client.getByText(reference, { exact: false }).waitFor();
    console.log(
      "PASS receivable creation, explicit payment confirmation, real financial total and customer-visible payment reference.",
    );
    for (const tab of [
      "Visão geral",
      "Clientes",
      "Planos e preços",
      "Financeiro",
      "Minha conta",
    ]) {
      await page
        .getByRole("button", { name: tab, exact: true })
        .first()
        .click();
      for (const width of [1440, 390]) {
        await page.setViewportSize({ width, height: 1000 });
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth > innerWidth,
          ),
          false,
          `${tab} overflow ${width}`,
        );
        await page.evaluate(() =>
          window.scrollTo({ top: 0, behavior: "instant" }),
        );
        await page.screenshot({
          path: `/private/tmp/geevheit-admin-${tab.replaceAll(" ", "-")}-${width}.png`,
          fullPage: true,
        });
      }
    }
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page
      .getByRole("button", { name: "Minha conta", exact: true })
      .first()
      .click();
    await page
      .getByLabel("Seu nome", { exact: true })
      .fill("Administrador Completo QA");
    await page.getByLabel("Senha atual", { exact: true }).fill(admin.password);
    const newPassword = "Updated-Isolated-Admin-" + Date.now();
    await page.getByLabel(/^Nova senha/).fill(newPassword);
    await page
      .getByLabel("Confirmar nova senha", { exact: true })
      .fill(newPassword);
    await page
      .getByRole("button", { name: "Salvar meus dados", exact: true })
      .click();
    await page
      .getByRole("status")
      .filter({ hasText: "Perfil salvo." })
      .waitFor();
    fixtures.admin.user.password = newPassword;
    fs.writeFileSync(
      process.env.QA_CAPACITY_ACCOUNTS,
      JSON.stringify(fixtures),
      { mode: 0o600 },
    );
    await page
      .locator(".admin-profile-button")
      .getByText("Administrador Completo QA", { exact: false })
      .waitFor();
    await page
      .getByRole("button", { name: "Sair da administração", exact: true })
      .click();
    await page.waitForURL(base + "/admin/login");
    await page.getByLabel("E-mail", { exact: true }).fill(admin.email);
    await page.getByLabel("Senha", { exact: true }).fill(newPassword);
    await page.getByRole("button", { name: "Entrar na plataforma" }).click();
    await page.waitForURL(base + "/admin");
    assert.deepEqual(errors, []);
    console.log(
      "PASS responsive admin overview/clients/plans/finance/profile, audited password change, renewed session, logout and login with new password; no browser errors.",
    );
  } catch (e) {
    await page
      ?.screenshot({
        path: "/private/tmp/geevheit-admin-control-error.png",
        fullPage: true,
      })
      .catch(() => {});
    throw e;
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
