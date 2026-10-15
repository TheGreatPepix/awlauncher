import 'dart:io';

/// What the window does for the core: the core asks, the window acts.
abstract class Desktop {
  void showWindow();
  void hideWindow();

  /// Opens a folder in Explorer or a link in the browser.
  void open(String target);

  /// Updates the tray menu's item that plays the main account.
  void setTrayPlay(String text, bool enabled);

  /// Closes the window for good; the core has already stopped.
  void quit();
}

/// The desktop of the running system. The window and the tray arrive with
/// the system step of the migration; until then they do nothing.
class SystemDesktop implements Desktop {
  @override
  void showWindow() {}

  @override
  void hideWindow() {}

  @override
  void open(String target) {
    final opener = Platform.isWindows ? 'explorer.exe' : (Platform.isMacOS ? 'open' : 'xdg-open');
    Process.run(opener, [target]);
  }

  @override
  void setTrayPlay(String text, bool enabled) {}

  @override
  void quit() => exit(0);
}
