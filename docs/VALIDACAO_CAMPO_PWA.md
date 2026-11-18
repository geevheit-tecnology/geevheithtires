# Validação de campo do PWA

Revisão: 17/11/2026. Prévia: https://geevheittire-h5ubzz24g-geevheits-projects.vercel.app (deploy `dpl_ExhABPKQr3TPAR7nvQmmdxcdQnMi`, Vercel READY). A prévia não comprova que sua API ou banco sejam isolados: confirme o destino antes de criar ou alterar registros. Não use dados reais para testes até confirmar isso.

## Estado e limites

O build e o TypeScript passaram; a fila de inspeções ainda não foi validada em navegador ou aparelho físico. Ela funciona somente com o painel carregado e sessão previamente identificada, tenta enviar ao recuperar conexão ou pelo botão de envio e guarda dados no armazenamento do navegador. Reabrir o aplicativo sem conexão não foi implementado. Não limpar dados do navegador, desinstalar o PWA ou trocar de aparelho enquanto houver pendências. A classificação só é fornecida após o servidor receber a inspeção. Cadastros e demais operações não têm fila offline.

Esta prévia é para homologação, não é evidência de lançamento comercial. Não foi confirmada a configuração de produção de API, e-mail, pagamentos, backup e restauração, monitoramento ou suporte. A documentação de prontidão anterior pode estar desatualizada.

## Roteiro após confirmar ambiente isolado

1. No PC e no celular, abra a prévia, entre com uma conta de teste e registre navegador, modelo e versão. Verifique se o nome da empresa é o esperado.
2. Percorra painel, veículos, pneus, alertas, menu Mais e formulários; confira leitura, rolagem, toque, orientação e temas claro/escuro. Recarregue com conexão e confira a preferência de tema.
3. Com um pneu de teste e o painel aberto, desligue a rede, registre uma inspeção e confira o aviso de que ficou apenas no aparelho. Religue a rede e aguarde ou selecione Enviar agora. Confira no histórico que a inspeção apareceu uma única vez com os valores corretos.
4. Repita interrompendo a conexão durante o envio e tente novamente. Confira que não surgiu duplicata. Teste saída e entrada na mesma conta antes de sincronizar; não envie usando outra conta.
5. Em Android e iPhone, teste a instalação na tela inicial, abertura e navegação com rede. Registre falhas, capturas e horário, sem incluir senhas ou informações pessoais.
6. Confira separadamente a API, login, permissões, suporte, faturamento, e-mail, monitoramento e restauração de backup em ambiente isolado antes de aprovar produção.

Se houver erro ou inspeção pendente, mantenha os dados do navegador e anote o aparelho, conta de teste, pneu, horário e mensagem. Não repita testes em dados de clientes.

## Verificações complementares de 17/11/2026

- `go test ./...` na API passou; parte dos resultados veio do cache do Go. Isso não valida serviços externos nem o fluxo PWA no navegador.
- A Vercel lista `API_URL` e variáveis de autenticação para produção e prévia. Os nomes não identificam os destinos: isolamento e segurança para gravar dados de teste permanecem sem confirmação.
- Consulta posterior das variáveis sensíveis com a CLI não retornou seus valores. Portanto, a comparação de `API_URL` e `NEON_AUTH_BASE_URL` não foi possível; não se conclui que os ambientes sejam diferentes.
- Correção local: a sincronização relê a fila antes de remover uma inspeção confirmada, para não sobrescrever uma inspeção acrescentada enquanto o envio estava em andamento. A alteração ainda requer teste de concorrência e novo deploy de prévia.

- Chrome local com API simulada: larguras 390 e 1440 px, troca para tema escuro, envio de uma inspeção pendente e ausência de overflow no painel inicial passaram. Não cobre gravação offline, concorrência entre abas nem API real. Build local e deploy de prévia READY passaram.

## API pública — 18/11/2026

- `https://geevheittire-api.vercel.app/health`: HTTP 200, `status=ok`, revisão administrativa 2 e ciclo de vida 3; o health executa ping no banco configurado. Isso não identifica o projeto ou branch do banco.
- `/api/v1/plans` na API e `/api/backend/plans` no site público: HTTP 200. `/api/v1/me` e `/api/backend/me` sem sessão: HTTP 401.
- A configuração `API_URL` de produção do site não foi confirmada. Não houve teste autenticado, alteração de variáveis, migração ou troca de domínio. O agendamento da manutenção de cinco minutos continua pendente na API serverless.

## Conferência complementar de 18/11/2026

Tentativa de comparar a `DATABASE_URL` da API Vercel com a conexão Neon local por `vercel env run -e production`: a variável protegida não foi disponibilizada para a comparação. O ping em `/health` não permite concluir qual branch foi usada. A API serverless não inicia `maintenanceWorker`; a varredura periódica continua pendente de desenho e validação de agendador que respeite limites de duração e plano. Não foi aplicado agendamento nem houve alteração em dados ou configurações remotas.
