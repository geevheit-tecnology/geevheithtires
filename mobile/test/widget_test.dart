import 'package:flutter_test/flutter_test.dart';
import 'package:geevheittire/main.dart';
import 'package:flutter/material.dart';

void main() {
  testWidgets('Empty position has a clear legend', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: Legend('Sem pneu', Colors.grey))),
    );
    expect(find.text('Sem pneu'), findsOneWidget);
  });
  test('Axle changes invalidate chassis drawing', () {
    expect(
      ChassisPainter(axles: 3).shouldRepaint(ChassisPainter(axles: 2)),
      isTrue,
    );
    expect(
      ChassisPainter(axles: 3).shouldRepaint(ChassisPainter(axles: 3)),
      isFalse,
    );
  });
}
