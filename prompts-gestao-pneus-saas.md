# Kit de Prompts: SaaS de Gestão de Pneus para Transportadoras

Flutter + Go + PostgreSQL (Neon) + deploy. Versão unificada e melhorada dos 3 prompts anteriores, com roadmap de MVP, planos, preços e custos.

---

## Como usar (leia primeiro)

1. Cole o **Prompt Mestre (P0)** no início de CADA conversa nova com a IA de código (Claude Code, Cursor etc.). Ele mantém todas as decisões consistentes.
2. Execute os módulos **na ordem**: P1 → P2 → P3 → P4 → P5. Não pule: cada um depende das tabelas do anterior.
3. Anexe a imagem do layout (mapa de pneus, detalhes, checklist, histórico) junto com o P1 e o P3.
4. Ao final de cada módulo, peça: "rode os testes, liste o que ficou pendente e atualize o README".
5. Trabalhe em um repositório Git com um branch por módulo. Faça commit a cada etapa aprovada.

---

## Decisões de arquitetura recomendadas (melhorias em relação ao rascunho inicial)

| Tema | Decisão | Motivo |
|---|---|---|
| Hospedagem da API Go | **Servidor Go contínuo** (Fly.io, Railway ou Google Cloud Run) | Go na Vercel funciona só como função serverless: ruim para jobs, filas, cron longo e importações grandes |
| Vercel / Netlify | Usar para **landing page e web admin** (Next.js) | É onde elas são fortes |
| Banco | **Neon** (Postgres) com pooler | Branching para testes, escala a zero no início |
| Fotos | **Cloudflare R2** (ou Supabase Storage) com URLs assinadas | Barato, sem custo de saída |
| Web admin | **Next.js** (gestão e relatórios) | Melhor SEO e ecossistema que Flutter Web para painel e landing |
| Mobile | **Flutter** (pátio, inspeção, offline) | Câmera, QR e modo offline |
| Filas e jobs | **River** (fila em Postgres para Go) ou Asynq + Redis (Upstash) | Evita infraestrutura extra no início |
| Pagamentos | **Asaas** (Pix, boleto, cartão, assinaturas, nota fiscal) | Padrão no Brasil; Stripe como alternativa para exterior |
| E-mail | Resend ou Amazon SES | Barato e simples |
| Push | Firebase FCM | Padrão |
| Observabilidade | Sentry + logs estruturados (slog) + PostHog | Cobre erros, logs e produto |

Regras de status de sulco (padrão, configuráveis por empresa e por tipo de veículo):
- **OK (verde):** sulco maior ou igual a 5 mm
- **ALERTA (amarelo):** de 3 mm até menos de 5 mm (planejar recapagem ou troca)
- **CRÍTICO (vermelho):** menor que 3 mm, ou dano estrutural (corte, bolha, banda soltando)
- **Mínimo legal no Brasil:** 1,6 mm (indicador TWI). Nunca permitir configurar abaixo disso.
- **SEM PNEU (cinza):** posição vazia

---

## P0: PROMPT MESTRE (colar no início de toda conversa)

```text
Você é um arquiteto sênior, product manager e desenvolvedor full-stack. Estamos construindo "TirePro" (nome provisório), um SaaS B2B de gestão de pneus para transportadoras, para vender em modelo self-service. Idioma da interface, mensagens e documentação: pt-BR. Código e nomes técnicos: inglês.

STACK
- Mobile: Flutter 3.x, Riverpod, go_router, Dio, freezed, json_serializable, drift (SQLite offline), image_picker, mobile_scanner (QR), fl_chart, flutter_secure_storage, Firebase FCM.
- Backend: Go 1.22+, chi, pgx, sqlc, golang-migrate, JWT (access + refresh), go-playground/validator, slog, River (jobs), testify.
- Banco: PostgreSQL no Neon, UUID v7, timestamptz, soft delete, Row Level Security por company_id.
- Web admin e landing: Next.js (App Router) + Tailwind + shadcn/ui na Vercel.
- Storage: Cloudflare R2 com URLs assinadas. Deploy da API: Fly.io (ou Cloud Run) com Docker.
- Cobrança: Asaas. E-mail: Resend. Erros: Sentry. Analytics: PostHog.

PRINCÍPIOS
1. Multi-tenant desde o dia 1: toda tabela de negócio tem company_id, RLS ativo e teste automatizado de isolamento.
2. Regras de negócio ficam no backend (nunca no app). O app só exibe e coleta.
3. Offline-first no mobile: toda inspeção funciona sem internet e sincroniza depois, com idempotência (client_id por registro).
4. Auditoria: toda alteração relevante grava em audit_log (quem, quando, antes, depois).
5. Regras primeiro, IA depois: cálculos determinísticos e auditáveis no Go; LLM só para linguagem natural e imagem.
6. Cada módulo termina com: migrations, código, testes unitários, seeds e README atualizado.
7. Erros de API padronizados (RFC 7807), paginação por cursor, versionamento /api/v1, OpenAPI gerado.
8. Segurança: LGPD, senhas com argon2id, segredos criptografados (AES-GCM), rate limit, CORS restrito.

COMO TRABALHAR
- Antes de codar cada etapa, apresente um plano curto e espere minha confirmação.
- Entregue código completo e executável, não pseudocódigo.
- Ao terminar cada etapa, liste o que foi feito, o que falta e riscos.
- Se houver ambiguidade, assuma o padrão mais comum do mercado brasileiro de frotas e registre a suposição.
```

---

## P1: FUNDAÇÃO + MAPA DE PNEUS + INSPEÇÃO (núcleo do MVP)

```text
MÓDULO 1: FUNDAÇÃO E INSPEÇÃO. Siga o Prompt Mestre. Use a imagem anexa como referência fiel do layout (cores, posições, abas, cards).

## ENTREGAR
1. Estrutura de pastas do monorepo (/mobile, /api, /web, /infra, /docs).
2. Migrations SQL: companies, users (roles: owner, manager, tire_tech, driver, inspector), vehicles, axle_templates, axle_template_positions, tires, tire_events, inspections, inspection_photos, alerts, audit_log, sync_log. UUID, timestamps, índices, RLS.
3. Autenticação: login, refresh, logout, recuperação de senha, RBAC por role, middleware de tenant.
4. Regra de classificação automática (pacote Go isolado, com testes de tabela):
   - OK, ALERTA, CRÍTICO conforme sulco e danos, thresholds configuráveis por empresa e por tipo de veículo.
   - Nível 1: desgaste, candidato a recapagem (sulco entre 3 e 5 mm, sem dano estrutural).
   - Nível 2: limite (TWI), aderência comprometida, intervenção imediata.
   - Nível 3: crítico, descarte imediato (rachadura, deformação, banda soltando).
   - Retorna status, nível, recomendação em texto e motivo.
5. API /api/v1: auth, vehicles (CRUD), tires (CRUD), GET /vehicles/{id}/tire-map, POST /tires/{id}/inspections, GET /tires/{id}/history, POST /checklists (lote, idempotente), POST /uploads/sign, GET /alerts, PATCH /alerts/{id}/read, GET /dashboard.
6. Alertas automáticos: gerados ao salvar inspeção (sulco crítico, pressão fora da faixa, temperatura alta), com push FCM e e-mail.

## TELAS FLUTTER (tema azul-marinho #0F1B3D, Material 3)
- Navegação inferior: Dashboard, Checklist, Pneus, Alertas, Mais.
- MAPA DE PNEUS (abas Mapa | Lista | Histórico): cabeçalho com placa, tipo e status; veículo desenhado com CustomPainter a partir do template de eixos; "FRENTE" no topo; steps abaixo; posições 1EE, 1EI, 1DI, 1DE etc.; cada pneu mostra código, mm e status com cor; toque abre os detalhes; legenda (OK, ALERTA, CRÍTICO, SEM PNEU).
- DETALHES DO PNEU (abas Geral | Histórico | Inspeções | Manutenções): marca, modelo, medida, DOT, tipo, vida útil estimada, km atual, km restante, custo de aquisição, custo por km; status atual (sulco, pressão, temperatura, desgaste, classificação, recomendação); botões Nova Inspeção, Histórico, Mais.
- CHECKLIST: filtros por eixo (Todos, Eixo 1, 2, 3, Step); por pneu: sulco, pressão, até 4 fotos; status calculado em tempo real ao digitar; salvamento offline (drift) e sincronização automática.
- HISTÓRICO COMPLETO: linha do tempo (instalação, rodízio, inspeção, manutenção) com filtros.
- ALERTAS: lista com severidade, lida/não lida.

## SEEDS
Empresa demo, veículo ABC-1234 (carreta 9 eixos) com os pneus do exemplo (P00123 a P00136, incluindo steps), usuários de cada perfil.

Comece pela estrutura, o ER (Mermaid) e as migrations. Aguarde minha confirmação antes de seguir.
```

---

## P2: CICLO DE VIDA, PASSAGEM POR VEÍCULOS, CUSTO E IMPORTAÇÃO

```text
MÓDULO 2: CICLO DE VIDA, VIDA ÚTIL E CADASTROS. Estende o módulo 1 sem quebrar migrations existentes.

## 1. MÁQUINA DE ESTADOS DO PNEU
Estados: EM_COMPRA, EM_ESTOQUE, INSTALADO, REMOVIDO, EM_RECAPAGEM, EM_GARANTIA, DESCARTADO.
- Transições validadas no backend com tabela de transições permitidas (ex.: não instalar pneu descartado ou já instalado).
- Cada transição grava tire_events: data, km do veículo, responsável, motivo, anexos.
- Vidas: Nova, 1ª, 2ª, 3ª recapagem (limite por empresa). Cada vida tem custo, km, sulco inicial/final e CPK próprios.
- Motivos de remoção: desgaste, dano, rodízio, venda do veículo, recapagem, sinistro.

## 2. CADASTRO E ENTRADA DE PNEUS
Campos: código (gerado ou manual), QR code e código de barras, etiqueta imprimível (PDF), marca, modelo, medida, DOT (semana/ano, alerta de pneu velho), tipo (direcional, tração, livre), índices de carga e velocidade, sulco inicial, pressão recomendada, fornecedor, nota fiscal, data de compra, custo, lote, garantia (prazo e km), vida útil estimada, observações, fotos.
- Entrada individual, em lote (várias unidades da mesma NF) e por importação CSV/XLSX.
- Leitura de QR/código de barras no app para localizar o pneu.

## 3. PASSAGEM POR VEÍCULOS (tire_assignments)
tire_id, vehicle_id, position, km_in, km_out, installed_at, removed_at, reason, user_id, tread_in, tread_out, life_number.
- Instalar fecha automaticamente a passagem anterior.
- Rodízio entre posições ou veículos com km e sulco.
- Linha do tempo: veículo, posição, período, km rodado, desgaste (mm) e mm por 1.000 km por posição.
- Visão inversa: pneus que já passaram por um veículo ou posição.

## 4. VIDA ÚTIL E CUSTO (jobs agendados + sob demanda)
- km total e por vida, km restante estimado.
- Taxa de desgaste (mm/1.000 km) e projeção de data/km de troca pelas últimas inspeções.
- CPK = (aquisição + recapagens + manutenções) / km total, por vida e acumulado.
- Indicadores: % de vida consumida, a vencer em 30 dias, desgaste irregular (diferença de sulco no mesmo eixo), pressão fora do padrão, DOT vencendo.
- Alertas por regra configurável.
- Relatórios: CPK por marca/modelo/medida/posição/veículo, vida média, novo x recapado, ranking de fornecedores; exportação CSV e PDF.

## 5. RECAPAGEM E MANUTENÇÃO
- Ordem de recapagem: pneus enviados, recapadora, envio/retorno, custo, tipo de banda, status (enviado, aprovado, reprovado na carcaça), garantia.
- Manutenções: calibragem, reparo, balanceamento, alinhamento, com custo e responsável.

## 6. CADASTRO DE VEÍCULOS (3 ORIGENS)
Campos: placa (antiga e Mercosul), chassi, RENAVAM, tipo, marca/modelo/ano, template de eixos, km atual, horímetro, status, filial/centro de custo, motorista, origem (manual | importado | integrado), external_id.
A) MANUAL: formulário no app e na web com validação.
B) IMPORTAÇÃO CSV/XLSX (veículos e pneus): upload, mapeamento de colunas, pré-visualização com validação linha a linha, deduplicação por placa/chassi, modo "criar e atualizar", relatório de erros baixável, tabela import_jobs, processamento assíncrono e idempotente.
C) INTEGRAÇÃO: adapters em Go (interface VehicleProvider: ListVehicles, GetOdometer, HandleWebhook) para ERPs, rastreadores e APIs de terceiros.
   - Tabelas integrations (credenciais criptografadas, mapeamento, última sincronização), sync_logs, odometer_readings (fonte, data, valor).
   - Sincronização agendada e webhooks de entrada com assinatura HMAC.
   - Conflitos: manual prevalece sobre integração (configurável); odômetro nunca diminui (gera alerta de inconsistência).
   - Entregar um adapter genérico REST/JSON configurável e um adapter mock para testes.

## 7. TELAS
Lista de pneus (busca, filtros, QR); wizard de cadastro (dados, compra, fotos, etiqueta); ficha do pneu (Resumo, Linha do tempo, Veículos, Custos, Recapagens, Documentos) com gráfico sulco x km e projeção; ações instalar/remover/rodízio/recapagem/descartar com formulário; veículos (lista, cadastro, importar planilha, integração, status da sincronização); dashboard de gestão.

## 8. API
POST/GET /tires, POST /tires/batch, POST /tires/import, GET /tires/{id}/lifecycle, /assignments, /cost, POST /tires/{id}/install, /remove, /move, /retread-orders, /discard, GET /vehicles/{id}/tire-history, POST /vehicles/import, POST /vehicles/{id}/odometer, POST /integrations, POST /integrations/{id}/sync, POST /webhooks/{provider}, GET /reports/cpk, /lifespan, /retread.

## ENTREGAR
ER atualizado (Mermaid) e migrations; regras de domínio com testes (estados, CPK, projeção); endpoints, adapters e jobs; telas na ordem cadastro, ficha, instalação/rodízio, veículos; seed (pneu com 2 vidas passando por 3 veículos); README.
Comece pelo ER e migrations e aguarde confirmação.
```

---

## P3: FROTA DE TRANSPORTADORA (tipos de veículo e operação)

```text
MÓDULO 3: FROTA DE TRANSPORTADORA. Estende os módulos 1 e 2.

## 1. TEMPLATES DE EIXO (seed, editáveis por empresa)
- Cavalo mecânico: 4x2, 6x2, 6x4 (direcional + tração, simples ou duplo).
- Carretas: 2, 3 e 4 eixos (bitrem, rodotrem, graneleira, baú, sider, prancha), com 1 ou 2 steps.
- Caminhão 3/4, VUC, toco e truck.
- Van, utilitário e furgão (4 pneus + step).
- Cada template define: posições, nomenclatura, pressão recomendada, sulco mínimo, medida padrão, regras de rodízio por posição e CPK de referência.
- Conjunto cavalo + carreta: tabela vehicle_couplings (acoplamento e desacoplamento com datas). O km do conjunto é atribuído ao cavalo; o histórico do pneu da carreta registra qual cavalo a tracionou.
- Mapa visual dinâmico no Flutter gerado a partir do template (sem desenhar um layout por tipo).

## 2. OPERAÇÃO
- Perfis: dono, gestor de frota, borracheiro, motorista (checklist simples do próprio veículo), inspetor terceirizado.
- Estoque por filial e transferência entre filiais.
- Indicadores: CPK por veículo, rota e cliente; custo de pneu por viagem; disponibilidade da frota.
- Fornecedores e recapadoras com avaliação de desempenho.
- Modo offline completo com resolução de conflitos (regra: última inspeção por data/hora do dispositivo, com log do conflito).

## ENTREGAR
Seed de todos os templates; mapa dinâmico; vehicle_couplings com regras de km; telas de acoplamento, estoque por filial e transferência; indicadores de transportadora; testes.
Comece propondo o modelo de dados e aguarde confirmação.
```

---

## P4: INTELIGÊNCIA ARTIFICIAL (módulo ai/)

```text
MÓDULO 4: IA. Arquitetura em camadas: (1) regras e estatística no Go, (2) LLM (API da Anthropic) apenas para linguagem natural e imagem. Use cache, limites por plano e registre o custo de tokens por empresa.

## RECURSOS
A) PREVISÃO DE DESGASTE E TROCA: regressão sobre inspeções (mm/1.000 km por posição, veículo, rota e carga); data prevista de troca com intervalo de confiança.
B) DESGASTE IRREGULAR: detecta padrões (ombros, centro, um lado só, serrilhado) e sugere causa provável (alinhamento, calibragem, suspensão) e ação.
C) ANÁLISE DE FOTO: LLM com imagem retorna JSON estruturado (sulco aproximado, cortes, bolhas, desgaste irregular, dano, confiança). Confiança baixa exige validação humana. A foto nunca substitui a medição com paquímetro, e a tela deve deixar isso claro.
D) RODÍZIO E RECAPAGEM: sugere movimentações para equalizar o desgaste; decide recapar ou descartar por custo, carcaça e vida.
E) ANOMALIAS: queda de pressão, perda acelerada de sulco, km inconsistente, possível fraude ou furto.
F) COMPRAS: previsão de demanda mensal por medida e frota, estoque mínimo sugerido, comparação de CPK por marca e fornecedor.
G) ASSISTENTE CONVERSACIONAL ("pergunte aos seus pneus"): tool calling com funções seguras do backend (sem acesso direto ao banco), respeitando tenant e RBAC. Exemplos: "quais pneus vencem no mês que vem?", "qual marca tem menor CPK nas carretas?".
H) RESUMO SEMANAL por e-mail e push: insights, economia estimada, ações prioritárias.

## REQUISITOS TÉCNICOS
- Interface Go AIProvider com implementação Anthropic e mock.
- Prompts versionados em arquivos, com testes usando fixtures e respostas simuladas.
- Tabela ai_insights (tipo, entidade, severidade, explicação, ação sugerida, confiança, status aceito/ignorado/ajustado) para medir utilidade.
- Tabela ai_usage (empresa, recurso, tokens entrada/saída, custo estimado).
- Limites por plano com bloqueio educado e aviso de upgrade.
- Guardrails: validar JSON de saída por schema; nunca enviar dados pessoais desnecessários ao LLM; registrar prompt e resposta para auditoria (com retenção).

## ENTREGAR
Pacote ai/ com interfaces, A, B, D, E, F em regras/estatística primeiro; depois C, G, H com LLM; telas de insights, assistente de chat e análise de foto; testes e README.
Comece pelo desenho do módulo e aguarde confirmação.
```

---

## P5: SaaS (onboarding, planos, cobrança, super admin, landing)

```text
MÓDULO 5: SaaS PRONTO PARA VENDER. Estende os módulos anteriores.

## 1. MULTI-TENANT E SEGURANÇA
RLS em todas as tabelas com testes automáticos de isolamento entre empresas; 2FA opcional (TOTP); LGPD (exportação e exclusão de dados da empresa, registro de consentimento); backups e retenção; criptografia de credenciais; auditoria.

## 2. ONBOARDING SELF-SERVICE
Cadastro, verificação de e-mail, criação da empresa (CNPJ), wizard "cadastre seu primeiro veículo e pneu" ou importe uma planilha, dados de demonstração opcionais, checklist de ativação, e-mails de ativação (dia 0, 2, 5, 12 do trial).

## 3. PLANOS, LIMITES E COBRANÇA
- Tabelas: plans, subscriptions, invoices, usage_counters, feature_flags.
- Planos Starter, Pro, Enterprise por número de veículos e usuários; recursos por plano (IA, integrações, API, relatórios).
- Enforcement no backend (middleware de limites e flags), nunca só no app.
- Asaas: Pix, boleto e cartão, trial de 14 dias sem cartão, upgrade e downgrade proporcional, inadimplência (aviso, bloqueio parcial somente leitura após X dias, reativação automática), recibos, webhooks com idempotência.

## 4. PAINEL SUPER ADMIN (dono do SaaS)
Empresas, MRR, churn, trials e conversão, uso de IA e custo, tickets de suporte, impersonation auditado (com aviso ao cliente), feature flags, saúde das integrações.

## 5. WEB ADMIN E LANDING (Next.js na Vercel)
Landing com proposta de valor, calculadora de economia (CPK), preços, CTA "Teste grátis 14 dias", FAQ, termos de uso, política de privacidade e página de segurança. Web admin com relatórios, importações, gestão de usuários, branding por empresa (logo e cores) em relatórios e etiquetas.

## 6. API PÚBLICA E WEBHOOKS
API keys por empresa, rate limit por plano, OpenAPI e página de documentação, webhooks de saída configuráveis (pneu crítico, inspeção criada) com assinatura HMAC e retentativas.

## 7. OBSERVABILIDADE E PRODUTO
Logs estruturados, Sentry, health checks, alertas, métricas por empresa, PostHog (ativação, retenção, uso por recurso).

## ENTREGAR
Migrations novas; billing e enforcement com testes; onboarding; super admin; landing; documentação OpenAPI; runbook de operação (incidente, backup, restore); checklist de lançamento.
Comece propondo o fluxo de onboarding e o modelo de planos e aguarde confirmação.
```

---

## ROADMAP DE MVP (para começar a vender cedo)

| Fase | Duração (1 dev full-time + IA) | Conteúdo | Marco de venda |
|---|---|---|---|
| **0. Validação** | 1 a 2 semanas | Conversar com 5 a 10 transportadoras, validar dor (CPK, retrabalho em planilha), fechar 2 pilotos | Pilotos combinados |
| **1. MVP** | 6 a 8 semanas | P0 + P1: mapa de pneus, inspeção offline, alertas por regra, dashboard simples, importação de planilha, cadastro manual | Primeiros pilotos usando (gratuito ou preço de fundador) |
| **2. Cobrança** | 3 a 4 semanas | Parte de P5: planos, trial, Asaas, onboarding self-service, landing | **Começa a vender sozinho** |
| **3. Gestão profissional** | 6 a 8 semanas | P2: ciclo de vida, CPK, recapagem, relatórios, QR | Argumento "controle de custo" |
| **4. Frota completa** | 4 semanas | P3: todos os tipos de veículo, cavalo + carreta, filiais | Atende transportadoras maiores |
| **5. IA** | 6 semanas | P4: previsão, irregular, foto, assistente | Diferencial e upsell (plano Pro) |
| **6. Integrações e escala** | contínuo | Adapters de ERP e rastreadores (um por vez, conforme demanda real), API pública, super admin completo | Plano Enterprise |

Dicas de execução:
- **Venda antes de construir tudo.** O MVP da fase 1 já resolve a dor principal (saber o estado de cada pneu sem planilha).
- **Integrações só com demanda:** construa o adapter do sistema que um cliente pagante usa, não de todos.
- **Foto com IA** é ótima demonstração comercial, mas valide a precisão com pneus reais antes de prometer.
- Crie um programa de **cliente fundador** (desconto vitalício para os 10 primeiros) para financiar o início.

---

## PLANOS E PREÇOS SUGERIDOS (hipóteses para validar com clientes)

Modelo: assinatura mensal por faixa de veículos, com desconto no plano anual (2 meses grátis). Valores em R$ e sujeitos a teste de mercado.

| | **Starter** | **Pro** | **Enterprise** |
|---|---|---|---|
| Preço mensal | R$ 149 | R$ 449 | A partir de R$ 1.200 (sob consulta) |
| Veículos | até 15 | até 60 | ilimitado (faixas) |
| Usuários | 3 | 10 | ilimitado |
| Mapa, inspeção offline, alertas por regra | Sim | Sim | Sim |
| Ciclo de vida, CPK, recapagem | Básico | Completo | Completo |
| Relatórios CSV/PDF | Limitado | Sim | Sim + personalizados |
| IA (previsão, irregular, resumo semanal) | Não | Sim | Sim |
| Análise de foto e assistente | Não | 300 usos/mês | Limite negociado |
| Integrações (ERP, rastreador) | Não | 1 integração | Múltiplas + suporte |
| API pública e webhooks | Não | Leitura | Completa |
| Filiais e estoque por filial | 1 | até 3 | Ilimitado |
| Suporte | E-mail | E-mail e chat | Gerente de conta + SLA |

Veículo extra: R$ 6 a R$ 9 por veículo/mês acima da faixa. Implantação e importação assistida (opcional): R$ 500 a R$ 2.500.

Raciocínio da precificação:
- Uma transportadora com 30 veículos costuma gastar dezenas de milhares de reais por ano com pneus. Reduzir o CPK em poucos pontos percentuais já paga a assinatura várias vezes; use isso no discurso comercial.
- Ancore o preço no valor (economia estimada), não no custo de desenvolvimento.
- Teste 2 ou 3 preços nos pilotos e ajuste.

---

## CUSTOS ESTIMADOS (ordens de grandeza; confirme nos sites oficiais, pois mudam)

**Infraestrutura fixa no início (por mês), estimativa:**

| Item | Estimativa |
|---|---|
| Neon (Postgres, plano pago de entrada) | US$ 19 a 69 |
| API Go (Fly.io / Railway / Cloud Run) | US$ 10 a 40 |
| Vercel (landing + web admin) | US$ 0 a 20 |
| Cloudflare R2 (fotos) | US$ 1 a 15 |
| E-mail (Resend / SES) | US$ 0 a 20 |
| Sentry + PostHog (planos de entrada) | US$ 0 a 50 |
| Apple Developer / Google Play | US$ 99 por ano / US$ 25 uma vez |
| **Total aproximado** | **US$ 50 a 250 por mês** no começo |

**Custo variável por cliente (por mês), estimativa:**
- Infra (banco, storage, banda): R$ 3 a R$ 15 por empresa pequena.
- Asaas: taxa por cobrança (Pix e boleto costumam ser baixos por transação; cartão é percentual). Considere de 2% a 4% da receita.
- **IA:** com regras e estatística no Go o custo é praticamente zero. O custo vem do LLM:
  - Análise de foto: cada imagem custa centavos; 300 fotos/mês por empresa tende a ficar em poucos dólares.
  - Assistente e resumo semanal: texto curto, custo baixo por empresa.
  - Mantenha o limite por plano e o registro em ai_usage para proteger a margem. Meta: custo de IA menor que 10% da mensalidade do plano.

**Margem esperada:** com a mensalidade do Pro em R$ 449 e custos variáveis na casa de algumas dezenas de reais por cliente, a margem bruta tende a ser alta. O que pesa de verdade é suporte, aquisição de clientes e seu tempo.

---

## CHECKLIST DE LANÇAMENTO

- [ ] Teste automatizado de isolamento entre empresas (RLS) passando
- [ ] Backup e restore do Neon testados
- [ ] Fluxo completo: cadastro, trial, cobrança, bloqueio e reativação
- [ ] Modo offline testado em aparelho real, sem sinal, com sincronização e conflito
- [ ] Termos de uso, política de privacidade e DPA (LGPD) revisados por advogado
- [ ] Monitoramento de erros e alertas configurados
- [ ] Documentação de ajuda e vídeo curto de onboarding
- [ ] Contrato e CNPJ para emissão de nota fiscal de serviço
- [ ] 3 clientes-piloto usando em produção por pelo menos 30 dias
- [ ] Meta de ativação definida (ex.: 70% cadastram 1 veículo e 1 inspeção em 7 dias)

---

*Observação: valores de preços, planos e custos são estimativas de referência para planejamento. Valide com clientes reais e confirme as tabelas atuais dos fornecedores antes de decidir.*
