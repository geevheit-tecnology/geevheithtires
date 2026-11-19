# Consulta gratuita de CNPJ e CEP

BrasilAPI fornece os dados públicos de CNPJ; ViaCEP fornece o endereço postal. Não exigem chave nesta integração. Fontes: https://github.com/BrasilAPI/BrasilAPI e https://viacep.com.br/.

## Comportamento

- **Consultar CNPJ** traz razão social, nome fantasia, situação e endereço cadastral. O nome da empresa continua editável; razão social, situação, origem e horário de consulta são gravados separadamente no servidor.
- O cadastro exige CNPJ com dígitos verificadores válidos, encontrado na base e com situação **ATIVA**. A API Go repete a verificação; chamar a API diretamente não permite ignorá-la.
- A consulta é feita antes de criar novas identidades no formulário. Contas Neon que já existem podem completar a empresa; contas já vinculadas continuam entrando sem uma consulta externa a cada login.
- Ao sair do campo CEP, rua, bairro, cidade e UF são preenchidos. Número e complemento informados pelo usuário são preservados. Também existe o botão **Buscar CEP** e edição manual se o serviço postal estiver indisponível.
- Endereço e confirmação cadastral ficam na empresa e respeitam o isolamento existente. `GET /api/v1/company` retorna somente a empresa do usuário autenticado.
- As consultas têm timeout de 8 segundos, limites de corpo de resposta, cache de até 256 entradas por processo por 15 minutos e o rate limit público da API. Somente os dados necessários são retornados; o quadro de sócios não é exposto.

## Limites

Consulta pública não comprova que o usuário representa a empresa e não constitui uma garantia contra fraude. Os provedores gratuitos podem ficar indisponíveis e suas bases podem ter defasagem. Uma falha do serviço de CNPJ solicita nova tentativa; não é apresentada como CNPJ falso nem libera um cadastro sem consulta. O ViaCEP documenta restrições a uso massivo.

A validação local suporta o formato alfanumérico, mas o endpoint atual da BrasilAPI utilizado nesta entrega aceita o formato numérico. Se a base não suportar o CNPJ informado, a aplicação informa a limitação e não inventa uma confirmação cadastral.

Empresas anteriores não são marcadas automaticamente como verificadas. A exigência vale para criação de novas empresas. O endereço é opcional para clientes antigos da API e, quando omitido, recebe os dados cadastrais disponíveis do provedor. No novo formulário web, os campos de endereço são solicitados.

## Deploy em duas etapas

1. Render: **Manual Deploy → Deploy latest commit** no serviço `geevheittire-api`. O script de início aplica a migration `000006_company_registration` antes de iniciar a API. Não é necessário cadastrar tokens dos provedores ou criar serviços pagos.
2. Confirmar `/api/v1/registration/cep/01001000` e `/api/v1/registration/cnpj/00000000000191`. Depois publicar a nova web em produção na Vercel. A versão anterior da API não aceita o novo endereço; por isso a web de produção deve ser atualizada depois da API.

O deploy automático da Render está desligado. A publicação da API requer acesso ao painel; os arquivos e a migration foram preparados e testados no branch Neon isolado.

## Validação

Build web, testes Go e persistência/isolamento no branch Neon de validação passaram. As consultas públicas de exemplo aos dois serviços responderam corretamente. Nos testes de registro, provedores simulados fornecem empresas QA; nenhuma empresa real é registrada como se fosse do testador.

O teste Go cobre CNPJ inválido/inativo/inexistente, indisponibilidade, rate limit do provedor, resposta inválida, cache e CEP não encontrado. A integração confere endereço persistido, origem da verificação e isolamento entre empresas.

Para reproduzir o navegador com dados QA, inicie `node scripts/company-directory-fixture.cjs` e configure **somente a API local de testes** com `CNPJ_API_BASE_URL=http://127.0.0.1:18098/cnpj` e `CEP_API_BASE_URL=http://127.0.0.1:18098/cep`. Use banco e Auth de um branch Neon isolado. Inicie a API na porta 8099 e a web na 3029 com `API_URL=http://localhost:8099` e execute `scripts/neon-onboarding-e2e.cjs` com Playwright/Chrome instalados. O teste verifica consulta de CNPJ, preenchimento de cidade/UF, preservação de número/complemento, persistência e regressões de autenticação.

Em produção, deixe `CNPJ_API_BASE_URL` e `CEP_API_BASE_URL` ausentes: os padrões são os provedores reais HTTPS. Nunca use a fixture local ou algum modo de simulação em produção.
