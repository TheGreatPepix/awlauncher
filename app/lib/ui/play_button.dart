import 'dart:math' as math;

import 'package:flutter/material.dart';

import 'widgets.dart';

/// The big Play button. While the selected account's game operation runs it
/// turns into a slowly spinning shape that morphs through seven outlines.
class PlayButton extends StatefulWidget {
  const PlayButton({
    super.key,
    required this.onPressed,
    required this.working,
    required this.paused,
    required this.tooltip,
  });

  final VoidCallback? onPressed;
  final bool working, paused;
  final String tooltip;

  @override
  State<PlayButton> createState() => _PlayButtonState();
}

class _PlayButtonState extends State<PlayButton> with TickerProviderStateMixin {
  var _hover = false, _pressed = false;
  late final _morph = AnimationController(vsync: this, duration: const Duration(milliseconds: 5600));
  late final _spin = AnimationController(vsync: this, duration: const Duration(seconds: 3));

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _sync();
  }

  @override
  void didUpdateWidget(PlayButton old) {
    super.didUpdateWidget(old);
    _sync();
  }

  void _sync() {
    final run = widget.working && !widget.paused && motionOn(context);
    for (final c in [_morph, _spin]) {
      if (run && !c.isAnimating) {
        c.repeat();
      } else if (!run && c.isAnimating) {
        c.stop();
      }
    }
  }

  @override
  void dispose() {
    _morph.dispose();
    _spin.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final enabled = widget.onPressed != null;
    final working = widget.working;
    final Color bg, fg;
    if (working) {
      (bg, fg) = (scheme.primaryContainer, scheme.onPrimaryContainer);
    } else if (enabled) {
      (bg, fg) = (scheme.primary, scheme.onPrimary);
    } else {
      (bg, fg) = (scheme.onSurface.withValues(alpha: 0.12), scheme.onSurface.withValues(alpha: 0.38));
    }
    final active = enabled && !working;
    final radius = working
        ? 68.0
        : active && _pressed
        ? 30.0
        : active && _hover
        ? 52.0
        : 40.0;
    final quick = active && _pressed;
    final overlay = active ? (_pressed ? 0.1 : (_hover ? 0.08 : 0.0)) : 0.0;
    final dark = Theme.of(context).brightness == Brightness.dark;
    final shadow = active
        ? [
            BoxShadow(
              color: Colors.black.withValues(alpha: dark ? 0.4 : 0.14),
              blurRadius: 3,
              offset: const Offset(0, 1),
            ),
            BoxShadow(
              color: Colors.black.withValues(alpha: dark ? 0.3 : 0.08),
              blurRadius: dark ? 12 : 10,
              offset: const Offset(0, 4),
            ),
          ]
        : const <BoxShadow>[];
    return Tooltip(
      message: widget.tooltip,
      child: MouseRegion(
        cursor: enabled ? SystemMouseCursors.click : SystemMouseCursors.basic,
        onEnter: (_) => setState(() => _hover = true),
        onExit: (_) => setState(() => _hover = false),
        child: GestureDetector(
          onTapDown: enabled ? (_) => setState(() => _pressed = true) : null,
          onTapUp: enabled ? (_) => setState(() => _pressed = false) : null,
          onTapCancel: () => setState(() => _pressed = false),
          onTap: widget.onPressed,
          child: Semantics(
            button: true,
            enabled: enabled,
            label: widget.tooltip,
            child: AnimatedScale(
              scale: quick ? 0.94 : 1,
              duration: quick ? const Duration(milliseconds: 120) : motion,
              curve: quick ? effectsCurve : spring,
              child: AnimatedContainer(
                key: const Key('play'),
                width: 136,
                height: 136,
                duration: quick ? const Duration(milliseconds: 120) : motion,
                curve: quick ? effectsCurve : spring,
                clipBehavior: Clip.antiAlias,
                decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(radius), boxShadow: shadow),
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    Positioned.fill(
                      child: AnimatedOpacity(
                        opacity: overlay,
                        duration: effects,
                        curve: effectsCurve,
                        child: ColoredBox(color: fg),
                      ),
                    ),
                    AnimatedOpacity(
                      opacity: working ? 0 : 1,
                      duration: effects,
                      curve: effectsCurve,
                      child: AnimatedScale(
                        scale: working ? 0.5 : 1,
                        duration: motion,
                        curve: spring,
                        child: Icon(Icons.play_arrow_rounded, size: 72, color: fg),
                      ),
                    ),
                    AnimatedOpacity(
                      opacity: working ? 1 : 0,
                      duration: effects,
                      curve: effectsCurve,
                      child: AnimatedScale(
                        scale: working ? 1 : 0.5,
                        duration: motion,
                        curve: spring,
                        child: RotationTransition(
                          turns: _spin,
                          child: AnimatedBuilder(
                            animation: _morph,
                            builder: (context, _) =>
                                CustomPaint(size: const Size.square(72), painter: _ShapePainter(_morph.value, fg)),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

const _points = 96;

/// The outlines the working button morphs through, as radii at [_points]
/// angles scaled so the largest is 21 of a 48 box.
final List<List<double>> _shapes = () {
  double Function(double) wave(int k, double a) =>
      (th) => 1 + a * math.cos(k * th);
  double oval(double th) => 0.74 / math.sqrt(math.pow(0.74 * math.cos(th), 2) + math.pow(math.sin(th), 2));
  double pill(double th) =>
      1 / math.pow(math.pow(math.cos(th).abs(), 4) + math.pow(math.sin(th).abs() / 0.62, 4), 0.25);
  final shapes = <double Function(double)>[
    wave(10, 0.07),
    wave(9, 0.11),
    wave(5, 0.13),
    pill,
    wave(8, 0.13),
    wave(4, 0.16),
    oval,
  ];
  return [
    for (final f in shapes)
      () {
        final r = [for (var i = 0; i < _points; i++) f(i / _points * math.pi * 2 - math.pi / 2)];
        final scale = 21 / r.reduce(math.max);
        return [for (final v in r) v * scale];
      }(),
  ];
}();

class _ShapePainter extends CustomPainter {
  _ShapePainter(this.t, this.color);

  /// The place in the 5.6 s cycle, from 0 to 1.
  final double t;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final n = _shapes.length;
    final step = 1 / n;
    final i = (t / step).floor() % n;
    final local = (t - i * step) / step;
    // Each shape springs into the next in the first 45% of its step and
    // then holds, as the CSS keyframes did.
    final k = local < 0.45 ? spring.transform(local / 0.45) : 1.0;
    final a = _shapes[i], b = _shapes[(i + 1) % n];
    final s = size.width / 48;
    final path = Path();
    for (var j = 0; j < _points; j++) {
      final th = j / _points * math.pi * 2 - math.pi / 2;
      final r = a[j] + (b[j] - a[j]) * k;
      final p = Offset((24 + math.cos(th) * r) * s, (24 + math.sin(th) * r) * s);
      j == 0 ? path.moveTo(p.dx, p.dy) : path.lineTo(p.dx, p.dy);
    }
    canvas.drawPath(path..close(), Paint()..color = color);
  }

  @override
  bool shouldRepaint(_ShapePainter old) => old.t != t || old.color != color;
}
