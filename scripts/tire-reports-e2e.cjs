// Run only against a local API backed by a disposable database branch.
// QA_SESSION_FILE is a temporary test session created in that branch.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const base=process.env.WEB_URL||'http://localhost:3031';
 const session=JSON.parse(fs.readFileSync(process.env.QA_SESSION_FILE,'utf8'));
 const browser=await chromium.launch({channel:'chrome',headless:true});let testPage;
 try {
 const context=await browser.newContext({viewport:{width:1440,height:1000}});
 await context.addCookies([{name:'gt_access',value:session.token,url:base,httpOnly:true,sameSite:'Lax'}]);
 const page=await context.newPage();testPage=page;const errors=[];page.on('pageerror',e=>errors.push(e.message));
 async function call(path,body){const res=await context.request.fetch(`${base}/api/backend/${path}`,{method:body?'POST':'GET',headers:{Origin:base},data:body});const out=await res.json();assert.ok(res.ok(),`${path}: ${res.status()} ${out.title||''}`);return out;}
 const suffix=Date.now().toString().slice(-6);
 const vehicle=await call('vehicles',{plate:'QAQ1F23',model:'Laudo QA',template_id:'tractor-6x2',current_km:1000});
 const tire=await call('tires',{code:`LAUDO-${suffix}`,serial_number:`SERIE-${suffix}`,dot_code:'1025',expires_on:'2035-12-31',road_use:'MISTO',brand:'Marca QA',model:'Modelo QA',size:'295/80 R22.5',tread_mm:8,acquisition_cost:1000,vehicle_id:vehicle.id,position:'1EE'});
 await page.goto(`${base}/app`);
 await page.getByRole('button',{name:'Pneus',exact:true}).click();
 await page.getByRole('button',{name:`LAUDO-${suffix}`,exact:true}).click();
 const dialog=page.locator('dialog[open]');await dialog.getByText(`SERIE-${suffix}`,{exact:false}).waitFor();
 assert.equal(await page.evaluate(()=>document.body.style.overflow),'hidden');
 async function remove(carcass,km,cause){await dialog.getByLabel('Movimentar',{exact:true}).selectOption('REMOVE');await dialog.locator('[name="source_km"]').fill(String(km));await dialog.getByLabel('Ocorrência',{exact:true}).selectOption(cause);await dialog.getByLabel('Condição da carcaça').selectOption(carcass);await dialog.getByLabel('Responsável pela avaliação').fill('Técnico QA');await dialog.getByLabel('Avaliação e conclusão').fill('Avaliação presencial da estrutura e condição da carcaça.');await dialog.getByLabel('Motivo / observações').fill('Retirada avaliada pela equipe técnica.');const response=page.waitForResponse(r=>r.url().includes(`/tires/${tire.id}/movements`)&&r.request().method()==='POST');await dialog.getByRole('button',{name:'Confirmar movimentação',exact:true}).click();const res=await response;assert.equal(res.status(),201,await res.text());await dialog.locator('.lifecycle-form').waitFor({state:'detached'});}
 await remove('PRESERVADA',11000,'DESGASTE');
 await dialog.getByRole('heading',{name:'Laudo de retirada / descarte'}).waitFor();
 async function move(action,extra){const snap=await call(`tires/${tire.id}/lifecycle`);await call(`tires/${tire.id}/movements`,{client_id:crypto.randomUUID(),action,expected_version:snap.tire.version,reason:'Operação física para teste de recapagem.',...extra});}
 await move('RECAP_SEND',{supplier:'Recapadora QA'});await move('RECAP_RETURN',{supplier:'Recapadora QA',new_tread_mm:10,stock_location:'Principal',cost_cents:30000});
 assert.equal((await call(`tires/${tire.id}/lifecycle`)).tire.current_life_km,0);
 await move('INSTALL',{vehicle_id:vehicle.id,position:'1EE',vehicle_km:11000});
 await dialog.getByRole('button',{name:'Fechar',exact:true}).first().click();
 await page.reload();await page.getByRole('button',{name:'Veículos',exact:true}).click();await page.locator('.vehicle-row').filter({hasText:'QAQ1F23'}).first().click();await page.locator('.tire').filter({hasText:`LAUDO-${suffix}`}).click();
 await dialog.getByText(`SERIE-${suffix}`,{exact:false}).waitFor();await remove('PERDIDA',16000,'ESTOURO');
 const life=await call(`tires/${tire.id}/lifecycle`);assert.equal(life.tire.manufacture_year,2025);assert.equal(life.tire.state,'DESCARTADO');assert.equal(life.tire.total_tracked_km,15000);assert.equal(life.tire.current_life_km,5000);assert.equal(life.movements[0].removal_report.cause,'ESTOURO');assert.equal(life.movements[0].life_km,5000);assert.equal((await call(`vehicles/${vehicle.id}/tire-map`)).tires.length,0);
 await dialog.getByText('Estouro',{exact:true}).waitFor();await dialog.evaluate(el=>el.scrollTo({top:0}));
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);assert.equal(await dialog.locator('[name="report_assessment"]').count(),0);await page.screenshot({path:`/private/tmp/tire-dossier-${width}.png`,fullPage:true});}
 await page.keyboard.press('Escape');await dialog.waitFor({state:'hidden'});
 await page.getByRole('button',{name:'Relatórios de pneus',exact:true}).click();await page.getByRole('heading',{name:'Ranking de marcas e modelos'}).waitFor();
 await page.getByLabel('Ocorrência',{exact:true}).selectOption('DISCARD');const filtered=page.waitForResponse(r=>r.url().includes('kind=DISCARD'));await page.getByRole('button',{name:'Filtrar',exact:true}).click();await filtered;await page.locator('.tire-reports table').first().locator('tr').filter({hasText:`LAUDO-${suffix}`}).first().waitFor();
 const reports=await call('tire-reports?kind=DISCARD');assert.equal(reports.events.filter(e=>e.tire_id===tire.id).length,1);const brand=reports.brands.find(b=>b.brand==='MARCA QA');assert.equal(Number(brand.carcass_losses),1);assert.equal(Number(brand.bursts),1);assert.equal(Number(brand.retreads),1);assert.ok(Math.abs(Number(brand.cost_per_km)-1300/15000)<.000001);
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`report overflow ${width}`);await page.screenshot({path:`/private/tmp/tire-reports-${width}.png`,fullPage:true});}
 await page.locator('.tire-reports table').first().getByRole('button',{name:'Abrir ficha'}).first().click();await dialog.getByText(`SERIE-${suffix}`,{exact:false}).waitFor();assert.deepEqual(errors,[]);
 await page.evaluate(()=>{window.print=()=>{}});await dialog.getByRole('button',{name:'Imprimir laudo / salvar PDF'}).first().click();
 assert.equal(await page.locator('#tire-report-print').count(),1);await page.pdf({path:'/private/tmp/tire-removal-report.pdf',format:'A4'});await page.evaluate(()=>window.dispatchEvent(new Event('afterprint')));assert.equal(await page.locator('#tire-report-print').count(),0);
 console.log('PASS stock dossier, vehicle-map dossier, removal assessment, recap mileage reset, burst/carcass loss, reports, brand costs and desktop/mobile layouts.');
 }catch(e){await testPage?.screenshot({path:"/private/tmp/tire-reports-error.png",fullPage:true}).catch(()=>{});throw e}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
