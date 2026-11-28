# Neon — configuração e validação

Projeto `purple-resonance-11883617`, organização `org-dark-tooth-62832207`, branch `production`. PostgreSQL e Neon Auth provisionados em 17/10/2026. Segredos estão apenas nos arquivos ignorados `.env.local` e `web/.env.local`.

## Executar

```sh
neon deploy
npm run migrate:neon
npm run build
npm run api:neon
# Em outro terminal:
npm run start -- --hostname 127.0.0.1 --port 3017
```

Abra http://localhost:3017. A API desse fluxo usa a porta 8097. Os containers anteriores nas portas 3000/8087 são outro ambiente.

`neon deploy` sincroniza os segredos Neon da raiz. A web também precisa de `API_URL=http://localhost:8097`, `NEXT_PUBLIC_NEON_AUTH_ENABLED=true`, `NEON_AUTH_BASE_URL`, `NEON_AUTH_JWKS_URL` e um `NEON_AUTH_COOKIE_SECRET` aleatório com pelo menos 32 caracteres. A API recebe `JWT_SECRET` aleatório e `APP_URL=http://localhost:3017`. Nunca copie essas credenciais para código cliente.

O runner usa `DATABASE_URL_UNPOOLED`: pgx configura uma role de sessão com `SET ROLE`, exigindo conexão direta. As migrations aplicam RLS e a role restrita da API. Trocar para pool transacional exige adaptar esse gerenciamento de role.

Neon Auth controla as novas credenciais. O Next usa o SDK oficial; a API verifica assinatura Ed25519, issuer, audience e expiração pelo JWKS antes de vincular o identificador Neon à empresa. O perfil de acesso permanece no banco da aplicação. Contas antigas continuam com a autenticação existente; não há associação automática por coincidência de e-mail. O login nativo Go também aceita credenciais Neon e encerra a sessão temporária do provedor após validar a identidade.

## Evidência

Build na raiz: `npm run build`, exit 0, TypeScript e compilação concluídos, incluindo `/api/auth/[...path]`; `Rotas validadas: landing em / e painel em /app.`

Migrations 1–5 e `npm run test:neon` passaram primeiro no branch isolado `verify-geevheittire` (TTL 7 dias). Saída: `TestPassword PASS`, `TestCNPJ PASS`, `TestAlphanumericCNPJ PASS`, `TestTenantFlow PASS`, `TestClassify PASS`.

Depois de aplicar as mesmas migrations em production, o navegador real passou cadastro, sessão Neon restaurada, criação de veículo, pneu e inspeção, logout dos dois serviços, novo login e login nativo Go. Cookies HttpOnly/Secure verificados, layouts desktop/mobile sem erros de JavaScript. Saída: `PASS managed Neon session restored`, `PASS vehicle created`, `PASS tire installed`. O teste criou uma conta e dados identificados como QA em production. O roteiro reproduzível está em `scripts/neon-e2e.cjs` e requer Playwright e Chrome instalados. Ele cria dados reais no branch configurado; prefira o branch de validação ao repetir.

## Pendências

API publicada em https://geevheittire-api.onrender.com e site em https://geevheittire.vercel.app. Health check da API validado; fluxo público completo ainda requer validação independente. Ao publicar a API, configure as variáveis acima na hospedagem com URLs HTTPS reais e origens autorizadas no Neon. Configure e valide o envio de e-mail de recuperação/verificação; entrega de e-mail não foi testada. Recuperação web usa o provedor Neon quando habilitado. Valide também recuperação nativa e convites antes do lançamento comercial.

O backend inclui preparação de checklists em lote com idempotência. Checklist offline Flutter ainda não foi implementado; as dependências mobile adicionadas são preparação.

A primeira inicialização falhou por sessão CLI expirada e foi resolvida com login no navegador. Um erro de assinatura TypeScript do handler SDK foi corrigido antes do build aprovado. Um rebuild restrito falhou no Keychain do macOS (`SecItemCopyMatching -50`); foi repetido fora do sandbox.


## GitHub e Vercel

O hook local oficial foi instalado com `neon git install`. Confira com `neon git status`. Esse hook pertence a esta máquina e não é enviado pelo Git. Em outro checkout, execute `neon link` e `neon git install` novamente.

O workflow `.github/workflows/neon-preview.yml` cria um banco isolado por PR do próprio repositório, aplica migrations, executa testes e remove o branch ao fechar o PR. Não migra production automaticamente. A execução remota ainda depende de autorizar o GitHub App Neon: no projeto Neon, abra **Integrations → GitHub → Add → Install GitHub App**, selecione apenas `geevheit-tecnology/geevheithtires` e clique **Connect**. O App configura `NEON_API_KEY` como secret e `NEON_PROJECT_ID` como variável no GitHub. Não envie a chave no chat.

A conexão Neon–Vercel é independente: conecte o projeto Neon existente pela integração de conta Neon, sem criar outro banco. GitHub hospedará o código e a Vercel fará os deploys. A aplicação atual acessa PostgreSQL pela API Go; portanto a web ainda precisa de `API_URL` apontando para uma API pública. O workflow de banco não configura variáveis da Vercel nem publica a API.


## Recuperação de cadastro incompleto

A identidade Neon pode existir mesmo quando o cadastro da empresa falha. O login agora abre os campos da empresa para esse caso, sem criar outra identidade nem associar contas por coincidência de e-mail. O formulário valida o CNPJ antes de criar novas identidades. Erros retornados ou lançados pelo SDK recebem tratamento equivalente, preservando o login de contas legadas e a retomada com e-mail já criado.

Regressão em branch Neon isolado: `scripts/neon-onboarding-e2e.cjs`, com Playwright e Chrome. Configure `WEB_URL` e `API_URL` para os servidores desse branch (padrões localhost:3029 e localhost:8099). O roteiro cria dados de QA. Passaram: CNPJ inválido sem criar identidade, conta sem empresa completando onboarding, login/reload/logout, login legado, cadastro novo e retomada de identidade existente com senha comprovada. Build da raiz também passou.
