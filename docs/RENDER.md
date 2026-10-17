# API gratuita na Render

Arquitetura: web Next.js na Vercel, API Go na Render Free, PostgreSQL e Auth no Neon existente.

O arquivo `render.yaml` declara somente um Web Service com `plan: free`. Ele usa o Dockerfile existente da API, verifica `/health` e executa migrations antes de iniciar o servidor pelo script `/app/start-render.sh`. Se a migration falhar, o servidor não inicia. O runner usa lock de banco para serializar migrations. O deploy automático está desligado nesta primeira configuração; não há banco Render nem disco contratado.

## Criar o serviço

1. Entre na Render com GitHub e autorize acesso apenas ao repositório necessário.
2. Abra **New → Blueprint**, selecione `geevheit-tecnology/geevheithtires`, branch `feat/u0-geevheittire` e arquivo `render.yaml`.
3. Confira o plano **Free** e preencha os campos privados abaixo no painel. Não coloque os valores no Git ou no chat.

| Variável Render | Origem |
| --- | --- |
| `DATABASE_URL` | Valor de `DATABASE_URL_UNPOOLED` do Neon, branch production, com TLS |
| `APP_URL` | URL HTTPS real do site Vercel |
| `NEON_AUTH_BASE_URL` | Variável de mesmo nome obtida com `neon deploy` |
| `NEON_AUTH_JWKS_URL` | Variável de mesmo nome obtida com `neon deploy` |
| `JWT_SECRET` | Gerado automaticamente pela Render; mantenha estável entre deploys |

Use conexão direta: a API aplica `SET ROLE` na sessão pgx. Não use a conexão pooled nesta implementação. Nunca exponha a conexão ao navegador.

4. Em **Settings → Docker Command**, confirme `/app/start-render.sh`. Se o serviço foi criado com o comando antigo contendo `&&`, substitua-o por esse caminho. Um deploy manual sozinho não altera a configuração antiga do serviço.
5. Crie o serviço. Aguarde build, migrations e estado Live. Anote a URL HTTPS atribuída pela Render; o subdomínio depende de disponibilidade.
6. Valide `<URL-API>/health`: HTTP 200, `status: ok`. Depois conecte a web e teste cadastro, login, veículo, pneu, inspeção, logout e restauração da sessão pelo site público.

## Conectar a Vercel

Configure no projeto web, inicialmente em Preview:

- `API_URL`: URL HTTPS da API Render, sem `/api/v1` no final.
- `NEXT_PUBLIC_NEON_AUTH_ENABLED=true`.
- `NEON_AUTH_BASE_URL` e `NEON_AUTH_JWKS_URL`: do mesmo branch Neon usado pela API.
- `NEON_AUTH_COOKIE_SECRET`: segredo aleatório de pelo menos 32 caracteres, estável entre deploys.

Autorize o domínio web no Neon Auth e faça novo deploy para aplicar as variáveis. Não use `localhost` nesses campos remotos. A web não precisa receber `DATABASE_URL`.

## Limites e estado

A Render Free adormece após 15 minutos sem tráfego e tem franquias de uso. O primeiro acesso após a pausa pode demorar; esta configuração serve para demonstração e validação. Veja https://render.com/docs/free. Não foi configurado tráfego artificial para impedir a pausa.

O workflow Neon do PR #1 passou em https://github.com/geevheit-tecnology/geevheithtires/actions/runs/39032721411: criação de branch, migrations e testes de persistência/isolamento. Isso ainda não comprova hospedagem da API: a criação do serviço Render e os testes remotos permanecem pendentes.

Verificação/recuperação de e-mail e convites precisam de validação própria antes da liberação comercial. O envio antigo de convites da API continua dependendo de configuração de e-mail.
