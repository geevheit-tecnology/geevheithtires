// Run against a local API + web build and a disposable validation account.
// QA_ACCOUNT_FILE holds only the temporary account, never a production credential.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const base = process.env.WEB_URL || "http://localhost:3029";
const credentials = JSON.parse(
  fs.readFileSync(
    process.env.QA_ACCOUNT_FILE || "/tmp/geevheit-lifecycle-qa.json",
    "utf8",
  ),
);
(async () => {
  const browser = await chromium.launch({ channel: "chrome", headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 1000 },
  });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  async function call(path, body, method) {
    const res = await context.request.fetch(`${base}/api/backend/${path}`, {
      method: method || (body ? "POST" : "GET"),
      headers: { Origin: base },
      data: body,
    });
    const data = await res.json();
    assert.ok(res.ok(), `${path}: ${res.status()} ${data.title || ""}`);
    return data;
  }
  try {
    await call("auth/login", {
      email: credentials.email,
      password: credentials.password,
    });
    const vehicle = await call("vehicles", {
      plate: "QAQ1B23",
      model: "Caminhão QA",
      template_id: "tractor-6x2",
      current_km: 1000,
    });
    const tire = await call("tires", {
      code: "QA-LIFECYCLE-001",
      serial_number: "SERIAL-LIFECYCLE-001",
      dot_code: "1025",
      expires_on: "2035-12-31",
      road_use: "MISTO",
      brand: "Marca QA",
      size: "295/80 R22.5",
      tread_mm: 8,
      acquisition_cost: 1000,
    });
    await page.goto(`${base}/app`);
    await page.getByRole("button", { name: "Pneus", exact: true }).click();
    await page
      .locator("tr")
      .filter({ hasText: "QA-LIFECYCLE-001" })
      .getByRole("button", { name: "Ciclo de vida" })
      .click();
    const detail = page.locator(".lifecycle-details");
    await detail
      .getByRole("heading", { name: "QA-LIFECYCLE-001 · Marca QA" })
      .waitFor();
    async function start(action) {
      await detail
        .getByLabel("Movimentar", { exact: true })
        .selectOption(action);
    }
    async function save(reason) {
      const form = detail.locator(".lifecycle-form");
      await form.getByLabel("Motivo / observações").fill(reason);
      const [response] = await Promise.all([
        page.waitForResponse(
          (r) =>
            r.url().includes(`/tires/${tire.id}/movements`) &&
            r.request().method() === "POST",
        ),
        form
          .getByRole("button", { name: "Confirmar movimentação", exact: true })
          .click(),
      ]);
      assert.equal(response.status(), 201, (await response.json()).title);
      await form.waitFor({ state: "detached" });
      await page
        .getByText(
          "Movimentação concluída. Histórico, estoque e alertas atualizados.",
          { exact: true },
        )
        .waitFor();
    }
    await start("INSTALL");
    await detail
      .getByLabel("Veículo de destino", { exact: true })
      .selectOption(vehicle.id);
    await detail.getByLabel("Posição de destino").selectOption("1EE");
    await save("Instalação física confirmada pela equipe QA");
    let status = (await call(`tires/${tire.id}/lifecycle`)).tire;
    assert.equal(status.state, "INSTALADO");
    assert.equal(status.position, "1EE");
    console.log("PASS: instalação no veículo com histórico real");
    await start("ROTATE");
    await detail.getByLabel("Posição de destino").selectOption("1DE");
    await detail.getByLabel("Odômetro atual de QAQ1B23 (km)").fill("1100");
    await detail.getByLabel("Odômetro no destino (km)").fill("1100");
    await save("Rodízio físico confirmado pela equipe QA");
    status = (await call(`tires/${tire.id}/lifecycle`)).tire;
    assert.equal(status.position, "1DE");
    assert.equal(status.tracked_km, 100);
    console.log("PASS: rodízio com quilometragem acumulada");
    await page.getByRole("button", { name: "Veículos", exact: true }).click();
    await page
      .locator(".vehicle-row")
      .filter({ hasText: "QAQ1B23" })
      .first()
      .click();
    const odo = page.locator(".odometer-form");
    await odo.getByRole("spinbutton").fill("12100");
    await odo
      .getByRole("button", { name: "Atualizar km", exact: true })
      .click();
    await odo
      .getByText("Quilometragem e alertas de manutenção atualizados.")
      .waitFor();
    await page.getByRole("button", { name: /^Alertas/ }).click();
    await page.getByText(/QA-LIFECYCLE-001: rodízio previsto/).waitFor();
    console.log("PASS: alerta automático por odômetro, sem inspeção completa");
    await page.getByRole("button", { name: "Pneus", exact: true }).click();
    await page
      .locator("tr")
      .filter({ hasText: "QA-LIFECYCLE-001" })
      .getByRole("button", { name: "Ciclo de vida" })
      .click();
    await detail
      .getByRole("heading", { name: "QA-LIFECYCLE-001 · Marca QA" })
      .waitFor();
    await start("REMOVE");
    await detail.getByLabel("Local no estoque").fill("Galpão B");
    await save("Retirada física para inspeção e manutenção");
    await start("STOCK_TRANSFER");
    await detail.getByLabel("Local no estoque").fill("Galpão C");
    await save("Transferência para o galpão de manutenção");
    assert.equal(
      (await call(`tires/${tire.id}/lifecycle`)).tire.stock_location,
      "Galpão C",
    );
    console.log("PASS: retirada e transferência entre locais do estoque");
    await detail
      .getByRole("button", { name: "Registrar inspeção", exact: true })
      .click();
    const modal = page.locator(".modal-backdrop form");
    await modal.locator("input[name=tread_mm]").fill("4.5");
    await modal.locator("input[name=pressure_psi]").fill("100");
    await modal.getByRole("button", { name: "Salvar", exact: true }).click();
    await modal.waitFor({ state: "detached" });
    const alerts = (await call("alerts")).data;
    assert.ok(alerts.some((a) => a.kind === "retread"));
    assert.ok(alerts.some((a) => a.kind === "wear"));
    console.log("PASS: alertas de desgaste e avaliação para recapagem");
    await start("RECAP_SEND");
    await detail.getByLabel("Fornecedor da recapagem").fill("Recapadora QA");
    await save("Carcaça avaliada e enviada à recapadora QA");
    await start("RECAP_RETURN");
    await detail.getByLabel("Sulco após recapagem (mm)").fill("9");
    await detail.getByLabel("Local no estoque").fill("Galpão de pneus prontos");
    await detail.getByLabel("Custo do serviço (R$)").fill("300");
    await save("Recapagem recebida e avaliada pela equipe QA");
    const life = await call(`tires/${tire.id}/lifecycle`);
    assert.equal(life.tire.retread_count, 1);
    assert.equal(life.tire.movement_cost_cents, 30000);
    assert.equal(life.tire.state, "EM_ESTOQUE");
    assert.ok(life.movements.some((m) => m.action === "RECAP_RETURN"));
    assert.ok(life.movements.some((m) => m.action === "ROTATE"));
    const active = (await call("alerts")).data;
    assert.ok(
      !active.some(
        (a) => a.tire_id === tire.id && ["wear", "retread"].includes(a.kind),
      ),
    );
    console.log("PASS: recapagem com custo e resolução automática dos alertas");
    await page.screenshot({
      path: "/tmp/geevheit-lifecycle-desktop.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({
      path: "/tmp/geevheit-lifecycle-mobile.png",
      fullPage: true,
    });
    assert.deepEqual(errors, []);
    console.log(
      "PASS: navegador sem erros JavaScript; capturas desktop e celular",
    );
  } catch (e) {
    await page
      .screenshot({ path: "/tmp/geevheit-lifecycle-error.png", fullPage: true })
      .catch(() => {});
    throw e;
  } finally {
    await browser.close();
  }
})().catch((e) => {
  console.error(e.message);
  process.exit(1);
});
