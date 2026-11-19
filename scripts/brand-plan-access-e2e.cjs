// Local browser verification. Fixtures must belong only to a disposable test branch.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=process.env.WEB_URL||'http://localhost:3032';
const sessions=JSON.parse(fs.readFileSync(process.env.QA_PLAN_SESSIONS,'utf8'));
(async()=>{
 const browser=await chromium.launch({channel:'chrome',headless:true});let current;
 try{
 const marketing=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce'});
 const landing=await marketing.newPage();current=landing;await landing.goto(base);
 const logo=landing.locator('.site-header .brand-icon img');await logo.waitFor();assert.equal(await logo.getAttribute('src'),'/icon.svg');assert.ok(await logo.evaluate(el=>el.complete&&el.naturalWidth>0));
 assert.equal(await landing.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--navy').trim()),'#293c38');
 assert.equal(await landing.locator('.plan').count(),3);assert.equal(await landing.locator('.plan .price').filter({hasText:'0,00'}).count(),3);await landing.getByText('Previstos, ainda indisponíveis',{exact:true}).first().waitFor();
 for(const width of [1440,390]){await landing.setViewportSize({width,height:1000});assert.equal(await landing.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`landing overflow ${width}`);await landing.screenshot({path:`/private/tmp/geevheit-brand-landing-${width}.png`,fullPage:true});}
 await marketing.close();
 for(const plan of ['starter','pro']){
 const ctx=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce'});await ctx.addCookies([{name:'gt_access',value:sessions[plan].token,url:base,httpOnly:true,sameSite:'Lax'}]);
 const page=await ctx.newPage();current=page;const errors=[];page.on('pageerror',e=>errors.push(e.message));
 async function call(path,body,want=200){const res=await ctx.request.fetch(`${base}/api/backend/${path}`,{method:body?'POST':'GET',headers:{Origin:base},data:body});const out=await res.json();assert.equal(res.status(),want,`${path}: ${out.title||''}`);return out;}
 const suffix=Date.now().toString().slice(-7);
 const tire=await call('tires',{code:`${plan.toUpperCase()}-${suffix}`,serial_number:`SERIAL-${plan.toUpperCase()}-${suffix}`,dot_code:'1025',expires_on:'2035-12-31',road_use:'MISTO',brand:'Plan QA',model:plan,size:'295/80 R22.5',tread_mm:9,acquisition_cost:1000},201);
 const access=await call('entitlements');assert.equal(access.plan_id,plan);assert.equal(access.features.advanced_lifecycle,plan==='pro');assert.equal(access.features.ai_assistant,false);assert.equal(access.features.integrations,false);
 await page.goto(`${base}/app`);await page.locator('.plan-context').getByText(plan==='pro'?'Pro':'Starter',{exact:true}).waitFor();
 assert.equal(await page.locator('.sidebar .brand-icon img').getAttribute('src'),'/icon.svg');
 await page.getByRole('button',{name:'Pneus',exact:true}).click();await page.getByRole('button',{name:`${plan.toUpperCase()}-${suffix}`,exact:true}).click();const dossier=page.locator('dialog[open]');await dossier.getByText(`SERIAL-${plan.toUpperCase()}-${suffix}`,{exact:false}).waitFor();
 assert.equal(await dossier.locator('option[value="RECAP_SEND"]').evaluate(el=>el.disabled),plan==='starter');assert.equal(await dossier.locator('option[value="INSTALL"]').evaluate(el=>el.disabled),false);
 if(plan==='starter'){
 assert.equal((await call(`tires/${tire.id}/movements`,{action:'RECAP_SEND'},403)).code,'feature_unavailable');
 await dossier.getByRole('heading',{name:'Ciclo de vida completo',exact:true}).waitFor();
 }else{
 await dossier.getByLabel('Movimentar',{exact:true}).selectOption('RECAP_SEND');await dossier.getByLabel('Fornecedor da recapagem').fill('Recapadora QA');await dossier.getByLabel('Motivo / observações').fill('Carcaça avaliada e enviada para recapagem.');const response=page.waitForResponse(r=>r.url().includes(`/tires/${tire.id}/movements`)&&r.request().method()==='POST');await dossier.getByRole('button',{name:'Confirmar movimentação',exact:true}).click();assert.equal((await response).status(),201);await dossier.locator('.lifecycle-form').waitFor({state:'detached'});
 }
 await dossier.getByRole('button',{name:'Fechar',exact:true}).first().click();await dossier.waitFor({state:'hidden'});await page.locator('summary').filter({hasText:'Regras dos alertas'}).click();assert.equal(await page.locator('[name="warning_mm"]').isDisabled(),plan==='starter');
 await page.getByRole('button',{name:'Relatórios de pneus',exact:true}).click();await page.getByRole('heading',{name:'Descartes, recapagens e retiradas'}).waitFor();assert.equal(await page.locator('[name="from"]').isDisabled(),plan==='starter');
 if(plan==='starter'){
 await page.getByText('Relatório essencial:',{exact:false}).waitFor();assert.equal(await page.getByRole('heading',{name:'Ranking de marcas e modelos'}).count(),0);
 const basic=await call('tire-reports?plan_id=pro&scope=brands');assert.deepEqual(basic.brands,[]);assert.equal(basic.limited,true);await call('tire-reports?from=2026-01-01',undefined,403);
 }else{await page.getByRole('heading',{name:'Ranking de marcas e modelos'}).waitFor();assert.equal((await call('tire-reports')).limited,false);}
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`${plan} overflow ${width}`);await page.screenshot({path:`/private/tmp/geevheit-${plan}-reports-${width}.png`,fullPage:true});}
 await page.locator('.plan-context').getByRole('button',{name:'Ver benefícios',exact:true}).click();await page.getByText(`Plano em uso: ${plan==='pro'?'Pro':'Starter'}.`,{exact:true}).waitFor();assert.equal(await page.locator('.commercial-plan').count(),3);assert.equal(await page.locator('.commercial-plan.current-plan').count(),1);await page.getByText('Valores em definição.',{exact:false}).waitFor();assert.equal(await page.locator('.commercial-plan .planned-benefits').count(),2);
 await page.setViewportSize({width:1440,height:1000});await page.screenshot({path:`/private/tmp/geevheit-${plan}-benefits.png`,fullPage:true});assert.deepEqual(errors,[]);await ctx.close();
 }
 console.log('PASS existing wheel identity, softened palette, responsive landing, Starter limits, Pro recap workflow, custom policies, essential/advanced reports, unavailable future modules and plan benefit cards.');
 }catch(e){await current?.screenshot({path:'/private/tmp/geevheit-plan-access-error.png',fullPage:true}).catch(()=>{});throw e}finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
