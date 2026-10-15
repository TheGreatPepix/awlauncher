import 'package:flutter/material.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';
import '../../theme/palette.dart';
import 'page_frame.dart';

class SettingsPage extends StatelessWidget {
  const SettingsPage({super.key, required this.c, required this.onExit});

  final Controller c;
  final VoidCallback onExit;

  String _paletteName(Palette p) => '${t(presetName(p) ?? 'Custom')} · ${t(p.style.label)}';

  @override
  Widget build(BuildContext context) {
    final prefs = c.prefs;
    final autostartHint = switch (c.autostart) {
      'window' => 'Opens its window when you sign in to Windows.',
      'tray' => 'Starts in the notification area when you sign in to Windows; click its icon to open it.',
      _ => 'AWLauncher starts only when you open it.',
    };
    return PageFrame(
      title: t('Settings'),
      children: [
        Section(
          title: t('Launcher'),
          children: [
            ListTile(
              title: Text(t('Launcher data')),
              subtitle: Text(c.dataDir),
              trailing: TextButton(onPressed: () => c.send({'cmd': 'openDataFolder'}), child: Text(t('Open'))),
            ),
            ListTile(
              title: Text(t('Start with Windows')),
              subtitle: Text(t(autostartHint)),
              trailing: SegmentedButton<String>(
                showSelectedIcon: false,
                segments: [
                  for (final m in ['off', 'window', 'tray']) ButtonSegment(value: m, label: Text(t(_capital(m)))),
                ],
                selected: {c.autostart},
                onSelectionChanged: (s) => c.send({'cmd': 'autostart', 'value': s.first}),
              ),
            ),
            ListTile(
              title: Text(t('Exit AWLauncher')),
              subtitle: Text(t('Close the launcher completely')),
              trailing: TextButton(onPressed: onExit, child: Text(t('Exit'))),
            ),
          ],
        ),
        Section(
          title: t('Appearance'),
          children: [
            ListTile(
              title: Text(t('Theme')),
              trailing: SegmentedButton<ThemeChoice>(
                showSelectedIcon: false,
                segments: [
                  for (final m in ThemeChoice.values) ButtonSegment(value: m, label: Text(t(_capital(m.name)))),
                ],
                selected: {prefs.theme},
                onSelectionChanged: (s) => c.setPrefs(prefs.copyWith(theme: s.first)),
              ),
            ),
            ListTile(title: Text(t('Color')), subtitle: Text(_paletteName(prefs.palette))),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
              child: Wrap(
                spacing: 10,
                runSpacing: 10,
                children: [
                  for (final (_, p) in presets)
                    _Swatch(
                      p: p,
                      tooltip: _paletteName(p),
                      selected: p == prefs.palette,
                      onTap: () => c.setPrefs(prefs.copyWith(palette: p)),
                    ),
                  for (final p in prefs.palettes)
                    _Swatch(
                      p: p,
                      tooltip: _paletteName(p),
                      selected: p == prefs.palette,
                      onTap: () => c.setPrefs(prefs.copyWith(palette: p)),
                    ),
                ],
              ),
            ),
            ListTile(
              title: Text(t('Language')),
              trailing: SegmentedButton<String>(
                showSelectedIcon: false,
                segments: [
                  ButtonSegment(value: 'auto', label: Text(t('Auto'))),
                  const ButtonSegment(value: 'en', label: Text('English')),
                  const ButtonSegment(value: 'ru', label: Text('Русский')),
                ],
                selected: {prefs.lang},
                onSelectionChanged: (s) => c.setPrefs(prefs.copyWith(lang: s.first)),
              ),
            ),
          ],
        ),
        Section(
          title: t('About'),
          children: [
            ListTile(
              title: Text(t('Version')),
              subtitle: Text(c.version),
              trailing: TextButton(
                onPressed: () => c.send({'cmd': 'checkUpdate'}),
                child: Text(t('Check for updates')),
              ),
            ),
          ],
        ),
      ],
    );
  }

  static String _capital(String s) => s[0].toUpperCase() + s.substring(1);
}

/// A palette's swatch: its primary, secondary and tertiary containers.
class _Swatch extends StatelessWidget {
  const _Swatch({required this.p, required this.tooltip, required this.selected, required this.onTap});

  final Palette p;
  final String tooltip;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = p.scheme(Theme.of(context).brightness);
    return Tooltip(
      message: tooltip,
      child: InkResponse(
        onTap: onTap,
        radius: 28,
        child: Container(
          width: 48,
          height: 48,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            border: Border.all(color: selected ? scheme.primary : Colors.transparent, width: 3),
          ),
          padding: const EdgeInsets.all(3),
          child: ClipOval(
            child: Column(
              children: [
                Expanded(
                  child: ColoredBox(color: scheme.primary, child: const SizedBox.expand()),
                ),
                Expanded(
                  child: Row(
                    children: [
                      Expanded(
                        child: ColoredBox(color: scheme.secondaryContainer, child: const SizedBox.expand()),
                      ),
                      Expanded(
                        child: ColoredBox(color: scheme.tertiaryContainer, child: const SizedBox.expand()),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
