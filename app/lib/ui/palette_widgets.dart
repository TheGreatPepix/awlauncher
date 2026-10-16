import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';
import '../theme/palette.dart';
import 'pages/settings_page.dart';
import 'widgets.dart';

/// The swatches of the presets and the saved palettes, and the button that
/// opens the palette editor.
class Palettes extends StatelessWidget {
  const Palettes({super.key, required this.c});

  final Controller c;

  void _remove(Palette p) {
    final prefs = c.prefs;
    c.setPrefs(
      prefs.copyWith(
        palettes: prefs.palettes.where((x) => x != p).toList(),
        palette: prefs.palette == p ? presets.first.$2 : prefs.palette,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final prefs = c.prefs;
    final shown = c.preview ?? prefs.palette;
    return Wrap(
      alignment: WrapAlignment.end,
      spacing: 10,
      runSpacing: 10,
      children: [
        for (final (_, p) in presets)
          Swatch(
            p: p,
            tooltip: paletteName(p),
            selected: p == shown,
            onTap: () => c.setPrefs(prefs.copyWith(palette: p)),
          ),
        for (final p in prefs.palettes)
          _Custom(
            onRemove: () => _remove(p),
            child: Swatch(
              p: p,
              tooltip: paletteName(p),
              selected: p == shown,
              onTap: () => c.setPrefs(prefs.copyWith(palette: p)),
            ),
          ),
        if (prefs.palettes.length < maxPalettes) _AddPalette(onTap: () => showPaletteEditor(context, c)),
      ],
    );
  }
}

/// A round swatch of a palette: its primary, tertiary and secondary colors
/// in quarters, with a dot and a check when it is the chosen one.
class Swatch extends StatefulWidget {
  const Swatch({super.key, required this.p, this.tooltip, this.selected = false, this.onTap, this.size = 44});

  final Palette p;
  final String? tooltip;
  final bool selected;
  final VoidCallback? onTap;
  final double size;

  @override
  State<Swatch> createState() => _SwatchState();
}

class _SwatchState extends State<Swatch> {
  var _hover = false, _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final c = widget.p.swatch;
    final interactive = widget.onTap != null;
    final scale = !interactive
        ? 1.0
        : _pressed
        ? 0.9
        : _hover
        ? 1.06
        : 1.0;
    final size = widget.size;
    final radius = interactive && _pressed ? size * 0.3 : size / 2;
    Widget swatch = AnimatedScale(
      scale: scale,
      duration: motionFast,
      curve: spring,
      child: AnimatedContainer(
        duration: motionFast,
        curve: spring,
        width: size,
        height: size,
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(radius),
          gradient: SweepGradient(
            startAngle: -math.pi / 2,
            endAngle: math.pi * 3 / 2,
            colors: [c.primary, c.primary, c.tertiary, c.tertiary, c.secondary, c.secondary, c.primary, c.primary],
            stops: const [0, 0.25, 0.25, 0.5, 0.5, 0.75, 0.75, 1],
            transform: const GradientRotation(-math.pi / 2),
          ),
          boxShadow: widget.selected
              ? [
                  BoxShadow(color: scheme.primary, spreadRadius: 5),
                  BoxShadow(color: scheme.surfaceContainerLow, spreadRadius: 3),
                ]
              : const [],
        ),
        alignment: Alignment.center,
        child: AnimatedScale(
          scale: widget.selected ? 1 : 0,
          duration: motionFast,
          curve: spring,
          child: Container(
            width: size / 2,
            height: size / 2,
            decoration: BoxDecoration(color: c.dot, shape: BoxShape.circle),
            child: Icon(Icons.check, size: size * 16 / 44, color: Colors.white),
          ),
        ),
      ),
    );
    if (!interactive) return swatch;
    swatch = MouseRegion(
      cursor: SystemMouseCursors.click,
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: GestureDetector(
        onTapDown: (_) => setState(() => _pressed = true),
        onTapUp: (_) => setState(() => _pressed = false),
        onTapCancel: () => setState(() => _pressed = false),
        onTap: widget.onTap,
        child: Semantics(button: true, selected: widget.selected, label: widget.tooltip, child: swatch),
      ),
    );
    return widget.tooltip == null ? swatch : Tooltip(message: widget.tooltip, child: swatch);
  }
}

class _Custom extends StatefulWidget {
  const _Custom({required this.child, required this.onRemove});

  final Widget child;
  final VoidCallback onRemove;

  @override
  State<_Custom> createState() => _CustomState();
}

class _CustomState extends State<_Custom> {
  var _hover = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return MouseRegion(
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          widget.child,
          Positioned(
            top: -4,
            right: -4,
            child: IgnorePointer(
              ignoring: !_hover,
              child: AnimatedOpacity(
                opacity: _hover ? 1 : 0,
                duration: effects,
                child: AnimatedScale(
                  scale: _hover ? 1 : 0.6,
                  duration: motionFast,
                  curve: spring,
                  child: Tooltip(
                    message: t('Remove palette'),
                    child: GestureDetector(
                      onTap: widget.onRemove,
                      child: Container(
                        width: 20,
                        height: 20,
                        decoration: BoxDecoration(color: scheme.inverseSurface, shape: BoxShape.circle),
                        child: Icon(Icons.close, size: 14, color: scheme.onInverseSurface),
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

class _AddPalette extends StatefulWidget {
  const _AddPalette({required this.onTap});

  final VoidCallback onTap;

  @override
  State<_AddPalette> createState() => _AddPaletteState();
}

class _AddPaletteState extends State<_AddPalette> {
  var _hover = false, _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final color = _hover ? scheme.primary : scheme.outline;
    return Tooltip(
      message: t('Create a palette'),
      child: MouseRegion(
        cursor: SystemMouseCursors.click,
        onEnter: (_) => setState(() => _hover = true),
        onExit: (_) => setState(() => _hover = false),
        child: GestureDetector(
          onTapDown: (_) => setState(() => _pressed = true),
          onTapUp: (_) => setState(() => _pressed = false),
          onTapCancel: () => setState(() => _pressed = false),
          onTap: widget.onTap,
          child: TweenAnimationBuilder<double>(
            tween: Tween(end: _pressed ? 12 : 22),
            duration: motionFast,
            curve: spring,
            builder: (context, r, _) => CustomPaint(
              painter: _DashedPainter(color, r),
              child: SizedBox(
                width: 44,
                height: 44,
                child: Icon(Icons.add, color: _hover ? scheme.primary : scheme.onSurfaceVariant),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _DashedPainter extends CustomPainter {
  _DashedPainter(this.color, this.radius);

  final Color color;
  final double radius;

  @override
  void paint(Canvas canvas, Size size) {
    final rect = RRect.fromRectAndRadius((Offset.zero & size).deflate(1), Radius.circular(radius));
    final pen = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2;
    for (final m in (Path()..addRRect(rect)).computeMetrics()) {
      for (double d = 0; d < m.length; d += 7) {
        canvas.drawPath(m.extractPath(d, math.min(d + 4, m.length)), pen);
      }
    }
  }

  @override
  bool shouldRepaint(_DashedPainter old) => old.color != color || old.radius != radius;
}

/// The palette editor: a source color and a style, previewed on the whole
/// window until it is saved or cancelled.
Future<void> showPaletteEditor(BuildContext context, Controller c) async {
  final saved = await showAw<Palette>(context, (context) => _PaletteEditor(c: c));
  c.previewPalette(null);
  if (saved == null) return;
  final prefs = c.prefs;
  final palettes = presets.any((p) => p.$2 == saved)
      ? prefs.palettes
      : [...prefs.palettes.where((p) => p != saved), saved];
  c.setPrefs(
    prefs.copyWith(
      palette: saved,
      palettes: palettes.length > maxPalettes ? palettes.sublist(palettes.length - maxPalettes) : palettes,
    ),
  );
}

class _PaletteEditor extends StatefulWidget {
  const _PaletteEditor({required this.c});

  final Controller c;

  @override
  State<_PaletteEditor> createState() => _PaletteEditorState();
}

class _PaletteEditorState extends State<_PaletteEditor> {
  late Palette _draft = widget.c.prefs.palette;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => widget.c.previewPalette(_draft));
  }

  void _set(Palette p) {
    setState(() => _draft = p);
    widget.c.previewPalette(p);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final label = TextStyle(fontSize: 12, letterSpacing: 0.4, color: scheme.onSurfaceVariant).weight(600);
    return AwDialog(
      icon: Icons.palette,
      title: t('New palette'),
      body: [
        Padding(
          padding: const EdgeInsets.only(bottom: 6),
          child: Center(child: Swatch(p: _draft, size: 96)),
        ),
        Transform.translate(
          offset: const Offset(0, 6),
          child: Text(t('Source color'), style: label),
        ),
        HueSlider(hue: _draft.hue, onChanged: (h) => _set(Palette(h, _draft.style))),
        Transform.translate(
          offset: const Offset(0, 6),
          child: Text(t('Style'), style: label),
        ),
        LayoutBuilder(
          builder: (context, box) {
            final w = (box.maxWidth - 16) / 3;
            return Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (final s in PaletteStyle.values)
                  SizedBox(
                    width: w,
                    child: _StyleChoice(
                      p: Palette(_draft.hue, s),
                      selected: s == _draft.style,
                      onTap: () => _set(Palette(_draft.hue, s)),
                    ),
                  ),
              ],
            );
          },
        ),
      ],
      actions: [
        TextButton(onPressed: () => closeDialog(context), child: Text(t('Cancel'))),
        FilledButton(onPressed: () => closeDialog(context, _draft), child: Text(t('Save'))),
      ],
    );
  }
}

class _StyleChoice extends StatefulWidget {
  const _StyleChoice({required this.p, required this.selected, required this.onTap});

  final Palette p;
  final bool selected;
  final VoidCallback onTap;

  @override
  State<_StyleChoice> createState() => _StyleChoiceState();
}

class _StyleChoiceState extends State<_StyleChoice> {
  var _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final on = widget.selected;
    final radius = BorderRadius.circular(
      _pressed
          ? 8
          : on
          ? 20
          : 16,
    );
    return AnimatedContainer(
      duration: motionFast,
      curve: spring,
      decoration: BoxDecoration(
        color: on ? scheme.primaryContainer : scheme.surfaceContainerHighest,
        borderRadius: radius,
      ),
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          borderRadius: radius,
          onTap: widget.onTap,
          onHighlightChanged: (v) => setState(() => _pressed = v),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(6, 12, 6, 10),
            child: Column(
              children: [
                Swatch(p: widget.p, size: 36),
                const SizedBox(height: 6),
                Text(
                  t(widget.p.style.label),
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 12.5,
                    color: on ? scheme.onPrimaryContainer : scheme.onSurface,
                  ).weight(500),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// A slider over the hue circle, drawn in the colors it picks.
class HueSlider extends StatefulWidget {
  const HueSlider({super.key, required this.hue, required this.onChanged});

  final int hue;
  final ValueChanged<int> onChanged;

  @override
  State<HueSlider> createState() => _HueSliderState();
}

class _HueSliderState extends State<HueSlider> {
  var _active = false, _focused = false;

  void _at(double x, double width) {
    final h = ((x - 13) / (width - 26) * 359).round().clamp(0, 359);
    if (h != widget.hue) widget.onChanged(h);
  }

  KeyEventResult _key(FocusNode _, KeyEvent e) {
    if (e is! KeyDownEvent && e is! KeyRepeatEvent) return KeyEventResult.ignored;
    final step = switch (e.logicalKey) {
      LogicalKeyboardKey.arrowLeft || LogicalKeyboardKey.arrowDown => -1,
      LogicalKeyboardKey.arrowRight || LogicalKeyboardKey.arrowUp => 1,
      LogicalKeyboardKey.pageDown => -30,
      LogicalKeyboardKey.pageUp => 30,
      _ => 0,
    };
    if (step == 0) return KeyEventResult.ignored;
    widget.onChanged((widget.hue + step).clamp(0, 359));
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Focus(
      onKeyEvent: _key,
      onFocusChange: (v) => setState(() => _focused = v),
      child: Semantics(
        slider: true,
        label: t('Source color'),
        value: '${widget.hue}',
        child: LayoutBuilder(
          builder: (context, box) {
            final w = box.maxWidth;
            final x = 13 + widget.hue / 359 * (w - 26);
            return MouseRegion(
              cursor: SystemMouseCursors.click,
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTapDown: (d) => _at(d.localPosition.dx, w),
                onHorizontalDragStart: (d) {
                  setState(() => _active = true);
                  _at(d.localPosition.dx, w);
                },
                onHorizontalDragUpdate: (d) => _at(d.localPosition.dx, w),
                onHorizontalDragEnd: (_) => setState(() => _active = false),
                child: SizedBox(
                  height: 32,
                  child: Stack(
                    alignment: Alignment.centerLeft,
                    clipBehavior: Clip.none,
                    children: [
                      Container(
                        height: 16,
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(8),
                          gradient: LinearGradient(colors: [for (var i = 0; i <= 12; i++) oklch(0.6, 0.14, i * 30.0)]),
                        ),
                      ),
                      Positioned(
                        left: x - 13,
                        child: AnimatedScale(
                          scale: _active ? 1.2 : 1,
                          duration: motionFast,
                          curve: spring,
                          child: Container(
                            width: 26,
                            height: 26,
                            decoration: BoxDecoration(
                              color: oklch(0.6, 0.14, widget.hue.toDouble()),
                              shape: BoxShape.circle,
                              border: Border.all(color: Colors.white, width: 3),
                              boxShadow: [
                                if (_focused) BoxShadow(color: scheme.primary, spreadRadius: 3),
                                BoxShadow(
                                  color: Colors.black.withValues(alpha: 0.4),
                                  blurRadius: 4,
                                  offset: const Offset(0, 1),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}
