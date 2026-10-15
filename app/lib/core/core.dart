import 'dart:async';
import 'dart:convert';
import 'dart:io';

typedef Event = Map<String, dynamic>;

/// The launcher's core: it takes {cmd: ...} commands and sends {type: ...}
/// events, the protocol of internal/gui/commands.go.
abstract class Core {
  Stream<Event> get events;
  void send(Map<String, Object?> command);
  Future<void> close();
}

/// Runs awcore.exe and talks to it with JSON lines on its stdin and stdout.
class ProcessCore implements Core {
  ProcessCore._(this._process) {
    _process.stdout.transform(utf8.decoder).transform(const LineSplitter()).listen(_line, onDone: _ended);
    _process.stderr.drain<void>();
  }

  final Process _process;
  final _events = StreamController<Event>();
  var _closed = false;

  /// Starts the core at [exe]. [app] is the window's own executable, which
  /// the core registers to start with Windows.
  static Future<ProcessCore> start(String exe, {String? app, Map<String, String>? environment}) async {
    final process = await Process.start(exe, [
      if (app != null) ...['--app', app],
    ], environment: environment);
    return ProcessCore._(process);
  }

  /// The core next to the window's executable, or null if there is none.
  static String? beside(String windowExe) {
    final dir = File(windowExe).parent.path;
    for (final name in ['awcore.exe', 'awcore']) {
      final path = '$dir${Platform.pathSeparator}$name';
      if (File(path).existsSync()) return path;
    }
    return null;
  }

  @override
  Stream<Event> get events => _events.stream;

  void _line(String line) {
    final Object? event;
    try {
      event = jsonDecode(line);
    } on FormatException {
      return;
    }
    if (event is Event) _events.add(event);
  }

  Future<void> _ended() async {
    final code = await _process.exitCode;
    _events.add({'type': 'coreExited', 'code': code});
    await _events.close();
  }

  @override
  void send(Map<String, Object?> command) {
    if (_closed) return;
    _process.stdin.writeln(jsonEncode(command));
  }

  @override
  Future<void> close() async {
    if (_closed) return;
    _closed = true;
    await _process.stdin.close();
    await _process.exitCode.timeout(
      const Duration(seconds: 10),
      onTimeout: () {
        _process.kill();
        return -1;
      },
    );
  }
}
