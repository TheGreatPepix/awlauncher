import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:material_color_utilities/material_color_utilities.dart';

/// The styles of the palette editor, named as in ui.json.
enum PaletteStyle {
  tonal('Tonal', DynamicSchemeVariant.tonalSpot),
  vibrant('Vibrant', DynamicSchemeVariant.vibrant),
  expressive('Expressive', DynamicSchemeVariant.expressive),
  fruit('Fruit salad', DynamicSchemeVariant.fruitSalad),
  neutral('Neutral', DynamicSchemeVariant.neutral),
  mono('Monochrome', DynamicSchemeVariant.monochrome);

  const PaletteStyle(this.label, this.variant);

  final String label;
  final DynamicSchemeVariant variant;

  static PaletteStyle parse(Object? name) => values.firstWhere((s) => s.name == name, orElse: () => tonal);
}

@immutable
class Palette {
  const Palette(this.hue, this.style);

  factory Palette.fromJson(Map<String, dynamic> json) =>
      Palette(validHue(json['hue']), PaletteStyle.parse(json['style']));

  final int hue;
  final PaletteStyle style;

  Map<String, Object> toJson() => {'hue': hue, 'style': style.name};

  /// The source color of the scheme: the same oklch(0.6 0.14 hue) the
  /// WebView page drew its hue slider with. Material's expressive scheme
  /// turns its primary color 240° away from the source; the source is turned
  /// back so that the primary color keeps the chosen hue, as it did before.
  Color get seed {
    final color = oklch(0.6, 0.14, hue.toDouble());
    if (style != PaletteStyle.expressive) return color;
    final hct = Hct.fromInt(color.toARGB32());
    return Color(Hct.from((hct.hue + 120) % 360, hct.chroma, hct.tone).toInt());
  }

  ColorScheme scheme(Brightness brightness) {
    final scheme = ColorScheme.fromSeed(seedColor: seed, brightness: brightness, dynamicSchemeVariant: style.variant);
    if (style != PaletteStyle.expressive) return scheme;
    // The turned source would also turn the surfaces; they keep the chosen
    // hue, as they did on the WebView page.
    final n = ColorScheme.fromSeed(
      seedColor: oklch(0.6, 0.14, hue.toDouble()),
      brightness: brightness,
      dynamicSchemeVariant: style.variant,
    );
    return scheme.copyWith(
      surface: n.surface,
      surfaceDim: n.surfaceDim,
      surfaceBright: n.surfaceBright,
      surfaceContainerLowest: n.surfaceContainerLowest,
      surfaceContainerLow: n.surfaceContainerLow,
      surfaceContainer: n.surfaceContainer,
      surfaceContainerHigh: n.surfaceContainerHigh,
      surfaceContainerHighest: n.surfaceContainerHighest,
      onSurface: n.onSurface,
      onSurfaceVariant: n.onSurfaceVariant,
      outline: n.outline,
      outlineVariant: n.outlineVariant,
      inverseSurface: n.inverseSurface,
      onInverseSurface: n.onInverseSurface,
    );
  }

  @override
  bool operator ==(Object other) => other is Palette && other.hue == hue && other.style == style;

  @override
  int get hashCode => Object.hash(hue, style);

  @override
  String toString() => '${style.name}:$hue';
}

const defaultHue = 300;
const maxPalettes = 12;

const presets = <(String, Palette)>[
  ('Lavender', Palette(300, PaletteStyle.tonal)),
  ('Rose', Palette(355, PaletteStyle.tonal)),
  ('Amber', Palette(60, PaletteStyle.tonal)),
  ('Mint', Palette(165, PaletteStyle.tonal)),
  ('Sky', Palette(240, PaletteStyle.tonal)),
  ('Coral', Palette(25, PaletteStyle.vibrant)),
  ('Aurora', Palette(280, PaletteStyle.expressive)),
  ('Graphite', Palette(300, PaletteStyle.mono)),
];

/// The preset's name for [p], or null for a custom palette.
String? presetName(Palette p) {
  for (final (name, preset) in presets) {
    if (preset == p) return name;
  }
  return null;
}

int validHue(Object? h) {
  final v = h is num ? h : num.tryParse('$h');
  return v != null && v.isFinite && v >= 0 && v < 360 ? v.round() : defaultHue;
}

/// Converts an OKLCH color to sRGB, clipping it into the gamut.
Color oklch(double l, double c, double hueDegrees) {
  final h = hueDegrees * math.pi / 180;
  final a = c * math.cos(h), b = c * math.sin(h);
  final l1 = math.pow(l + 0.3963377774 * a + 0.2158037573 * b, 3);
  final m1 = math.pow(l - 0.1055613458 * a - 0.0638541728 * b, 3);
  final s1 = math.pow(l - 0.0894841775 * a - 1.2914855480 * b, 3);
  double gamma(num x) {
    final v = x <= 0.0031308 ? 12.92 * x : 1.055 * math.pow(x, 1 / 2.4) - 0.055;
    return v.clamp(0, 1).toDouble();
  }

  return Color.from(
    alpha: 1,
    red: gamma(4.0767416621 * l1 - 3.3077115913 * m1 + 0.2309699292 * s1),
    green: gamma(-1.2684380046 * l1 + 2.6097574011 * m1 - 0.3413193965 * s1),
    blue: gamma(-0.0041960863 * l1 - 0.7034186147 * m1 + 1.7076147010 * s1),
  );
}

/// How strongly each style colors the brand logo, and how far it turns the
/// hue: the WebView page's --pc and --h for the style.
const _logoTint = <PaletteStyle, (double, int)>{
  PaletteStyle.tonal: (1, 0),
  PaletteStyle.vibrant: (1.45, 0),
  PaletteStyle.expressive: (1.2, 0),
  PaletteStyle.fruit: (1.25, -50),
  PaletteStyle.neutral: (0.4, 0),
  PaletteStyle.mono: (0, 0),
};

extension PaletteColors on Palette {
  DynamicScheme _dynamic(bool dark) {
    final source = Hct.fromInt(seed.toARGB32());
    return switch (style) {
      PaletteStyle.tonal => SchemeTonalSpot(sourceColorHct: source, isDark: dark, contrastLevel: 0),
      PaletteStyle.vibrant => SchemeVibrant(sourceColorHct: source, isDark: dark, contrastLevel: 0),
      PaletteStyle.expressive => SchemeExpressive(sourceColorHct: source, isDark: dark, contrastLevel: 0),
      PaletteStyle.fruit => SchemeFruitSalad(sourceColorHct: source, isDark: dark, contrastLevel: 0),
      PaletteStyle.neutral => SchemeNeutral(sourceColorHct: source, isDark: dark, contrastLevel: 0),
      PaletteStyle.mono => SchemeMonochrome(sourceColorHct: source, isDark: dark, contrastLevel: 0),
    };
  }

  /// The colors of a swatch: the quarters of its circle and the dot of the
  /// chosen one, the same in light and dark themes.
  ({Color primary, Color tertiary, Color secondary, Color dot}) get swatch {
    final s = _dynamic(false);
    Color tone(TonalPalette p, int t) => Color(p.get(t));
    return (
      primary: tone(s.primaryPalette, 55),
      tertiary: tone(s.tertiaryPalette, 72),
      secondary: tone(s.secondaryPalette, 82),
      dot: tone(s.primaryPalette, 35),
    );
  }

  /// The gradient and slats of the brand logo.
  ({Color top, Color bottom, Color slats}) logo(bool dark) {
    final (pc, shift) = _logoTint[style]!;
    final h = (hue + shift - 8 + 360) % 360.0;
    return dark
        ? (top: oklch(0.55, 0.13 * pc, h), bottom: oklch(0.36, 0.13 * pc, h), slats: oklch(0.48, 0.12 * pc, h))
        : (top: oklch(0.63, 0.11 * pc, h), bottom: oklch(0.39, 0.12 * pc, h), slats: oklch(0.48, 0.12 * pc, h));
  }
}
