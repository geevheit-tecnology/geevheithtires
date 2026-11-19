# Prontidão para 100 empresas e 500 usuários

Data: 29/10/2026. Cenário informado pelo proprietário: até 100 empresas e 500 usuários cadastrados. Isso não significa 500 logins ou 500 operações simultâneas.

## Entrega implementada

O centro administrativo reúne visão geral, clientes, Starter/Pro/Enterprise, financeiro de mensalidades, chamados, auditoria e dados/senha do administrador. Todos os dados operacionais vêm da API e do PostgreSQL.

Salvar e publicar um plano atualiza preço e limites na landing e na tela de cobrança do cliente, independentemente de Asaas estar ativo. Páginas abertas consultam alterações a cada 15 segundos e ao recuperar foco. O proxy web aceita PUT com a mesma proteção de origem dos demais métodos.

A opção de sincronização aplica limites aos contratos já vinculados ao plano; recusa a alteração se algum cliente já ultrapassar os novos limites. Preço e limites de uma mensalidade já registrada ficam preservados. A avaliação inicial mantém a política existente de 15 veículos / 3 usuários, com recursos Starter; ela não se transforma em contrato pago.

O financeiro permite criar mensalidade manual, informar link HTTPS existente, confirmar recebimento com referência e data, cancelar título em aberto e registrar estorno. A confirmação aplica um mês de acesso com as condições da mensalidade. A mesma baixa concorrente não estende a vigência duas vezes. Recebimentos e estornos alteram os indicadores e o histórico do cliente. Estorno bancário e conciliação manual continuam sendo responsabilidade do operador; o sistema não realiza transferência de dinheiro.

A troca da senha administrativa exige a senha atual, invalida tokens e sessões anteriores por versão e renova a sessão atual. A permissão de plataforma é consultada no banco; ser owner de uma empresa não concede administração global.

## Evidências obtidas

Ambiente: artefato Next standalone e API Go locais, conectados a uma branch Neon real e isolada. Nenhuma resposta da API foi interceptada. A carga foi preparada com 100 empresas, 500 usuários, 1.000 veículos, 8.000 pneus e 8.000 inspeções, além da identidade administrativa separada. O banco também continha os registros herdados da cópia de produção; a carga de teste ficou identificada e não foi gravada no banco público.

- Go vet e suíte com detector de corridas aprovados; testes cobrem isolamento, proteção de funções SQL, preços, concorrência, financeiro, estorno, cotas compartilhadas e revogação de sessões.
- Build e TypeScript aprovados. Auditoria npm de produção: zero vulnerabilidades.
- Govulncheck: zero vulnerabilidades alcançáveis. O scanner mencionou um advisory em módulo requerido, sem chamada alcançável no código.
- Chrome desktop/celular: planos, publicação no site sem reload, limites e preço no cliente, lançamento e recebimento com referência no cliente, navegação completa, senha, logout e novo login aprovados, sem erros JavaScript.
- Capturas: `docs/qa/admin-control-desktop.png` e `docs/qa/admin-control-mobile.png`.
- Relatório de carga: `docs/qa/capacity-2026-10-29.json`; roteiro reproduzível: `scripts/capacity-e2e.mjs`.

| Requisições simultâneas | Requisições | Erros | Requisições/s | p95 |
| --- | ---: | ---: | ---: | ---: |
| 10 | 934 | 0 | 61,08 | 402,1 ms |
| 25 | 1.257 | 0 | 80,80 | 711,2 ms |
| 50 | 1.398 | 0 | 88,79 | 961,1 ms |

Total: 3.589 respostas, sendo 3.230 leituras HTTP 200 e 359 inspeções HTTP 201. As 500 identidades apareceram na execução. A tentativa de abrir o mapa de outra empresa retornou 404. As rodadas duraram cerca de 15 segundos cada; não são teste de longa duração nem certificação de capacidade da Render/Vercel. A carga não inclui as APIs externas de e-mail, pagamento ou consultas cadastrais.

## Proteções operacionais

- Pool PostgreSQL explícito: `DB_MAX_CONNS=20` por instância por padrão, configurável entre 2 e 100. Conexão direta preservada por causa de `SET ROLE`; não apontar essa implementação para pool transacional.
- Conexão limitada a 10 s; comandos e transações ociosas limitados a 20 s; espera de locks limitada a 8 s. Conexões ociosas são liberadas.
- Até quatro verificações de senha simultâneas por processo para limitar memória Argon2. Excesso recebe 429 com Retry-After; o cliente pode tentar novamente.
- Cotas de autenticação compartilhadas no PostgreSQL, por conta e por origem TCP, usando somente HMACs. O sistema não confia em X-Forwarded-For arbitrário. Cotas de operações autenticadas são separadas por usuário.
- Catálogo público de planos separado da cota das tentativas de login.
- Limpeza de cotas antigas e de janelas em memória.
- Requisições lentas/5xx registradas com request ID, rota normalizada, duração e status; senhas/tokens não são registrados. Admin mostra estado do pool.
- A manutenção verifica se a empresa continua ativa antes de trabalhar; empresas encerradas após capturar a lista são ignoradas.

## Publicação segura

1. Conferir migrações 000015–000017 em branch isolada. Aplicar as mesmas migrations em produção pelo runner oficial, com conexão direta e lock de migration. A migração 000015 corrige o alias reservado do relatório mensal antes da sua primeira aplicação.
2. Enviar a revisão final ao GitHub e publicar a API `geevheittire-api` na Render. O serviço versionado usa deploy manual, não automático.
3. Aguardar `/health` responder `admin_revision=2` e `release=2026-10-29-admin-control`. A revisão anterior só expõe a administração antiga; não publicar a nova interface contra ela.
4. Publicar o frontend de produção na Vercel a partir da raiz do repositório, preservando as variáveis de produção. Conferir READY e aliases.
5. Fazer homologação do host com tráfego autorizado, janela de observação e monitoramento de CPU, RAM, conexões, 5xx e latências. Começar com poucos clientes e ampliar após observar o uso.
6. Em rollback do código, preservar as tabelas e registros financeiros. Não apagar migrations/dados nem executar down destrutivo; os campos adicionados são compatíveis com o código anterior, mas a revogação por versão exige a API nova.

## Condições antes de declarar produção em escala

A configuração versionada da API ainda é Render Free. A Render informa que instâncias Free não são indicadas para produção, adormecem após 15 minutos e podem levar cerca de um minuto para retomar: https://render.com/docs/free. Contratar ou alterar uma instância paga depende do proprietário; nenhum plano pago foi contratado nesta entrega. Escolher uma instância sempre ativa e validar a carga nela, na região e no plano reais. O resultado local acima não permite escolher um plano apenas pelo nome ou garantir disponibilidade.

Também é necessário conferir no Neon o limite total de conexões/compute, autoscaling, retenção de recuperação e orçamento. Se houver N instâncias, a soma dos pools precisa caber no orçamento de conexões do banco, reservando capacidade para migrations e operação. Não aumentar DB_MAX_CONNS sem medir CPU/locks/latência do banco.

Antes de receber todos os clientes: exercitar restauração de backup em outra branch; configurar monitoramento externo e alertas com responsáveis; aprovar identificação do fornecedor, termos, privacidade e retenção; concluir os fluxos comerciais de e-mail se forem oferecidos. Asaas e Resend permanecem desativados por decisão anterior. Sucesso em teste local não comprova entregabilidade, pagamento externo, operação móvel em aparelho físico ou CI remota.

A aplicação contém controles de isolamento e testes de segurança; isso não equivale a uma auditoria independente de todos os endpoints. Não há promessa de “100% sem falhas”. Os resultados delimitam o que foi implementado e medido, e as condições restantes devem ser verificadas na infraestrutura de produção.
