# Publicação GeevheitTire

## Situação

Neon PostgreSQL e Auth foram configurados e validados com a aplicação local. Veja [NEON.md](NEON.md) para comandos e evidências. A hospedagem pública da API e a entrega real de e-mail continuam pendentes.

## 1. PostgreSQL no Neon

1. Crie um projeto exclusivo do GeevheitTire e um banco PostgreSQL.
2. Guarde a conexão direta em `DATABASE_URL` nos segredos do serviço da API. Use TLS (`sslmode=require`). Não coloque credenciais no Git.
3. Execute `go run ./cmd/migrate` dentro de `api`, com `DATABASE_URL` carregado. Use conexão direta também na API: a implementação atual usa `SET ROLE` por sessão e não é compatível com pooling transacional.
4. As migrations criam `geevheit_runtime`, sem login, superuser ou bypass de RLS, concedem membership ao usuário que migra e permissões mínimas nas tabelas. A API executa `SET ROLE geevheit_runtime` em cada conexão. Se usar usuário de conexão separado, conceda essa membership explicitamente.
5. Execute os testes com `TEST_DATABASE_URL` em um branch Neon de testes, nunca no banco de produção com clientes ativos.
6. Configure backups e faça um exercício de restore antes de receber clientes reais.

## 2. API Go no Fly.io

O `api/fly.toml` fornece health check, porta 8080 e release command para migrations. O nome `geevheittire-api` depende de disponibilidade global; ajuste se necessário.

```sh
cd api
fly auth login
fly launch --no-deploy --copy-config
```

Configure os segredos pelo painel ou `fly secrets set`, sem salvá-los no repositório:

- `DATABASE_URL`: conexão PostgreSQL TLS.
- `JWT_SECRET`: segredo aleatório com pelo menos 32 caracteres.
- `APP_URL`: URL real da web, usada nos links de e-mail.
- `RESEND_API_KEY` e `EMAIL_FROM`: remetente em domínio verificado no Resend.

```sh
fly deploy
fly status
fly checks list
```

A API escuta `PORT` e expõe `/health`, que verifica o banco. Migrations executam antes do novo release. Use projeto e banco exclusivos do GeevheitTire.

Alternativa Cloud Run: publique a imagem do Dockerfile, forneça os mesmos segredos, exponha porta 8080 e execute o binário `/app/migrate` como job antes de atualizar o serviço. Configure a URL pública com HTTPS.

## 3. Web na Vercel

1. Importe o repositório GitHub existente `geevheit-tecnology/geevheithtires`.
2. Defina **Root Directory = web**, framework Next.js e comando `npm run build:vercel` (declarado em `web/vercel.json`).
3. Configure `API_URL=https://<sua-api>` nos ambientes correspondentes, sem `/api/v1` no final. Essa variável é usada no servidor e não tem prefixo `NEXT_PUBLIC_`.
4. Publique primeiro uma preview. Somente configure produção e domínio definitivo após validar a API e o banco.
5. Confirme a integração GitHub, branch, commit SHA e estado Ready na Vercel. Um deploy manual não comprova sincronização automática com o Git.

Pela CLI, dentro de `web`:

```sh
vercel link --yes --project geevheittire --scope geevheits-projects
vercel deploy --yes
```

Não publique `.env` nem `.env.local`. `.vercelignore` e `.gitignore` os excluem. Sem `API_URL`, o proxy devolve 503 com mensagem explícita; a landing e as páginas públicas continuam disponíveis.

## 4. E-mail e mobile

Verifique o remetente no Resend e teste cadastro → verificação, recuperação de senha e convite → definição de senha. As mensagens não são enviadas por um provedor simulado.

No Flutter, use `--dart-define=API_BASE_URL=https://<sua-api>/api/v1`. Antes das lojas, configure ícones definitivos, assinatura release, políticas de privacidade e testes em aparelho real. Não há offline completo nesta etapa.

## 5. Smoke test de publicação

- `/health` retorna 200 com banco conectado.
- Cadastro cria apenas uma empresa e seu owner.
- Cookie web é HttpOnly e Secure; tokens não aparecem no localStorage.
- Cadastrar veículo e pneu, abrir mapa, inspecionar, ver alerta e histórico.
- Reabrir a sessão e confirmar persistência.
- Repetir com outra empresa e confirmar isolamento.
- Convidar borracheiro/inspetor; confirmar perfil e login mobile.
- Verificar logs, backups e restore.

Não marcar produção como concluída até executar o fluxo com API e banco remotos.

A checagem adicional de HTML roda no build local/container (`npm run build`). Na Vercel, o adapter move os arquivos antes de devolver o comando; por isso `build:vercel` executa o build e a validação TypeScript do Next sem tentar ler o layout interno de artefatos do provedor. A mesma interface é validada no navegador contra o container local.
