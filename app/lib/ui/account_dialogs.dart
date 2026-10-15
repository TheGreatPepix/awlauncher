import 'package:flutter/material.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import '../state/models.dart';
import 'widgets.dart';

const gameLanguages = {
  'en': 'English',
  'de': 'Deutsch',
  'fr': 'Français',
  'pl': 'Polski',
  'ru': 'Русский',
  'zh': '中文 (简体)',
};

Future<void> showAddAccount(BuildContext context, Controller c) {
  return showAw<void>(context, (context) {
    Widget choice(String provider, String title) => ChoiceTile(
      onTap: () {
        closeDialog(context);
        c.send({'cmd': 'add', 'provider': provider});
      },
      child: Text(title, style: const TextStyle(fontSize: 16).weight(500)),
    );
    return AwDialog(
      title: t('Add account'),
      body: [
        LayoutBuilder(
          builder: (context, box) {
            final w = (box.maxWidth - 10) / 2;
            return Wrap(
              spacing: 10,
              runSpacing: 10,
              children: [
                for (final (p, name) in const [('vkplay', 'VK Play'), ('fxid', 'FX ID'), ('steam', 'Steam')])
                  SizedBox(width: w, child: choice(p, name)),
              ],
            );
          },
        ),
      ],
      actions: [TextButton(onPressed: () => closeDialog(context), child: Text(t('Cancel')))],
    );
  });
}

/// A big button of the Add account dialog.
class ChoiceTile extends StatefulWidget {
  const ChoiceTile({super.key, required this.onTap, required this.child});

  final VoidCallback onTap;
  final Widget child;

  @override
  State<ChoiceTile> createState() => _ChoiceTileState();
}

class _ChoiceTileState extends State<ChoiceTile> {
  var _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final radius = BorderRadius.circular(_pressed ? 12 : 20);
    return AnimatedContainer(
      duration: motionFast,
      curve: spring,
      constraints: const BoxConstraints(minHeight: 72),
      decoration: BoxDecoration(color: scheme.surfaceContainerHighest, borderRadius: radius),
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          borderRadius: radius,
          onTap: widget.onTap,
          onHighlightChanged: (v) => setState(() => _pressed = v),
          child: Padding(
            padding: const EdgeInsets.all(18),
            child: Center(
              child: DefaultTextStyle.merge(
                style: TextStyle(color: scheme.onSurface),
                child: widget.child,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// A row of a list of options in a dialog; the chosen one is filled.
class OptionTile extends StatefulWidget {
  const OptionTile({
    super.key,
    required this.selected,
    required this.onTap,
    this.onDoubleTap,
    this.leading,
    required this.title,
    this.detail,
    this.trailing,
  });

  final bool selected;
  final VoidCallback onTap;
  final VoidCallback? onDoubleTap;
  final Widget? leading;
  final String title;
  final String? detail;
  final Widget? trailing;

  @override
  State<OptionTile> createState() => _OptionTileState();
}

class _OptionTileState extends State<OptionTile> {
  var _pressed = false;
  DateTime? _lastTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final on = widget.selected;
    final radius = BorderRadius.circular(
      _pressed
          ? 8
          : on
          ? 20
          : 14,
    );
    final fg = on ? scheme.onPrimaryContainer : scheme.onSurface;
    return AnimatedContainer(
      duration: motionFast,
      curve: spring,
      decoration: BoxDecoration(
        color: on ? scheme.primaryContainer : scheme.surfaceContainerHighest,
        borderRadius: radius,
      ),
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          borderRadius: radius,
          onHighlightChanged: (v) => setState(() => _pressed = v),
          onTap: () {
            final now = DateTime.now();
            final double = _lastTap != null && now.difference(_lastTap!) < const Duration(milliseconds: 400);
            _lastTap = double ? null : now;
            widget.onTap();
            if (double) widget.onDoubleTap?.call();
          },
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            child: Row(
              children: [
                if (widget.leading != null) ...[widget.leading!, const SizedBox(width: 12)],
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(widget.title, style: TextStyle(fontSize: 14, color: fg)),
                      if (widget.detail != null && widget.detail!.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 2),
                          child: Text(
                            widget.detail!,
                            style: TextStyle(
                              fontSize: 12,
                              color: on ? fg.withValues(alpha: 0.8) : scheme.onSurfaceVariant,
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
                if (widget.trailing != null) ...[const SizedBox(width: 12), widget.trailing!],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

Future<void> showGameLanguage(BuildContext context, Controller c, Account a) {
  void choose(BuildContext context, String code) {
    closeDialog(context);
    c.send({'cmd': 'language', 'account': a.id, 'value': code});
  }

  final current = a.language ?? '';
  Widget option(BuildContext context, String code, String label) => OptionTile(
    selected: current == code,
    onTap: () => choose(context, code),
    title: label,
    trailing: current == code
        ? Icon(Icons.check, size: 20, color: Theme.of(context).colorScheme.onPrimaryContainer)
        : null,
  );
  return showAw<void>(
    context,
    (context) => AwDialog(
      icon: Icons.language,
      title: t('Game language'),
      body: [
        DialogText('${a.service} · ${a.name}'),
        Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            option(context, '', t('Automatically ({lang})', {'lang': gameLanguages[a.autoLanguage] ?? a.autoLanguage})),
            for (final code in a.languages) ...[
              const SizedBox(height: 4),
              option(context, code, gameLanguages[code] ?? code),
            ],
          ],
        ),
        Hint(t('Automatically follows the Windows languages.')),
      ],
      actions: [TextButton(onPressed: () => closeDialog(context), child: Text(t('Cancel')))],
    ),
  );
}

Future<void> showRename(BuildContext context, Controller c, Account a) {
  return showAw<void>(context, (context) => _RenameDialog(c: c, a: a));
}

class _RenameDialog extends StatefulWidget {
  const _RenameDialog({required this.c, required this.a});

  final Controller c;
  final Account a;

  @override
  State<_RenameDialog> createState() => _RenameDialogState();
}

class _RenameDialogState extends State<_RenameDialog> {
  late final _input = TextEditingController(
    text: widget.a.name == widget.a.login ? '' : widget.a.name,
  )..selection = TextSelection(baseOffset: 0, extentOffset: widget.a.name == widget.a.login ? 0 : widget.a.name.length);

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  void _save() {
    closeDialog(context);
    widget.c.send({'cmd': 'rename', 'account': widget.a.id, 'value': _input.text.trim()});
  }

  @override
  Widget build(BuildContext context) {
    final a = widget.a;
    return AwDialog(
      icon: Icons.edit,
      title: t('Rename account'),
      body: [
        DialogText('${a.service} · ${a.login}'),
        TextField(
          controller: _input,
          autofocus: true,
          maxLength: 64,
          decoration: fieldDecoration(context, t('Name')),
          onSubmitted: (_) => _save(),
        ),
        Hint(
          t(a.provider == 'fxid' ? 'Leave it empty to show the e-mail.' : 'Leave it empty to show the account number.'),
        ),
      ],
      actions: [
        TextButton(onPressed: () => closeDialog(context), child: Text(t('Cancel'))),
        FilledButton(onPressed: _save, child: Text(t('Save'))),
      ],
    );
  }
}

Future<void> confirmRemove(BuildContext context, Controller c, Account a) {
  return showAw<void>(
    context,
    (context) => AwDialog(
      icon: Icons.delete,
      danger: true,
      title: t('Remove account?'),
      body: [
        DialogText(
          t('{name} ({service}) is removed from this launcher, with its saved sign-in. The game files stay.', {
            'name': a.name,
            'service': a.service,
          }),
        ),
      ],
      actions: [
        TextButton(onPressed: () => closeDialog(context), child: Text(t('Cancel'))),
        dangerButton(context, t('Remove'), () {
          closeDialog(context);
          c.send({'cmd': 'remove', 'account': a.id});
        }),
      ],
    ),
  );
}

Future<void> confirmCloseGame(BuildContext context, Controller c) {
  return showAw<void>(
    context,
    (context) => AwDialog(
      icon: Icons.close,
      danger: true,
      title: t('Close the game?'),
      body: [
        DialogText(
          t(
            'Armored Warfare is asked to close; if it does not respond in 10 seconds, it is ended. If you are in a battle, you leave it.',
          ),
        ),
      ],
      actions: [
        TextButton(onPressed: () => closeDialog(context), child: Text(t('Cancel'))),
        dangerButton(context, t('Close game'), () {
          closeDialog(context);
          c.send({'cmd': 'closeGame'});
        }),
      ],
    ),
  );
}
