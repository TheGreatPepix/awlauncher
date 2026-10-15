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

  ColorScheme scheme(Brightness brightness) =>
      ColorScheme.fromSeed(seedColor: seed, brightness: brightness, dynamicSchemeVariant: style.variant);

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
