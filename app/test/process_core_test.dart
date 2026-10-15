import 'dart:async';
import 'dart:io';

import 'package:awlauncher/core/core.dart';
import 'package:flutter_test/flutter_test.dart';

/// Talks to a real awcore built from this repository: AWCORE names it.
void main() {
  final exe = Platform.environment['AWCORE'];

  test('awcore answers the window over JSON lines', () async {
    final data = await Directory.systemTemp.createTemp('awlauncher-test-');
    addTearDown(() => data.delete(recursive: true));
    final core = await ProcessCore.start(exe!, environment: {'LOCALAPPDATA': data.path, 'XDG_DATA_HOME': data.path});
    final events = StreamIterator(core.events);
    Future<Event> next(String type) async {
      while (await events.moveNext().timeout(const Duration(seconds: 10))) {
        if (events.current['type'] == type) return events.current;
      }
      throw StateError('the core stopped before a $type event');
    }

    core.send({'cmd': 'ready'});
    final state = await next('state');
    expect(state['accounts'], isEmpty);
    expect(state['prefs'], isA<Map<String, dynamic>>());

    core.send({'cmd': 'openDataFolder'});
    expect((await next('open'))['target'], isNotEmpty);

    core.send({'cmd': 'requestExit'});
    await next('exit');
    expect((await next('coreExited'))['code'], 0);
  }, skip: exe == null ? 'set AWCORE to an awcore build' : null);
}
