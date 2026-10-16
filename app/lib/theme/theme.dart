import 'package:flutter/material.dart';

import '../ui/widgets.dart';
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
  final text = _variable(base.textTheme);
  // The buttons of the WebView page: 40 px high, round, and squarer while
  // pressed, with the shape following the spring.
  final button = ButtonStyle(
    minimumSize: const WidgetStatePropertyAll(Size(64, 40)),
    animationDuration: motionFast,
    shape: WidgetStateProperty.resolveWith(
      (s) => RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(
          s.contains(WidgetState.pressed) && !s.contains(WidgetState.disabled) ? 8 : 20,
        ),
      ),
    ),
    textStyle: WidgetStatePropertyAll(text.labelLarge?.copyWith(fontSize: 14)),
  );
  return base.copyWith(
    textTheme: text,
    scaffoldBackgroundColor: scheme.surface,
    cardTheme: const CardThemeData(margin: EdgeInsets.zero),
    filledButtonTheme: FilledButtonThemeData(
      style: button.copyWith(padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: 20))),
    ),
    textButtonTheme: TextButtonThemeData(
      style: button.copyWith(padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: 14))),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(style: button),
    iconButtonTheme: IconButtonThemeData(
      style: ButtonStyle(
        animationDuration: motionFast,
        shape: WidgetStateProperty.resolveWith(
          (s) => RoundedRectangleBorder(borderRadius: BorderRadius.circular(s.contains(WidgetState.pressed) ? 12 : 20)),
        ),
      ),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: scheme.surfaceContainerHigh,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(28)),
    ),
    menuTheme: MenuThemeData(
      style: MenuStyle(
        backgroundColor: WidgetStatePropertyAll(scheme.surfaceContainer),
        padding: const WidgetStatePropertyAll(EdgeInsets.all(4)),
        shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: BorderRadius.circular(16))),
        elevation: const WidgetStatePropertyAll(3),
      ),
    ),
    menuButtonTheme: MenuButtonThemeData(
      style: ButtonStyle(
        minimumSize: const WidgetStatePropertyAll(Size(210, 44)),
        padding: const WidgetStatePropertyAll(EdgeInsets.symmetric(horizontal: 12)),
        shape: WidgetStatePropertyAll(RoundedRectangleBorder(borderRadius: BorderRadius.circular(12))),
        iconSize: const WidgetStatePropertyAll(20),
        textStyle: WidgetStatePropertyAll(text.bodyLarge?.copyWith(fontSize: 14)),
      ),
    ),
    tooltipTheme: const TooltipThemeData(waitDuration: Duration(milliseconds: 600)),
  );
}

/// Roboto Flex is a variable font: its weight is the wght axis, not a separate file.
TextTheme _variable(TextTheme theme) {
  TextStyle? weighted(TextStyle? s) {
    if (s == null) return null;
    return s.weight((s.fontWeight ?? FontWeight.w400).value);
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
