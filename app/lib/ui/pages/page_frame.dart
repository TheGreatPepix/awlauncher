import 'package:flutter/material.dart';

/// A page with a large title and scrolling content.
class PageFrame extends StatelessWidget {
  const PageFrame({super.key, required this.title, this.actions = const [], required this.children});

  final String title;
  final List<Widget> actions;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(32, 28, 32, 32),
      children: [
        Row(
          children: [
            Expanded(child: Text(title, style: Theme.of(context).textTheme.headlineMedium)),
            ...actions,
          ],
        ),
        const SizedBox(height: 20),
        ...children,
      ],
    );
  }
}

/// A titled group of settings or items on a card.
class Section extends StatelessWidget {
  const Section({super.key, this.title, required this.children});

  final String? title;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (title != null)
            Padding(
              padding: const EdgeInsets.fromLTRB(4, 0, 4, 8),
              child: Text(title!, style: Theme.of(context).textTheme.titleSmall?.copyWith(color: scheme.primary)),
            ),
          Card.filled(
            color: scheme.surfaceContainerLow,
            clipBehavior: Clip.antiAlias,
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: children),
          ),
        ],
      ),
    );
  }
}
