# Identidade e privilégios por plano

A marca web usa o mesmo desenho de roda de `web/public/icon.svg`, sem a antiga letra G desenhada por CSS. O aplicativo usa uma cópia exata do ícone PNG em `mobile/assets/brand-icon.png`. O site, autenticação, painel e tema Flutter adotam grafite esverdeado, superfícies claras quentes e verde discreto; amarelo/vermelho ficam reservados às condições operacionais.

## Matriz em vigor

| Recurso | Starter e avaliação | Pro | Enterprise |
| --- | --- | --- | --- |
| Veículos / usuários | 15 / 3 | 60 / 10 | Limites de contrato; contratação ainda sob consulta |
| Identificação, estoque, mapa, inspeções e alertas | Sim | Sim | Sim |
| Km e ficha individual, instalação, retirada, rodízio e transferência de veículo | Sim | Sim | Sim |
| Laudo de retirada/descarte e impressão | Sim | Sim | Sim |
| Relatório essencial | 50 ocorrências recentes por tipo | Sim | Sim |
| Envio/retorno de recapagem e transferência entre locais | Não | Sim | Sim |
| Relatórios por período e páginas anteriores | Não | Sim | Sim |
| Comparativo de marcas, durabilidade e custo/km | Não | Sim | Sim |
| Regras de manutenção personalizadas | Não; aplica padrões do sistema | Sim | Sim |
| IA, integrações, API pública/webhooks e filiais | Indisponíveis | Apenas os itens indicados como previstos; indisponíveis | Previstos; indisponíveis |

Enterprise é uma oferta sob consulta; nenhum plano Enterprise ou contrato foi criado no banco neste ajuste. Sua matriz técnica está preparada para contratos futuros. A avaliação atual é Starter, sem liberação automática de recursos Pro. Preços seguem provisórios em `R$ 0,00 / mês` e `Valores em definição`; pagamentos e e-mails permanecem dependentes de configuração explícita. Os preços internos já existentes no banco não foram alterados.

## Fonte de verdade e bloqueios

`api/internal/commercial/catalog.json` define benefícios, recursos atuais e previstos. A API incorpora esse catálogo; o frontend usa `web/lib/plan-catalog.generated.json`, sincronizado por `npm run sync:plans`. `npm run check:plans` e a CI impedem divergências.

`GET /api/v1/entitlements` resolve o plano na assinatura da empresa autenticada. Claims extras no JWT, cabeçalhos, parâmetros e alterações no navegador não liberam recursos. A decisão é recalculada por requisição, com cache apenas dentro daquela requisição. Os handlers de movimentações e regras também verificam privilégios, além do middleware.

Starter recebe relatórios essenciais sem o agregado de marcas e sem cursor de paginação. Filtros de período e acesso a páginas anteriores retornam `403` com `code=feature_unavailable` e `required_plan=pro`. A interface mostra os benefícios e o caminho para Plano e cobrança; não representa uma liberação de acesso.

Ao rebaixar para Starter ou vencer o pagamento Pro, privilégios Pro são retirados imediatamente, sem exigir novo login. As regras personalizadas ficam preservadas no banco, mas os padrões voltam a valer para Starter. Históricos e laudos existentes permanecem consultáveis na ficha; isso não autoriza novas operações premium. Uma empresa suspensa não mantém privilégios premium nem acesso de escrita. Perfis continuam sendo respeitados: plano Pro não transforma inspetor ou motorista em gestor/borracheiro.

## Evidências e como repetir

- `TestPlanEntitlementsCannotBeElevatedByClient`: trial, Starter pago, Pro, downgrade com o mesmo JWT, Pro vencido, suspensão, tentativa de forjar plano/features, relatórios limitados e aplicação de regras padrão após downgrade.
- `TestTireLifecycleAndAutomation`: ciclo completo executado em assinatura Pro de teste.
- `scripts/brand-plan-access-e2e.cjs`: ícone e paleta, landing, Starter/Pro, formulários, relatórios, benefícios e desktop/celular.
- `scripts/tire-reports-e2e.cjs`: retirada, laudo, recapagem, km por vida, perda da carcaça e impressão, usando assinatura Pro de teste.
- `flutter analyze` e `flutter test`: validação estática e testes existentes do aplicativo; não substituem validação em aparelho físico.

O teste de identidade/planos usa `QA_PLAN_SESSIONS`, JSON temporário com `{starter:{token},pro:{token}}`, exclusivamente de duas empresas fictícias em banco isolado. Execute com Playwright e os serviços locais. Nunca alterar uma assinatura real apenas para validar planos.

Este trabalho permanece local. Não houve commit, push, publicação, ativação de cobrança nem alteração de produção.

Validação local concluída: build web, TypeScript, catálogo sincronizado, Go vet, integração com race detector na branch temporária, dois fluxos Chrome em desktop/celular, Flutter analyze e os dois testes Flutter existentes passaram. Não houve execução em aparelho físico nem publicação em produção.
