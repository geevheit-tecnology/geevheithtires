# Validação de campo do PWA — GeevheitTire

Revisão: 18/11/2026. Produção: https://geevheittire.vercel.app · API: https://geevheittire-api.vercel.app.

## Estado atual em produção

- **PWA e responsividade:**
  - Site público, painel do usuário (`/app`) e painel administrativo (`/admin`) responsivos para celular, tablet, notebook e monitor largo.
  - Menu lateral retrátil (`☰`) com fechamento automático no celular e tablet (`<= 1024px`) tanto no `/app` quanto no `/admin`.
  - Visualizador 360° / AR com área central 100% livre e controles compactos na barra inferior.
  - Service Worker (`web/public/sw.js`, cache `geevheittire-v3`) com precache do shell `/app`, assets estáticos do Next.js (cache-first) e fallback offline de navegação.
  - Modo offline no painel (`web/components/panel.tsx`): salva snapshot leve local (limpo no logout), exibe banner claro de modo offline, permite abrir mapa de pneus por veículo e registrar inspeções na fila offline mesmo ao reabrir o app sem sinal, sincronizando automaticamente ao recuperar conexão.
- **API, Neon e Alertas Agendados:**
  - Projeto Neon confirmado: `geevheithtire` (`purple-resonance-11883617`, PostgreSQL 18, `aws-sa-east-1`, autoscaling `0.25–2 CU`).
  - Branch de produção confirmada e isolada das branches de preview: `production` (`br-damp-recipe-b657pl3x`, `vercel production branch`).
  - Snapshot baseline criado: `snap-blue-morning-b6pstd73` (`pre-field-launch-2026-11-18`). Atenção: retenção PITR atual é de 6 horas (`21600s`, padrão do plano atual) e sem agendamento automático de snapshots.
  - Agendamento de alertas automáticos ativo na Vercel (`maintenance-sweep`, `0 * * * *`, `America/Sao_Paulo`, `POST /api/v1/internal/maintenance` protegido por `CRON_SECRET`), validado em produção (`Succeeded`, `0%` erro).
  - Edição de nome, preço e limites dos planos liberada no `/admin` (migration `000019_plan_custom_name` aplicada em produção), refletindo automaticamente no site público e no painel dos clientes.

## Roteiro rápido de validação em aparelho físico (Android e iPhone)

1. Abra https://geevheittire.vercel.app no celular, instale na tela inicial ("Adicionar à tela de início") e faça login com uma conta de teste.
2. Navegue por Painel, Veículos (mapa de pneus), Pneus (cards + dossiê 360°/AR), Alertas e menu lateral (`☰`). Teste tema claro e escuro.
3. Com o app instalado aberto e dados carregados, ative o **Modo Avião** (sem internet), feche e reabra o PWA:
   - Confira se o `/app` abre com o banner **"Modo offline ativo"**.
   - Abra um veículo, toque em um pneu e registre uma inspeção (sulco/pressão).
   - Confira o aviso de que a inspeção ficou salva no aparelho.
4. Desative o Modo Avião e aguarde (ou toque em **Enviar agora**). Confira no histórico do pneu que a inspeção entrou uma única vez com os valores corretos.

## Pendências comerciais / operacionais antes de escala aberta

- **E-mail transacional (Resend):** permanece desativado; ativar se desejar envio automático de convites de equipe e recuperação de senha.
- **Cobrança automática (Asaas):** permanece desativada; o financeiro funciona em modo manual pelo `/admin` até decisão de ativação.
- **Backup e retenção no Neon:** avaliar plano pago do Neon para aumentar retenção PITR (acima de 6h) e ativar snapshots automáticos recorrentes.
- **Segurança da conta Vercel:** ativar 2FA em `https://vercel.com/account/settings/authentication?setup=authenticator`.
