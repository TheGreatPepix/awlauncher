import 'package:awlauncher/app.dart';
import 'package:awlauncher/core/fake_core.dart';
import 'package:awlauncher/i18n/strings.dart';
import 'package:awlauncher/platform/desktop.dart';
import 'package:awlauncher/state/controller.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

class _Desktop implements Desktop {
  final opened = <String>[];
  var shown = 0, hidden = 0, quits = 0;
  String? trayPlay, trayOpen;
  bool? darkTitle;
  late DesktopEvents events;

  @override
  void listen(DesktopEvents events) => this.events = events;

  @override
  void setTrayLabels({required String open, required String exit}) => trayOpen = open;

  @override
  void setTitleBar({required bool dark, required Color caption, required Color text}) => darkTitle = dark;

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
  tester.platformDispatcher.accessibilityFeaturesTestValue = const FakeAccessibilityFeatures(disableAnimations: true);
  addTearDown(tester.view.reset);
  addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
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
    expect(tester.widget<Text>(find.byKey(const Key('status'))).data, 'Ready');
  });

  testWidgets('play asks, downloads and starts the game', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.byKey(const Key('play')));
    await tester.pumpAndSettle();
    expect(find.text('Install now?'), findsOneWidget);
    await tester.tap(find.text('Yes'));
    await tester.pump(const Duration(milliseconds: 50));
    expect(find.textContaining('Downloading', findRichText: true), findsOneWidget);
    expect(find.text('Pause'), findsOneWidget);
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
    expect(find.text('Аккаунты'), findsOneWidget);
    expect(tester.widget<Text>(find.byKey(const Key('account-count'))).data, '3');
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

  testWidgets('the tray plays, reports the window and exits', (tester) async {
    final (core, desktop) = await _start(tester);
    expect(desktop.trayOpen, 'Open AWLauncher');
    expect(desktop.darkTitle, isFalse);
    desktop.events.visible(false);
    expect(core.sent.last, {'cmd': 'visible', 'ok': false});
    desktop.events.play();
    await tester.pumpAndSettle();
    expect(core.sent.any((c) => c['cmd'] == 'playLast'), isTrue);
    expect(find.text('Install now?'), findsOneWidget);
    await tester.tap(find.text('No'));
    await tester.pumpAndSettle();
    desktop.events.exit();
    await tester.pumpAndSettle();
    expect(core.sent.last['cmd'], 'requestExit');
    expect(desktop.quits, 1);
  });

  testWidgets('ten clicks on the status open a ping-pong that plays', (tester) async {
    await _start(tester);
    for (var i = 0; i < 10; i++) {
      await tester.tap(find.byKey(const Key('status')));
      await tester.pump(const Duration(milliseconds: 50));
    }
    await tester.pump(const Duration(milliseconds: 600));
    expect(find.text('Ping-pong'), findsOneWidget);
    final field = find.byWidgetPredicate((w) => w is CustomPaint && '${w.painter.runtimeType}' == '_PongPainter');
    dynamic painter() => tester.widget<CustomPaint>(field).painter;
    // The ball waits 0.8 s after the serve, then moves.
    await tester.pump(const Duration(milliseconds: 16));
    final start = painter().x as double;
    for (var i = 0; i < 60; i++) {
      await tester.pump(const Duration(milliseconds: 16));
    }
    expect(painter().x, isNot(start));
    // The paddle follows the mouse, even outside the field.
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    addTearDown(mouse.removePointer);
    await mouse.addPointer(location: Offset(10, tester.getRect(field).top));
    await tester.pump(const Duration(milliseconds: 16));
    await mouse.moveTo(Offset(10, tester.getRect(field).bottom));
    await tester.pump(const Duration(milliseconds: 16));
    expect(painter().player, closeTo(300 - 64, 0.5));
    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    expect(find.text('Ping-pong'), findsNothing);
  });

  testWidgets('an FX ID sign-in asks for the e-mail and the code', (tester) async {
    await _start(tester);
    await tester.tap(find.byTooltip('Add account'));
    await tester.pumpAndSettle();
    await tester.tap(find.descendant(of: find.byType(Dialog), matching: find.text('FX ID')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'player@example.com');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('Code from the e-mail'), findsOneWidget);
    expect(find.text('A code was sent to player@example.com.'), findsOneWidget);
    // Pasting the code fills the six boxes and sends it.
    await tester.enterText(find.byType(TextField).first, '12 34 56');
    await tester.pumpAndSettle();
    expect(find.text('Code from the e-mail'), findsNothing);
    expect(find.text('Account added'), findsOneWidget);
  });

  testWidgets('the Game page lists the clients of the accounts', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.text('Game').first);
    await tester.pumpAndSettle();
    expect(find.text('VK Play'), findsOneWidget);
    expect(find.text('FX ID main branch'), findsOneWidget);
    expect(find.text('FX ID supertest'), findsOneWidget);
    expect(find.textContaining('Not installed'), findsOneWidget);
    await tester.tap(find.text('Check files').first);
    expect(core.sent.last, {'cmd': 'verify', 'value': 'vkplay', 'branch': ''});
    await tester.tap(find.text('Download'));
    expect(core.sent.last, {'cmd': 'downloadClient', 'account': '3', 'value': 'branch', 'branch': 'supertest'});
  });

  testWidgets('the folder question checks the folder as it is typed', (tester) async {
    await _start(tester);
    await tester.tap(find.text('Game').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Folder…'));
    await tester.pumpAndSettle();
    expect(find.text('Game folder'), findsOneWidget);
    expect(
      find.text('Armored Warfare is installed here. The launcher checks it and installs updates.'),
      findsOneWidget,
    );
    await tester.enterText(find.byType(TextField), 'nowhere');
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pumpAndSettle();
    expect(find.textContaining('Enter a full path'), findsOneWidget);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(find.text('Game folder'), findsNothing);
  });

  testWidgets('a branch is chosen from the account menu', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.byTooltip('More').at(1));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Client branches'));
    await tester.pumpAndSettle();
    expect(find.text('Which client branch should EU main play?'), findsOneWidget);
    await tester.tap(find.text('supertest'));
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(core.accounts[1]['branch'], 'supertest');
  });

  testWidgets('a new palette is previewed and saved', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.text('Settings').first);
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Create a palette'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Fruit salad'));
    await tester.pumpAndSettle();
    expect(find.text('Custom · Fruit salad'), findsOneWidget);
    await tester.tap(find.text('Save'));
    await tester.pumpAndSettle();
    expect(core.prefs['style'], 'fruit');
    expect(core.prefs['palettes'], [
      {'hue': 300, 'style': 'fruit'},
    ]);
  });

  testWidgets('Escape answers no to a question', (tester) async {
    final (core, _) = await _start(tester);
    await tester.tap(find.byKey(const Key('play')));
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.text('Install now?'), findsNothing);
    expect(core.sent.where((c) => c['cmd'] == 'answer').single['value'], 'no');
  });
}
