# Status de implementação

Consulte a [revisão sppec05](SPPEC05-READINESS.md) para evidências atuais, correções e bloqueios comerciais. Publicações históricas abaixo não comprovam que as novas alterações já estejam em produção.

## U0 — fundação conectada

| Item | Situação |
| --- | --- |
| Estrutura /api, /web, /mobile, /infra, /docs | Implementada |
| PostgreSQL, migrations versionadas e seeds de 2 templates | Implementados |
| Cadastro, CNPJ, trial, login, refresh e logout | Implementados |
| RBAC para cadastros e convites | Implementado |
| RLS em todas as tabelas com dados de empresa | Implementado; teste com role sem bypass |
| Landing, calculadora e mapa ilustrativo | Implementados |
| Painel, veículos, pneus e instalação inicial | Implementados |
| App Flutter login, lista e mapa dinâmico | Implementados |
| Verificação, recuperação e convite | Código implementado; envio real depende de Resend/domínio |
| Inspeção, histórico e alertas internos | Fatia inicial do P1 implementada |
| Configuração de deploy | Docker Compose, Dockerfiles, Fly.io e guia Neon/Vercel |
| Neon PostgreSQL e Auth | Configurados no branch production; fluxo real validado localmente |
| API publicada | Render Free, health check conectado ao Neon validado |
| Web publicada | Vercel em geevheittire.vercel.app; toolbar desativada |
| Publicação nas lojas | Pendente |

## Limites desta entrega

- Trial de 14 dias com limites de veículos/usuários e modo de consulta após vencimento. Super Admin permite publicar planos, suspender/restaurar empresas e prorrogar avaliação com auditoria.
- Pagamentos Asaas estão pausados por decisão do proprietário. O adaptador permanece preparado, com `BILLING_PAYMENTS_ENABLED=false` por padrão; cobrança automática e renovação recorrente não estão liberadas.
- O acesso Super Admin depende de uma conta já cadastrada e concessão pelo comando operacional `api/cmd/platform-admin`; não é concedido automaticamente pelo e-mail ou pelo perfil owner da empresa.
- Sete categorias e dezoito configurações de eixos/rodado estão disponíveis; filiais e acoplamento continuam pendentes.
- O aplicativo ainda não tem checklist offline, fotos, QR, push ou sincronização. São próximos itens do P1.
- Sulco, rodízio, inspeções vencidas, estoque mínimo e recapagens têm regras por empresa. Pressão usa a referência inicial 90–120 psi e temperatura usa aviso a partir de 85 °C; parametrização desses dois valores por aplicação ainda está pendente.
- Estoque, ciclo de vida, rodízio, transferências, recapagem, descarte e alertas automáticos estão implementados para a web/PWA e API; veja [fluxos e limites](TIRE-LIFECYCLE.md). CPK, importação, integrações, relatórios e paridade dessas telas no Flutter continuam pendentes.
- IA não foi integrada. Landing identifica esses recursos como roadmap.
- Cadastro e instalação inicial estão disponíveis. Edição/exclusão não têm botões fictícios e serão ampliadas nas próximas etapas.
- Sem Resend configurado não há entrega de e-mail. Em produção, verificação obrigatória, fila/outbox, retentativas e proteção contra abuso precisam ser concluídas antes de liberar onboarding público em escala.
- A API e o proxy web precisam de observabilidade, políticas de retenção, backups/restore e revisão de LGPD antes do lançamento comercial.
- A implementação usa pgx diretamente e um runner transacional de migrations com lock. sqlc e golang-migrate poderão ser incorporados na expansão; não são apresentados como já integrados.
- Credenciais de autenticação têm RLS, com funções SECURITY DEFINER restritas à role da API para localizar identidade antes de conhecer o tenant. O proprietário do banco conserva privilégios administrativos; nunca use suas credenciais no cliente.

## Sequência autorizada pelo guia

U0 → P1 completo → P5 cobrança → P2 → P3 → P4.

Esta entrega implementa e valida o U0 local e prepara os serviços externos, sem confundir a prévia da interface com um SaaS de produção completo.
