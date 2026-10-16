import 'dart:math' as math;

import 'package:flutter/material.dart';

extension Weight on TextStyle {
  /// This style at [w]: both the weight and the wght axis of the variable font.
  TextStyle weight(num w) => copyWith(
    fontWeight: FontWeight.values[((w / 100).round() - 1).clamp(0, 8)],
    fontVariations: [FontVariation.weight(w.toDouble())],
  );
}

/// Whether the window may run endless animations; off when the system asks
/// for less motion, and in tests.
bool motionOn(BuildContext context) => !(MediaQuery.maybeDisableAnimationsOf(context) ?? false);

/// The spring of the WebView page's --spring: a CSS linear() easing with a
/// small overshoot, sampled at the same points.
class SpringCurve extends Curve {
  const SpringCurve();

  static const _points = <(double, double)>[
    (0, 0), (0.036, 0.0682), (0.071, 0.2229), (0.107, 0.4073), (0.143, 0.5854), (0.179, 0.7378), //
    (0.214, 0.8567), (0.25, 0.9417), (0.286, 0.9971), (0.321, 1.0288), (0.357, 1.0431), (0.393, 1.0458),
    (0.429, 1.0417), (0.464, 1.0341), (0.5, 1.0256), (0.536, 1.0176), (0.571, 1.0109), (0.607, 1.0057),
    (0.643, 1.0021), (0.679, 0.9998), (0.714, 0.9985), (0.75, 0.998), (0.786, 0.9979), (0.821, 0.9981),
    (0.857, 0.9985), (0.893, 0.9989), (0.929, 0.9993), (0.964, 0.9996), (1, 1),
  ];

  @override
  double transformInternal(double t) {
    for (var i = 1; i < _points.length; i++) {
      final (x1, y1) = _points[i];
      if (t <= x1) {
        final (x0, y0) = _points[i - 1];
        return y0 + (y1 - y0) * (t - x0) / (x1 - x0);
      }
    }
    return 1;
  }
}

const spring = SpringCurve();
const motion = Duration(milliseconds: 500);
const motionFast = Duration(milliseconds: 350);
const effects = Duration(milliseconds: 200);
const effectsCurve = Cubic(0.2, 0, 0, 1);

/// The blue of VK Play and the orange of FX ID, as the WebView page had them.
Color serviceColor(BuildContext context, bool fx) {
  final dark = Theme.of(context).brightness == Brightness.dark;
  return fx
      ? (dark ? const Color(0xFFF0A066) : const Color(0xFFD9692A))
      : (dark ? const Color(0xFF7FA9F0) : const Color(0xFF3E73D6));
}

/// A label over a group of items, in the primary color.
class SectionLabel extends StatelessWidget {
  const SectionLabel(this.text, {super.key, this.first = false});

  final String text;
  final bool first;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(16, first ? 0 : 22, 16, 8),
      child: Text(
        text,
        style: Theme.of(context).textTheme.titleSmall
            ?.copyWith(color: Theme.of(context).colorScheme.primary, fontWeight: FontWeight.w600),
      ),
    );
  }
}

/// A card of list items separated by thin lines.
class ListCard extends StatelessWidget {
  const ListCard({super.key, required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 4),
      decoration: BoxDecoration(color: scheme.surfaceContainerLow, borderRadius: BorderRadius.circular(24)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          for (var i = 0; i < children.length; i++) ...[
            if (i > 0) Divider(height: 1, thickness: 1, color: scheme.outlineVariant),
            children[i],
          ],
        ],
      ),
    );
  }
}

/// A row of a list card: an icon, a title with a line under it, and
/// controls. A wide control moves under the text when the row is narrow.
class ListItem extends StatelessWidget {
  const ListItem({
    super.key,
    this.icon,
    required this.title,
    this.sub,
    this.subStyle,
    this.trailing = const [],
    this.wide,
    this.wideWidth = 300,
  });

  final IconData? icon;
  final Widget title;
  final String? sub;
  final TextStyle? subStyle;
  final List<Widget> trailing;

  /// A control such as a segmented button that takes its own width.
  final Widget? wide;
  final double wideWidth;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final body = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        DefaultTextStyle.merge(style: text.titleMedium?.copyWith(fontSize: 15), child: title),
        if (sub != null && sub!.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: SelectableText(sub!, style: (subStyle ?? text.bodyMedium)?.copyWith(color: scheme.onSurfaceVariant)),
          ),
      ],
    );
    return ConstrainedBox(
      constraints: const BoxConstraints(minHeight: 64),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 12, 16, 12),
        child: LayoutBuilder(
          builder: (context, box) {
            final lead = icon == null
                ? null
                : Padding(
                    padding: const EdgeInsets.only(right: 16),
                    child: Icon(icon, color: scheme.onSurfaceVariant),
                  );
            final narrow = wide != null && box.maxWidth < 600;
            final row = Row(
              children: [
                ?lead,
                Expanded(child: body),
                for (final w in trailing) Padding(padding: const EdgeInsets.only(left: 8), child: w),
                if (wide != null && !narrow)
                  Padding(
                    padding: const EdgeInsets.only(left: 16),
                    child: SizedBox(width: wideWidth, child: wide),
                  ),
              ],
            );
            if (!narrow) return row;
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                row,
                const SizedBox(height: 12),
                Padding(
                  padding: EdgeInsets.only(left: icon == null ? 0 : 40),
                  child: wide,
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

/// The connected button group of the WebView page: the chosen button fills
/// with the primary color and rounds its inner corners.
class Segmented<T> extends StatelessWidget {
  const Segmented({super.key, required this.options, required this.selected, required this.onChanged});

  final List<(T, String)> options;
  final T selected;
  final ValueChanged<T>? onChanged;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        for (var i = 0; i < options.length; i++) ...[
          if (i > 0) const SizedBox(width: 2),
          Expanded(
            child: _Segment(
              label: options[i].$2,
              on: options[i].$1 == selected,
              first: i == 0,
              last: i == options.length - 1,
              scheme: scheme,
              onTap: onChanged == null ? null : () => onChanged!(options[i].$1),
            ),
          ),
        ],
      ],
    );
  }
}

class _Segment extends StatefulWidget {
  const _Segment({
    required this.label,
    required this.on,
    required this.first,
    required this.last,
    required this.scheme,
    required this.onTap,
  });

  final String label;
  final bool on, first, last;
  final ColorScheme scheme;
  final VoidCallback? onTap;

  @override
  State<_Segment> createState() => _SegmentState();
}

class _SegmentState extends State<_Segment> {
  var _pressed = false;

  @override
  Widget build(BuildContext context) {
    const outer = Radius.circular(20);
    final inner = Radius.circular(
      _pressed
          ? (widget.on ? 12 : 4)
          : widget.on
          ? 20
          : 8,
    );
    final radius = BorderRadius.horizontal(left: widget.first ? outer : inner, right: widget.last ? outer : inner);
    final bg = widget.on ? widget.scheme.primary : widget.scheme.secondaryContainer;
    final fg = widget.on ? widget.scheme.onPrimary : widget.scheme.onSecondaryContainer;
    return AnimatedContainer(
      duration: motionFast,
      curve: spring,
      height: 40,
      decoration: BoxDecoration(color: bg, borderRadius: radius),
      child: Material(
        type: MaterialType.transparency,
        child: InkWell(
          borderRadius: radius,
          onTap: widget.onTap,
          onHighlightChanged: (v) => setState(() => _pressed = v),
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: Text(
                widget.label,
                maxLines: 1,
                overflow: TextOverflow.fade,
                softWrap: false,
                style: Theme.of(context).textTheme.labelLarge?.copyWith(color: fg, fontSize: 13),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// A small uppercase label: the service or the branch of an account.
class Tag extends StatelessWidget {
  const Tag(this.text, {super.key, this.color, this.tooltip});

  final String text;

  /// The service color; null for the tertiary color of a branch.
  final Color? color;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final bg = color == null ? scheme.tertiaryContainer : color!.withValues(alpha: 0.2);
    final fg = color == null ? scheme.onTertiaryContainer : Color.lerp(color, scheme.onSurface, 0.22)!;
    final tag = Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 1),
      decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(6)),
      child: Text(
        text.toUpperCase(),
        style: TextStyle(
          fontSize: 11,
          height: 18 / 11,
          fontWeight: FontWeight.w600,
          fontVariations: const [FontVariation.weight(600)],
          letterSpacing: 0.5,
          color: fg,
        ),
      ),
    );
    return tooltip == null ? tag : Tooltip(message: tooltip, child: tag);
  }
}

/// A dot that slowly fades in and out while something runs.
class Pulse extends StatefulWidget {
  const Pulse({super.key, required this.color, this.size = 8, this.active = true});

  final Color color;
  final double size;
  final bool active;

  @override
  State<Pulse> createState() => _PulseState();
}

class _PulseState extends State<Pulse> with SingleTickerProviderStateMixin {
  late final _c = AnimationController(vsync: this, duration: const Duration(milliseconds: 1200));

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _sync();
  }

  @override
  void didUpdateWidget(Pulse old) {
    super.didUpdateWidget(old);
    _sync();
  }

  void _sync() {
    if (widget.active && motionOn(context)) {
      if (!_c.isAnimating) _c.repeat(reverse: true);
    } else {
      _c.value = 1;
    }
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: Tween(begin: 0.35, end: 1.0).animate(CurvedAnimation(parent: _c, curve: Curves.easeInOut)),
      child: Container(
        width: widget.size,
        height: widget.size,
        decoration: BoxDecoration(color: widget.color, shape: BoxShape.circle),
      ),
    );
  }
}

/// The cookie-shaped avatar with the first letter of an account's name.
class CookieAvatar extends StatelessWidget {
  const CookieAvatar({super.key, required this.letter, required this.color});

  final String letter;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 44,
      height: 44,
      child: CustomPaint(
        painter: _CookiePainter(color),
        child: Center(
          child: Text(
            letter,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 17,
              fontWeight: FontWeight.w600,
              fontVariations: [FontVariation.weight(600)],
            ),
          ),
        ),
      ),
    );
  }
}

class _CookiePainter extends CustomPainter {
  _CookiePainter(this.color);

  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawPath(cookiePath(size.width, 9, 0.07), Paint()..color = color);
  }

  @override
  bool shouldRepaint(_CookiePainter old) => old.color != color;
}

/// A circle with [lobes] gentle bumps on its edge.
Path cookiePath(double size, int lobes, double depth) {
  final r = size / 2;
  final path = Path();
  for (var i = 0; i <= 120; i++) {
    final t = i / 120 * math.pi * 2;
    final rr = r * (1 - depth + depth * math.cos(lobes * t));
    final p = Offset(r + rr * math.cos(t), r + rr * math.sin(t));
    i == 0 ? path.moveTo(p.dx, p.dy) : path.lineTo(p.dx, p.dy);
  }
  return path..close();
}

/// Text in a monospace box: what an operation said before it asked.
class ContextBox extends StatelessWidget {
  const ContextBox(this.lines, {super.key});

  final List<String> lines;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: double.infinity,
      constraints: const BoxConstraints(maxHeight: 220),
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(color: scheme.surfaceContainerHighest, borderRadius: BorderRadius.circular(12)),
      child: SingleChildScrollView(
        reverse: true,
        child: SelectableText(lines.join('\n'), style: monoStyle(context, size: 12.5)),
      ),
    );
  }
}

TextStyle monoStyle(BuildContext context, {double size = 12.5}) => TextStyle(
  fontFamily: 'Cascadia Mono',
  fontFamilyFallback: const ['Consolas', 'monospace'],
  fontSize: size,
  height: 19 / 12.5,
  color: Theme.of(context).colorScheme.onSurface,
);

/// One entry of a "more" menu; null entries draw a divider.
class MenuEntry {
  const MenuEntry(this.icon, this.label, this.onTap, {this.danger = false, this.enabled = true});

  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final bool danger, enabled;
}

/// The ⋮ button with its menu.
class MoreMenu extends StatelessWidget {
  const MoreMenu({super.key, required this.tooltip, required this.entries, this.color});

  final String tooltip;
  final List<MenuEntry?> entries;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return MenuAnchor(
      alignmentOffset: const Offset(-180, 4),
      menuChildren: [
        for (final e in entries)
          if (e == null)
            const Divider(height: 9)
          else
            MenuItemButton(
              leadingIcon: Icon(e.icon, color: e.danger ? scheme.error : null),
              onPressed: e.enabled ? e.onTap : null,
              child: Text(e.label, style: e.danger ? TextStyle(color: scheme.error) : null),
            ),
      ],
      builder: (context, controller, _) => IconButton(
        tooltip: tooltip,
        color: color ?? scheme.onSurfaceVariant,
        icon: const Icon(Icons.more_vert),
        onPressed: () => controller.isOpen ? controller.close() : controller.open(),
      ),
    );
  }
}

/// A dialog in the WebView page's shape: an icon, the operation that asks,
/// a title, a body and buttons.
class AwDialog extends StatelessWidget {
  const AwDialog({
    super.key,
    this.icon,
    this.danger = false,
    this.op,
    required this.title,
    this.body = const [],
    required this.actions,
    this.width = 560,
  });

  final IconData? icon;
  final bool danger;
  final String? op;
  final String title;
  final List<Widget> body;
  final List<Widget> actions;
  final double width;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final center = icon != null;
    return Dialog(
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: width),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: center ? CrossAxisAlignment.center : CrossAxisAlignment.start,
            children: [
              if (icon != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 14),
                  child: Icon(icon, color: danger ? scheme.error : scheme.primary),
                ),
              if (op != null && op!.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Text(
                    op!.toUpperCase(),
                    textAlign: center ? TextAlign.center : TextAlign.start,
                    style: text.labelMedium
                        ?.copyWith(fontSize: 12, height: 16 / 12, letterSpacing: 0.5, color: scheme.primary)
                        .weight(600),
                  ),
                ),
              Padding(
                padding: const EdgeInsets.only(bottom: 14),
                child: Text(
                  title,
                  textAlign: center ? TextAlign.center : TextAlign.start,
                  style: text.headlineSmall
                      ?.copyWith(fontSize: 22, height: 30 / 22, color: scheme.onSurface)
                      .weight(500),
                ),
              ),
              if (body.isNotEmpty)
                Flexible(
                  child: SingleChildScrollView(
                    child: DefaultTextStyle.merge(
                      style: text.bodyMedium?.copyWith(fontSize: 14, color: scheme.onSurfaceVariant),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          for (var i = 0; i < body.length; i++) ...[if (i > 0) const SizedBox(height: 14), body[i]],
                        ],
                      ),
                    ),
                  ),
                ),
              const SizedBox(height: 22),
              Align(
                alignment: Alignment.centerRight,
                child: Wrap(spacing: 8, runSpacing: 8, alignment: WrapAlignment.end, children: actions),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Shows a dialog that grows in on the spring, as the WebView page's did.
/// A tap outside or Esc closes it with null.
Future<T?> showAw<T>(BuildContext context, WidgetBuilder builder, {bool dismissible = true}) {
  final dark = Theme.of(context).brightness == Brightness.dark;
  return Navigator.of(context).push(
    _AwRoute<T>(
      builder: builder,
      dismissible: dismissible,
      label: MaterialLocalizations.of(context).modalBarrierDismissLabel,
      scrim: Colors.black.withValues(alpha: dark ? 0.5 : 0.32),
      themes: InheritedTheme.capture(from: context, to: Navigator.of(context).context),
    ),
  );
}

class _AwRoute<T> extends PopupRoute<T> {
  _AwRoute({
    required this.builder,
    required this.dismissible,
    required this.label,
    required this.scrim,
    required this.themes,
  });

  final WidgetBuilder builder;
  final bool dismissible;
  final String label;
  final Color scrim;
  final CapturedThemes themes;

  @override
  bool get barrierDismissible => dismissible;

  @override
  String? get barrierLabel => label;

  @override
  Color? get barrierColor => scrim;

  @override
  Duration get transitionDuration => motion;

  @override
  Duration get reverseTransitionDuration => effects;

  @override
  Widget buildPage(BuildContext context, Animation<double> animation, Animation<double> secondaryAnimation) =>
      themes.wrap(Builder(builder: builder));

  @override
  Widget buildTransitions(
    BuildContext context,
    Animation<double> animation,
    Animation<double> secondaryAnimation,
    Widget child,
  ) {
    final fade = CurvedAnimation(
      parent: animation,
      curve: const Interval(0, 0.4, curve: effectsCurve),
      reverseCurve: effectsCurve,
    );
    final grow = CurvedAnimation(parent: animation, curve: spring, reverseCurve: effectsCurve);
    return FadeTransition(
      opacity: fade,
      child: AnimatedBuilder(
        animation: grow,
        builder: (context, child) => Transform.translate(
          offset: Offset(0, (1 - grow.value) * 18),
          child: Transform.scale(scale: 0.94 + 0.06 * grow.value, child: child),
        ),
        child: child,
      ),
    );
  }
}

/// Closes the dialog over [context] with [result].
void closeDialog<T>(BuildContext context, [T? result]) => Navigator.of(context).pop(result);

/// The text of a dialog's body.
class DialogText extends StatelessWidget {
  const DialogText(this.text, {super.key});

  final String text;

  @override
  Widget build(BuildContext context) => SelectableText(text);
}

/// A small note under a field of a dialog.
class Hint extends StatelessWidget {
  const Hint(this.text, {super.key});

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 0, 16, 0),
    child: Transform.translate(
      offset: const Offset(0, -8),
      child: Text(text, style: const TextStyle(fontSize: 12)),
    ),
  );
}

/// The filled field of a dialog: a label over the text, a line under it.
InputDecoration fieldDecoration(BuildContext context, String label, {Widget? suffix}) {
  final scheme = Theme.of(context).colorScheme;
  return InputDecoration(
    labelText: label,
    filled: true,
    fillColor: scheme.surfaceContainerHighest,
    counterText: '',
    suffixIcon: suffix,
    floatingLabelBehavior: FloatingLabelBehavior.always,
    contentPadding: const EdgeInsets.fromLTRB(16, 22, 16, 8),
    border: const UnderlineInputBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(4))),
  );
}

/// The filled red button of a dialog that removes or stops something.
Widget dangerButton(BuildContext context, String label, VoidCallback onPressed) {
  final scheme = Theme.of(context).colorScheme;
  return FilledButton(
    autofocus: true,
    style: FilledButton.styleFrom(backgroundColor: scheme.error, foregroundColor: scheme.onError),
    onPressed: onPressed,
    child: Text(label),
  );
}

/// Shortens [s] from the left, keeping its end: the file name of a path.
String clipLeft(String s, int n) => s.length <= n ? s : '…${s.substring(s.length - n + 1)}';
