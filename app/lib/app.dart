import 'package:flutter/material.dart';

import 'state/controller.dart';
import 'state/models.dart';
import 'theme/theme.dart';
import 'ui/shell.dart';

class AwApp extends StatefulWidget {
  const AwApp({super.key, required this.controller});

  final Controller controller;

  @override
  State<AwApp> createState() => _AwAppState();
}

class _AwAppState extends State<AwApp> {
  @override
  void initState() {
    super.initState();
    widget.controller.start();
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    return ListenableBuilder(
      listenable: c,
      builder: (context, _) {
        final prefs = c.prefs;
        return MaterialApp(
          title: 'AWLauncher',
          debugShowCheckedModeBanner: false,
          locale: Locale(c.lang),
          theme: buildTheme(c.preview ?? prefs.palette, Brightness.light),
          darkTheme: buildTheme(c.preview ?? prefs.palette, Brightness.dark),
          themeMode: switch (prefs.theme) {
            ThemeChoice.light => ThemeMode.light,
            ThemeChoice.dark => ThemeMode.dark,
            ThemeChoice.system => ThemeMode.system,
          },
          home: Shell(c: c),
        );
      },
    );
  }
}
