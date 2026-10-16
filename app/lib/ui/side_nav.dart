import 'dart:async';

import 'package:flutter/material.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import 'brand_logo.dart';
import 'widgets.dart';

/// The drawer on the left: the logo, the pages and the launcher's status.
class SideNav extends StatelessWidget {
  const SideNav({super.key, required this.c, required this.width, required this.onToast, required this.onPong});

  final Controller c;
  final double width;
  final ValueChanged<String> onToast;
  final VoidCallback onPong;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final pages = [
      (AppPage.home, Icons.home, t('Play'), false),
      (AppPage.game, Icons.sports_esports, t('Game'), false),
      (AppPage.activity, Icons.list, t('Logs'), c.logUnseen),
      (AppPage.settings, Icons.settings, t('Settings'), c.updateUnseen),
    ];
    return Container(
      width: width,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 16),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        border: Border(right: BorderSide(color: scheme.outlineVariant)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _Brand(c: c, onToast: onToast),
          const SizedBox(height: 16),
          Container(
            padding: const EdgeInsets.only(top: 14),
            decoration: BoxDecoration(
              border: Border(top: BorderSide(color: scheme.outlineVariant)),
            ),
            child: Column(
              children: [
                for (final (page, icon, label, dot) in pages) ...[
                  _NavItem(icon: icon, label: label, dot: dot, selected: c.page == page, onTap: () => c.showPage(page)),
                  const SizedBox(height: 2),
                ],
              ],
            ),
          ),
          const Spacer(),
          _Status(c: c, onPong: onPong),
        ],
      ),
    );
  }
}

class _NavItem extends StatefulWidget {
  const _NavItem({
    required this.icon,
    required this.label,
    required this.dot,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool dot, selected;
  final VoidCallback onTap;

  @override
  State<_NavItem> createState() => _NavItemState();
}

class _NavItemState extends State<_NavItem> {
  var _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final on = widget.selected;
    final fg = on ? scheme.onSecondaryContainer : scheme.onSurfaceVariant;
    final radius = BorderRadius.circular(_pressed ? 16 : 26);
    return AnimatedContainer(
      duration: motionFast,
      curve: spring,
      height: 52,
      decoration: BoxDecoration(color: on ? scheme.secondaryContainer : Colors.transparent, borderRadius: radius),
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          borderRadius: radius,
          onTap: widget.onTap,
          onHighlightChanged: (v) => setState(() => _pressed = v),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 18),
            child: Row(
              children: [
                Stack(
                  clipBehavior: Clip.none,
                  children: [
                    Icon(widget.icon, color: fg),
                    if (widget.dot)
                      Positioned(
                        left: 18,
                        top: -3,
                        child: Container(
                          width: 8,
                          height: 8,
                          decoration: BoxDecoration(color: scheme.error, shape: BoxShape.circle),
                        ),
                      ),
                  ],
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Text(
                    widget.label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 14, color: fg).weight(on ? 600 : 500),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Counts quick clicks: [onCount] fires on the [n]th click in a row with no
/// pause over two seconds.
class _Clicks {
  _Clicks(this.n, this.onCount);

  final int n;
  final VoidCallback onCount;
  var _count = 0;
  DateTime _last = DateTime(0);

  void click() {
    final now = DateTime.now();
    _count = now.difference(_last) > const Duration(seconds: 2) ? 1 : _count + 1;
    _last = now;
    if (_count >= n) {
      _count = 0;
      onCount();
    }
  }
}

/// The logo and the name. Fifteen quick clicks on the logo knock it off the
/// drawer; it comes back ten seconds later.
class _Brand extends StatefulWidget {
  const _Brand({required this.c, required this.onToast});

  final Controller c;
  final ValueChanged<String> onToast;

  @override
  State<_Brand> createState() => _BrandState();
}

class _BrandState extends State<_Brand> with TickerProviderStateMixin {
  late final _fall = AnimationController(vsync: this, duration: const Duration(milliseconds: 900));
  late final _back = AnimationController(vsync: this, duration: const Duration(seconds: 2), value: 1);
  late final _clicks = _Clicks(15, _drop);
  Timer? _timer;
  var _busy = false;

  @override
  void dispose() {
    _timer?.cancel();
    _fall.dispose();
    _back.dispose();
    super.dispose();
  }

  Future<void> _drop() async {
    if (_busy) return;
    _busy = true;
    widget.onToast('${t('Stop fooling around')} (╯°□°)╯︵ ┻━┻');
    await _fall.forward(from: 0);
    if (!mounted) return;
    _back.value = 0;
    _fall.value = 0;
    _timer = Timer(const Duration(seconds: 10), () async {
      if (!mounted) return;
      await _back.forward(from: 0);
      _busy = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final screen = MediaQuery.sizeOf(context).height;
    const curve = Cubic(0.5, 0, 0.9, 0.4);
    return AnimatedBuilder(
      animation: Listenable.merge([_fall, _back]),
      builder: (context, child) {
        final f = _fall.value;
        double dy, turn;
        if (f < 0.18) {
          final k = f / 0.18;
          dy = -8 * k;
          turn = -6 * k;
        } else {
          final k = curve.transform((f - 0.18) / 0.82);
          dy = -8 + (screen * 1.1 + 8) * k;
          turn = -6 + 46 * k;
        }
        return Opacity(
          opacity: _back.value,
          child: Transform.translate(
            offset: Offset(0, dy),
            child: Transform.rotate(angle: turn * 3.14159265 / 180, child: child),
          ),
        );
      },
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Row(
          children: [
            GestureDetector(
              onTap: _clicks.click,
              child: BrandLogo(palette: widget.c.preview ?? widget.c.prefs.palette),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'AWLauncher',
                    style: TextStyle(fontSize: 16, height: 22 / 16, color: scheme.onSurface).weight(600),
                  ),
                  Text('Armored Warfare', style: TextStyle(fontSize: 12, color: scheme.onSurfaceVariant)),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// What the launcher is doing, under the pages. Ten quick clicks open a game.
class _Status extends StatefulWidget {
  const _Status({required this.c, required this.onPong});

  final Controller c;
  final VoidCallback onPong;

  @override
  State<_Status> createState() => _StatusState();
}

class _StatusState extends State<_Status> {
  late final _clicks = _Clicks(10, widget.onPong);

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final c = widget.c;
    final dark = Theme.of(context).brightness == Brightness.dark;
    final good = dark ? const Color(0xFF7FCFA8) : const Color(0xFF2E8061);
    final dot = c.busy
        ? scheme.primary
        : c.running
        ? serviceColor(context, true)
        : good;
    return GestureDetector(
      onTap: _clicks.click,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(color: scheme.surfaceContainer, borderRadius: BorderRadius.circular(16)),
        child: Row(
          children: [
            Pulse(color: dot, active: c.busy && !c.paused),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                c.status,
                key: const Key('status'),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 12.5, height: 16 / 12.5, color: scheme.onSurfaceVariant),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
