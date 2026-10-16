import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// What the window and the tray tell the launcher.
class DesktopEvents {
  const DesktopEvents({required this.visible, required this.play, required this.exit});

  /// The window was shown or hidden; minimised counts as hidden.
  final ValueChanged<bool> visible;

  /// The tray menu's Play item was picked.
  final VoidCallback play;

  /// The tray menu's Exit item was picked.
  final VoidCallback exit;
}

/// What the window does for the core: the core asks, the window acts.
abstract class Desktop {
  /// Starts reporting [events].
  void listen(DesktopEvents events);

  void showWindow();
  void hideWindow();

  /// Opens a folder in Explorer or a link in the browser.
  void open(String target);

  /// Updates the tray menu's item that plays the main account.
  void setTrayPlay(String text, bool enabled);

  /// Names the tray menu's Open and Exit items in the window's language.
  void setTrayLabels({required String open, required String exit});

  /// Paints the title bar in the page's colours.
  void setTitleBar({required bool dark, required Color caption, required Color text});

  /// Closes the window for good; the core has already stopped.
  void quit();
}

/// The desktop of the running system. On Windows the runner keeps the
/// window and the tray icon (windows/runner/flutter_window.cpp); elsewhere
/// only links and folders open.
class SystemDesktop implements Desktop {
  static const _channel = MethodChannel('awlauncher/desktop');

  var _play = '', _open = '', _exit = '';
  var _enabled = false;
  Map<String, Object>? _titleBar;

  Future<void> _call(String method, [Object? args]) async {
    if (!Platform.isWindows) return;
    try {
      await _channel.invokeMethod<void>(method, args);
    } on MissingPluginException {
      // A runner without the channel, as in tests.
    }
  }

  @override
  void listen(DesktopEvents events) {
    if (!Platform.isWindows) return;
    _channel.setMethodCallHandler((call) async {
      switch (call.method) {
        case 'visible':
          events.visible(call.arguments == true);
        case 'play':
          events.play();
        case 'exit':
          events.exit();
      }
    });
    _visibleNow(events);
  }

  Future<void> _visibleNow(DesktopEvents events) async {
    try {
      events.visible(await _channel.invokeMethod<bool>('visible') ?? true);
    } on MissingPluginException {
      // A runner without the channel.
    }
  }

  @override
  void showWindow() => _call('show');

  @override
  void hideWindow() => _call('hide');

  @override
  void open(String target) {
    final opener = Platform.isWindows ? 'explorer.exe' : (Platform.isMacOS ? 'open' : 'xdg-open');
    Process.run(opener, [target]);
  }

  void _tray() => _call('tray', {'play': _play, 'enabled': _enabled, 'open': _open, 'exit': _exit});

  @override
  void setTrayPlay(String text, bool enabled) {
    if (text == _play && enabled == _enabled) return;
    _play = text;
    _enabled = enabled;
    _tray();
  }

  @override
  void setTrayLabels({required String open, required String exit}) {
    if (open == _open && exit == _exit) return;
    _open = open;
    _exit = exit;
    _tray();
  }

  @override
  void setTitleBar({required bool dark, required Color caption, required Color text}) {
    final args = {'dark': dark, 'caption': caption.toARGB32(), 'text': text.toARGB32()};
    if (mapEquals(args, _titleBar)) return;
    _titleBar = args;
    _call('titleBar', args);
  }

  @override
  void quit() {
    if (Platform.isWindows) {
      _call('quit');
    } else {
      exit(0);
    }
  }
}
