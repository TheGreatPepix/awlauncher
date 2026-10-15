import 'dart:async';

import 'core.dart';

/// A core with sample accounts, for design work and tests: the Flutter
/// counterpart of the WebView page's dev/demo.js.
class FakeCore implements Core {
  FakeCore({this.tick = const Duration(milliseconds: 250), this.systemLang = 'en'});

  final Duration tick;
  final String systemLang;

  final _events = StreamController<Event>();
  final sent = <Map<String, Object?>>[];

  final accounts = <Map<String, Object?>>[
    {
      'id': '1',
      'name': 'Tanker',
      'login': '123456789',
      'service': 'VK Play',
      'provider': 'vkplay',
      'last': true,
      'autoLanguage': 'ru',
      'languages': ['ru', 'en'],
    },
    {
      'id': '2',
      'name': 'EU main',
      'login': 'player@example.com',
      'service': 'FX ID',
      'provider': 'fxid',
      'last': false,
      'language': 'pl',
      'autoLanguage': 'ru',
      'languages': ['en', 'de', 'fr', 'pl', 'ru', 'zh'],
    },
    {
      'id': '3',
      'name': 'Supertest',
      'login': 'tester@example.com',
      'service': 'FX ID, supertest',
      'provider': 'fxid',
      'branch': 'supertest',
      'last': false,
      'autoLanguage': 'ru',
      'languages': ['en', 'de', 'fr', 'pl', 'ru'],
    },
  ];
  final ops = <Map<String, Object?>>[];
  Map<String, Object?> prefs = {
    'theme': 'system',
    'hue': 300,
    'style': 'tonal',
    'palettes': <Object>[],
    'lang': 'auto',
  };
  var autostart = 'off';
  var allowMods = false, backups = true, keepOpen = false, running = false, paused = false;
  var _nextPrompt = 100;
  Timer? _download;

  @override
  Stream<Event> get events => _events.stream;

  void _emit(Event e) {
    if (!_events.isClosed) scheduleMicrotask(() => _events.add(e));
  }

  Event _state({bool full = false}) => {
    'type': 'state',
    'accounts': accounts,
    'game': r'H:\Games\Armored Warfare',
    'fxGame': '',
    'suggestedGame': r'C:\Games\Armored Warfare',
    'data': r'C:\Users\player\AppData\Local\AWLauncher',
    'version': 'v0.1.1',
    'autostart': autostart,
    'allowMods': allowMods,
    'patchBackups': backups,
    'hideOnLaunch': !keepOpen,
    'systemLang': systemLang,
    'running': running,
    'ops': List.of(ops),
    if (full) 'log': 'Game: H:\\Games\\Armored Warfare (build 442)\nBuild 442 is up to date.\n',
    if (full) 'prefs': prefs,
  };

  void _begin(Map<String, Object?> op) {
    ops.add(op);
    _emit(_state());
  }

  void _end(int id) {
    ops.removeWhere((o) => o['id'] == id);
    _emit(_state());
  }

  void _prompt(
    String op,
    String kind,
    String question, {
    List<String> context = const [],
    List<Event>? options,
    bool def = false,
  }) => _emit({
    'type': 'prompt',
    'id': _nextPrompt++,
    'op': op,
    'kind': kind,
    'question': question,
    'context': context,
    'default': def,
    'options': ?options,
  });

  final _waiting = <int, void Function(String value, bool ok)>{};

  void _ask(
    String op,
    String kind,
    String question,
    void Function(String value, bool ok) then, {
    List<String> context = const [],
    List<Event>? options,
    bool def = false,
  }) {
    _waiting[_nextPrompt] = then;
    _prompt(op, kind, question, context: context, options: options, def: def);
  }

  Map<String, Object?>? _account(Object? id) => accounts.where((a) => a['id'] == id).firstOrNull;

  @override
  void send(Map<String, Object?> c) {
    sent.add(c);
    switch (c['cmd']) {
      case 'ready':
        _emit(_state(full: true));
      case 'prefs':
        prefs = {
          for (final k in ['theme', 'hue', 'style', 'palettes', 'lang']) k: c[k],
        };
      case 'answer':
        _waiting.remove(c['prompt'])?.call('${c['value'] ?? ''}', c['ok'] == true);
      case 'play':
        final a = _account(c['account']);
        if (a == null) return;
        final title = 'Starting ${a['name']}';
        _begin({'id': 1, 'title': title, 'game': true, 'account': a['id']});
        _emit({'type': 'log', 'text': 'Checking for updates...\nUpdate available: 442 -> 443 (1 patches).\n'});
        _ask(
          title,
          'confirm',
          'Install now?',
          def: true,
          context: ['Checking for updates...', 'Update available: 442 -> 443 (1 patches).'],
          (value, ok) {
            if (ok && value == 'yes') {
              _startDownload(title);
            } else {
              _end(1);
              _emit({'type': 'done', 'status': 'cancelled', 'id': 1, 'title': title});
            }
          },
        );
      case 'pause':
        paused = true;
      case 'resume':
        paused = false;
      case 'closeGame':
        running = false;
        _emit({'type': 'game', 'running': false});
        _emit({'type': 'notice', 'message': 'The game is closed'});
      case 'pin':
        for (final a in accounts) {
          a['last'] = a['id'] == c['account'];
        }
        _emit(_state());
        _emit({'type': 'notice', 'message': '${_account(c['account'])?['name']} is the main account'});
      case 'rename':
        _account(c['account'])?['name'] = c['value'];
        _emit(_state());
      case 'remove':
        accounts.removeWhere((a) => a['id'] == c['account']);
        _emit(_state());
        _emit({'type': 'done', 'status': 'ok', 'title': 'Removing', 'message': 'Account removed'});
      case 'add':
        final title = c['provider'] == 'fxid' ? 'Signing in to FX ID' : 'Signing in to VK Play';
        _begin({'id': 4, 'title': title});
        if (c['provider'] != 'fxid') {
          _emit({'type': 'signin', 'active': true, 'window': false});
          return;
        }
        _ask(title, 'email', 'E-mail of the FX ID account', (email, ok) {
          if (!ok) {
            _end(4);
            _emit({'type': 'done', 'status': 'cancelled', 'title': title});
            return;
          }
          _ask(title, 'code', 'Code from the e-mail', context: ['A code was sent to $email.'], (code, ok) {
            _end(4);
            _emit({'type': 'done', 'status': ok ? 'ok' : 'cancelled', 'title': title, 'message': 'Account added'});
          });
        });
      case 'signinCancel':
        _end(4);
        _emit({'type': 'signin', 'active': false});
        _emit({'type': 'done', 'status': 'cancelled', 'title': 'Signing in to VK Play'});
      case 'autostart':
        autostart = '${c['value']}';
        _emit(_state());
      case 'allowMods':
        allowMods = c['value'] == 'on';
        _emit(_state());
      case 'patchBackups':
        backups = c['value'] == 'on';
        _emit(_state());
      case 'hideOnLaunch':
        keepOpen = c['value'] != 'on';
        _emit(_state());
      case 'gameInfo':
        _emit({
          'type': 'gameInfo',
          'dir': r'H:\Games\Armored Warfare',
          'downloads': 1288490188,
          'free': 182536110080,
          'clients': [
            {'kind': 'vkplay', 'dir': r'H:\Games\Armored Warfare', 'version': 'build 442', 'downloads': 1288490188},
            {'kind': 'fxid', 'dir': r'H:\Games\Armored Warfare FX', 'version': '0.566.1'},
          ],
          'branchPaths': {'supertest': r'H:\Games\Armored Warfare FX supertest'},
        });
      case 'availableClients':
        _emit({
          'type': 'availableClients',
          'clients': [
            {'kind': 'vkplay', 'account': '1'},
            {'kind': 'fxid', 'account': '2', 'version': '0.566.1'},
            {'kind': 'branch', 'branch': 'supertest', 'account': '3', 'version': '0.567.0'},
          ],
        });
      case 'language':
        final a = _account(c['account']);
        if (a == null) return;
        if ('${c['value']}'.isEmpty) {
          a.remove('language');
        } else {
          a['language'] = c['value'];
        }
        _emit(_state());
      case 'branches':
        final a = _account(c['account']);
        if (a == null) return;
        _begin({'id': 5, 'title': 'Choosing a branch', 'account': a['id']});
        _ask(
          'Choosing a branch',
          'choice',
          'Which client branch should ${a['name']} play?',
          options: [
            {'value': 'default', 'label': 'default', 'detail': 'The live servers', 'current': a['branch'] == null},
            {'value': 'supertest', 'label': 'supertest', 'detail': 'Test servers', 'current': a['branch'] == 'supertest'},
          ],
          (value, ok) {
            if (ok) {
              value == 'default' ? a.remove('branch') : a['branch'] = value;
            }
            _end(5);
          },
        );
      case 'key':
        _begin({'id': 6, 'title': 'Activating a key', 'account': c['account']});
        _ask('Activating a key', 'key', 'Key to activate', (value, ok) {
          _end(6);
          _emit({'type': 'done', 'status': ok ? 'ok' : 'cancelled', 'title': 'Activating a key', 'message': 'Key activated'});
        });
      case 'gameFolder' || 'fxFolder' || 'branchFolder':
        _begin({'id': 7, 'title': 'Choosing the game folder', 'game': true});
        _waiting[_nextPrompt] = (value, ok) => _end(7);
        _emit({
          'type': 'prompt',
          'id': _nextPrompt++,
          'op': 'Choosing the game folder',
          'kind': 'folder',
          'question': 'Game folder',
          'suggest': r'H:\Games\Armored Warfare',
          'context': <String>[],
        });
      case 'folderInfo':
        final path = '${c['value'] ?? ''}';
        final valid = RegExp(r'^[A-Za-z]:\\').hasMatch(path);
        _emit({
          'type': 'folderInfo',
          'prompt': c['prompt'],
          'path': path,
          'valid': valid,
          'free': valid ? 182536110080 : 0,
          'drive': valid ? '${path[0].toUpperCase()}:' : '',
          'install': valid && path.toLowerCase().contains('armored warfare'),
          'branch': '',
          'used': false,
        });
      case 'checkUpdate':
        _emit({
          'type': 'update',
          'current': 'v0.1.1',
          'latest': 'v0.1.1',
          'page': 'https://github.com/TheGreatPepix/awlauncher/releases/latest',
          'available': false,
          'canApply': false,
        });
      case 'requestExit':
        if (ops.isEmpty) {
          _emit({'type': 'exit'});
        } else {
          _emit({'type': 'window', 'show': true});
          _emit({'type': 'confirmExit'});
        }
      case 'exit':
        _emit({'type': 'exit'});
    }
  }

  void _startDownload(String title) {
    const gib = 1 << 30;
    const total = 3.4 * gib;
    var done = 0.0;
    _download?.cancel();
    _download = Timer.periodic(tick, (timer) {
      if (!paused) done = (done + 0.045 * gib).clamp(0, total);
      _emit({
        'type': 'progress',
        'active': done < total,
        'pausable': true,
        'paused': paused,
        'title': 'Downloading',
        'done': done.round(),
        'total': total.round(),
        'speed': paused ? 0 : 44 << 20,
        'tasks': [
          {
            'label': 'payload-442-443/gamesdk/textures_hi-0134.pak',
            'done': done.round() % (205 << 20),
            'total': 205 << 20,
          },
        ],
      });
      if (done >= total) {
        timer.cancel();
        _emit({'type': 'log', 'text': 'Downloading   done: 3.4 GiB in 1m19s, 44.0 MiB/s\n'});
        _end(1);
        running = true;
        _emit({'type': 'done', 'status': 'launched', 'id': 1, 'title': title, 'hidden': !keepOpen});
        _emit({'type': 'game', 'running': true});
      }
    });
  }

  @override
  Future<void> close() async {
    _download?.cancel();
    await _events.close();
  }
}
