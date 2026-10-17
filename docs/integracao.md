# GeevheitTire: como tudo se junta e como usar

## 1. O sistema é UM produto com 3 partes

```
Cliente
  │
  ├─ Site (landing)  geevheittire.com.br        → vende, cadastra, leva ao login
  │
  ├─ Painel web      app.geevheittire.com.br    → dono e gestor: relatórios, CPK, planos
  │        │
  ├─ App mobile (Flutter)                       → pátio: inspeção, câmera, QR, offline
  │        │
  └────────┴──► API em Go  api.geevheittire.com.br ──► Banco Neon (PostgreSQL)
```

- Site, painel e app usam **a mesma conta e a mesma API**. O cliente cria a conta no site, entra no painel e a equipe usa o app com o mesmo login.
- A API é o "cérebro": regras de status, ciclo de vida, CPK, IA, cobrança. Site e app só mostram e enviam dados.

## 2. O que cada arquivo que você recebeu é

| Arquivo | O que é | Como usar |
|---|---|---|
| `prompts-gestao-pneus-saas.md` | Instruções (P0 a P5) para uma IA de código construir o sistema | Você cola o P0 e depois cada módulo na IA de código, na ordem |
| `modelo-de-dados-tirepro.md` | Desenho do banco de dados (ER) | Você entrega à IA de código para gerar as tabelas (migrations) |
| `landing-tirepro.html` | Landing page pronta (visual e textos) | É o ponto de partida do site; a IA de código converte para Next.js e liga os botões ao cadastro |
| Este guia | Como juntar tudo e o prompt de integração | Siga o passo a passo abaixo |

Importante: os prompts **não são o aplicativo**. São as ordens para uma IA de programação gerar o código do aplicativo. O sistema só existe depois que o código é gerado, testado e publicado.

## 3. Passo a passo para colocar no ar

### Etapa A: preparar (1 dia)
1. Crie uma pasta `geevheittire` no computador e dentro dela uma pasta `docs`.
2. Copie para `docs` os 3 arquivos (prompts, modelo de dados e landing) e este guia.
3. Instale: Git, Go, Flutter, Node.js e **Claude Code** (a IA de código no terminal) ou outra ferramenta equivalente (Cursor, por exemplo).
4. Crie as contas (todas têm plano gratuito ou barato para começar):
   - GitHub (guardar o código)
   - Neon (banco de dados)
   - Vercel (site e painel)
   - Fly.io, Railway ou Google Cloud Run (API em Go)
   - Cloudflare R2 (fotos)
   - Asaas (cobranças) e Resend (e-mails)
   - Firebase (notificações push)
   - Domínio `.com.br` no Registro.br
   - Apple Developer e Google Play Console (só quando for publicar o app)

### Etapa B: construir (use o prompt de integração da seção 4)
1. Abra o terminal na pasta `geevheittire` e inicie o Claude Code.
2. Cole o **Prompt de Integração (U0)** da seção 4. Ele monta a estrutura completa e liga as 3 partes.
3. Depois cole, um de cada vez, os módulos **P1, P2, P3, P4, P5** do arquivo de prompts. Revise e teste ao fim de cada um antes de seguir.
4. Em cada módulo, peça: "rode os testes, suba localmente e me diga como testar".

### Etapa C: testar com clientes reais
1. Publique em ambiente de teste (Vercel + API + Neon).
2. Rode o app em celulares reais, inclusive sem internet.
3. Coloque 2 ou 3 transportadoras para usar por 30 dias (clientes-piloto).

### Etapa D: lançar e vender
1. Ative a cobrança no Asaas e o domínio definitivo.
2. Publique o app nas lojas (a aprovação leva alguns dias).
3. Revise termos de uso e política de privacidade com um advogado.
4. Divulgue para transportadoras (indicação, LinkedIn, grupos de frotistas, anúncios).

## 4. PROMPT DE INTEGRAÇÃO (U0): cole primeiro, antes do P1

```text
Você é um arquiteto sênior e desenvolvedor full-stack. Leia primeiro os arquivos da pasta /docs: prompts-gestao-pneus-saas.md (use o P0 como regras permanentes do projeto), modelo-de-dados-tirepro.md (modelo de dados) e landing-tirepro.html (landing page de referência visual e de textos). O produto se chama GeevheitTire.

OBJETIVO DESTA ETAPA: criar o monorepo completo com as três partes conectadas, funcionando de ponta a ponta com o mínimo necessário, para depois evoluirmos módulo a módulo (P1 a P5).

ESTRUTURA
/api      backend Go (chi, pgx, sqlc, golang-migrate), Dockerfile, deploy em Fly.io ou Cloud Run
/web      Next.js (App Router) + Tailwind, contendo:
            - site público (/) convertido a partir de landing-tirepro.html, mantendo design, textos, mapa interativo e calculadora
            - /cadastro, /login, /recuperar-senha
            - /app (painel logado): dashboard vazio, lista de veículos, lista de pneus, usuários, plano e cobrança (telas básicas)
/mobile   Flutter (login, lista de veículos, tela do mapa de pneus com dados reais da API)
/infra    docker-compose (Postgres local), variáveis de ambiente de exemplo, scripts
/docs     os arquivos que recebi

FLUXO QUE DEVE FUNCIONAR AO FINAL DESTA ETAPA
1. Visitante abre o site e clica em "Começar teste de 14 dias" ou "Testar grátis": vai para /cadastro.
2. No cadastro informa nome, e-mail, senha, nome da empresa e CNPJ. O backend cria a empresa (trial de 14 dias), o usuário com role owner e devolve a sessão. Enviar e-mail de verificação.
3. O usuário cai em /app, vê um guia de primeiros passos (cadastrar veículo, cadastrar pneu, convidar equipe) e consegue cadastrar um veículo e um pneu manualmente.
4. O dono convida um usuário por e-mail (perfil borracheiro ou inspetor). O convidado define a senha.
5. No app Flutter, esse usuário faz login com o mesmo e-mail e senha e vê os veículos da empresa e o mapa de pneus do veículo cadastrado.
6. Dados de uma empresa nunca aparecem para outra (RLS e teste automático).

REGRAS
- Autenticação única: JWT de acesso (curto) e refresh token, mesma API para web e app. Cookies httpOnly na web, secure storage no app.
- Multi-tenant: company_id em tudo, RLS ativo, teste de isolamento.
- Os botões da landing devem apontar para /cadastro e /login, e o botão "Entrar" deve aparecer no topo do site.
- Mapa de pneus do app e do painel web gerado a partir do template de eixos (comece com carreta de 3 eixos e cavalo 6x2).
- Variáveis de ambiente em arquivos .env.example, nunca segredos no código.
- README na raiz com: como subir tudo localmente com um comando, como rodar os testes e como publicar (Vercel para /web, Fly.io para /api, Neon para o banco).

ENTREGA
1. Primeiro apresente o plano e a árvore de pastas e aguarde minha confirmação.
2. Depois implemente na ordem: banco e migrations mínimas, API (cadastro, login, convite, veículos, pneus), web (site, cadastro, login, painel básico), mobile (login e mapa).
3. Ao final, explique em passos simples como testar o fluxo completo no meu computador.
```

## 5. Ordem final de trabalho

1. **U0** (este guia): estrutura e fluxo site → cadastro → painel → app funcionando.
2. **P1**: inspeção, checklist offline, alertas e mapa completo.
3. **P5 (parte de cobrança e planos)**: já dá para vender.
4. **P2**: ciclo de vida, CPK, recapagem, importação.
5. **P3**: todos os tipos de veículo e filiais.
6. **P4**: inteligência artificial.

## 6. Expectativas realistas

- Gerar tudo por IA exige **revisão e teste** a cada etapa. Espere de 2 a 4 meses de trabalho até um produto vendável, mesmo com IA, dependendo do tempo que você dedicar.
- Se você não programa, há dois caminhos: aprender o básico de acompanhar a IA de código (é viável, mas exige dedicação) ou contratar um desenvolvedor para revisar, publicar e manter. Em qualquer caso, vale ter um profissional para revisar **segurança, cobrança e LGPD** antes de vender.
- Comece pequeno: U0 + P1 já mostram o produto funcionando e permitem fechar pilotos.