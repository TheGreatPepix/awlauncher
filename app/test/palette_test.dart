import 'package:awlauncher/state/models.dart';
import 'package:awlauncher/theme/palette.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('the seed is the hue of the old hue slider', () {
    final red = oklch(0.6, 0.14, 25), blue = oklch(0.6, 0.14, 250), lavender = oklch(0.6, 0.14, 300);
    expect(red.r, greaterThan(red.b));
    expect(blue.b, greaterThan(blue.r));
    expect(lavender.b, greaterThan(lavender.g));
    expect(lavender.r, greaterThan(lavender.g));
  });

  test('every style gives its own scheme in both brightnesses', () {
    final schemes = <int>{};
    for (final style in PaletteStyle.values) {
      for (final b in Brightness.values) {
        final s = Palette(300, style).scheme(b);
        expect(s.brightness, b);
        schemes.add(Object.hash(s.primary, s.secondaryContainer, s.tertiaryContainer, s.surfaceContainer).hashCode);
      }
    }
    expect(schemes.length, PaletteStyle.values.length * 2);
  });

  test('the primary color keeps the chosen hue in every style but fruit salad', () {
    double hueOf(Color c) => HSVColor.fromColor(c).hue;
    final tonal = hueOf(const Palette(280, PaletteStyle.tonal).scheme(Brightness.light).primary);
    final expressive = hueOf(const Palette(280, PaletteStyle.expressive).scheme(Brightness.light).primary);
    expect((tonal - expressive).abs(), lessThan(30));
  });

  test('ui.json preferences read and write back', () {
    final prefs = Prefs.fromJson({
      'theme': 'dark',
      'hue': 25,
      'style': 'vibrant',
      'palettes': [
        {'hue': 120, 'style': 'mono'},
        {'hue': 999, 'style': 'unknown'},
      ],
      'lang': 'ru',
    });
    expect(prefs.theme, ThemeChoice.dark);
    expect(prefs.palette, const Palette(25, PaletteStyle.vibrant));
    expect(prefs.palettes, [const Palette(120, PaletteStyle.mono), const Palette(defaultHue, PaletteStyle.tonal)]);
    expect(prefs.toCommand(), {
      'cmd': 'prefs',
      'theme': 'dark',
      'hue': 25,
      'style': 'vibrant',
      'palettes': [
        {'hue': 120, 'style': 'mono'},
        {'hue': 300, 'style': 'tonal'},
      ],
      'lang': 'ru',
    });
    expect(presetName(const Palette(25, PaletteStyle.vibrant)), 'Coral');
    expect(Prefs.fromJson({}).lang, 'auto');
  });
}
