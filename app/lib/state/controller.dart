import 'dart:async';

import 'package:flutter/foundation.dart';

import '../core/core.dart';
import '../i18n/strings.dart';
import '../platform/desktop.dart';
import 'models.dart';

enum AppPage { home, game, activity, settings }

const _logLimit = 200000;

final _release = RegExp(r'^v\d+\.\d+\.\d+$');

/// The window's state, kept in step with the core's events.
class Controller extends ChangeNotifier {
  Controller(this.core, this.desktop) {
    _sub = core.events.listen(_receive);
  }

  final Core core;
  final Desktop desktop;
  late final StreamSubscription<Event> _sub;

  final _notices = StreamController<Notice>.broadcast();
  final _exitAsks = StreamController<void>.broadcast();

  /// Short messages for snackbars.
  Stream<Notice> get notices => _notices.stream;

  /// Fires when the core asks to confirm leaving with operations running.
  Stream<void> get exitAsks => _exitAsks.stream;

  var page = AppPage.home;
  var ready = false;
  String? fatal;

  var accounts = <Account>[];
  var selected = '';
  var _lastId = '';
  var ops = <Operation>[];
  var running = false;
  var progress = const Progress.idle();

  var game = '', fxGame = '', suggestedGame = '', dataDir = '';
  var allowMods = false, patchBackups = true, hideOnLaunch = true;
  var version = '', autostart = 'off', systemLang = 'en';
  var prefs = const Prefs();

  final _log = StringBuffer();
  String get log => _log.toString();
  var logUnseen = false;

  final prompts = <PromptRequest>[];
  var signInActive = false;
  Event? gameInfo;
  Event? availableClients;
  UpdateInfo? update;
  var updateUnseen = false;

  bool get busy => ops.isNotEmpty;
  Operation? get gameOp => ops.where((o) => o.game).firstOrNull;
  Operation? accountOp(String id) => ops.where((o) => o.account == id).firstOrNull;
  Account? get selectedAccount => accounts.where((a) => a.id == selected).firstOrNull;
  bool canPlay(Account? a) => a != null && gameOp == null && accountOp(a.id) == null;

  String get lang => resolveLang(prefs.lang, systemLang);

  /// The line under the navigation: what the launcher is doing now.
  String get status {
    if (busy) {
      final op = gameOp;
      final loading =
          op != null && (op.title == 'Downloading game' || (op.title.startsWith('Starting ') && progress.active));
      final what = loading ? t('Loading game') : ops.map((o) => tb(o.title)).join(' · ');
      return (progress.paused ? t('Paused · ') : '') + what;
    }
    return running ? t('The game is running') : t('Ready');
  }

  void start() => send({'cmd': 'ready'});

  void send(Map<String, Object?> command) => core.send(command);

  void showPage(AppPage p) {
    page = p;
    if (p == AppPage.game) {
      send({'cmd': 'gameInfo'});
      send({'cmd': 'availableClients'});
    }
    if (p == AppPage.activity) logUnseen = false;
    if (p == AppPage.settings) updateUnseen = false;
    notifyListeners();
  }

  void select(String id) {
    selected = id;
    notifyListeners();
  }

  void setPrefs(Prefs p) {
    prefs = p;
    currentLang = lang;
    send(p.toCommand());
    notifyListeners();
  }

  void answer(PromptRequest p, String value, {bool ok = true}) {
    prompts.removeWhere((x) => x.id == p.id);
    send({'cmd': 'answer', 'prompt': p.id, 'value': value, 'ok': ok});
    notifyListeners();
  }

  void play([Account? a]) {
    a ??= selectedAccount;
    if (canPlay(a)) send({'cmd': 'play', 'account': a!.id});
  }

  void clearLog() {
    _log.clear();
    notifyListeners();
  }

  void _appendLog(String text) {
    _log.write(text);
    if (_log.length > _logLimit) {
      final s = _log.toString();
      var cut = s.length - _logLimit * 3 ~/ 4;
      final nl = s.indexOf('\n', cut);
      if (nl >= 0) cut = nl + 1;
      _log
        ..clear()
        ..write(s.substring(cut));
    }
    if (page != AppPage.activity) logUnseen = true;
  }

  void _receive(Event ev) {
    switch (ev['type']) {
      case 'state':
        _onState(ev);
      case 'log':
        _appendLog('${ev['text'] ?? ''}');
      case 'progress':
        progress = Progress.fromJson(ev);
      case 'prompt':
        final p = PromptRequest.fromJson(ev);
        prompts
          ..removeWhere((x) => x.id == p.id)
          ..add(p);
      case 'done':
        _onDone(ev);
      case 'notice':
        final kind = ev['kind'] == 'ok' ? NoticeKind.ok : NoticeKind.info;
        _notices.add(Notice(tb('${ev['message'] ?? ''}'), kind: kind));
      case 'game':
        running = ev['running'] == true;
      case 'signin':
        signInActive = ev['active'] == true;
      case 'gameInfo':
        gameInfo = ev;
      case 'availableClients':
        availableClients = ev;
      case 'update':
        _onUpdate(UpdateInfo.fromJson(ev));
      case 'confirmExit':
        _exitAsks.add(null);
      case 'window':
        ev['show'] == true ? desktop.showWindow() : desktop.hideWindow();
      case 'tray':
        desktop.setTrayPlay('${ev['play'] ?? ''}', ev['enabled'] == true);
      case 'open':
        desktop.open('${ev['target'] ?? ''}');
      case 'exit':
        desktop.quit();
      case 'fatal':
        fatal = '${ev['message'] ?? ''}';
      case 'coreExited':
        fatal ??= t('The launcher core stopped (code {code}).', {'code': ev['code']});
      default:
        return;
    }
    notifyListeners();
  }

  void _onState(Event ev) {
    accounts = [for (final a in (ev['accounts'] as List? ?? const [])) Account.fromJson(a as Event)];
    final last = accounts.where((a) => a.last).firstOrNull?.id ?? '';
    if (last.isNotEmpty && last != _lastId) selected = last;
    _lastId = last;
    if (!accounts.any((a) => a.id == selected)) {
      selected = last.isNotEmpty ? last : (accounts.firstOrNull?.id ?? '');
    }
    ops = [for (final o in (ev['ops'] as List? ?? const [])) Operation.fromJson(o as Event)];
    game = '${ev['game'] ?? ''}';
    fxGame = '${ev['fxGame'] ?? ''}';
    suggestedGame = '${ev['suggestedGame'] ?? ''}';
    dataDir = '${ev['data'] ?? ''}';
    allowMods = ev['allowMods'] == true;
    patchBackups = ev['patchBackups'] != false;
    hideOnLaunch = ev['hideOnLaunch'] != false;
    running = ev['running'] == true;
    version = '${ev['version'] ?? ''}';
    autostart = '${ev['autostart'] ?? 'off'}';
    if (ev['systemLang'] is String) systemLang = ev['systemLang'] as String;
    if (ev['log'] is String) {
      _log.clear();
      _appendLog(ev['log'] as String);
      logUnseen = false;
    }
    if (ev['prefs'] is Event) prefs = Prefs.fromJson(ev['prefs'] as Event);
    currentLang = lang;
    ready = true;
    if (page == AppPage.game) send({'cmd': 'gameInfo'});
  }

  void _onDone(Event ev) {
    if (page == AppPage.game) {
      send({'cmd': 'gameInfo'});
      send({'cmd': 'availableClients'});
    }
    final message = '${ev['message'] ?? ''}';
    switch (ev['status']) {
      case 'error':
        _notices.add(Notice(message, kind: NoticeKind.error, title: tb('${ev['title'] ?? ''}'), showLogs: true));
      case 'launched' when ev['hidden'] == true:
        _notices.add(Notice(t('Game started. AWLauncher is in the notification area.')));
      case 'cancelled':
        _notices.add(Notice(t('Cancelled')));
      case 'ok' when message.isNotEmpty:
        _notices.add(Notice(tb(message), kind: NoticeKind.ok));
    }
  }

  void _onUpdate(UpdateInfo u) {
    update = u;
    if (u.available && page != AppPage.settings) updateUnseen = true;
    if (u.quiet) return;
    if (u.error != null) {
      _notices.add(Notice(t('Could not check for updates: {error}', {'error': u.error})));
    } else if (!u.available) {
      _notices.add(
        Notice(
          _release.hasMatch(u.current)
              ? t('AWLauncher {version} is up to date', {'version': u.current})
              : t('Development build; the latest release is {latest}', {'latest': u.latest}),
        ),
      );
    }
  }

  @override
  void dispose() {
    _sub.cancel();
    _notices.close();
    _exitAsks.close();
    super.dispose();
  }
}
