# Diagnóstico e prontidão comercial — sppec05

Revisão: 23/10/2026. Este relatório distingue validação local, banco de testes e publicação. **O lançamento comercial ainda está bloqueado.**

## Decisões do proprietário nesta revisão

- Manter mensalidade por link Asaas e adiar recorrência. Não implementar renovação automática, troca de cartão ou cancelamento de assinatura recorrente nesta entrega.
- Asaas sandbox e Resend ainda não estão disponíveis: registrar a ativação como pendência. Nenhuma cobrança real foi habilitada.

## Fase 1 — arquitetura e diagnóstico

| Área | Implementação observada |
| --- | --- |
| Web | Next.js 16, React 19, TypeScript; landing, cadastro, login, recuperação, painel e Super Admin |
| Sessão web | Proxy na mesma origem, cookies HttpOnly, Secure em produção, SameSite=Lax; verificação de Origin nas mutações |
| API | Go, chi, pgx; JWT HS256 com expiração, refresh rotativo e Argon2id |
| Identidade externa | Integração opcional Neon Auth; preservar o fluxo existente |
| Dados | PostgreSQL/Neon, migrations 000001–000009, role runtime sem bypass de RLS e escopo de empresa por transação |
| Produto | Veículos, sete categorias/dezoito configurações, pneus, inspeções, estoque, movimentações, manutenção e alertas |
| Comercial | Trial, limites no backend/banco, planos, mensalidade por link, faturas e conciliação idempotente |
| E-mail | Resend configurável para verificação, recuperação e convites; sem credenciais disponíveis nesta revisão |
| Serviços externos | Consultas cadastrais CNPJ/CEP, Neon Auth e adaptador Asaas |
| Jobs | Worker de manutenção; outbox de e-mails e retentativas persistentes ainda ausentes |
| Mobile | Flutter com Riverpod, Dio, go_router e armazenamento seguro; sem paridade de todos os fluxos web |
| Hospedagem configurada | API em Render/Docker, web na Vercel e banco no Neon; Fly.io permanece alternativa documentada |

### Priorização

**Bloqueiam o lançamento:** validar pagamento completo no Asaas sandbox e os webhooks reais; configurar e testar e-mails; concluir verificação obrigatória de e-mail no onboarding; definir/implementar exportação, exclusão e retenção de dados; aprovar documentos legais e identificação do fornecedor; exercitar backup/restore; completar homologação antes de promover produção.

**Importantes:** proteção contra abuso distribuída e por conta, limpeza dos registros do limitador em memória, tratamento de concorrência na renovação de sessão web, outbox com retentativas, monitoramento/alertas operacionais, cobertura IDOR de todos os endpoints e fluxo de conta ativa/revogação. O limitador atual identifica o peer TCP: tráfego por proxies pode compartilhar uma cota; não se deve confiar em X-Forwarded-For arbitrário para contornar isso.

**Desejáveis:** paridade Flutter, recursos de roadmap, relatórios adicionais e CSP com nonces para retirar a permissão de scripts inline.

Não foram encontrados marcadores TODO/FIXME nos diretórios de código próprios pesquisados. Isso não significa ausência de funcionalidades pendentes: o diagnóstico confrontou comportamento e documentação.

## Correções aplicadas e evidências

| Problema | Causa e correção | Validação |
| --- | --- | --- |
| Clientes IPv6 compartilhavam indevidamente a mesma cota | Separação pelo primeiro `:` foi substituída por `net.SplitHostPort`; resposta 429 informa Retry-After | Teste distingue dois IPv6 e impede evasão pela troca de porta |
| JSON aceitava outro objeto ou lixo após o primeiro documento | Decoder agora exige EOF após o objeto e permite somente whitespace final | Testes de documentos concatenados, lixo, campos desconhecidos e objeto válido |
| Compilação local usava biblioteca padrão vulnerável | Go mínimo e Docker de build atualizados para 1.26.10 | Testes, integração com detector de corridas e govulncheck repetidos |
| Web não definia cabeçalhos de segurança | CSP, HSTS em produção, nosniff, bloqueio de frames, Referrer-Policy e Permissions-Policy | Build e resposta HTTP local 200 com os cabeçalhos |
| Documentos legais não existiam | Rotas `/termos` e `/privacidade` com rascunho explícito e links no rodapé | Build web inclui as duas rotas; conteúdo requer revisão jurídica |
| CI não cobria build e segurança em todo push/PR | Novo workflow para Go, web, auditoria de dependências e histórico de segredos | Comandos equivalentes executados localmente; execução remota da CI ainda pendente |

### Verificações executadas

- `go test ./...`, `go vet ./...` e `go build ./cmd/server ./cmd/migrate`: aprovados.
- Migrations e `go test -count=1 -v ./...` em branch Neon exclusivo: aprovados, incluindo TestTenantFlow, TestBillingAndPlatformIsolation e TestTireLifecycleAndAutomation.
- `go test -race -count=1 ./...` no mesmo branch, com Go 1.26.10: aprovado.
- API local conectada ao branch isolado: `/health` retornou `status=ok` e `tire_lifecycle_revision=2`.
- `npm run build`: aprovado, com checagem de rotas e TypeScript.
- Chrome headless: landing, login, cadastro, termos e privacidade aprovados em 1440 px e 390 px, sem overflow horizontal nem erros JavaScript. Não substitui o E2E autenticado e de pagamento.
- `npm audit --omit=dev`: zero vulnerabilidades de produção reportadas na web.
- `flutter analyze` e `flutter test`: aprovados. Não comprovam uso em aparelho real nem build release.
- Govulncheck inicial com Go 1.26.5: 16 vulnerabilidades alcançáveis na biblioteca padrão. Após atualização: zero vulnerabilidades alcançáveis e zero em pacotes importados. Persiste um aviso de módulo para `golang.org/x/crypto/openpgp`, que o aplicativo não importa, sem versão corrigida informada pelo scanner. Argon2id foi preservado.
- Gitleaks 8.30.1 analisou 19 commits. Dois achados eram senhas fixas de contas sintéticas de QA; `.gitleaksignore` contém somente os dois fingerprints históricos. Reexecução: sem achados fora dessas exceções. Não foi identificada chave real que exigisse rotação nessa varredura.

O Docker estava desligado. Falhas iniciais de Keychain/cache no sandbox foram contornadas executando as verificações com as permissões necessárias; não eram falhas de compilação do produto. Não houve alteração de schema ou promoção no banco de produção.

A disponibilidade do Go 1.26.10 foi confirmada no [catálogo oficial de releases](https://go.dev/dl/?mode=json).

## Checklist pronto para vender

- [x] Build web, compilação Go e verificações Flutter.
- [x] Testes com PostgreSQL real em ambiente isolado.
- [x] Regressão dos defeitos de IPv6 e JSON.
- [x] Atualização do compilador e auditoria de vulnerabilidades alcançáveis.
- [x] Cookies protegidos e controle de Origin existentes; cabeçalhos web adicionados.
- [x] RLS, limites de plano, isolamento de cobrança e conciliação idempotente cobertos pelos testes existentes.
- [x] Histórico Git varrido com exceções específicas de fixtures.
- [x] Rascunhos de Termos e Privacidade disponíveis.
- [x] Arquivo de exemplo documenta variáveis Asaas sem credenciais reais.
- [ ] Recorrência: adiada por decisão do proprietário, fora da oferta atual.
- [ ] Pagamento completo, notificações e falhas no sandbox: faltam conta/chave/webhook Asaas.
- [ ] Verificação obrigatória de e-mail, envio real e e-mails transacionais comerciais: faltam configuração Resend e implementação complementar.
- [ ] Exportação/exclusão de conta e retenção: recursos e política ainda precisam ser concluídos.
- [ ] Termos finais, controlador, canal de privacidade e suporte: revisão jurídica/operacional pendente.
- [ ] Auditoria de segurança completa de todos os endpoints e sessões: os testes aprovados cobrem uma parte dos riscos, não certificam toda a aplicação.
- [ ] Outbox, observabilidade operacional e proteção distribuída contra abuso: pendentes.
- [ ] Backup com recuperação exercitada e política de retenção: não validados nesta revisão.
- [ ] Homologação completa e promoção para produção: bloqueadas pelos itens acima.
- [ ] CI remota, domínio próprio, alertas e revisão publicada: não verificados nesta revisão.

## Ativação manual e próxima validação

1. Disponibilizar conta Asaas **sandbox** e guardar `ASAAS_API_KEY` somente nos segredos da API. Definir `ASAAS_ENVIRONMENT=sandbox`, `ASAAS_WEBHOOK_TOKEN` aleatório com pelo menos 32 caracteres, `ASAAS_LIVE_ENABLED=false`. Habilitar `BILLING_PAYMENTS_ENABLED=true` somente no ambiente de homologação preparado para o teste.
2. Configurar no Asaas o webhook de homologação em `https://<api-homologacao>/api/v1/webhooks/asaas`, com o token correspondente. Testar criação de cobrança, confirmação, reenvio do mesmo evento, divergência de valor e estorno. Os testes SQL existentes não substituem essas notificações reais.
3. Disponibilizar Resend, verificar o domínio remetente e configurar `RESEND_API_KEY`, `EMAIL_FROM` e `APP_URL` na API. Validar cadastro → verificação, recuperação e convite. Não enviar chaves pelo chat nem versionar arquivos `.env`.
4. Preencher identificação do fornecedor, contato e política de retenção com revisão jurídica. Concluir os recursos e controles pendentes antes de aceitar clientes reais.
5. Configurar monitoramento e backups, restaurar em banco isolado e registrar evidência do exercício.
6. Executar CI e homologação completa. Só depois aplicar o roteiro de [deploy](DEPLOY.md), conferir revisão da API, migrations, Ready da web e domínio.

Não foi contratado plano de hospedagem nem alterado domínio nesta revisão. Custos devem ser confirmados nos painéis dos provedores antes de contratar; preços históricos não foram usados como orçamento.

## Rollback

Para estas correções não há nova migration a reverter. Guardar o SHA anterior e o novo SHA; publicar API e web primeiro em homologação. Em incidente, interromper a promoção e reimplantar a revisão conhecida. Não reativar pagamentos reais ao reverter. O retorno a um compilador vulnerável exige avaliar o risco; priorizar correção progressiva quando possível. Para mudanças futuras de schema, restaurar um branch separado e verificar compatibilidade antes de trocar conexões; nunca executar rollback destrutivo sem preservar os dados.
