import 'dart:async';

import 'package:flutter/material.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import '../state/models.dart';
import 'pages/game_page.dart';
import 'pages/home_page.dart';
import 'pages/logs_page.dart';
import 'pages/settings_page.dart';
import 'prompt_dialog.dart';
import 'side_nav.dart';

/// The window: the navigation, the current page, and the dialogs and
/// snackbars the core asks for.
class Shell extends StatefulWidget {
  const Shell({super.key, required this.c});

  final Controller c;

  @override
  State<Shell> createState() => _ShellState();
}

class _ShellState extends State<Shell> {
  final _messenger = GlobalKey<ScaffoldMessengerState>();
  final _subs = <StreamSubscription<void>>[];
  var _prompting = false;
  var _askingExit = false;

  Controller get c => widget.c;

  @override
  void initState() {
    super.initState();
    _subs
      ..add(c.notices.listen(_notice))
      ..add(c.exitAsks.listen((_) => _confirmExit()));
    c.addListener(_nextPrompt);
  }

  @override
  void dispose() {
    for (final s in _subs) {
      s.cancel();
    }
    c.removeListener(_nextPrompt);
    super.dispose();
  }

  void _notice(Notice n) {
    final scheme = Theme.of(context).colorScheme;
    final error = n.kind == NoticeKind.error;
    _messenger.currentState?.showSnackBar(
      SnackBar(
        backgroundColor: error ? scheme.errorContainer : null,
        showCloseIcon: true,
        closeIconColor: error ? scheme.onErrorContainer : null,
        duration: Duration(seconds: error ? 10 : 4),
        content: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            if (n.title != null)
              Text(
                n.title!,
                style: TextStyle(fontWeight: FontWeight.w600, color: error ? scheme.onErrorContainer : null),
              ),
            Text(n.message, style: TextStyle(color: error ? scheme.onErrorContainer : null)),
          ],
        ),
        action: n.showLogs ? SnackBarAction(label: t('Logs'), onPressed: () => c.showPage(AppPage.activity)) : null,
      ),
    );
  }

  /// Shows the core's questions one at a time, oldest first.
  Future<void> _nextPrompt() async {
    if (_prompting || _askingExit || c.prompts.isEmpty || !mounted) return;
    _prompting = true;
    final p = c.prompts.first;
    final answer = await showPrompt(context, p);
    _prompting = false;
    if (c.prompts.any((x) => x.id == p.id)) {
      final (value, ok) = answer ?? ('', false);
      c.answer(p, value, ok: ok);
    }
    unawaited(_nextPrompt());
  }

  Future<void> _confirmExit() async {
    if (!c.busy) {
      c.send({'cmd': 'exit'});
      return;
    }
    if (_askingExit) return;
    _askingExit = true;
    final quit = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        icon: const Icon(Icons.logout),
        title: Text(t('Exit AWLauncher?')),
        content: Text(
          t('{ops} is still in progress. If you exit now, it is interrupted.', {
            'ops': c.ops.map((o) => tb(o.title)).join(' · '),
          }),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(t('Cancel'))),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: Theme.of(context).colorScheme.error),
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(t('Exit')),
          ),
        ],
      ),
    );
    _askingExit = false;
    if (quit == true) c.send({'cmd': 'exit'});
    unawaited(_nextPrompt());
  }

  @override
  Widget build(BuildContext context) {
    return ScaffoldMessenger(
      key: _messenger,
      child: Scaffold(
        body: ListenableBuilder(
          listenable: c,
          builder: (context, _) {
            if (c.fatal != null) return _Fatal(message: c.fatal!);
            if (!c.ready) return const Center(child: CircularProgressIndicator());
            final page = switch (c.page) {
              AppPage.home => HomePage(c: c),
              AppPage.game => GamePage(c: c),
              AppPage.activity => LogsPage(c: c, onNotice: _notice),
              AppPage.settings => SettingsPage(c: c, onExit: _confirmExit),
            };
            return LayoutBuilder(
              builder: (context, box) => Row(
                children: [
                  SideNav(c: c, width: box.maxWidth < 900 ? 200 : 232),
                  Expanded(child: page),
                ],
              ),
            );
          },
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
