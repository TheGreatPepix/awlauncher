import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../theme/palette.dart';
import 'widgets.dart';

/// The AWLauncher logo: a bin on a tile in the palette's hue. Every 22
/// seconds its lid flips open and the bin hops.
class BrandLogo extends StatefulWidget {
  const BrandLogo({super.key, required this.palette, this.size = 44});

  final Palette palette;
  final double size;

  @override
  State<BrandLogo> createState() => _BrandLogoState();
}

class _BrandLogoState extends State<BrandLogo> with SingleTickerProviderStateMixin {
  // The flip takes 8% of the 22 s cycle, from 84% to 92%; the rest of the
  // time the logo stands still and nothing is drawn.
  late final _c = AnimationController(vsync: this, duration: const Duration(milliseconds: 1760));
  Timer? _first, _every;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (motionOn(context)) {
      _first ??= Timer(const Duration(milliseconds: 18480), () {
        _flip();
        _every = Timer.periodic(const Duration(seconds: 22), (_) => _flip());
      });
    } else {
      _first?.cancel();
      _every?.cancel();
    }
  }

  void _flip() {
    if (mounted) _c.forward(from: 0);
  }

  @override
  void dispose() {
    _first?.cancel();
    _every?.cancel();
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.palette.logo(Theme.of(context).brightness == Brightness.dark);
    return AnimatedBuilder(
      animation: _c,
      builder: (context, _) => CustomPaint(
        size: Size.square(widget.size),
        painter: _LogoPainter(_c.value, colors.top, colors.bottom, colors.slats),
      ),
    );
  }
}

/// A keyframe track: the value at [t] between ease-in-out keyframes.
double _track(double t, List<(double, double)> keys) {
  if (t <= keys.first.$1 || t >= keys.last.$1) return keys.first.$2;
  for (var i = 1; i < keys.length; i++) {
    final (t1, v1) = keys[i];
    if (t <= t1) {
      final (t0, v0) = keys[i - 1];
      return v0 + (v1 - v0) * Curves.easeInOut.transform((t - t0) / (t1 - t0));
    }
  }
  return keys.last.$2;
}

class _LogoPainter extends CustomPainter {
  _LogoPainter(this.t, this.top, this.bottom, this.slats);

  final double t;
  final Color top, bottom, slats;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 64);
    const tile = Rect.fromLTWH(0.5, 0.5, 63, 63);
    canvas.drawRRect(
      RRect.fromRectAndRadius(tile, const Radius.circular(15)),
      Paint()
        ..shader = LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [top, bottom],
        ).createShader(tile),
    );

    final hop = _track(t, const [(0, 0), (0.25, 0), (0.5, -2), (0.75, 0.5), (1, 0)]);
    canvas.save();
    canvas.translate(0, hop);
    final bin = Path()
      ..moveTo(17.5, 25)
      ..lineTo(46.5, 25)
      ..lineTo(43.75, 50.5)
      ..lineTo(20.25, 50.5)
      ..close();
    canvas.drawPath(bin, Paint()..color = Colors.white);
    final slat = Paint()
      ..color = slats
      ..strokeWidth = 3
      ..strokeCap = StrokeCap.round;
    for (final x in [26.0, 32.0, 38.0]) {
      canvas.drawLine(Offset(x, 31.5), Offset(x, 44.25), slat);
    }
    canvas.restore();

    final lid = _track(t, const [(0, 0), (0.25, -14), (0.5, -19), (0.75, 2), (1, 0)]);
    canvas.save();
    canvas.translate(16, 20);
    canvas.rotate(lid * math.pi / 180);
    canvas.translate(-16, -20);
    final pen = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round;
    final handle = Path()
      ..moveTo(26.4, 16.5)
      ..lineTo(25.8, 13.5)
      ..arcToPoint(const Offset(28, 10.2), radius: const Radius.circular(2.8))
      ..lineTo(32.3, 9.3)
      ..arcToPoint(const Offset(35.6, 11.5), radius: const Radius.circular(2.8))
      ..lineTo(36.2, 14.5);
    canvas.drawPath(handle, pen..strokeWidth = 3.4);
    canvas.drawLine(const Offset(16, 20.2), const Offset(46, 14.7), pen..strokeWidth = 5.2);
    canvas.restore();
  }

  @override
  bool shouldRepaint(_LogoPainter old) => old.t != t || old.top != top || old.bottom != bottom || old.slats != slats;
}
