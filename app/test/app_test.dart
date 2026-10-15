import 'package:awlauncher/app.dart';
import 'package:awlauncher/core/fake_core.dart';
import 'package:awlauncher/i18n/strings.dart';
import 'package:awlauncher/platform/desktop.dart';
import 'package:awlauncher/state/controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _Desktop implements Desktop {
  final opened = <String>[];
  var shown = 0, hidden = 0, quits = 0;
  String? trayPlay;

  @override
  void showWindow() => shown++;

  @override
  void hideWindow() => hidden++;

  @override
  void open(String target) => opened.add(target);

  @override
  void setTrayPlay(String text, bool enabled) => trayPlay = text;

  @override
  void quit() => quits++;
}

Future<(FakeCore, _Desktop)> _start(WidgetTester tester, {String systemLang = 'en'}) async {
  tester.view.physicalSize = const Size(1280, 800);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  addTearDown(() => currentLang = 'en');
  final core = FakeCore(tick: const Duration(milliseconds: 10), systemLang: systemLang);
  final desktop = _Desktop();
  await tester.pumpWidget(AwApp(controller: Controller(core, desktop)));
  await tester.pumpAndSettle();
  return (core, desktop);
}

void main() {
  testWidgets('shows the accounts and the status', (tester) async {
    await _start(tester);
    expect(find.text('Tanker'), findsWidgets);
    expect(find.text('EU main'), findsOneWidget);
    expect(find.byKey(const Key('status')), findsOneWidget);
    expect(find.text('Ready'), findsOneWidget);
  });

  testWidgets('play asks, downloads and starts the game', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.byKey(const Key('play')));
    await tester.pumpAndSettle();
    expect(find.text('Install now?'), findsOneWidget);
    await tester.tap(find.text('Yes'));
    await tester.pump(const Duration(milliseconds: 50));
    expect(find.text('Downloading'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();
    expect(find.text('Close game'), findsOneWidget);
    expect(core.sent.where((c) => c['cmd'] == 'answer').single['value'], 'yes');
  });

  testWidgets('the pages switch and the language follows the setting', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.text('Settings').first);
    await tester.pumpAndSettle();
    expect(find.text('Appearance'), findsOneWidget);
    await tester.tap(find.text('Русский'));
    await tester.pumpAndSettle();
    expect(find.text('Оформление'), findsOneWidget);
    expect(find.text('Играть'), findsOneWidget);
    expect(core.prefs['lang'], 'ru');
  });

  testWidgets('a Russian Windows gets the Russian window', (tester) async {
    await _start(tester, systemLang: 'ru');
    expect(find.text('Аккаунты · 3'), findsOneWidget);
  });

  testWidgets('a palette changes the theme and is saved', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.text('Settings').first);
    await tester.pumpAndSettle();
    final before = Theme.of(tester.element(find.text('Appearance'))).colorScheme.primary;
    await tester.tap(find.byTooltip('Coral · Vibrant'));
    await tester.pumpAndSettle();
    final after = Theme.of(tester.element(find.text('Appearance'))).colorScheme.primary;
    expect(after, isNot(before));
    expect(core.prefs['style'], 'vibrant');
    expect(core.prefs['hue'], 25);
  });

  testWidgets('Exit stops the core and closes the window', (tester) async {
    final (core, desktop) = await _start(tester);
    await tester.tap(find.text('Settings').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Exit'));
    await tester.pumpAndSettle();
    expect(desktop.quits, 1);
    expect(core.sent.last['cmd'], 'exit');
  });

  testWidgets('an FX ID sign-in asks for the e-mail and the code', (tester) async {
    await _start(tester);
    await tester.tap(find.text('Add account').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('FX ID'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'player@example.com');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('Code from the e-mail'), findsOneWidget);
    expect(find.text('A code was sent to player@example.com.'), findsOneWidget);
    await tester.enterText(find.byType(TextField), '123456');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('Account added'), findsOneWidget);
  });
}
