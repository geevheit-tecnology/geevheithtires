# Estoque, histórico e desempenho de pneus

## Centro de controle administrativo

- [ ] Corrigir edição dos três planos e publicação dos preços na landing e no painel do cliente.
- [ ] Permitir sincronizar limites com clientes existentes, com validação de uso e auditoria.
- [ ] Implementar financeiro com filtros, recebíveis, baixas manuais e histórico por cliente.
- [ ] Adicionar perfil, alteração de senha e encerramento seguro da sessão administrativa.
- [ ] Reorganizar o admin com navegação, indicadores, ações e layouts desktop/celular.
- [ ] Validar API, permissões, banco real e fluxos completos no navegador.
- [ ] Publicar API e frontend e verificar as revisões dos serviços.

## Validação e publicação dos alinhamentos — 28/10/2026

- [x] Padronizar alinhamento de ícones, botões, cards e formulários responsivos.
- [x] Confirmar API pública saudável na revisão 3 e catálogo de planos acessível.
- [x] Validar integração/race e persistência na branch Neon temporária desta entrega.
- [x] Executar os fluxos no Chrome com API e banco reais, sem interceptar respostas.
- [x] Confirmar build final, publicar na Vercel e registrar o estado do deploy.

Os dados usados no teste visual anterior ficaram em um script temporário fora do produto. A aplicação consulta a API; os testes desta entrega usarão persistência real em banco isolado, sem inserir dados de teste em produção.

Evidência desta entrega: branch `verify-alignment-release-20261028` (`br-little-fire-b697r3zg`), migrações conferidas, `go test -race -count=1 ./...`, `go vet ./...`, catálogo de planos, build e TypeScript aprovados. Chrome aprovou pneus/laudos/recapagem/relatórios, limites Starter/Pro, identidade/motorista e administração/suporte. O build final também passou cadastro de identidade Neon, restauração da sessão, logout e login real no formulário. A auditoria de dependências terminou com zero vulnerabilidades após fixar versões transitivas corrigidas.

Produção publicada em 28/10/2026: deployment `dpl_2JghfLZomwGCiM27zrsdzKYrCr2T`, target `production`, estado `READY` conferido pela CLI. Aliases confirmados: `https://geevheittire.vercel.app` e `https://geevheittire-geevheits-projects.vercel.app`. Detalhes e limites da validação em `docs/VALIDACAO_ALINHAMENTO_2026-10-28.md`.

### Correção dos botões dos planos

- [x] Starter: “Avaliar o Starter”; Pro: “Conhecer o Pro”; Enterprise: “Conhecer o Enterprise”.
- [x] Explicar que a avaliação inicial usa os recursos Starter, preservando os preços provisórios.
- [x] Conferir os três CTAs no HTML compilado, sem respostas simuladas.
- [x] Publicar a correção em produção: `dpl_6MykW2djpWLxT9YPJWEqnUVjEjVx`, estado `READY`, alias `https://geevheittire.vercel.app` confirmado. Build e TypeScript local/remoto aprovados.

- [x] Conferir estoque, ficha individual e rastreamento existentes.
- [x] Registrar laudo de retirada/descarte com ocorrência e condição da carcaça.
- [x] Preservar laudo e km no histórico, com impressão.
- [x] Exibir km por vida e total rastreado.
- [x] Criar relatórios de descartes, recapagens e comparação de marcas.
- [x] Validar API, interface e migração em ambiente isolado.

Trabalho local; publicação e migração de produção não executadas.

## Evidências

- Migrações 000010 e 000011 aplicadas na branch temporária `verify-tire-reports-20261027` (`br-dawn-tooth-b6cc1mgj`), com expiração automática. Produção preservada.
- `go test -race -count=1 ./...` passou com `TEST_DATABASE_URL` da branch temporária, incluindo laudos, descartes, isolamento por empresa, validações e concorrência.
- Build web e TypeScript passaram; fluxo `scripts/tire-reports-e2e.cjs` passou no Chrome em 1440 px e 390 px.
- Ciclo validado: 10.000 km na primeira vida, recapagem, mais 5.000 km na segunda vida; estouro com perda da carcaça encerrou o pneu em 15.000 km totais.
- Custo/km conferido com aquisição de R$ 1.000 e recapagem de R$ 300. PDF do laudo gerado em uma página.
- Guias de uso e publicação: `docs/ESTOQUE_LAUDOS_RELATORIOS.md`.

## Identidade visual e recursos por plano

- [x] Aplicar o ícone existente à marca e suavizar a paleta no site, acesso e painel.
- [x] Definir um catálogo único de benefícios e recursos disponíveis/previstos.
- [x] Bloquear recursos Pro no backend para Starter, trial e assinatura sem vigência.
- [x] Explicar limites e upgrades na interface, com preços provisórios preservados.
- [x] Validar isolamento, tentativas diretas na API, downgrade e desktop/celular.

### Evidências de identidade e planos

- Ícone PNG do aplicativo idêntico ao original web (SHA-256 conferido); marca web referencia `/icon.svg`.
- `npm run check:plans` passou; CI verifica o catálogo único.
- `go test -race -count=1 ./...` e `go vet ./...` passaram com o banco temporário. Testes incluem trial/Starter pago/Pro, claims e cabeçalhos forjados, filtros premium, downgrade com o mesmo JWT, Pro vencido e suspensão.
- Estoque, indicadores, mapa e manutenção usam as regras efetivas do plano, preservando a configuração premium sem aplicá-la ao Starter.
- `scripts/brand-plan-access-e2e.cjs` e `scripts/tire-reports-e2e.cjs` passaram no Chrome em 1440 px e 390 px. O ciclo completo permaneceu funcional em Pro.
- Build web final e TypeScript passaram. `flutter analyze` não encontrou problemas; os dois testes Flutter existentes passaram. Aparelho físico não foi usado.
- Capturas de validação em `/private/tmp/geevheit-brand-landing-1440.png`, `/private/tmp/geevheit-starter-benefits.png` e variantes móveis/Pro.
- Matriz de benefícios e limites documentada em `docs/PLANOS_E_IDENTIDADE.md`. Sem commit, push, deploy ou alteração de produção.

## Administração, atendimento e encerramento de conta

- [x] Dar visibilidade ao Super Admin e preparar acesso separado do cliente.
- [x] Gerenciar clientes, vigência e planos com auditoria.
- [x] Configurar valores, limites e publicação dos planos sem liberar recursos indevidos.
- [x] Registrar e acompanhar tickets, respostas e notas internas.
- [x] Encerrar conta mediante confirmação e reautenticação, preservando auditoria.
- [x] Validar migrações, isolamento e fluxos com administrador fictício (identidade real criada depois da validação).

### Entrega local — fabricação, alertas e administração (27/10/2026)
- [x] Ano de fabricação visível no cadastro/edição/ficha e coerente com DOT.
- [x] Aviso antecipado de rodízio, alerta comparativo de CPK alto e rankings de marcas/modelos e pneus.
- [x] Administração: clientes, planos, acesso com prazo, auditoria e chamados.
- [x] Suporte do cliente, resposta da equipe e notas internas protegidas.
- [x] Encerramento com reautenticação, confirmação, arquivamento e revogação de acesso.
- [x] Criação e primeiro acesso de administrador fictício validados em banco temporário.
- [x] Integração/race, build/typecheck e Chrome desktop/celular.
- [x] Criar a identidade administrativa real indicada, com convite privado de uso único e auditoria.
- [ ] Aplicar migrações de produção e publicar as novas telas (não executados nesta etapa).
