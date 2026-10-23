// Use an isolated Neon branch and the local company-directory fixture for QA CNPJs.
const {chromium}=require('playwright');
const fs=require('fs');
(async()=>{
 const browser=await chromium.launch({channel:'chrome',headless:true});
 const page=await browser.newPage({viewport:{width:1440,height:1000}});
 const errors=[];page.on('pageerror',e=>errors.push(e.message));page.on('response',async r=>{if(r.url().includes('/api/')&&r.status()>=400){let m;try{const j=await r.json();m=j.title||j.message||j.error?.message;}catch{}console.log('HTTP',r.status(),new URL(r.url()).pathname,m||'');}});
 const base=process.env.WEB_URL || 'http://localhost:3017';
 await page.goto(base);await page.getByRole('heading',{name:/Mais estrada/}).waitFor();
 await page.screenshot({path:'docs/qa/landing-desktop.png',fullPage:true});
 await page.getByRole('button',{name:/2DI: P00127/}).click();
 await page.getByLabel('Redução estimada').fill('10');
 const savings=await page.locator('.calculation-result strong').innerText();if(!savings.includes('18.000'))throw Error('calculator: '+savings);
 await page.setViewportSize({width:390,height:844});
 await page.getByRole('button',{name:'Abrir menu'}).click();await page.getByRole('link',{name:'Planos',exact:true}).click();
 const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);if(overflow)throw Error('landing horizontal overflow');
 await page.screenshot({path:'docs/qa/landing-mobile.png',fullPage:true});
 await page.setViewportSize({width:1440,height:1000});
 await page.goto(base+'/cadastro');
 const stamp=Date.now();console.log('PASS landing desktop/mobile, map and calculator');const email=`qa-neon-${stamp}@example.com`;
 const cnpjBase=(''+stamp).slice(-12).padStart(12,'0');let cnpj=cnpjBase;for(let n=12;n<14;n++){let weight=n-7,sum=0;for(let i=0;i<n;i++){sum+=Number(cnpj[i])*weight;if(--weight===1)weight=9;}let digit=11-sum%11;cnpj+=digit>=10?0:digit;}
 await page.getByLabel('Seu nome').fill('Gestor QA');await page.getByLabel('Nome da empresa').fill('Frota QA');await page.getByLabel('CNPJ').fill(cnpj);await page.getByRole('button',{name:'Consultar CNPJ'}).click();await page.getByText('Situação cadastral: ATIVA',{exact:true}).waitFor();await page.getByLabel('E-mail',{exact:true}).fill(email);await page.getByLabel(/^Senha/).fill('senha-qa-12345');
 await page.getByRole('button',{name:'Criar conta e começar'}).click();await page.waitForURL('**/app');console.log('PASS authenticated navigation');await page.getByRole('heading',{name:'Olá, Gestor.'}).waitFor();
 const neonSession=await page.request.get(base+'/api/auth/get-session');const ns=await neonSession.json();if(!ns?.user?.id)throw Error('Neon session missing');console.log('PASS managed Neon session restored');await page.reload();await page.getByRole('heading',{name:'Olá, Gestor.'}).waitFor();await page.getByRole('button',{name:'Cadastre um veículo'}).click();await page.getByLabel('Placa',{exact:true}).fill('QAQ1A23');await page.getByLabel('Marca e modelo').fill('Volvo FH 540');await page.getByLabel('Odômetro atual').fill('120000');await page.getByRole('button',{name:'Salvar',exact:true}).click();await page.getByText('Cadastro salvo.').waitFor();
 console.log('PASS vehicle created');await page.getByRole('button',{name:'Adicione seus pneus'}).click();await page.getByLabel('Número de fogo',{exact:true}).fill('QA001');await page.getByLabel('Número de série de fábrica').fill('SERIAL-QA001');await page.getByLabel(/^DOT/).fill('1025');await page.getByLabel(/^Validade operacional/).fill('2035-12-31');await page.getByLabel('Tipo de via').selectOption('MISTO');await page.getByLabel('Marca',{exact:true}).fill('Michelin');await page.getByLabel('Instalar em veículo').selectOption({label:'QAQ1A23 · Volvo FH 540'});await page.getByLabel(/^Posição/).selectOption('1EE');await page.getByRole('button',{name:'Salvar',exact:true}).click();await page.getByRole('button',{name:/QAQ1A23.*Volvo/}).click();
 console.log('PASS tire installed');await page.getByRole('button',{name:'1EE: QA001, 15 milímetros'}).click();await page.getByLabel('Sulco (mm)').fill('2.4');await page.getByLabel('Pressão (psi)').fill('80');await page.getByRole('button',{name:'Salvar',exact:true}).click();await page.getByText(/Intervenção imediata:/).waitFor();
 await page.getByRole('button',{name:'Visão geral',exact:true}).click();await page.screenshot({path:'docs/qa/painel-desktop.png',fullPage:true});
 const cookies=await page.context().cookies();if(!cookies.some(c=>c.name==='gt_access'&&c.httpOnly&&c.secure))throw Error('secure session cookie missing');
 await page.setViewportSize({width:390,height:844});await page.screenshot({path:'docs/qa/painel-mobile.png',fullPage:true});if(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw Error('panel horizontal overflow');
 await page.getByRole('button',{name:'Sair da conta'}).click();await page.waitForURL('**/login');if((await page.context().cookies()).some(c=>c.name.startsWith('gt_')))throw Error('session cookie survived logout');const anonymous=await page.request.get(base+'/api/backend/me');if(anonymous.status()!==401)throw Error('protected business route not denied after logout');const signedOut=await page.request.get(base+'/api/auth/get-session');const signedOutData=await signedOut.json();if(signedOutData?.user)throw Error('Neon session survived logout');await page.getByLabel('E-mail',{exact:true}).fill(email);await page.getByLabel(/^Senha/).fill('senha-qa-12345');await page.getByRole('button',{name:'Entrar na plataforma'}).click();await page.waitForURL('**/app');await page.getByRole('heading',{name:'Olá, Gestor.'}).waitFor();
 if(errors.length)throw Error(errors.join('\n'));
 const native=await page.request.post((process.env.API_URL || 'http://localhost:8097')+'/api/v1/auth/login',{data:{email,password:'senha-qa-12345'}});if(native.status()!==200)throw Error('Native Go login via Neon failed: '+native.status());fs.writeFileSync('/tmp/geevheit-qa-neon.json',JSON.stringify({email,cnpj}));
 console.log('PASS: desktop/mobile layout, interactive map, calculator, registration, httpOnly Secure cookies, vehicle+tire creation, inspection, logout and persistent login; no browser errors.');await browser.close();
})().catch(e=>{console.error(e);process.exit(1)});
