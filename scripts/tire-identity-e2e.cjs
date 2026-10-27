// Local API/web and an isolated database only. The account must be disposable.
const assert=require("node:assert/strict");
const fs=require("node:fs");
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||"playwright");
const base=process.env.WEB_URL||"http://localhost:3039";
const credentials=JSON.parse(fs.readFileSync(process.env.QA_ACCOUNT_FILE,"utf8"));
(async()=>{
 const browser=await chromium.launch({channel:"chrome",headless:true});
 try{
  const context=await browser.newContext({viewport:{width:1440,height:1000}});
  const page=await context.newPage(),errors=[];
  page.on("pageerror",e=>errors.push(e.message));
  await context.addCookies([{name:"gt_access",value:credentials.access_token,url:base,httpOnly:true,sameSite:"Lax"}]);
  async function call(path,body,method){const r=await context.request.fetch(`${base}/api/backend/${path}`,{method:method||(body?"POST":"GET"),headers:{Origin:base},data:body});const data=await r.json();assert.ok(r.ok(),`${path}: ${r.status()} ${data.title||""}`);return data;}
  const vehicle=await call("vehicles",{plate:"QAQ1D23",model:"Veículo de identificação QA",template_id:"tractor-6x2",current_km:1000});
  await page.goto(base+"/app");
  await page.getByRole("button",{name:"Motoristas",exact:true}).click();
  await page.getByLabel("Nome do motorista").fill("Motorista Identidade QA");
  await page.getByRole("button",{name:"Salvar motorista",exact:true}).click();
  await page.getByText("Motorista salvo.",{exact:false}).waitFor();
  const drivers=await call("drivers"),driver=drivers.data.find(d=>d.name==="Motorista Identidade QA");assert.ok(driver);
  await page.getByRole("button",{name:"Veículos",exact:true}).click();
  await page.locator(".vehicle-row").filter({hasText:"QAQ1D23"}).first().click();
  await page.getByLabel("Vincular motorista").selectOption(driver.id);
  await page.getByRole("button",{name:"Salvar responsável",exact:true}).click();
  await page.locator(".vehicle-driver").getByText("Motorista Identidade QA",{exact:false}).first().waitFor();
  assert.equal((await call(`vehicles/${vehicle.id}/tire-map`)).vehicle.driver_name,driver.name);
  await page.getByRole("button",{name:"Pneus",exact:true}).click();
  await page.getByRole("button",{name:"Novo pneu",exact:true}).click();
  const form=page.getByRole("dialog");
  await form.getByLabel("Número de fogo",{exact:true}).fill("FOGO-QA-001");
  await form.getByLabel("Número de série de fábrica").fill("SERIAL-QA-001");
  await form.getByLabel(/^DOT/).fill("DOT AB 1025");
  await form.getByLabel(/^Validade operacional/).fill("2035-12-31");
  await form.getByLabel("Tipo de via").selectOption("MISTO");
  await form.getByLabel("Recapagens anteriores à entrada").selectOption("1");
  await form.getByLabel("Marca",{exact:true}).fill("Marca QA");
  await form.getByLabel("Instalar em veículo",{exact:false}).selectOption(vehicle.id);
  await form.getByLabel("Posição",{exact:true}).selectOption("1EE");
  assert.match(await form.getByLabel("Posição",{exact:true}).innerText(),/lado esquerdo/);
  await form.getByRole("button",{name:"Salvar",exact:true}).click();
  await page.getByRole("dialog").waitFor({state:"hidden"});
  const tire=(await call("tires")).data.find(t=>t.code==="FOGO-QA-001");assert.ok(tire);assert.equal(tire.serial_number,"SERIAL-QA-001");assert.equal(tire.dot_code,"AB1025");assert.equal(tire.retread_count,1);
  await page.locator("tr").filter({hasText:"FOGO-QA-001"}).getByRole("button",{name:/Dados e histórico/}).click();
  const detail=page.locator(".lifecycle-details");
  await detail.getByText("2ª vida",{exact:true}).waitFor();
  await detail.getByText("SERIAL-QA-001",{exact:false}).waitFor();
  await detail.getByText("Motorista Identidade QA",{exact:false}).waitFor();
  await detail.locator("summary").filter({hasText:"Editar identificação"}).click();
  await detail.getByLabel("Tipo de via").selectOption("TERRA");
  await detail.getByRole("button",{name:"Salvar identificação",exact:true}).click();
  await page.waitForFunction(()=>document.querySelector('.lifecycle-details')?.textContent.includes('Terra'));
  const life=await call(`tires/${tire.id}/lifecycle`);assert.equal(life.tire.road_use,"TERRA");assert.equal(life.movements[0].road_use,"MISTO");assert.equal(life.movements[0].life_number,2);assert.equal(life.movements[0].to_driver_name,driver.name);
  for(const width of [1440,390]){await page.setViewportSize({width,height:1000});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`overflow ${width}`);}
  await page.setViewportSize({width:1440,height:1000});
  await page.keyboard.press("Escape");
  await page.getByRole("button",{name:"Veículos",exact:true}).click();
  await page.locator(".vehicle-row").filter({hasText:"QAQ1D23"}).first().click();
  const wheel=page.locator('.tire[title*="lado esquerdo"]').filter({hasText:"FOGO-QA-001"});assert.equal(await wheel.count(),1);assert.match(await wheel.innerText(),/1 · 1EE/);
  await page.locator(".map-sides").getByText("← Lado esquerdo",{exact:true}).waitFor();
  await page.setViewportSize({width:390,height:844});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,"map overflow mobile");
  assert.deepEqual(errors,[]);
  console.log("PASS driver create/assignment, fire and serial numbers, DOT, expiry, road use, imported retread life, metadata edit, immutable history, numbered left/right map and desktop/mobile layout.");
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
