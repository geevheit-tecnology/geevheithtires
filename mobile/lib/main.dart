import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

const navy = Color(0xFF293C38);
final apiProvider = Provider((ref) => TireApi());
final sessionProvider = FutureProvider((ref) => ref.read(apiProvider).resume());
final router = GoRouter(
  routes: [
    GoRoute(path: '/', builder: (c, s) => const SessionGate()),
    GoRoute(path: '/vehicles', builder: (c, s) => const VehiclesPage()),
    GoRoute(
      path: '/vehicles/:id',
      builder: (c, s) => MapPage(id: s.pathParameters['id']!),
    ),
  ],
);
void main() => runApp(const ProviderScope(child: GeevheitTireApp()));

class GeevheitTireApp extends StatelessWidget {
  const GeevheitTireApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp.router(
    title: 'GeevheitTire',
    debugShowCheckedModeBanner: false,
    theme: ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.fromSeed(seedColor: navy).copyWith(
        primary: navy,
        secondary: const Color(0xFF6E8F79),
        surface: const Color(0xFFFAFAF7),
      ),
      textTheme: const TextTheme(
        bodyMedium: TextStyle(fontSize: 16, height: 1.5),
        bodySmall: TextStyle(fontSize: 14, height: 1.5),
      ),
      scaffoldBackgroundColor: const Color(0xFFF6F6F2),
      appBarTheme: const AppBarTheme(
        backgroundColor: navy,
        foregroundColor: Colors.white,
      ),
      inputDecorationTheme: const InputDecorationTheme(
        border: OutlineInputBorder(),
        filled: true,
        fillColor: Colors.white,
      ),
    ),
    routerConfig: router,
  );
}

class TireApi {
  final storage = const FlutterSecureStorage();
  final dio = Dio(
    BaseOptions(
      baseUrl: const String.fromEnvironment(
        'API_BASE_URL',
        defaultValue: 'http://10.0.2.2:8087/api/v1',
      ),
      connectTimeout: const Duration(seconds: 15),
      receiveTimeout: const Duration(seconds: 20),
    ),
  );
  String? access;
  Future<void>? refreshing;
  TireApi() {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (o, h) {
          if (access != null) o.headers['Authorization'] = 'Bearer $access';
          h.next(o);
        },
        onError: (e, h) async {
          if (e.response?.statusCode == 401 &&
              e.requestOptions.extra['retry'] != true &&
              !e.requestOptions.path.startsWith('/auth/')) {
            try {
              refreshing ??= refresh().whenComplete(() => refreshing = null);
              await refreshing;
              final o = e.requestOptions;
              o.extra['retry'] = true;
              o.headers['Authorization'] = 'Bearer $access';
              h.resolve(await dio.fetch(o));
              return;
            } catch (_) {
              await logout();
            }
          }
          h.next(e);
        },
      ),
    );
  }
  Future<void> save(Map<String, dynamic> data) async {
    access = data['access_token'] as String;
    await storage.write(
      key: 'refresh_token',
      value: data['refresh_token'] as String,
    );
  }

  Future<void> login(String email, String password) async {
    final r = await dio.post(
      '/auth/login',
      data: {'email': email, 'password': password},
    );
    await save(Map<String, dynamic>.from(r.data));
  }

  Future<void> refresh() async {
    final token = await storage.read(key: 'refresh_token');
    if (token == null) throw Exception('Entre novamente.');
    final r = await dio.post('/auth/refresh', data: {'refresh_token': token});
    await save(Map<String, dynamic>.from(r.data));
  }

  Future<bool> resume() async {
    try {
      if (await storage.read(key: 'refresh_token') == null) return false;
      await refresh();
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<void> logout() async {
    final token = await storage.read(key: 'refresh_token');
    access = null;
    await storage.delete(key: 'refresh_token');
    if (token != null) {
      try {
        await dio.post('/auth/logout', data: {'refresh_token': token});
      } catch (_) {}
    }
  }

  Future<List<dynamic>> vehicles() async {
    final result = <dynamic>[];
    String cursor = '';
    do {
      final r = await dio.get(
        '/vehicles',
        queryParameters: cursor.isEmpty ? null : {'cursor': cursor},
      );
      result.addAll(r.data['data'] as List);
      cursor = r.data['next_cursor'] as String;
    } while (cursor.isNotEmpty);
    return result;
  }

  Future<Map<String, dynamic>> tireMap(String id) async =>
      Map<String, dynamic>.from((await dio.get('/vehicles/$id/tire-map')).data);
}

String message(Object e) {
  if (e is DioException) {
    final data = e.response?.data;
    if (data is Map && data['title'] is String) return data['title'] as String;
    return 'Não foi possível conectar ao servidor. Verifique sua conexão.';
  }
  return 'Não foi possível carregar os dados.';
}

class SessionGate extends ConsumerWidget {
  const SessionGate({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) => ref
      .watch(sessionProvider)
      .when(
        data: (signedIn) => signedIn ? const VehiclesPage() : const LoginPage(),
        loading: () =>
            const Scaffold(body: Center(child: CircularProgressIndicator())),
        error: (e, s) => const LoginPage(),
      );
}

class LoginPage extends ConsumerStatefulWidget {
  const LoginPage({super.key});
  @override
  ConsumerState<LoginPage> createState() => _LoginState();
}

class _LoginState extends ConsumerState<LoginPage> {
  final email = TextEditingController();
  final password = TextEditingController();
  final form = GlobalKey<FormState>();
  bool busy = false;
  String? error;
  @override
  void dispose() {
    email.dispose();
    password.dispose();
    super.dispose();
  }

  Future<void> login() async {
    if (!form.currentState!.validate()) return;
    setState(() {
      busy = true;
      error = null;
    });
    try {
      await ref.read(apiProvider).login(email.text.trim(), password.text);
      if (mounted) context.go('/vehicles');
    } catch (e) {
      if (mounted) setState(() => error = message(e));
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    body: SafeArea(
      child: Center(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(28),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Form(
              key: form,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Center(
                    child: Image.asset(
                      'assets/brand-icon.png',
                      width: 80,
                      height: 80,
                      semanticLabel: 'Símbolo GeevheitTire',
                    ),
                  ),
                  const SizedBox(height: 20),
                  const Text(
                    'GeevheitTire',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 32,
                      fontWeight: FontWeight.bold,
                      color: navy,
                    ),
                  ),
                  const SizedBox(height: 8),
                  const Text(
                    'Sua operação, em cada quilômetro.',
                    textAlign: TextAlign.center,
                  ),
                  const SizedBox(height: 38),
                  TextFormField(
                    controller: email,
                    keyboardType: TextInputType.emailAddress,
                    autofillHints: const [AutofillHints.email],
                    decoration: const InputDecoration(labelText: 'E-mail'),
                    validator: (v) => v != null && v.contains('@')
                        ? null
                        : 'Informe seu e-mail',
                  ),
                  const SizedBox(height: 18),
                  TextFormField(
                    controller: password,
                    obscureText: true,
                    autofillHints: const [AutofillHints.password],
                    decoration: const InputDecoration(labelText: 'Senha'),
                    onFieldSubmitted: (_) => busy ? null : login(),
                    validator: (v) =>
                        v == null || v.isEmpty ? 'Informe sua senha' : null,
                  ),
                  const SizedBox(height: 20),
                  if (error != null)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 16),
                      child: Text(
                        error!,
                        style: const TextStyle(color: Colors.red),
                      ),
                    ),
                  FilledButton(
                    onPressed: busy ? null : login,
                    style: FilledButton.styleFrom(
                      padding: const EdgeInsets.all(18),
                      backgroundColor: navy,
                    ),
                    child: Text(busy ? 'Entrando…' : 'Entrar'),
                  ),
                  const SizedBox(height: 25),
                  const Text(
                    'Use a conta criada no painel web ou o acesso recebido por convite.',
                    textAlign: TextAlign.center,
                    style: TextStyle(fontSize: 12, color: Colors.blueGrey),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

class VehiclesPage extends ConsumerStatefulWidget {
  const VehiclesPage({super.key});
  @override
  ConsumerState<VehiclesPage> createState() => _VehiclesState();
}

class _VehiclesState extends ConsumerState<VehiclesPage> {
  late Future<List<dynamic>> data;
  @override
  void initState() {
    super.initState();
    data = ref.read(apiProvider).vehicles();
  }

  void reload() {
    setState(() => data = ref.read(apiProvider).vehicles());
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: const Text('Minha frota'),
      actions: [
        IconButton(
          tooltip: 'Atualizar',
          onPressed: reload,
          icon: const Icon(Icons.refresh),
        ),
        IconButton(
          tooltip: 'Sair',
          onPressed: () async {
            await ref.read(apiProvider).logout();
            ref.invalidate(sessionProvider);
            if (context.mounted) context.go('/');
          },
          icon: const Icon(Icons.logout),
        ),
      ],
    ),
    body: FutureBuilder<List<dynamic>>(
      future: data,
      builder: (c, s) {
        if (s.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (s.hasError) {
          return ErrorView(text: message(s.error!), retry: reload);
        }
        final list = s.data!;
        if (list.isEmpty) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(30),
              child: Text(
                'Nenhum veículo cadastrado.\nCadastre sua frota no painel web.',
                textAlign: TextAlign.center,
              ),
            ),
          );
        }
        return RefreshIndicator(
          onRefresh: () async {
            reload();
            await data;
          },
          child: ListView.separated(
            padding: const EdgeInsets.all(18),
            itemCount: list.length,
            separatorBuilder: (c, i) => const SizedBox(height: 10),
            itemBuilder: (c, i) {
              final v = list[i];
              return Card(
                child: ListTile(
                  contentPadding: const EdgeInsets.all(16),
                  leading: const Icon(
                    Icons.local_shipping_outlined,
                    color: navy,
                  ),
                  title: Text(
                    v['plate'],
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
                  subtitle: Text("${v['model']}\n${v['current_km']} km"),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => context.push('/vehicles/${v['id']}'),
                ),
              );
            },
          ),
        );
      },
    ),
  );
}

class MapPage extends ConsumerStatefulWidget {
  final String id;
  const MapPage({super.key, required this.id});
  @override
  ConsumerState<MapPage> createState() => _MapState();
}

class _MapState extends ConsumerState<MapPage> {
  late Future<Map<String, dynamic>> data;
  @override
  void initState() {
    super.initState();
    data = ref.read(apiProvider).tireMap(widget.id);
  }

  void reload() {
    setState(() => data = ref.read(apiProvider).tireMap(widget.id));
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: const Text('Mapa de pneus'),
      actions: [
        IconButton(
          tooltip: 'Atualizar',
          onPressed: reload,
          icon: const Icon(Icons.refresh),
        ),
      ],
    ),
    body: FutureBuilder<Map<String, dynamic>>(
      future: data,
      builder: (c, s) {
        if (s.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator());
        }
        if (s.hasError) {
          return ErrorView(text: message(s.error!), retry: reload);
        }
        final m = s.data!;
        final positions = List<String>.from(m['positions']);
        final tires = List<Map<String, dynamic>>.from(
          (m['tires'] as List).map((t) => Map<String, dynamic>.from(t)),
        );
        return SingleChildScrollView(
          padding: const EdgeInsets.all(22),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                m['vehicle']['plate'],
                style: const TextStyle(
                  fontSize: 30,
                  fontWeight: FontWeight.bold,
                  color: navy,
                ),
              ),
              Text(
                "${m['vehicle']['model']} · ${m['vehicle']['current_km']} km",
              ),
              const SizedBox(height: 25),
              const Text(
                '↑ FRENTE',
                textAlign: TextAlign.center,
                style: TextStyle(
                  letterSpacing: 3,
                  color: Colors.blueGrey,
                  fontSize: 11,
                ),
              ),
              const SizedBox(height: 22),
              LayoutBuilder(
                builder: (c, b) => SizedBox(
                  height:
                      positions
                          .where((p) => !p.startsWith('STEP'))
                          .map((p) => p[0])
                          .toSet()
                          .length *
                      114.0,
                  child: Stack(
                    children: [
                      Positioned.fill(
                        child: CustomPaint(
                          painter: ChassisPainter(
                            axles: positions
                                .where((p) => !p.startsWith('STEP'))
                                .map((p) => p[0])
                                .toSet()
                                .length,
                          ),
                        ),
                      ),
                      Column(
                        children: positions
                            .where((p) => !p.startsWith('STEP'))
                            .map((p) => p[0])
                            .toSet()
                            .map(
                              (axle) => Padding(
                                padding: const EdgeInsets.only(bottom: 18),
                                child: Row(
                                  mainAxisAlignment:
                                      MainAxisAlignment.spaceBetween,
                                  children: [
                                    for (final side in ['E', 'D'])
                                      Row(
                                        children: positions
                                            .where(
                                              (p) =>
                                                  p[0] == axle && p[1] == side,
                                            )
                                            .map(
                                              (p) =>
                                                  wheel(p, tires, b.maxWidth),
                                            )
                                            .toList(),
                                      ),
                                  ],
                                ),
                              ),
                            )
                            .toList(),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 15),
              Wrap(
                alignment: WrapAlignment.center,
                children: positions
                    .where((p) => p.startsWith('STEP'))
                    .map((p) => wheel(p, tires, 350))
                    .toList(),
              ),
              const SizedBox(height: 25),
              const Wrap(
                spacing: 15,
                alignment: WrapAlignment.center,
                children: [
                  Legend('OK', Colors.green),
                  Legend('Alerta', Colors.orange),
                  Legend('Crítico', Colors.red),
                  Legend('Sem pneu', Colors.grey),
                ],
              ),
              const SizedBox(height: 25),
              const Text(
                'Toque em um pneu para consultar seus dados.',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 12, color: Colors.blueGrey),
              ),
            ],
          ),
        );
      },
    ),
  );
  Widget wheel(
    String position,
    List<Map<String, dynamic>> tires,
    double width,
  ) {
    final matches = tires.where((t) => t['position'] == position);
    final t = matches.isEmpty ? null : matches.first;
    final status = t?['status'];
    final color = status == 'critico'
        ? Colors.red
        : status == 'alerta'
        ? Colors.orange
        : status == 'ok'
        ? Colors.green
        : Colors.grey;
    return Padding(
      padding: const EdgeInsets.all(3),
      child: InkWell(
        onTap: t == null
            ? null
            : () => showModalBottomSheet(
                context: context,
                showDragHandle: true,
                builder: (c) => Padding(
                  padding: const EdgeInsets.all(28),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        t['code'],
                        style: const TextStyle(
                          fontSize: 28,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 12),
                      Text("${t['brand']} ${t['model']} · ${t['size']}"),
                      Text('Posição: $position'),
                      Text(
                        "Sulco: ${t['tread_mm']} mm",
                        style: TextStyle(
                          color: color,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      Text("Estado: ${t['state']}"),
                      const SizedBox(height: 25),
                    ],
                  ),
                ),
              ),
        borderRadius: BorderRadius.circular(8),
        child: Container(
          width: ((width - 38) / 4).clamp(48.0, 76.0),
          height: 88,
          decoration: BoxDecoration(
            color: color.withValues(alpha: .08),
            border: Border.all(color: color.withValues(alpha: .5)),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(position, style: TextStyle(fontSize: 10, color: color)),
              const SizedBox(height: 5),
              Text(
                t?['code'] ?? '—',
                style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.bold,
                  color: color,
                ),
              ),
              Text(
                t == null ? 'Vazio' : "${t['tread_mm']} mm",
                style: TextStyle(fontSize: 10, color: color),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class ChassisPainter extends CustomPainter {
  final int axles;
  ChassisPainter({required this.axles});
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()..color = const Color(0xFFDCE2D9);
    canvas.drawRRect(
      RRect.fromRectAndRadius(
        Rect.fromLTWH(size.width / 2 - 18, 10, 36, size.height - 34),
        const Radius.circular(12),
      ),
      paint,
    );
    paint.strokeWidth = 5;
    for (var i = 0; i < axles; i++) {
      canvas.drawLine(
        Offset(8, i * 114 + 44),
        Offset(size.width - 8, i * 114 + 44),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(ChassisPainter old) => old.axles != axles;
}

class Legend extends StatelessWidget {
  final String label;
  final Color color;
  const Legend(this.label, this.color, {super.key});
  @override
  Widget build(BuildContext c) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      Icon(Icons.circle, size: 8, color: color),
      const SizedBox(width: 5),
      Text(label, style: const TextStyle(fontSize: 11)),
    ],
  );
}

class ErrorView extends StatelessWidget {
  final String text;
  final VoidCallback retry;
  const ErrorView({super.key, required this.text, required this.retry});
  @override
  Widget build(BuildContext c) => Center(
    child: Padding(
      padding: const EdgeInsets.all(28),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(text, textAlign: TextAlign.center),
          const SizedBox(height: 18),
          FilledButton(onPressed: retry, child: const Text('Tentar novamente')),
        ],
      ),
    ),
  );
}
