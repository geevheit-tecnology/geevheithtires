# Validação do U0 — 17/10/2026

- Go: testes unitários e de integração com PostgreSQL real passaram. Cobrem Argon2id, CNPJ numérico/alfanumérico, classificação, duas empresas, leitura/gravação RLS sem bypass, instalação em posição única, dano estrutural no mapa, inspeção idempotente e contrato da resposta, refresh rotativo/revogado, verificação e recuperação de senha com uso único.
- Next.js: build de produção e TypeScript passaram. O build de container verifica os títulos reais do HTML de `/` e `/app` para detectar troca de páginas.
- Docker Compose: PostgreSQL saudável, migrations concluídas, API e web iniciados. A web usa `/workspace` no container para evitar a colisão de resolução observada com o diretório de trabalho `/app`.
- Chrome headless: fluxo real contra os containers passou em 1440×1000 e 390×844: landing, menu mobile, mapa ilustrativo, calculadora, cadastro, cookies HttpOnly/Secure, cadastro de veículo e pneu, instalação, inspeção, logout com remoção dos cookies e login novamente com os dados persistidos. Sem exceções JavaScript; sem overflow horizontal nas telas verificadas.
- Flutter: análise estática sem problemas; testes passaram; APK Android debug compilado com a API local na porta 8087.

## Capturas

- [Landing desktop](qa/landing-desktop.png)
- [Landing mobile](qa/landing-mobile.png)
- [Painel desktop](qa/painel-desktop.png)
- [Painel mobile](qa/painel-mobile.png)

## Limites da verificação

Não foram testados: entrega real de e-mail (sem Resend configurado), API/Neon remotos (ainda não criados), mobile em aparelho físico, assinatura/release iOS, lojas, offline completo, cobrança ou módulos P2–P5. A publicação web é uma prévia da interface e não comprova esses serviços.

O deploy Vercel é verificado pelo estado terminal retornado pela plataforma; o roteiro de navegador foi executado localmente. O deploy manual não comprova integração automática GitHub → Vercel.

## Prévia Vercel confirmada

Estado terminal `READY` retornado pela Vercel para [GeevheitTire](https://geevheittire-ln8vlylmc-geevheits-projects.vercel.app), deployment `dpl_CBJ9x91nT9zaZuxF4VQJ2Qo56m2s`. O backend remoto e o banco continuam pendentes conforme combinado. Esta publicação foi manual pela CLI.
