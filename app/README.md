# AWLauncher window

The Flutter window of AWLauncher, Material 3. It runs `awcore` beside its executable and talks to it with JSON lines: `{cmd: ...}` commands on its standard input and `{type: ...}` events on its standard output, the protocol of `internal/gui/commands.go`.

```powershell
go build -ldflags '-H=windowsgui' -o app\build\awcore.exe ./cmd/awcore
cd app
$env:AWCORE = "$PWD\build\awcore.exe"; flutter run -d windows
flutter run -d windows --dart-entrypoint-args=--demo   # sample accounts, no core
flutter test                                          # with AWCORE set, also against the real core
```

- `lib/core`: the core process, and `FakeCore` with sample data for design work and tests.
- `lib/state`: the window's state, kept in step with the core's events.
- `lib/theme`: the Material 3 theme; a palette is a hue and one of Material's scheme variants.
- `lib/i18n`: the Russian text, keyed by the English text as the core sends it.
- `lib/ui`: the navigation, the pages and the dialogs.
