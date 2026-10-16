import 'dart:async';

import 'package:flutter/material.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import '../state/models.dart';
import 'pages/game_page.dart';
import 'pages/home_page.dart';
import 'pages/logs_page.dart';
import 'pages/page_frame.dart';
import 'pages/settings_page.dart';
import 'pong.dart';
import 'prompt_dialog.dart';
import 'side_nav.dart';
import 'widgets.dart';

/// The window: the drawer, the current page, and the dialogs and toasts the
/// core asks for.
class Shell extends StatefulWidget {
  const Shell({super.key, required this.c});

  final Controller c;

  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  final _subs = <StreamSubscription<void>>[];
  final _toasts = <_ToastEntry>[];
  var _toastId = 0;

  // The core's dialogs (questions, update offers, the exit question) wait
  // for each other, as the WebView page's whenDialogFree did.
  Future<void> _dialogs = Future.value();
  var _prompting = false, _askingExit = false;

  Controller get c => widget.c;

  @override
  void initState() {
    super.initState();
    _subs
      ..add(c.notices.listen(_notice))
      ..add(c.exitAsks.listen((_) => _askExit()))
      ..add(c.updateOffers.listen((u) => _whenFree(() => _offerUpdate(u))));
    c.addListener(_nextPrompt);
  }

  @override
  void dispose() {
    for (final s in _subs) {
      s.cancel();
    }
    for (final t in _toasts) {
      t.timer.cancel();
    }
    c.removeListener(_nextPrompt);
    super.dispose();
  }

  void _whenFree(Future<void> Function() show) {
    _dialogs = _dialogs.then((_) => mounted ? show() : null).catchError((Object _) {});
  }

  void _notice(Notice n) {
    final id = _toastId++;
    final entry = _ToastEntry(
      id,
      n,
      Timer(Duration(seconds: n.kind == NoticeKind.error ? 12 : 5), () => _closeToast(id)),
    );
    setState(() {
      _toasts.add(entry);
      while (_toasts.length > 4) {
        _toasts.removeAt(0).timer.cancel();
      }
    });
  }

  void _closeToast(int id) {
    if (!mounted) return;
    setState(() => _toasts.removeWhere((t) => t.id == id));
  }

  /// Shows the core's questions one at a time, oldest first.
  void _nextPrompt() {
    if (_prompting || c.prompts.isEmpty || !mounted) return;
    _prompting = true;
    _whenFree(() async {
      while (mounted && c.prompts.isNotEmpty) {
        final p = c.prompts.first;
        final (value, ok) = await showPrompt(context, c, p);
        if (c.prompts.any((x) => x.id == p.id)) c.answer(p, value, ok: ok);
      }
      _prompting = false;
    });
  }

  void _askExit() {
    if (_askingExit) return;
    _askingExit = true;
    _whenFree(_confirmExit);
  }

  Future<void> _confirmExit() async {
    _askingExit = true;
    if (!c.busy) {
      _askingExit = false;
      c.send({'cmd': 'exit'});
      return;
    }
    final quit = await showAw<bool>(
      context,
      (context) => AwDialog(
        icon: Icons.logout,
        danger: true,
        title: t('Exit AWLauncher?'),
        body: [
          DialogText(t('{ops} is still in progress. If you exit now, it is interrupted.', {'ops': c.opsTitle})),
        ],
        actions: [
          TextButton(onPressed: () => closeDialog(context, false), child: Text(t('Cancel'))),
          dangerButton(context, t('Exit'), () => closeDialog(context, true)),
        ],
      ),
    );
    _askingExit = false;
    if (quit == true) c.send({'cmd': 'exit'});
  }

  Future<void> _offerUpdate(UpdateInfo u) async {
    final notes = (u.notes ?? '').replaceAll('\r', '');
    var text = t('You have {current}.', {'current': u.current});
    if (u.published != null) text += t(' {latest} was released {date}.', {'latest': u.latest, 'date': u.published});
    text += t(
      u.canApply
          ? ' AWLauncher downloads it and restarts; your accounts and settings stay.'
          : ' Download it from the release page.',
    );
    final choice = await showAw<String>(
      context,
      (context) => AwDialog(
        icon: Icons.update,
        title: t('AWLauncher {latest} is available', {'latest': u.latest}),
        body: [
          DialogText(text),
          if (notes.isNotEmpty) ContextBox([notes.length > 1500 ? '${notes.substring(0, 1500)}…' : notes]),
        ],
        actions: [
          TextButton(onPressed: () => closeDialog(context), child: Text(t('Later'))),
          if (u.canApply) ...[
            TextButton(onPressed: () => closeDialog(context, 'page'), child: Text(t('Release page'))),
            FilledButton(autofocus: true, onPressed: () => closeDialog(context, 'update'), child: Text(t('Update'))),
          ] else
            FilledButton(
              autofocus: true,
              onPressed: () => closeDialog(context, 'page'),
              child: Text(t('Release page')),
            ),
        ],
      ),
    );
    if (choice == 'page') c.openLink(u.page);
    if (choice == 'update') c.applyUpdate();
  }

  /// Keeps the title bar and the tray menu in the page's theme and language.
  void _syncWindow(ColorScheme scheme) {
    c.desktop
      ..setTitleBar(dark: scheme.brightness == Brightness.dark, caption: scheme.surface, text: scheme.onSurface)
      ..setTrayLabels(open: t('Open AWLauncher'), exit: t('Exit'));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: ListenableBuilder(
        listenable: c,
        builder: (context, _) {
          _syncWindow(Theme.of(context).colorScheme);
          if (c.fatal != null) return _Fatal(message: c.fatal!);
          if (!c.ready) return const Center(child: CircularProgressIndicator());
          final page = switch (c.page) {
            AppPage.home => HomePage(c: c),
            AppPage.game => GamePage(c: c),
            AppPage.activity => LogsPage(c: c, onNotice: _notice),
            AppPage.settings => SettingsPage(c: c, onExit: () => _whenFree(_confirmExit)),
          };
          return LayoutBuilder(
            builder: (context, box) {
              final drawer = box.maxWidth <= 900 ? 200.0 : 232.0;
              // The drawer is painted over the page so that the falling logo
              // passes in front of it.
              return Stack(
                children: [
                  Positioned.fill(
                    left: drawer,
                    child: PageEnter(key: ValueKey(c.page), child: page),
                  ),
                  Positioned(
                    left: 0,
                    top: 0,
                    bottom: 0,
                    child: SideNav(
                      c: c,
                      width: drawer,
                      onToast: (s) => _notice(Notice(s)),
                      onPong: () => _whenFree(() => showPong(context)),
                    ),
                  ),
                  Positioned(
                    left: drawer + 24,
                    right: 24,
                    bottom: 24,
                    child: _Toasts(toasts: _toasts, onClose: _closeToast, onLogs: () => c.showPage(AppPage.activity)),
                  ),
                ],
              );
            },
          );
        },
      ),
    );
  }
}

class _ToastEntry {
  _ToastEntry(this.id, this.notice, this.timer);

  final int id;
  final Notice notice;
  final Timer timer;
}

class _Toasts extends StatelessWidget {
  const _Toasts({required this.toasts, required this.onClose, required this.onLogs});

  final List<_ToastEntry> toasts;
  final ValueChanged<int> onClose;
  final VoidCallback onLogs;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final t in toasts)
          Padding(
            key: ValueKey(t.id),
            padding: const EdgeInsets.only(top: 8),
            child: _Toast(entry: t, onClose: () => onClose(t.id), onLogs: onLogs),
          ),
      ],
    );
  }
}

/// A short message at the bottom of the window, as the WebView page had them.
class _Toast extends StatelessWidget {
  const _Toast({required this.entry, required this.onClose, required this.onLogs});

  final _ToastEntry entry;
  final VoidCallback onClose;
  final VoidCallback onLogs;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final n = entry.notice;
    final error = n.kind == NoticeKind.error;
    final dark = scheme.brightness == Brightness.dark;
    final fg = scheme.onInverseSurface;
    return TweenAnimationBuilder<double>(
      tween: Tween(begin: 0, end: 1),
      duration: motion,
      builder: (context, v, child) => Opacity(
        opacity: effectsCurve.transform((v * 2.5).clamp(0, 1)),
        child: Transform.translate(offset: Offset(0, (1 - spring.transform(v)) * 20), child: child),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 560, minHeight: 48),
        child: Material(
          color: scheme.inverseSurface,
          elevation: 6,
          borderRadius: BorderRadius.circular(4),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 4, 4, 4),
            child: Row(
              children: [
                if (error) ...[
                  Icon(Icons.error, color: dark ? const Color(0xFFBA1A1A) : const Color(0xFFFFB4AB)),
                  const SizedBox(width: 12),
                ],
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(0, 10, 8, 10),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if (n.title != null && n.title!.isNotEmpty)
                          Text(
                            n.title!,
                            style: TextStyle(fontSize: 14, height: 20 / 14, color: fg).weight(600),
                          ),
                        Text(
                          n.message,
                          style: TextStyle(fontSize: 14, height: 20 / 14, color: fg),
                        ),
                      ],
                    ),
                  ),
                ),
                if (n.showLogs)
                  TextButton(
                    style: TextButton.styleFrom(
                      foregroundColor: scheme.inversePrimary,
                      minimumSize: const Size(0, 36),
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                    ),
                    onPressed: () {
                      onClose();
                      onLogs();
                    },
                    child: Text(t('Logs')),
                  ),
                IconButton(
                  tooltip: t('Close'),
                  color: fg,
                  iconSize: 18,
                  constraints: const BoxConstraints.tightFor(width: 36, height: 36),
                  padding: EdgeInsets.zero,
                  onPressed: onClose,
                  icon: const Icon(Icons.close),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _Fatal extends StatelessWidget {
  const _Fatal({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: Theme.of(context).colorScheme.error),
            const SizedBox(height: 16),
            Text(t('AWLauncher cannot start: {error}', {'error': message}), textAlign: TextAlign.center),
          ],
        ),
      ),
    );
  }
}
