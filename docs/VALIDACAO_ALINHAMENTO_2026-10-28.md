# Alinhamento e validação para publicação — 28/10/2026

## Alterações

- Botões e ícones com dimensões e centralização consistentes.
- Cards, grades, estados vazios e formulários com alinhamento responsivo.
- Correção de seletores e espera pela gravação real no teste de identidade do pneu.
- Versões transitivas corrigidas no frontend: Better Auth/Core/API Key 1.7.7, React Email Render 2.2.0 e Zod 4.6.7. Overrides explícitos mantêm o SDK Neon existente; revisar quando o SDK incorporar as correções.
- A pasta local `.vscode` não integra o pacote de publicação.

## Validação executada

Ambiente: frontend compilado localmente, API Go real e branch Neon `verify-alignment-release-20261028` (`br-little-fire-b697r3zg`), derivada de produção e com expiração automática. Nenhuma resposta de API foi interceptada nos testes desta etapa. As gravações de QA ficaram somente na branch temporária.

- Migrations conferidas no banco temporário; nenhuma migration nova nesta alteração.
- `go test -race -count=1 ./...` e `go vet ./...`: aprovados.
- `npm run check:plans`: aprovado.
- `npm --prefix web run typecheck` e `npm --prefix web run build`: aprovados.
- `npm --prefix web audit --omit=dev`: zero vulnerabilidades.
- `scripts/tire-reports-e2e.cjs`: ficha pelo estoque e mapa, retirada com laudo, recapagem, km por vida, perda de carcaça, descarte, relatórios, CPK e impressão aprovados.
- `scripts/brand-plan-access-e2e.cjs`: limites Starter/Pro, bloqueio de recursos na API, benefícios, relatórios e responsividade aprovados.
- `scripts/tire-identity-e2e.cjs`: cadastro e vínculo de motorista, identidade/DOT/validade/via, edição persistida e preservação do histórico aprovados.
- `scripts/platform-management-e2e.cjs`: primeiro acesso administrativo, login, clientes, planos, chamado do cliente e atendimento administrativo aprovados.
- Neon Auth real da branch temporária: cadastro de identidade, restauração de sessão após reload, logout, login pelo formulário e proteção de conta sem empresa aprovados no build final.
- Alinhamento em 1440, 1024, 768 e 390 px: ícones centralizados, formulário completo de veículo e catálogo persistido de configurações, sem transbordamento horizontal.
- Capturas de evidência: `/private/tmp/geevheit-release-alignment-*.png`, `/private/tmp/geevheit-release-vehicle-*.png` e capturas dos roteiros de pneus/administração.

O teste visual anterior usou um script descartável com respostas simuladas. Esse script não faz parte do produto nem do pacote de deploy. Os fluxos acima foram repetidos com os serviços reais.

## Serviços publicados

Antes da publicação web, a API Render retornou HTTP 200 em `/health`, `status=ok`, revisão `3`, release `2026-10-27-tire-management`. O endpoint público `/api/v1/plans` também retornou HTTP 200 e os três planos. A configuração de produção na Vercel foi mantida; não foram enviadas variáveis do ambiente temporário.

A Vercel oculta os valores sensíveis ao exportar as variáveis pela CLI. Sua presença foi conferida por metadados; valores vazios da exportação não foram usados nem reenviados.

Esta validação cobre os fluxos descritos. Pagamentos Asaas, entrega transacional por Resend e testes em aparelhos físicos continuam fora desta entrega, conforme as integrações previamente adiadas. O build pode emitir aviso de peer dependency na biblioteca transitiva de UI de autenticação; compilação, auditoria e login real foram aprovados.

## Publicação

Vercel: deployment `dpl_2JghfLZomwGCiM27zrsdzKYrCr2T`, target `production`, estado `READY` confirmado por `vercel inspect`. Build remoto e TypeScript aprovados. Aliases associados: `https://geevheittire.vercel.app` e `https://geevheittire-geevheits-projects.vercel.app`.

Publicação realizada pela CLI a partir do workspace validado; nenhum commit ou push foi feito nesta entrega. A verificação pós-publicação foi do estado e dos aliases no provedor; os testes de navegador descritos acima foram executados no build local com os serviços reais isolados.

Após a publicação, os servidores locais desta validação foram encerrados, a branch temporária foi removida pelo Neon e os arquivos privados de conexão/sessão foram apagados. Produção foi preservada.
