import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';
import '../widgets.dart';
import 'page_frame.dart';

final _url = RegExp(r'''https://[^\s"'<>]+''');
final _error = RegExp(r'^(Error|.*\bfailed\b)', caseSensitive: false);

/// The log of the launcher: errors in red, section heads in the primary
/// color, and links that open in the browser. It keeps to the bottom while
/// the person has not scrolled up.
class LogsPage extends StatefulWidget {
  const LogsPage({super.key, required this.c, required this.onNotice});

  final Controller c;
  final void Function(Notice) onNotice;

  @override
  State<LogsPage> createState() => _LogsPageState();
}

class _LogsPageState extends State<LogsPage> {
  final _scroll = ScrollController();
  final _links = <TapGestureRecognizer>[];
  String? _text;
  List<InlineSpan> _spans = const [];
  Brightness? _brightness;
  Color? _primary;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _toBottom());
  }

  @override
  void dispose() {
    _scroll.dispose();
    _clearLinks();
    super.dispose();
  }

  void _clearLinks() {
    for (final r in _links) {
      r.dispose();
    }
    _links.clear();
  }

  void _toBottom() {
    if (_scroll.hasClients) _scroll.jumpTo(_scroll.position.maxScrollExtent);
  }

  @override
  void didUpdateWidget(LogsPage old) {
    super.didUpdateWidget(old);
    final stick = !_scroll.hasClients || _scroll.position.pixels >= _scroll.position.maxScrollExtent - 24;
    if (stick) WidgetsBinding.instance.addPostFrameCallback((_) => _toBottom());
  }

  Future<void> _copy() async {
    try {
      await Clipboard.setData(ClipboardData(text: widget.c.log));
      widget.onNotice(Notice(t('Logs copied')));
    } on PlatformException {
      widget.onNotice(Notice(t('Could not copy')));
    }
  }

  List<InlineSpan> _build(String text, ColorScheme scheme) {
    _clearLinks();
    final spans = <InlineSpan>[];
    for (final line in text.split('\n')) {
      if (line.isEmpty) continue;
      final style = _error.hasMatch(line)
          ? TextStyle(color: scheme.error)
          : line.startsWith('===')
          ? TextStyle(color: scheme.primary).weight(600)
          : null;
      final parts = <InlineSpan>[];
      var at = 0;
      for (final m in _url.allMatches(line)) {
        parts.add(TextSpan(text: line.substring(at, m.start)));
        final url = m[0]!;
        final tap = TapGestureRecognizer()..onTap = () => widget.c.openLink(url);
        _links.add(tap);
        parts.add(
          TextSpan(
            text: url,
            recognizer: tap,
            mouseCursor: SystemMouseCursors.click,
            style: TextStyle(
              color: scheme.primary,
              decoration: TextDecoration.underline,
              decorationColor: scheme.primary,
            ),
          ),
        );
        at = m.end;
      }
      parts.add(TextSpan(text: '${line.substring(at)}\n'));
      spans.add(TextSpan(style: style, children: parts));
    }
    return spans;
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = widget.c.log;
    if (text != _text || scheme.brightness != _brightness || scheme.primary != _primary) {
      _text = text;
      _brightness = scheme.brightness;
      _primary = scheme.primary;
      _spans = _build(text, scheme);
    }
    return Padding(
      padding: const EdgeInsets.fromLTRB(28, 24, 28, 28),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          PageHeader(
            title: t('Logs'),
            actions: [
              TextButton.icon(onPressed: _copy, icon: const Icon(Icons.copy, size: 18), label: Text(t('Copy'))),
              TextButton.icon(
                onPressed: widget.c.clearLog,
                icon: const Icon(Icons.clear_all, size: 18),
                label: Text(t('Clear')),
              ),
            ],
          ),
          Expanded(
            child: Container(
              padding: const EdgeInsets.all(4),
              decoration: BoxDecoration(color: scheme.surfaceContainerLow, borderRadius: BorderRadius.circular(24)),
              child: Container(
                constraints: const BoxConstraints(minHeight: 200),
                decoration: BoxDecoration(
                  color: scheme.surfaceContainerLowest,
                  borderRadius: BorderRadius.circular(20),
                ),
                clipBehavior: Clip.antiAlias,
                child: Scrollbar(
                  controller: _scroll,
                  child: SingleChildScrollView(
                    controller: _scroll,
                    padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
                    child: SelectionArea(
                      child: Text.rich(
                        TextSpan(children: _spans),
                        key: const Key('log'),
                        style: monoStyle(context),
                      ),
                    ),
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
