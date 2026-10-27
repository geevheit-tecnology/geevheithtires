# Administração e suporte

Implementação local; não publicada nesta etapa. As migrações 000012–000014 foram aplicadas somente na branch temporária de validação.

## Acesso

O endereço de entrada é `/admin/login`. A conta administrativa escolhida pelo responsável é `getuliopedromoreirafilho@gmail.com`. Ela deve ser independente do cadastro cliente da BRASPORT; esse cliente não foi elevado nem encerrado.

A associação administrativa é mantida no banco e conferida pelo backend em cada operação. Ser proprietário de uma empresa não concede administração da plataforma. A interface fornece clientes com busca e paginação, configuração de valores e limites dos planos, concessões de acesso com prazo, suspensão, prorrogação de avaliação, encerramento de clientes e auditoria.

O operador pode criar uma nova identidade interna com `go run ./cmd/platform-admin -create -email EMAIL -reason JUSTIFICATIVA -setup-file CAMINHO_NOVO`, usando a conexão direta do proprietário das migrações e `APP_URL` correto. O comando recusa e-mail já cadastrado. O arquivo privado contém um convite de uso único, válido por 24 horas, para o próprio administrador definir sua senha. O link não é enviado por e-mail automaticamente. A conta real indicada foi criada e recebeu associação administrativa em 27/10/2026, após validar a identidade fictícia e seu primeiro acesso na branch temporária. O convite real está em arquivo privado temporário (permissão 0600), não versionado, e aguarda a definição de senha pelo responsável. As novas telas e migrações continuam locais. Antes da publicação, o acesso já existente usa `/login` e depois `/admin`.

Conceder vigência não gera fatura nem registra pagamento. Alterar plano não ativa Asaas. Os recursos continuam limitados pelo catálogo do nível, com controle no servidor. Alterações de configuração têm verificação de versão e justificativa auditada; valores zero são permitidos em rascunho e não podem ser publicados para cobrança.

## Chamados

No painel do cliente, `Suporte e bugs` permite relatar problemas e acompanhar respostas. O botão `Reportar este erro` preenche um rascunho a partir do erro visível; o cliente revisa e envia. Não é coleta automática de todas as exceções.

Na administração, `Chamados e bugs` permite filtrar, priorizar, alterar o andamento e responder. Notas internas são visíveis somente à administração, inclusive no controle de acesso do banco. Clientes só consultam chamados da própria empresa. Não incluir senhas ou tokens nas descrições.

## Encerramento

`Minha conta` exige senha atual e confirmação explícita. O proprietário confirma o CNPJ e encerra o acesso de toda a empresa; outro membro encerra apenas o próprio acesso. Sessões e convites são revogados. O histórico operacional e a auditoria ficam arquivados: esta opção não apaga definitivamente os dados. Faturas em aberto impedem o encerramento da empresa. Contas administrativas são protegidas contra esse encerramento.

## Validação

Testes de integração com a função de runtime e banco temporário verificaram restrições administrativas, isolamento entre empresas, notas internas, concorrência de alterações, reautenticação e revogação de acesso. Chrome em 1440 e 390 px verificou convite de uso único, login administrativo, clientes, planos, abertura de chamado, resposta administrativa e formulário de encerramento. A única gravação real desta etapa foi a criação da nova identidade administrativa solicitada e sua auditoria. O cadastro da BRASPORT foi preservado.
