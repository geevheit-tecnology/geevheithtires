# GeevheitTire

Gestão de pneus para transportadoras, em pt-BR. Monorepo Go + PostgreSQL, Next.js e Flutter.

## Entrega atual: U0

- Landing responsiva: mapa ilustrativo interativo, calculadora e planos com recursos futuros identificados.
- Cadastro de empresa com CNPJ numérico ou alfanumérico válido, proprietário e trial de 14 dias.
- Login com Argon2id, JWT de 15 minutos e refresh token rotativo de 30 dias, armazenado como hash.
- Web usa um proxy de mesma origem e cookies `httpOnly`, `Secure` em produção e `SameSite=Lax`. Tokens não são expostos ao JavaScript do navegador.
- Flutter usa Dio, Riverpod, go_router e armazenamento seguro para o refresh token.
- Veículos, pneus e mapas de cavalo 6x2 e carreta de 3 eixos.
- Estoque, movimentações, rodízio, recapagem, descarte e histórico com custos na web/PWA e API; detalhes em [ciclo de vida](docs/TIRE-LIFECYCLE.md). As telas desses novos fluxos ainda não foram portadas para Flutter.
- Inspeções técnicas, histórico, classificação determinística e alertas internos; idempotência por empresa e `client_id`.
- Convites por e-mail para borracheiro/inspetor, verificação e redefinição de senha (exigem Resend configurado).
- PostgreSQL com UUID v7, constraints por empresa, auditoria e RLS. A API assume uma role sem `SUPERUSER` nem `BYPASSRLS`.

Não representa os módulos P1–P5 completos. Veja [status e pendências](docs/STATUS.md).

## Estrutura

```text
api/       API Go, regras de domínio, migrations, testes, Dockerfile e Fly.io
web/       Next.js App Router + Tailwind, landing e painel
mobile/    Flutter Android/iOS
infra/     Docker Compose com PostgreSQL, migrations, API e web
docs/      Requisitos originais, modelo de dados, API, deploy e validação
```

## Subir localmente com um comando

Pré-requisitos: Docker Desktop iniciado, portas 3000, 5432 e 8087 disponíveis.

```sh
make up
```

Acesse http://localhost:3000. API: http://localhost:8087/health. Os dados persistem no volume PostgreSQL; `make down` mantém esse volume. A primeira execução baixa imagens e compila o projeto.

As credenciais e a chave JWT do Compose são **exclusivamente locais**. Em produção, use segredos próprios. Não conecte o banco local à Internet.

1. Clique em **Começar teste de 14 dias**, informe um CNPJ válido e senha com pelo menos 10 caracteres.
2. Cadastre um veículo (placa e modelo, cavalo 6x2 ou carreta 3 eixos).
3. Cadastre um pneu e selecione o veículo e uma posição, ou mantenha em estoque.
4. Abra **Veículos**, selecione o veículo e toque no pneu do mapa.
5. Registre uma inspeção: 5 mm ou mais = OK; 3 a menos de 5 mm = alerta; menos de 3 mm ou dano estrutural = crítico.
6. Confira os indicadores, alertas e histórico. Saia e entre novamente: os registros permanecem no banco.

Sem `RESEND_API_KEY`/`EMAIL_FROM`, o cadastro e o login funcionam localmente, mas a verificação por e-mail não é enviada e convites retornam indisponibilidade explícita. Não existem e-mails ou links de convite simulados no painel.

## Desenvolvimento sem containers de aplicação

```sh
docker compose -f infra/compose.yaml up -d db
cd api
cp .env.example .env
```

Carregue as variáveis do `.env` no terminal e execute `go run ./cmd/migrate`, depois `go run ./cmd/server`. O exemplo usa a porta 8087. Em outro terminal, copie `web/.env.example` para `web/.env.local`, rode `npm ci` e `npm run dev` dentro de `web`.

## Aplicativo

```sh
cd mobile
flutter pub get
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8087/api/v1
```

O endereço acima é para o emulador Android. Em aparelho físico use o IP local do computador, permitindo acesso à porta da API; em produção use HTTPS. O HTTP é habilitado somente no manifest Android de debug. iOS requer configuração de assinatura e ambiente de desenvolvimento apropriado. Crie a conta no site e use o mesmo login no aplicativo.

## Testes e builds

```sh
cd api
go test ./...
TEST_DATABASE_URL='postgres://geevheit:local-development@localhost:5432/geevheittire?sslmode=disable' go test -v ./...
```

O segundo comando exige as migrations aplicadas em um banco de testes. Cria duas empresas, testa RLS, fluxo de cadastro, instalações, inspeções, idempotência e rotação/revogação de tokens, e remove os registros ao terminar.

```sh
cd web
npm ci
npm run build
```

```sh
cd mobile
flutter analyze
flutter test
flutter build apk --debug
```

APK de testes: `mobile/build/app/outputs/flutter-apk/app-debug.apk`. APK debug não é uma publicação na Play Store.

## Deploy

Consulte [docs/DEPLOY.md](docs/DEPLOY.md). Site/painel: Vercel, com diretório raiz `web`. API contínua: Fly.io. Banco: Neon. A prévia web sozinha não oferece cadastro ou persistência sem `API_URL` apontando para a API publicada.

Os arquivos fornecidos pelo usuário foram preservados na raiz e copiados para `docs/`. `sprit-3-prompremane` repete o modelo de dados; não havia HTML de referência da landing. O visual foi criado a partir dos requisitos.
