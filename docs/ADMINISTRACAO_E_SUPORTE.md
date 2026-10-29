# Administração e suporte

Atualização de 29/10/2026: o novo centro de controle foi validado com API e banco reais isolados. Inclui três planos, preços publicados, financeiro manual, perfil/senha, auditoria e controles de capacidade. O estado de publicação e as condições para escala estão em [PRODUCAO_100_EMPRESAS.md](PRODUCAO_100_EMPRESAS.md).

## Acesso

O endereço de entrada é `/admin/login`. A conta administrativa escolhida pelo responsável é `getuliopedromoreirafilho@gmail.com`. Ela deve ser independente do cadastro cliente da BRASPORT; esse cliente não foi elevado nem encerrado.

A associação administrativa é mantida no banco e conferida pelo backend em cada operação. Ser proprietário de uma empresa não concede administração da plataforma. A interface fornece clientes com busca e paginação, configuração de valores e limites dos planos, concessões de acesso com prazo, suspensão, prorrogação de avaliação, encerramento de clientes e auditoria.

O operador pode criar uma nova identidade interna com `go run ./cmd/platform-admin -create -email EMAIL -reason JUSTIFICATIVA -setup-file CAMINHO_NOVO`, usando a conexão direta do proprietário das migrações e `APP_URL` correto. O comando recusa e-mail já cadastrado. O arquivo privado contém um convite de uso único, válido por 24 horas, para o próprio administrador definir sua senha. O link não é enviado por e-mail automaticamente. A conta real indicada foi criada e recebeu associação administrativa em 27/10/2026, após validar a identidade fictícia e seu primeiro acesso na branch temporária. O primeiro acesso foi concluído por solicitação do titular. O acesso atual é `/admin/login`; a própria conta pode atualizar nome e senha em Minha conta. Senhas não são documentadas no repositório.

Conceder vigência não gera fatura nem registra pagamento. Alterar plano não ativa Asaas. Os recursos continuam limitados pelo catálogo do nível, com controle no servidor. Alterações de configuração têm verificação de versão e justificativa auditada; valores zero são permitidos em rascunho e não podem ser publicados. Publicar atualiza a landing e os planos visíveis ao cliente sem ativar gateway. A opção de sincronizar limites aplica as novas cotas aos contratos vinculados e verifica o uso atual.

## Chamados

No painel do cliente, `Suporte e bugs` permite relatar problemas e acompanhar respostas. O botão `Reportar este erro` preenche um rascunho a partir do erro visível; o cliente revisa e envia. Não é coleta automática de todas as exceções.

Na administração, `Chamados e bugs` permite filtrar, priorizar, alterar o andamento e responder. Notas internas são visíveis somente à administração, inclusive no controle de acesso do banco. Clientes só consultam chamados da própria empresa. Não incluir senhas ou tokens nas descrições.

## Encerramento

`Minha conta` exige senha atual e confirmação explícita. O proprietário confirma o CNPJ e encerra o acesso de toda a empresa; outro membro encerra apenas o próprio acesso. Sessões e convites são revogados. O histórico operacional e a auditoria ficam arquivados: esta opção não apaga definitivamente os dados. Faturas em aberto impedem o encerramento da empresa. Contas administrativas são protegidas contra esse encerramento.

## Validação

Testes de integração com a função de runtime e banco temporário verificaram restrições administrativas, isolamento entre empresas, notas internas, concorrência de alterações, reautenticação e revogação de acesso. Chrome em 1440 e 390 px verificou convite de uso único, login administrativo, clientes, planos, abertura de chamado, resposta administrativa e formulário de encerramento. A única gravação real desta etapa foi a criação da nova identidade administrativa solicitada e sua auditoria. O cadastro da BRASPORT foi preservado.
