# Plano de Próximos Passos — GeevheitTire (Prontidão para Campo)

Atualizado em: 18/11/2026 (branch `feat/u0-geevheittire`).

## Concluído e em produção

- **Site / PWA / Admin (`https://geevheittire.vercel.app`)**
  - Layout fluido e responsivo para celular, tablet, notebook e desktop no site público, `/app` e `/admin`.
  - Menu lateral retrátil (`☰`) com fechamento automático no celular e tablet tanto no perfil dos usuários quanto no `/admin`.
  - Visualizador 360° / AR com viewport 100% livre e controles compactos na linha inferior.
  - **PWA offline real:** `sw.js` (`v3`) com precache do shell `/app` + assets estáticos do Next.js; painel com snapshot local leve (limpo no logout), banner de modo offline, mapa de pneus offline e fila de inspeções com sincronização automática ao voltar online.
  - Edição de **nome**, preço e limites dos planos no `/admin` refletindo no site e no painel dos clientes.

- **API + Neon + Alertas Agendados (`https://geevheittire-api.vercel.app`)**
  - Projeto Neon `geevheithtire` (`purple-resonance-11883617`, `aws-sa-east-1`, PG 18) confirmado na branch `production` (`br-damp-recipe-b657pl3x`, isolada das branches de preview).
  - Snapshot manual de segurança criado: `snap-blue-morning-b6pstd73` (`pre-field-launch-2026-11-18`).
  - Migration `000019_plan_custom_name` aplicada em produção.
  - Vercel Schedules (`maintenance-sweep`, `0 * * * *`, `America/Sao_Paulo`) ativo e validado em produção (`Succeeded`, `0%` erro) com `CRON_SECRET`.
  - Rascunho `correcaopwa.md` removido e `docs/VALIDACAO_CAMPO_PWA.md` atualizado.

---

## Itens finais de decisão do proprietário

- [ ] **Teste em aparelho físico (Android e iPhone):** instalar o PWA na tela inicial, ativar Modo Avião, reabrir o app offline, registrar 1 inspeção e religar a rede para confirmar sincronização.
- [ ] **Merge para `main`:** confirmar se deseja mesclar `feat/u0-geevheittire` em `main` agora.
- [ ] **E-mail (Resend) e Cobrança (Asaas):** decidir se ativa agora (informando chaves/domínio) ou se inicia operação em campo com financeiro manual no `/admin`.
- [ ] **Neon backup recorrente & Vercel 2FA:** ativar 2FA na conta Vercel e, ao entrar em escala comercial, habilitar snapshots agendados / maior retenção no Neon.
