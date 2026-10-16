import 'dart:math' as math;

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';

import '../i18n/strings.dart';
import 'widgets.dart';

/// Ping-pong against the launcher, found by clicking the status ten times.
Future<void> showPong(BuildContext context) {
  return showAw<void>(
    context,
    (context) => AwDialog(
      title: t('Ping-pong'),
      body: const [_Pong()],
      actions: [FilledButton(autofocus: true, onPressed: () => closeDialog(context), child: Text(t('Close')))],
    ),
  );
}

const _w = 480.0, _h = 300.0, _padW = 10.0, _padH = 64.0, _padX = 18.0, _ball = 7.0;

class _Pong extends StatefulWidget {
  const _Pong();

  @override
  State<_Pong> createState() => _PongState();
}

class _PongState extends State<_Pong> with SingleTickerProviderStateMixin {
  late final Ticker _ticker;
  final _field = GlobalKey();
  final _random = math.Random();
  var _player = _h / 2 - _padH / 2, _cpu = _h / 2 - _padH / 2;
  final _score = [0, 0];
  var _wait = 0.8, _x = 0.0, _y = 0.0, _vx = 0.0, _vy = 0.0;
  Duration _last = Duration.zero;

  @override
  void initState() {
    super.initState();
    _serve(_random.nextBool() ? -1 : 1);
    _ticker = createTicker(_tick)..start();
    // The paddle follows the pointer anywhere in the window, as the WebView
    // page's did, not only over the field.
    GestureBinding.instance.pointerRouter.addGlobalRoute(_follow);
  }

  @override
  void dispose() {
    GestureBinding.instance.pointerRouter.removeGlobalRoute(_follow);
    _ticker.dispose();
    super.dispose();
  }

  void _follow(PointerEvent e) {
    if (e is! PointerHoverEvent && e is! PointerMoveEvent && e is! PointerDownEvent) return;
    final box = _field.currentContext?.findRenderObject() as RenderBox?;
    if (box == null || !box.hasSize || box.size.height == 0) return;
    final y = box.globalToLocal(e.position).dy * _h / box.size.height - _padH / 2;
    _player = y.clamp(0, _h - _padH);
  }

  void _serve(int dir) {
    final angle = (_random.nextDouble() - 0.5) * math.pi / 3;
    const speed = 300.0;
    _x = _w / 2;
    _y = _h / 2;
    _vx = math.cos(angle) * speed * dir;
    _vy = math.sin(angle) * speed;
    _wait = 0.8;
  }

  void _bounce(double paddle, int dir) {
    final hit = ((_y - (paddle + _padH / 2)) / (_padH / 2)).clamp(-1.0, 1.0);
    final speed = math.min(math.sqrt(_vx * _vx + _vy * _vy) * 1.06, 720.0);
    final angle = hit * math.pi / 3;
    _vx = math.cos(angle) * speed * dir;
    _vy = math.sin(angle) * speed;
  }

  void _tick(Duration elapsed) {
    final dt = _last == Duration.zero ? 0.0 : math.min((elapsed - _last).inMicroseconds / 1e6, 0.05);
    _last = elapsed;
    final target = (_vx > 0 ? _y : _h / 2) - _padH / 2;
    final reach = 250 * dt;
    _cpu = (_cpu + (target - _cpu).clamp(-reach, reach)).clamp(0, _h - _padH);
    if (_wait > 0) {
      _wait -= dt;
    } else {
      _x += _vx * dt;
      _y += _vy * dt;
      if (_y < _ball) {
        _y = _ball;
        _vy = _vy.abs();
      }
      if (_y > _h - _ball) {
        _y = _h - _ball;
        _vy = -_vy.abs();
      }
      if (_vx < 0 &&
          _x - _ball <= _padX + _padW &&
          _x > _padX &&
          _y > _player - _ball &&
          _y < _player + _padH + _ball) {
        _x = _padX + _padW + _ball;
        _bounce(_player, 1);
      }
      if (_vx > 0 &&
          _x + _ball >= _w - _padX - _padW &&
          _x < _w - _padX &&
          _y > _cpu - _ball &&
          _y < _cpu + _padH + _ball) {
        _x = _w - _padX - _padW - _ball;
        _bounce(_cpu, -1);
      }
      if (_x < -_ball) {
        _score[1]++;
        _serve(1);
      }
      if (_x > _w + _ball) {
        _score[0]++;
        _serve(-1);
      }
    }
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return LayoutBuilder(
      builder: (context, box) {
        final scale = box.maxWidth / _w;
        return MouseRegion(
          cursor: SystemMouseCursors.none,
          child: Container(
            key: _field,
            width: box.maxWidth,
            height: _h * scale,
            decoration: BoxDecoration(color: scheme.surfaceContainerHighest, borderRadius: BorderRadius.circular(16)),
            child: CustomPaint(
              painter: _PongPainter(
                scale: scale,
                player: _player,
                cpu: _cpu,
                x: _x,
                y: _y,
                score: '${_score[0]}   ${_score[1]}',
                playerColor: scheme.primary,
                cpuColor: scheme.onSurfaceVariant,
                ballColor: scheme.onSurface,
                lineColor: scheme.outlineVariant,
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PongPainter extends CustomPainter {
  _PongPainter({
    required this.scale,
    required this.player,
    required this.cpu,
    required this.x,
    required this.y,
    required this.score,
    required this.playerColor,
    required this.cpuColor,
    required this.ballColor,
    required this.lineColor,
  });

  final double scale, player, cpu, x, y;
  final String score;
  final Color playerColor, cpuColor, ballColor, lineColor;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(scale);
    final line = Paint()
      ..color = lineColor
      ..strokeWidth = 2;
    for (var d = 12.0; d < _h - 12; d += 18) {
      canvas.drawLine(Offset(_w / 2, d), Offset(_w / 2, math.min(d + 8, _h - 12)), line);
    }
    final text = TextPainter(
      text: TextSpan(
        text: score,
        style: TextStyle(fontSize: 28, color: cpuColor).weight(600),
      ),
      textDirection: TextDirection.ltr,
    )..layout();
    text.paint(canvas, Offset(_w / 2 - text.width / 2, 42 - text.height * 0.8));
    void paddle(double px, double py, Color c) => canvas.drawRRect(
      RRect.fromRectAndRadius(Rect.fromLTWH(px, py, _padW, _padH), const Radius.circular(_padW / 2)),
      Paint()..color = c,
    );
    paddle(_padX, player, playerColor);
    paddle(_w - _padX - _padW, cpu, cpuColor);
    canvas.drawCircle(Offset(x, y), _ball, Paint()..color = ballColor);
  }

  @override
  bool shouldRepaint(_PongPainter old) => true;
}
