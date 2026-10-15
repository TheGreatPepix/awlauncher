import 'package:flutter/material.dart';

import 'palette.dart';

const fontFamily = 'Roboto Flex';

ThemeData buildTheme(Palette palette, Brightness brightness) {
  final scheme = palette.scheme(brightness);
  final base = ThemeData(
    useMaterial3: true,
    colorScheme: scheme,
    fontFamily: fontFamily,
    visualDensity: VisualDensity.standard,
  );
  return base.copyWith(
    textTheme: _variable(base.textTheme),
    scaffoldBackgroundColor: scheme.surface,
    snackBarTheme: const SnackBarThemeData(behavior: SnackBarBehavior.floating),
    cardTheme: const CardThemeData(margin: EdgeInsets.zero),
  );
}

/// Roboto Flex is a variable font: its weight is the wght axis, not a separate file.
TextTheme _variable(TextTheme theme) {
  TextStyle? weighted(TextStyle? s) {
    if (s == null) return null;
    final w = (s.fontWeight ?? FontWeight.w400).value.toDouble();
    return s.copyWith(fontVariations: [FontVariation.weight(w)]);
  }

  return theme.copyWith(
    displayLarge: weighted(theme.displayLarge),
    displayMedium: weighted(theme.displayMedium),
    displaySmall: weighted(theme.displaySmall),
    headlineLarge: weighted(theme.headlineLarge),
    headlineMedium: weighted(theme.headlineMedium),
    headlineSmall: weighted(theme.headlineSmall),
    titleLarge: weighted(theme.titleLarge),
    titleMedium: weighted(theme.titleMedium),
    titleSmall: weighted(theme.titleSmall),
    bodyLarge: weighted(theme.bodyLarge),
    bodyMedium: weighted(theme.bodyMedium),
    bodySmall: weighted(theme.bodySmall),
    labelLarge: weighted(theme.labelLarge),
    labelMedium: weighted(theme.labelMedium),
    labelSmall: weighted(theme.labelSmall),
  );
}
