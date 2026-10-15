import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';

class LogsPage extends StatelessWidget {
  const LogsPage({super.key, required this.c, required this.onNotice});

  final Controller c;
  final void Function(Notice) onNotice;

  Future<void> _copy() async {
    try {
      await Clipboard.setData(ClipboardData(text: c.log));
      onNotice(Notice(t('Logs copied')));
    } on PlatformException {
      onNotice(Notice(t('Could not copy')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(32, 28, 32, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(child: Text(t('Logs'), style: Theme.of(context).textTheme.headlineMedium)),
              TextButton.icon(onPressed: _copy, icon: const Icon(Icons.copy), label: Text(t('Copy'))),
              TextButton.icon(onPressed: c.clearLog, icon: const Icon(Icons.clear_all), label: Text(t('Clear'))),
            ],
          ),
          const SizedBox(height: 20),
          Expanded(
            child: Card.filled(
              color: scheme.surfaceContainerLow,
              child: SingleChildScrollView(
                reverse: true,
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  c.log,
                  style: TextStyle(
                    fontFamily: 'Consolas',
                    fontFamilyFallback: const ['monospace'],
                    fontSize: 13,
                    color: scheme.onSurface,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
