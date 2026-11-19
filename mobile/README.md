# GeevheitTire Mobile

Aplicativo Flutter Android/iOS do U0: login compartilhado com o painel, veículos da empresa e mapa dinâmico de pneus.

```sh
flutter pub get
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8087/api/v1
```

Use a conta criada na web. `10.0.2.2` é o endereço do computador no emulador Android. Para aparelho físico use o IP local; em produção use a URL HTTPS da API.

```sh
flutter analyze
flutter test
flutter build apk --debug
```

O refresh token usa `flutter_secure_storage`; o access token fica em memória. Checklist offline, fotos, leitura de QR e push ficam nas próximas entregas do P1. Consulte o [README do monorepo](../README.md) e [status](../docs/STATUS.md).
