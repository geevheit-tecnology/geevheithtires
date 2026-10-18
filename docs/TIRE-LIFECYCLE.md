# Estoque, ciclo de vida e manutenção de pneus

A tela **Pneus** reúne estoque, pneus instalados, recapagens e descartes. O dossiê de cada pneu mostra movimentos, responsáveis, motivos, veículos/posições, odômetros e custos. As inspeções continuam disponíveis no cadastro do pneu.

## Movimentações

- Instalar um pneu do estoque em uma posição livre.
- Retirar do veículo para um local de estoque.
- Transferir entre locais de estoque ou veículos.
- Fazer rodízio no mesmo veículo, inclusive trocando dois pneus em posições ocupadas. Os dois registros são atômicos.
- Enviar para recapagem com fornecedor; receber com novo sulco e custo do serviço.
- Confirmar descarte após retirada do veículo. O estado de descarte é definitivo.

Gestores e borracheiros podem movimentar; motoristas e inspetores não podem. Regras comerciais continuam valendo para alterações. A API valida empresa, posição, disponibilidade, versão, odômetros e condição do pneu. Reenvios da mesma operação são idempotentes e o histórico não pode ser editado pela role de execução da API.

A quilometragem rastreada é acumulada ao retirar/mover o pneu, usando o odômetro de cada veículo. Enquanto instalado, o dossiê inclui o percurso desde a montagem atual. O odômetro pode ser atualizado na tela do veículo ou durante uma inspeção. Ele não pode retroceder. O sistema não estima percursos anteriores ao início do rastreamento.

## Alertas automáticos

O motor verifica sulco/dano, avaliação de recapagem ou descarte, intervalo de rodízio, inspeção vencida e estoque mínimo por medida. Preserva os avisos de pressão e temperatura das inspeções.

Os limites de sulco, intervalo de rodízio, dias entre inspeções, estoque mínimo por medida e quantidade de recapagens são configuráveis pela gestão. Os valores iniciais são referências operacionais, não homologação técnica para todos os pneus. Marca, aplicação, carcaça e especificações do fabricante precisam ser avaliadas pela equipe.

- Movimentações, cadastros, inspeções aplicadas, alterações de regras e odômetros recalculam os avisos na mesma transação.
- Uma verificação interna roda na inicialização da API e a cada cinco minutos, com escopo separado por empresa.
- O painel atualiza seus alertas e indicadores a cada minuto enquanto estiver visível.
- Cada condição tem um aviso ativo, sem duplicação a cada verificação. A resolução é automática quando os dados deixam de satisfazer a regra. Mudanças de gravidade reabrem a atenção do usuário.
- Alertas são internos ao sistema. E-mail, WhatsApp, push, telemetria e encomendas automáticas não fazem parte desta entrega.
- A confirmação de uma operação física continua necessária. Nenhum pneu é descartado, enviado à recapadora ou trocado automaticamente por uma sugestão.
- Na hospedagem gratuita da Render, o processo pode dormir. A verificação periódica depende da API em execução; ao reiniciar, ela reconcilia os avisos novamente.

## API

| Método | Rota relativa a `/api/v1` | Uso |
| --- | --- | --- |
| GET | `/tires/{id}/lifecycle?before={cursor}` | Dossiê e movimentos paginados |
| POST | `/tires/{id}/movements` | Movimentação com `client_id`, `expected_version`, ação e motivo |
| PATCH | `/vehicles/{id}/odometer` | Odômetro crescente e recálculo dos avisos |
| GET / PATCH | `/tire-policy` | Consultar/configurar regras |

As ações são `INSTALL`, `REMOVE`, `TRANSFER`, `ROTATE`, `STOCK_TRANSFER`, `RECAP_SEND`, `RECAP_RETURN`, `DISCARD`. Rodízios em posição ocupada exigem o identificador e a versão do outro pneu. O custo é informado em centavos inteiros. Instalações exigem odômetro de destino; retiradas e movimentações de pneus instalados exigem odômetro de origem.

## Migração e publicação

A migração `000008_tire_lifecycle` registra a situação atual de pneus antigos como **BASELINE**, sem inventar movimentações passadas. Inicia o rastreamento de quilometragem pelo odômetro atual. A migração arquiva os alertas antigos de inspeção; o motor gera os novos avisos por condição na inicialização da API.

Aplicar e testar na branch de validação antes de produção. Publicar a API com a migração antes do frontend, porque os novos endpoints são obrigatórios para a tela de estoque. Asaas continua desativado por padrão.

## Validação

`TestTireLifecycleAndAutomation` cobre o ciclo completo, reenvios, troca atômica, posições concorrentes, quilometragem, isolamento, recapagem, descarte, eventos imutáveis, deduplicação e inspeções vencidas. A suíte anterior continua cobrindo cadastro, autenticação e cobrança pausada.

`scripts/tire-lifecycle-e2e.cjs` executa o fluxo real no navegador com uma conta temporária em ambiente de validação. Requer Playwright e `QA_ACCOUNT_FILE` (JSON com e-mail/senha de teste), além do frontend e API locais. Não usar uma conta de produção.
