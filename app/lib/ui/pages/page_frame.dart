import 'package:flutter/material.dart';

import '../widgets.dart';

/// A page: an optional large title and scrolling content.
class PageFrame extends StatelessWidget {
  const PageFrame({super.key, this.title, this.actions = const [], required this.children});

  final String? title;
  final List<Widget> actions;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(28, 24, 28, 96),
      children: [
        if (title != null) PageHeader(title: title!, actions: actions),
        ...children,
      ],
    );
  }
}

class PageHeader extends StatelessWidget {
  const PageHeader({super.key, required this.title, this.actions = const []});

  final String title;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 4, 0, 18),
      child: SizedBox(
        height: 40,
        child: Row(
          children: [
            Expanded(
              child: Text(
                title,
                style: TextStyle(
                  fontSize: 28,
                  height: 36 / 28,
                  color: Theme.of(context).colorScheme.onSurface,
                ).weight(500),
              ),
            ),
            for (final a in actions) Padding(padding: const EdgeInsets.only(left: 8), child: a),
          ],
        ),
      ),
    );
  }
}

/// Plays the page's entrance once: it rises 12 px and fades in on the spring.
class PageEnter extends StatefulWidget {
  const PageEnter({super.key, required this.child});

  final Widget child;

  @override
  State<PageEnter> createState() => _PageEnterState();
}

class _PageEnterState extends State<PageEnter> with SingleTickerProviderStateMixin {
  late final _c = AnimationController(vsync: this, duration: motion)..forward();

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final rise = CurvedAnimation(parent: _c, curve: spring);
    return FadeTransition(
      opacity: CurvedAnimation(parent: _c, curve: Curves.linear),
      child: AnimatedBuilder(
        animation: rise,
        builder: (context, child) => Transform.translate(offset: Offset(0, (1 - rise.value) * 12), child: child),
        child: widget.child,
      ),
    );
  }
}
