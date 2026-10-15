import 'package:flutter/material.dart';

import '../i18n/strings.dart';
import '../state/models.dart';

/// Asks the question of a core operation. The result is the answer's value
/// and whether it was given, or null if the dialog was dismissed.
Future<(String, bool)?> showPrompt(BuildContext context, PromptRequest p) {
  return showDialog<(String, bool)>(context: context, barrierDismissible: false, builder: (_) => _PromptDialog(p));
}

class _PromptDialog extends StatefulWidget {
  const _PromptDialog(this.p);

  final PromptRequest p;

  @override
  State<_PromptDialog> createState() => _PromptDialogState();
}

class _PromptDialogState extends State<_PromptDialog> {
  late final _input = TextEditingController(text: widget.p.suggest ?? widget.p.folder ?? '');
  late String _choice =
      widget.p.options.where((o) => o.current).firstOrNull?.value ?? widget.p.options.firstOrNull?.value ?? '';

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  void _reply(String value, bool ok) => Navigator.of(context).pop((value, ok));

  @override
  Widget build(BuildContext context) {
    final p = widget.p;
    final text = Theme.of(context).textTheme;
    final scheme = Theme.of(context).colorScheme;
    final body = <Widget>[
      if (p.op.isNotEmpty) Text(tb(p.op), style: text.labelMedium?.copyWith(color: scheme.primary)),
      if (p.context.isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 8),
          child: Text(p.context.join('\n'), style: text.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
        ),
    ];
    final List<Widget> actions;
    switch (p.kind) {
      case 'confirm':
        actions = [
          p.isDefault
              ? TextButton(onPressed: () => _reply('no', true), child: Text(t('No')))
              : FilledButton(onPressed: () => _reply('no', true), child: Text(t('No'))),
          p.isDefault
              ? FilledButton(autofocus: true, onPressed: () => _reply('yes', true), child: Text(t('Yes')))
              : TextButton(onPressed: () => _reply('yes', true), child: Text(t('Yes'))),
        ];
      case 'choice':
        body.add(
          RadioGroup<String>(
            groupValue: _choice,
            onChanged: (v) => setState(() => _choice = v ?? _choice),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                for (final o in p.options)
                  RadioListTile<String>(
                    value: o.value,
                    title: Text(o.label),
                    subtitle: o.detail == null ? null : Text(o.detail!),
                    secondary: o.current ? Text(t('current'), style: text.labelSmall) : null,
                  ),
              ],
            ),
          ),
        );
        actions = [
          TextButton(onPressed: () => _reply('', false), child: Text(t('Cancel'))),
          FilledButton(onPressed: () => _reply(_choice, true), child: Text(t('OK'))),
        ];
      default:
        final label = switch (p.kind) {
          'email' => 'E-mail',
          'key' => t('Key'),
          'name' => t('Name'),
          'code' => t('Code'),
          'folder' => t('Folder'),
          _ => t('Answer'),
        };
        body.add(
          Padding(
            padding: const EdgeInsets.only(top: 16),
            child: TextField(
              controller: _input,
              autofocus: true,
              decoration: InputDecoration(labelText: label, border: const OutlineInputBorder()),
              onSubmitted: (v) => _reply(v, true),
            ),
          ),
        );
        actions = [
          TextButton(onPressed: () => _reply('', false), child: Text(t('Cancel'))),
          FilledButton(onPressed: () => _reply(_input.text, true), child: Text(t('OK'))),
        ];
    }
    return AlertDialog(
      title: Text(tb(p.question)),
      content: SizedBox(
        width: 420,
        child: SingleChildScrollView(
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: body),
        ),
      ),
      actions: actions,
    );
  }
}
