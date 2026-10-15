import 'dart:async';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import '../state/models.dart';
import 'account_dialogs.dart';
import 'format.dart';
import 'widgets.dart';

const _gib = 1 << 30;

/// About how much a new install takes.
const clientSize = 70 * _gib;

/// Asks the question of a core operation and returns the answer's value and
/// whether it was given. Esc answers "no" to a yes/no question and cancels
/// the others.
Future<(String, bool)> showPrompt(BuildContext context, Controller c, PromptRequest p) async {
  final answer = await showAw<(String, bool)>(context, dismissible: false, (context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) closeDialog(context, p.kind == 'confirm' ? ('no', true) : ('', false));
      },
      child: CallbackShortcuts(
        bindings: {const SingleActivator(LogicalKeyboardKey.escape): () => Navigator.of(context).maybePop()},
        child: switch (p.kind) {
          'confirm' => _Confirm(p),
          'folder' => _FolderPrompt(c: c, p: p),
          'code' => _CodePrompt(p),
          'choice' => _ChoicePrompt(p),
          _ => _TextPrompt(p),
        },
      ),
    );
  });
  return answer ?? (p.kind == 'confirm' ? ('no', true) : ('', false));
}

List<String> _context(PromptRequest p) => p.context.length > 14 ? p.context.sublist(p.context.length - 14) : p.context;

void _reply(BuildContext context, String value, bool ok) => closeDialog(context, (value, ok));

class _Confirm extends StatelessWidget {
  const _Confirm(this.p);

  final PromptRequest p;

  @override
  Widget build(BuildContext context) {
    final lines = _context(p);
    Widget button(String label, String value, bool primary) => primary
        ? FilledButton(autofocus: true, onPressed: () => _reply(context, value, true), child: Text(label))
        : TextButton(onPressed: () => _reply(context, value, true), child: Text(label));
    return AwDialog(
      icon: Icons.help,
      op: tb(p.op),
      title: tb(p.question),
      body: [if (lines.isNotEmpty) ContextBox(lines)],
      actions: [button(t('No'), 'no', !p.isDefault), button(t('Yes'), 'yes', p.isDefault)],
    );
  }
}

class _TextPrompt extends StatefulWidget {
  const _TextPrompt(this.p);

  final PromptRequest p;

  @override
  State<_TextPrompt> createState() => _TextPromptState();
}

class _TextPromptState extends State<_TextPrompt> {
  final _input = TextEditingController();

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.p;
    final lines = _context(p);
    final (label, icon, type) = switch (p.kind) {
      'email' => ('E-mail', Icons.mail, TextInputType.emailAddress),
      'key' => ('Key', Icons.key, TextInputType.text),
      'name' => ('Name', Icons.edit, TextInputType.text),
      _ => ('Answer', Icons.edit, TextInputType.text),
    };
    return AwDialog(
      icon: icon,
      op: tb(p.op),
      title: tb(p.question),
      body: [
        if (lines.isNotEmpty) ContextBox(lines),
        TextField(
          controller: _input,
          autofocus: true,
          keyboardType: type,
          autocorrect: false,
          enableSuggestions: false,
          decoration: fieldDecoration(context, t(label)),
          onSubmitted: (v) => _reply(context, v, true),
        ),
      ],
      actions: [
        TextButton(onPressed: () => _reply(context, '', false), child: Text(t('Cancel'))),
        FilledButton(onPressed: () => _reply(context, _input.text, true), child: Text(t('OK'))),
      ],
    );
  }
}

class _ChoicePrompt extends StatefulWidget {
  const _ChoicePrompt(this.p);

  final PromptRequest p;

  @override
  State<_ChoicePrompt> createState() => _ChoicePromptState();
}

class _ChoicePromptState extends State<_ChoicePrompt> {
  late String _chosen =
      (widget.p.options.where((o) => o.current).firstOrNull ?? widget.p.options.firstOrNull)?.value ?? '';

  @override
  Widget build(BuildContext context) {
    final p = widget.p;
    final lines = _context(p);
    final scheme = Theme.of(context).colorScheme;
    return AwDialog(
      icon: Icons.alt_route,
      op: tb(p.op),
      title: tb(p.question),
      body: [
        if (lines.isNotEmpty) ContextBox(lines),
        Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            for (var i = 0; i < p.options.length; i++) ...[
              if (i > 0) const SizedBox(height: 4),
              () {
                final o = p.options[i];
                final on = o.value == _chosen;
                return OptionTile(
                  selected: on,
                  onTap: () => setState(() => _chosen = o.value),
                  onDoubleTap: () => _reply(context, o.value, true),
                  leading: Container(
                    width: 28,
                    height: 28,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: on ? scheme.primary : scheme.secondaryContainer,
                      shape: BoxShape.circle,
                    ),
                    child: Text(
                      '${i + 1}',
                      style: TextStyle(
                        fontSize: 13,
                        color: on ? scheme.onPrimary : scheme.onSecondaryContainer,
                      ).weight(600),
                    ),
                  ),
                  title: o.label,
                  detail: o.detail,
                  trailing: o.current
                      ? Text(
                          t('current').toUpperCase(),
                          style: TextStyle(fontSize: 11, letterSpacing: 0.5, color: scheme.primary).weight(600),
                        )
                      : null,
                );
              }(),
            ],
          ],
        ),
      ],
      actions: [
        TextButton(onPressed: () => _reply(context, '', false), child: Text(t('Cancel'))),
        FilledButton(onPressed: () => _reply(context, _chosen, true), child: Text(t('OK'))),
      ],
    );
  }
}

const _codeLength = 6;

/// The six boxes of the code from the e-mail. Typing or pasting digits
/// moves along the boxes, and the sixth digit sends the code.
class _CodePrompt extends StatefulWidget {
  const _CodePrompt(this.p);

  final PromptRequest p;

  @override
  State<_CodePrompt> createState() => _CodePromptState();
}

class _CodePromptState extends State<_CodePrompt> {
  final _boxes = [for (var i = 0; i < _codeLength; i++) TextEditingController()];
  late final _focus = [
    for (var i = 0; i < _codeLength; i++)
      FocusNode(onKeyEvent: (node, e) => _key(i, e))..addListener(() => setState(() {})),
  ];
  var _sent = false;

  @override
  void dispose() {
    for (final b in _boxes) {
      b.dispose();
    }
    for (final f in _focus) {
      f.dispose();
    }
    super.dispose();
  }

  String get _code => _boxes.map((b) => b.text).join();

  void _submit() {
    if (_sent) return;
    if (_code.length != _codeLength) {
      final empty = _boxes.indexWhere((b) => b.text.isEmpty);
      if (empty >= 0) _focus[empty].requestFocus();
      return;
    }
    _sent = true;
    _reply(context, _code, true);
  }

  void _select(int i) {
    _focus[i].requestFocus();
    _boxes[i].selection = TextSelection(baseOffset: 0, extentOffset: _boxes[i].text.length);
  }

  void _fill(int from, String digits) {
    var i = from;
    for (final d in digits.split('')) {
      if (i >= _codeLength) break;
      _boxes[i++].text = d;
    }
    _select(i < _codeLength ? i : _codeLength - 1);
    if (_code.length == _codeLength) _submit();
  }

  void _changed(int i, String value) {
    final digits = value.replaceAll(RegExp(r'\D'), '');
    _boxes[i].text = '';
    if (digits.isEmpty) return;
    if (digits.length >= _codeLength) {
      for (final b in _boxes) {
        b.text = '';
      }
      _fill(0, digits.substring(0, _codeLength));
    } else {
      _fill(i, digits);
    }
  }

  KeyEventResult _key(int i, KeyEvent e) {
    if (e is! KeyDownEvent && e is! KeyRepeatEvent) return KeyEventResult.ignored;
    final k = e.logicalKey;
    if (k == LogicalKeyboardKey.backspace && _boxes[i].text.isEmpty && i > 0) {
      _boxes[i - 1].text = '';
      _select(i - 1);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.arrowLeft && i > 0) {
      _select(i - 1);
      return KeyEventResult.handled;
    }
    if (k == LogicalKeyboardKey.arrowRight && i < _codeLength - 1) {
      _select(i + 1);
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.p;
    final lines = _context(p);
    final scheme = Theme.of(context).colorScheme;
    return AwDialog(
      icon: Icons.mail,
      op: tb(p.op),
      title: t('Code from the e-mail'),
      body: [
        if (lines.isNotEmpty) ContextBox(lines),
        Padding(
          padding: const EdgeInsets.only(top: 6, bottom: 2),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              for (var i = 0; i < _codeLength; i++) ...[
                if (i > 0) const SizedBox(width: 10),
                () {
                  final focused = _focus[i].hasFocus;
                  final radius = BorderRadius.circular(focused ? 20 : 14);
                  return AnimatedContainer(
                    duration: motionFast,
                    curve: spring,
                    width: 48,
                    height: 58,
                    decoration: BoxDecoration(
                      color: focused ? scheme.surfaceContainerLowest : scheme.surfaceContainerHighest,
                      borderRadius: radius,
                      border: Border.all(color: focused ? scheme.primary : scheme.outlineVariant, width: 2),
                    ),
                    alignment: Alignment.center,
                    child: TextField(
                      controller: _boxes[i],
                      focusNode: _focus[i],
                      autofocus: i == 0,
                      textAlign: TextAlign.center,
                      keyboardType: TextInputType.number,
                      cursorColor: scheme.primary,
                      style: TextStyle(
                        fontSize: 26,
                        color: scheme.onSurface,
                        fontFeatures: const [FontFeature.tabularFigures()],
                      ).weight(600),
                      decoration: const InputDecoration(border: InputBorder.none, isCollapsed: true, counterText: ''),
                      onTap: () => _select(i),
                      onChanged: (v) => _changed(i, v),
                      onSubmitted: (_) => _submit(),
                    ),
                  );
                }(),
              ],
            ],
          ),
        ),
      ],
      actions: [
        TextButton(
          onPressed: () {
            _sent = true;
            _reply(context, '', false);
          },
          child: Text(t('Cancel')),
        ),
        FilledButton(onPressed: _submit, child: Text(t('OK'))),
      ],
    );
  }
}

/// The game folder question: a path, a Browse button, and what the core
/// found there, checked as the path changes.
class _FolderPrompt extends StatefulWidget {
  const _FolderPrompt({required this.c, required this.p});

  final Controller c;
  final PromptRequest p;

  @override
  State<_FolderPrompt> createState() => _FolderPromptState();
}

class _FolderPromptState extends State<_FolderPrompt> {
  late final _input = TextEditingController(text: widget.p.suggest ?? '');
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    widget.c.checkFolder(widget.p.id, _input.text);
  }

  @override
  void dispose() {
    _timer?.cancel();
    _input.dispose();
    super.dispose();
  }

  void _changed(String _) {
    setState(() {});
    _timer?.cancel();
    _timer = Timer(const Duration(milliseconds: 250), () => widget.c.checkFolder(widget.p.id, _input.text));
  }

  Future<void> _browse() async {
    final initial = _input.text.trim();
    final dir = await getDirectoryPath(initialDirectory: initial.isEmpty ? null : initial);
    if (dir == null || !mounted) return;
    _input.text = dir;
    _changed(dir);
  }

  void _use() {
    final v = _input.text.trim();
    if (v.isNotEmpty) _reply(context, v, true);
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.p;
    final notes = p.context.map((l) => l.trim()).where((l) => l.isNotEmpty).toList();
    return AwDialog(
      icon: Icons.folder,
      op: tb(p.op),
      title: t('Game folder'),
      body: [
        if (notes.isEmpty)
          DialogText(t('Choose the folder of an existing install, or an empty folder to install the game into.'))
        else
          for (final n in notes) DialogText(tb(n)),
        Row(
          children: [
            Expanded(
              child: TextField(
                controller: _input,
                autofocus: true,
                autocorrect: false,
                enableSuggestions: false,
                decoration: fieldDecoration(context, t('Folder')),
                onChanged: _changed,
                onSubmitted: (_) => _use(),
              ),
            ),
            const SizedBox(width: 10),
            SizedBox(
              height: 56,
              child: FilledButton.tonalIcon(
                style: ButtonStyle(
                  shape: WidgetStateProperty.resolveWith(
                    (s) => RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(s.contains(WidgetState.pressed) ? 8 : 16),
                    ),
                  ),
                ),
                onPressed: _browse,
                icon: const Icon(Icons.folder, size: 18),
                label: Text(t('Browse…')),
              ),
            ),
          ],
        ),
        ListenableBuilder(
          listenable: widget.c,
          builder: (context, _) {
            final info = widget.c.folderInfo[p.id];
            return _FolderStatus(info: info != null && info.path == _input.text ? info : null, path: _input.text, p: p);
          },
        ),
      ],
      actions: [
        TextButton(onPressed: () => _reply(context, '', false), child: Text(t('Cancel'))),
        FilledButton(onPressed: _use, child: Text(t('Use this folder'))),
      ],
    );
  }
}

class _FolderStatus extends StatelessWidget {
  const _FolderStatus({required this.info, required this.path, required this.p});

  final FolderInfo? info;
  final String path;
  final PromptRequest p;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final good = Theme.of(context).brightness == Brightness.dark ? const Color(0xFF7FCFA8) : const Color(0xFF2E8061);
    final rows = <Widget>[];
    void row(IconData icon, String text, {Color? color, Color? iconColor}) => rows.add(
      Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: Icon(icon, size: 18, color: iconColor ?? color ?? scheme.onSurfaceVariant),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(text, style: TextStyle(fontSize: 14, color: color ?? scheme.onSurfaceVariant)),
          ),
        ],
      ),
    );
    final i = info;
    if (path.trim().isEmpty) {
      row(Icons.info, t('Type a folder path or choose Browse.'));
    } else if (i == null) {
      // Waiting for the core's answer.
    } else if (!i.valid) {
      row(
        Icons.error,
        t(r'Enter a full path on an existing drive, like D:\Games\Armored Warfare.'),
        color: scheme.error,
      );
    } else {
      final sameBranch = i.branch.isNotEmpty && i.branch.toLowerCase() == (p.branch ?? '').toLowerCase();
      if (sameBranch) {
        row(Icons.check, t('This folder contains the selected FX ID branch.'), color: good);
      } else if (i.branch.isNotEmpty) {
        row(
          Icons.error,
          t('This folder holds the FX ID {branch} branch. Choose another folder.', {'branch': i.branch}),
          color: scheme.error,
        );
      } else if (i.install) {
        row(
          Icons.check,
          t('Armored Warfare is installed here. The launcher checks it and installs updates.'),
          color: good,
        );
      } else if (i.used) {
        row(
          Icons.info,
          t('The folder is not empty. The client is installed into it next to the existing files.'),
          iconColor: serviceColor(context, true),
        );
      } else {
        row(Icons.add, t('The full client is installed here. The folder is created if it does not exist.'));
      }
      final enough = i.install || sameBranch || i.free >= clientSize;
      final fg = enough ? scheme.onSurface : scheme.onErrorContainer;
      rows.add(
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            color: enough ? scheme.surfaceContainerHighest : scheme.errorContainer,
            borderRadius: BorderRadius.circular(14),
          ),
          child: DefaultTextStyle.merge(
            style: TextStyle(fontSize: 13, color: fg, fontFeatures: const [FontFeature.tabularFigures()]),
            child: Row(
              children: [
                Icon(Icons.storage, size: 18, color: enough ? scheme.onSurfaceVariant : fg),
                const SizedBox(width: 10),
                Text(t('Drive {drive}', {'drive': i.drive}), style: const TextStyle().weight(600)),
                const SizedBox(width: 10),
                Expanded(
                  child: Text(
                    t('{free} free', {'free': formatBytes(i.free)}) +
                        (i.install || sameBranch
                            ? ''
                            : t(' · the client needs about {size}', {'size': formatBytes(clientSize)})),
                    textAlign: TextAlign.right,
                  ),
                ),
              ],
            ),
          ),
        ),
      );
      if (!enough) {
        row(Icons.error, t('There is not enough free space on this drive for a new install.'), color: scheme.error);
      }
    }
    return ConstrainedBox(
      constraints: const BoxConstraints(minHeight: 64),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var k = 0; k < rows.length; k++) ...[if (k > 0) const SizedBox(height: 10), rows[k]],
        ],
      ),
    );
  }
}
