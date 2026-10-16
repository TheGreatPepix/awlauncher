import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import '../state/models.dart';
import 'format.dart';
import 'widgets.dart';

/// The line under the Play button: a wavy progress line, what runs, how
/// far it is, and the files it works on.
class ProgressBlock extends StatelessWidget {
  const ProgressBlock({super.key, required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final p = c.progress;
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    var title = t('Ready');
    var meta = '';
    if (p.active) {
      title = tb(p.title);
      final frac = p.total > 0 ? math.min(1, p.done / p.total) : 0;
      final parts = [
        '${(frac * 100).toStringAsFixed(1)}%',
        p.files ? t('{done} / {total} files', {'done': p.done, 'total': p.total}) : formatPair(p.done, p.total),
      ];
      if (p.paused && p.pausable) {
        parts.add(t('paused'));
      } else if (p.speed > 0) {
        parts.add('${formatBytes(p.speed)}/s');
        if (p.total > p.done) parts.add(t('{time} left', {'time': formatDuration((p.total - p.done) / p.speed)}));
      }
      meta = parts.join(' · ');
    } else if (c.busy) {
      title = c.opsTitle;
    }
    if (!p.active && p.paused && c.busy) meta = t('paused');
    final pausable = p.paused ? c.busy : p.active && p.pausable;
    final tasks = p.active ? p.tasks.take(4).toList() : const <TaskProgress>[];
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 0, 4, 22),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          WaveLine(progress: p, busy: c.busy),
          const SizedBox(height: 10),
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Flexible(
                child: Text.rich(
                  TextSpan(
                    children: [
                      TextSpan(text: title, style: text.bodyMedium?.copyWith(fontSize: 14).weight(500)),
                      if (meta.isNotEmpty)
                        TextSpan(
                          text: '   $meta',
                          style: text.bodyMedium?.copyWith(
                            fontSize: 14,
                            color: scheme.onSurfaceVariant,
                            fontFeatures: const [FontFeature.tabularFigures()],
                          ),
                        ),
                    ],
                  ),
                  key: const Key('progress'),
                ),
              ),
              const Spacer(),
              if (pausable)
                SizedBox(
                  height: 32,
                  child: Tooltip(
                    message: t(
                      p.paused
                          ? 'Go on from where it stopped'
                          : 'Pause the download or check; it goes on from the same place',
                    ),
                    child: TextButton.icon(
                      style: TextButton.styleFrom(
                        minimumSize: const Size(0, 32),
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                      ),
                      onPressed: () => c.send({'cmd': p.paused ? 'resume' : 'pause'}),
                      icon: Icon(p.paused ? Icons.play_arrow_rounded : Icons.pause_rounded, size: 18),
                      label: Text(t(p.paused ? 'Resume' : 'Pause')),
                    ),
                  ),
                ),
            ],
          ),
          if (tasks.isNotEmpty) ...[
            const SizedBox(height: 10),
            for (final task in tasks)
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: _TaskRow(key: ValueKey(task.label), task: task, paused: p.paused),
              ),
          ],
        ],
      ),
    );
  }
}

class _TaskRow extends StatelessWidget {
  const _TaskRow({super.key, required this.task, required this.paused});

  final TaskProgress task;
  final bool paused;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final style = TextStyle(
      fontSize: 12.5,
      color: scheme.onSurfaceVariant,
      fontFeatures: const [FontFeature.tabularFigures()],
    );
    final f = task.total > 0 ? math.min(1.0, task.done / task.total) : 0.0;
    final narrow = MediaQuery.sizeOf(context).width <= 900;
    return Row(
      children: [
        Expanded(
          child: Tooltip(
            message: task.label,
            child: Text(clipLeft(task.label, 64), maxLines: 1, overflow: TextOverflow.ellipsis, style: style),
          ),
        ),
        const SizedBox(width: 12),
        SizedBox(
          width: narrow ? 70 : 110,
          height: 10,
          child: TaskBar(fraction: f, paused: paused),
        ),
        const SizedBox(width: 12),
        ConstrainedBox(
          constraints: const BoxConstraints(minWidth: 150),
          child: Text(
            task.total > 0 ? '${(f * 100).toStringAsFixed(0)}% · ${formatPair(task.done, task.total)}' : '',
            textAlign: TextAlign.right,
            style: style,
          ),
        ),
      ],
    );
  }
}

/// A file's small progress bar: a running wave over a thin track.
class TaskBar extends StatefulWidget {
  const TaskBar({super.key, required this.fraction, required this.paused});

  final double fraction;
  final bool paused;

  @override
  State<TaskBar> createState() => _TaskBarState();
}

class _TaskBarState extends State<TaskBar> with TickerProviderStateMixin {
  late final _wave = AnimationController(vsync: this, duration: const Duration(seconds: 1));
  late final _fill = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 260),
    value: widget.fraction,
  );

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _sync();
  }

  @override
  void didUpdateWidget(TaskBar old) {
    super.didUpdateWidget(old);
    if (widget.fraction < _fill.value) {
      _fill.value = widget.fraction;
    } else if (widget.fraction != old.fraction) {
      _fill.animateTo(widget.fraction, curve: Curves.linear);
    }
    _sync();
  }

  void _sync() {
    final run = !widget.paused && motionOn(context);
    if (run && !_wave.isAnimating) {
      _wave.repeat();
    } else if (!run) {
      _wave.stop();
    }
  }

  @override
  void dispose() {
    _wave.dispose();
    _fill.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return AnimatedBuilder(
      animation: Listenable.merge([_wave, _fill]),
      builder: (context, _) =>
          CustomPaint(painter: _TaskBarPainter(_fill.value, _wave.value, scheme.primary, scheme.secondaryContainer)),
    );
  }
}

class _TaskBarPainter extends CustomPainter {
  _TaskBarPainter(this.fraction, this.phase, this.active, this.track);

  final double fraction, phase;
  final Color active, track;

  @override
  void paint(Canvas canvas, Size size) {
    final x = fraction * size.width;
    if (x + 6 < size.width) {
      canvas.drawRRect(RRect.fromLTRBR(x + 6, 3, size.width, 7, const Radius.circular(2)), Paint()..color = track);
    }
    if (x <= 0) return;
    // A wave of 20 px, drawn with a round pen and moved one wavelength a second.
    final path = Path();
    final shift = phase * 20;
    for (double px = 0; px <= x; px += 1) {
      final y = 5 - 1.75 * math.sin((px - shift) / 20 * math.pi * 2);
      px == 0 ? path.moveTo(px, y) : path.lineTo(px, y);
    }
    canvas.save();
    canvas.clipRect(Rect.fromLTWH(0, 0, x, size.height));
    canvas.drawPath(
      path,
      Paint()
        ..color = active
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3
        ..strokeCap = StrokeCap.round,
    );
    canvas.restore();
  }

  @override
  bool shouldRepaint(_TaskBarPainter old) =>
      old.fraction != fraction || old.phase != phase || old.active != active || old.track != track;
}

/// The wavy progress line of Material 3 Expressive: a sine wave that grows
/// while something runs and flattens when it stops.
class WaveLine extends StatefulWidget {
  const WaveLine({super.key, required this.progress, required this.busy});

  final Progress progress;
  final bool busy;

  @override
  State<WaveLine> createState() => _WaveLineState();
}

class _WaveLineState extends State<WaveLine> with SingleTickerProviderStateMixin {
  late final Ticker _ticker = createTicker(_tick);
  var _amp = 0.0, _phase = 0.0;
  Duration _last = Duration.zero;
  var _now = 0.0;

  // The shown fraction eases toward the real one over 260 ms.
  double? _from, _to;
  double _t0 = 0;
  String? _title;

  bool get _moving => (widget.busy || widget.progress.active) && !widget.progress.paused;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _sync();
  }

  @override
  void didUpdateWidget(WaveLine old) {
    super.didUpdateWidget(old);
    _sync();
  }

  void _sync() {
    if (!motionOn(context)) {
      _amp = _moving ? 3.2 : 0;
      if (_ticker.isActive) _ticker.stop();
      return;
    }
    if ((_moving || _amp > 0.05) && !_ticker.isActive) {
      _last = Duration.zero;
      _ticker.start();
    }
  }

  void _tick(Duration elapsed) {
    final dt = _last == Duration.zero ? 0.0 : math.min(64.0, (elapsed - _last).inMicroseconds / 1000);
    _last = elapsed;
    _now += dt;
    _amp += ((_moving ? 3.2 : 0) - _amp) * math.min(1, dt / 180);
    _phase -= dt * 0.006;
    setState(() {});
    if (!_moving && _amp <= 0.05) {
      _amp = 0;
      _ticker.stop();
    }
  }

  double _shown(Progress p) {
    final target = math.min(1.0, p.done / p.total);
    if (_to == null || _title != p.title || target < _to!) {
      _from = _to = target;
      _t0 = _now;
      _title = p.title;
    } else if (target != _to) {
      final shown = _from! + (_to! - _from!) * math.min(1, (_now - _t0) / 260);
      _from = shown;
      _to = target;
      _t0 = _now;
    }
    final k = motionOn(context) ? math.min(1.0, (_now - _t0) / 260) : 1.0;
    return _from! + (_to! - _from!) * k;
  }

  @override
  void dispose() {
    _ticker.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final p = widget.progress;
    return SizedBox(
      height: 18,
      child: CustomPaint(
        size: const Size(double.infinity, 18),
        painter: _WavePainter(
          fraction: p.active && p.total > 0 ? _shown(p) : null,
          indeterminate: !(p.active && p.total > 0) && widget.busy && !p.paused,
          now: _now,
          amp: _amp,
          phase: _phase,
          active: scheme.primary,
          track: scheme.secondaryContainer,
        ),
      ),
    );
  }
}

class _WavePainter extends CustomPainter {
  _WavePainter({
    required this.fraction,
    required this.indeterminate,
    required this.now,
    required this.amp,
    required this.phase,
    required this.active,
    required this.track,
  });

  final double? fraction;
  final bool indeterminate;
  final double now, amp, phase;
  final Color active, track;

  static const _mid = 9.0, _pad = 3.0, _gap = 10.0;

  Path _sine(double x0, double x1) {
    final path = Path();
    if (x1 <= x0) return path;
    double y(double x) => _mid + amp * math.sin(x / 7 + phase);
    path.moveTo(x0, y(x0));
    for (var x = x0 + 2; x < x1; x += 2) {
      path.lineTo(x, y(x));
    }
    return path..lineTo(x1, y(x1));
  }

  @override
  void paint(Canvas canvas, Size size) {
    final w = size.width;
    final pen = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 6
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round;
    final activePen = Paint.from(pen)..color = active;
    final trackPen = Paint.from(pen)..color = track;
    void line(double a, double b) => canvas.drawLine(Offset(a, _mid), Offset(b, _mid), trackPen);
    if (fraction != null) {
      final x1 = _pad + fraction! * (w - 2 * _pad);
      canvas.drawPath(_sine(_pad, x1), activePen);
      if (x1 + _gap < w - _pad) {
        line(x1 + _gap, w - _pad);
        canvas.drawCircle(Offset(w - _pad, _mid), 2.5, Paint()..color = active);
      }
    } else if (indeterminate) {
      final t = (now % 1800) / 1800;
      final a = _pad + math.max(0, t * 1.5 - 0.5) * (w - 2 * _pad);
      final b = _pad + math.min(1, t * 1.5) * (w - 2 * _pad);
      canvas.drawPath(_sine(a, b), activePen);
      if (a - _gap > _pad) line(_pad, a - _gap);
      if (b + _gap < w - _pad) line(b + _gap, w - _pad);
    } else {
      line(_pad, w - _pad);
    }
  }

  @override
  bool shouldRepaint(_WavePainter old) => true;
}
