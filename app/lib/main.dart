import 'dart:io';

import 'package:flutter/material.dart';

import 'app.dart';
import 'core/core.dart';
import 'core/fake_core.dart';
import 'platform/desktop.dart';
import 'state/controller.dart';

/// Starts the window. With --demo it runs on sample data; otherwise it
/// starts awcore next to its executable, or the one AWCORE names.
Future<void> main(List<String> args) async {
  WidgetsFlutterBinding.ensureInitialized();
  final exe = Platform.resolvedExecutable;
  final Core core;
  if (args.contains('--demo') || const bool.fromEnvironment('AWLAUNCHER_DEMO')) {
    core = FakeCore();
  } else {
    final path = Platform.environment['AWCORE'] ?? ProcessCore.beside(exe);
    if (path == null) {
      core = _MissingCore();
    } else {
      core = await ProcessCore.start(path, app: exe, updated: args.contains('--updated'));
    }
  }
  runApp(AwApp(controller: Controller(core, SystemDesktop())));
}

class _MissingCore implements Core {
  @override
  Stream<Event> get events =>
      Stream.value({'type': 'fatal', 'message': 'awcore.exe is missing next to AWLauncher.exe'});

  @override
  void send(Map<String, Object?> command) {}

  @override
  Future<void> close() async {}
}
