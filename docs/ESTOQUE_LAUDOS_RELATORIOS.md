# Estoque, laudos e relatórios de pneus

Em **Pneus**, clique no número de fogo ou em **Dados e histórico**. O mapa do veículo abre a mesma ficha em uma janela. Cada pneu mantém sua identidade e suas movimentações, mesmo após descarte.

## Retirada e perda da carcaça

Envio/retorno de recapagem e transferência entre locais de estoque exigem Pro ou Enterprise. A ficha, km e laudos permanecem disponíveis no Starter.

A ação **Retirar / avaliar carcaça** exige odômetro do veículo, motivo e laudo com ocorrência, condição da carcaça, avaliação e responsável. A quilometragem é consolidada na mesma transação da retirada. Carcaça preservada retorna ao estoque; carcaça perdida é descartada definitivamente, sem poder ser reinstalada. Descarte de um pneu fora do veículo também exige laudo. O responsável informado e o usuário que registrou a operação ficam identificados no histórico.

O laudo pode ser consultado na ficha ou em **Relatórios de pneus**, com opção de imprimir/salvar PDF pelo navegador. Registros antigos permanecem sem laudo, sem inventar uma avaliação retroativa. A impressão é um registro operacional da avaliação informada; não gera assinatura digital.

## Km e recapagens

A ficha mostra km da vida atual e total rastreado. O retorno da recapagem inicia uma nova vida com zero km, preservando o total anterior. Cada movimentação mostra km totais e km daquela vida. Estepe não acumula percurso como pneu rodante. Percursos anteriores à entrada no sistema não são estimados.

## Relatórios e comparação de marcas

Gestores, proprietários e borracheiros acessam **Relatórios de pneus**. No Starter, a consulta é limitada às 50 ocorrências mais recentes por tipo, sem comparativos ou filtros de período. Pro e Enterprise incluem os recursos avançados descritos abaixo. Consulte [a matriz de planos](PLANOS_E_IDENTIDADE.md). Os filtros de data (UTC) e ocorrência selecionam retiradas, descartes/perdas de carcaça, envios e retornos da recapagem. A lista é paginada em 50 registros.

A comparação usa a posição atual de toda a frota, independente do período das ocorrências. Agrupa marca/modelo/medida/via e exibe amostra, descartados, média de km de pneus descartados com percurso, custo/km, recapagens registradas, estouros e perdas de carcaça. O custo/km é a soma de aquisição e serviços dividida pelo percurso dos pneus com km positivo. Recapagens anteriores à entrada não são contadas como serviços realizados no sistema.

O feedback exige três pneus descartados com km por grupo para comparar médias, sempre dentro da mesma medida/via. Mostra maior média e menor custo/km registrado entre os grupos elegíveis; não representa garantia de desempenho nem corrige diferenças de carga, posição, motorista ou histórico incompleto. O painel avisa quando faltam amostras e quando existem vidas anteriores sem rastreamento.

## Validação e publicação

A migração necessária é `000011_tire_removal_reports.up.sql`. Aplicar primeiro em banco isolado. Backend e frontend precisam dessa migração antes de publicar; código novo de movimentação depende da coluna `removal_report`.

- `cd api && go test -race -count=1 ./...` com `TEST_DATABASE_URL` de um banco descartável e migrado.
- `npm --prefix web run typecheck` e `npm --prefix web run build`.
- `PLAYWRIGHT_MODULE=<caminho-do-playwright> QA_SESSION_FILE=<sessao-qa-temporaria> WEB_URL=http://localhost:3031 node scripts/tire-reports-e2e.cjs`.

A sessão QA é um JSON com `token`, pertencente exclusivamente à empresa fictícia do banco de testes. O teste cria veículo/pneu e cobre retirada, laudo, recapagem, vida atual, estouro, perda definitiva, relatórios, custos e dimensões desktop/celular. Não executar contra produção.

## Fabricação, alertas e rankings (27/10/2026)

- Campo próprio de ano de fabricação no cadastro e na edição da identificação, com sugestão pelo DOT. O servidor recusa divergências; o banco deriva o ano do DOT em coluna gerada. Registros sem DOT não recebem ano inventado. A validade operacional continua independente e informada pela empresa.
- Aviso de validade vencida ou a vencer em 30 dias; rodízio próximo a partir de 90% do intervalo e devido ao atingir o intervalo. Continuam os avisos de sulco, inspeção, pressão, temperatura, dano estrutural, avaliação de recapagem/descarte e estoque mínimo.
- Alerta `cpk_high`: acima de 125% do CPK ponderado dos demais pneus comparáveis. Exige pelo menos 3 outros pneus, todos com ao menos 1.000 km, custo de aquisição positivo, sem recapagens anteriores ao cadastro, mesma medida/via/estado e quantidade de recapagens. O aviso pede revisão dos custos, km e aplicação; não declara defeito da marca. É reconciliado após alterações e pela rotina de manutenção. Não se trata de IA preditiva.
- Ranking Pro de marcas/modelos, por medida e via, ordenado por menor CPK ou maior km médio até descarte. Sem 3 pneus com km para CPK ou 3 descartados com km para durabilidade, o grupo fica sem classificação. Há sinalização de CPK mais de 25% acima do menor grupo comparável.
- Ranking individual Pro por menor CPK, até 100 pneus, com pelo menos 1.000 km, custo positivo e sem recapagens anteriores ao cadastro. Empates usam a mesma posição dentro de medida/via. CPK de pneu em uso é provisório. Os indicadores não controlam diferenças de carga ou eixo; exigem análise operacional antes da compra.

Migração 000014 e regras de alerta validadas apenas na branch temporária. Testes conferem divergência ano/DOT, amostra mínima de CPK, resolução do alerta e transição do aviso antecipado de rodízio para o aviso devido. Fluxo de laudos, km, ficha e relatórios exercitado no Chrome desktop e celular.
