# Plano de Próximos Passos — GeevheitTire (Prontidão para Campo)

Atualizado em: 18/11/2026 (branch `feat/u0-geevheittire`).

## Concluído e em produção

- **Site / PWA (`https://geevheittire.vercel.app`)**
  - Removida regra legada de `72px` que empurrava os cards para a direita no celular.
  - Barra inferior substituída por **menu lateral retrátil** (botão `☰` no topo esquerdo que desliza a gaveta e recolhe automaticamente ao escolher uma tela ou tocar fora).
  - Tipografia mobile ampliada para leitura em campo (`14px–16px`), inputs em `16px` (sem zoom automático no iOS), mapa de pneus e odômetro ajustados e modo escuro corrigido nos cards de pneus e dossiê 360°.
  - Commits: `244d354` e `38cd45d`.

- **API + Neon + Alertas Agendados (`https://geevheittire-api.vercel.app`)**
  - Conexão da API com o Neon validada (`/health` retornando HTTP 200, `admin_revision: 2`, `tire_lifecycle_revision: 3`, `/api/backend/plans` HTTP 200).
  - Criado endpoint `POST /api/v1/internal/maintenance` protegido por `CRON_SECRET` (comparação em tempo constante; acesso sem token retorna HTTP 401).
  - Configurado **Vercel Schedules** (`maintenance-sweep`, `0 * * * *`, fuso `America/Sao_Paulo`) em `api/vercel.json` com `CRON_SECRET` salvo como Secret em produção e preview.
  - Disparo real validado em produção (`schx_7T5J18PZEKK176X0RJ8TEWHH72`, status `Succeeded`, `0%` erro, `7.8s`).
  - Commits: `f426b8a` e `78b403c`.

---

## Checklist para fazer amanhã

### 1. PWA offline real para pátio e estrada (prioridade alta)
- [ ] Evoluir `web/public/sw.js` para cachear o shell do `/app` e assets estáticos do Next.js (permitir abrir o PWA instalado mesmo sem sinal de internet).
- [ ] Garantir fallback visual claro de "Modo offline" no painel quando não houver rede.
- [ ] Testar em aparelho físico (Android e iPhone):
  - [ ] Instalar na tela inicial ("Adicionar à tela de início").
  - [ ] Desligar internet, registrar uma inspeção, religar internet e confirmar sincronização sem duplicata.
  - [ ] Conferir se o dossiê 360° e upload/foto otimizada funcionam bem na câmera do celular.

### 2. Conferência de produção no Neon (banco de dados)
- [ ] Confirmar qual projeto/branch do Neon está ligado à `DATABASE_URL` de produção da API.
- [ ] Conferir política de snapshots/backups (PITR) e fazer 1 teste de restauração em branch isolada.
- [ ] Revisar limites de compute/conexões do Neon para o volume inicial de clientes.

### 3. Fluxos comerciais externos (conforme estratégia de venda)
- [ ] **E-mail (Resend):** decidir se ativa agora para envio automático de convites de equipe e recuperação de senha (configurar `RESEND_API_KEY` e domínio remetente verificado).
- [ ] **Pagamentos (Asaas):** decidir se mantém cobrança manual pelo painel `/admin` inicialmente ou ativa o checkout/webhook automático.
- [ ] **Segurança e monitoramento:**
  - [ ] Ativar 2FA na conta da Vercel (`https://vercel.com/account/settings/authentication?setup=authenticator`).
  - [ ] Configurar monitor externo simples (ping a cada 5 min em `https://geevheittire-api.vercel.app/health`).

### 4. Higiene de repositório e publicação final
- [ ] Atualizar `docs/VALIDACAO_CAMPO_PWA.md` com as evidências dos testes em aparelho físico.
- [ ] Remover o arquivo de rascunho `correcaopwa.md` da raiz.
- [ ] Conferir conexão Git automática do projeto `geevheittire-api` na Vercel (ou documentar deploy via CLI).
- [ ] Revisar e fazer merge da branch `feat/u0-geevheittire` na branch principal (`main`).
