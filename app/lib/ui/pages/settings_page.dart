import 'dart:io';

import 'package:flutter/material.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';
import '../../theme/palette.dart';
import '../palette_widgets.dart';
import '../widgets.dart';
import 'page_frame.dart';

const _autostartText = {
  'off': 'AWLauncher starts only when you open it.',
  'window': 'Opens its window when you sign in to Windows.',
  'tray': 'Starts in the notification area when you sign in to Windows; click its icon to open it.',
};

String paletteName(Palette p) => '${t(presetName(p) ?? 'Custom')} · ${t(p.style.label)}';

class SettingsPage extends StatelessWidget {
  const SettingsPage({super.key, required this.c, required this.onExit});

  final Controller c;
  final VoidCallback onExit;

  @override
  Widget build(BuildContext context) {
    final prefs = c.prefs;
    return PageFrame(
      title: t('Settings'),
      children: [
        SectionLabel(t('Launcher'), first: true),
        ListCard(
          children: [
            ListItem(
              icon: Icons.storage,
              title: Text(t('Launcher data')),
              sub: c.dataDir.isEmpty ? r'%LOCALAPPDATA%\AWLauncher' : c.dataDir,
              subStyle: monoStyle(context, size: 13),
              trailing: [
                TextButton(onPressed: () => c.send({'cmd': 'openDataFolder'}), child: Text(t('Open'))),
              ],
            ),
            if (Platform.isWindows)
              ListItem(
                icon: Icons.power_settings_new,
                title: Text(t('Start with Windows')),
                sub: t(_autostartText[c.autostart] ?? _autostartText['off']!),
                wide: Segmented<String>(
                  options: [('off', t('Off')), ('window', t('Window')), ('tray', t('Tray'))],
                  selected: c.autostart,
                  onChanged: c.setAutostart,
                ),
              ),
            if (Platform.isWindows)
              ListItem(
                icon: Icons.sports_esports,
                title: Text(t('Hide after the game starts')),
                sub: t(
                  'The launcher goes to the notification area when the game starts. When off, its window stays open.',
                ),
                wide: Segmented<bool>(
                  options: [(false, t('Off')), (true, t('On'))],
                  selected: c.hideOnLaunch,
                  onChanged: (v) => c.send({'cmd': 'hideOnLaunch', 'value': v ? 'on' : 'off'}),
                ),
              ),
            if (c.protons.isNotEmpty) _ProtonItem(c: c),
            ListItem(
              icon: Icons.logout,
              title: Text(t('Exit AWLauncher')),
              sub: t('Close the launcher completely'),
              trailing: [TextButton(onPressed: onExit, child: Text(t('Exit')))],
            ),
          ],
        ),
        SectionLabel(t('Appearance')),
        ListCard(
          children: [
            ListItem(
              icon: Icons.contrast,
              title: Text(t('Theme')),
              wide: Segmented<ThemeChoice>(
                options: [
                  (ThemeChoice.system, t('System')),
                  (ThemeChoice.light, t('Light')),
                  (ThemeChoice.dark, t('Dark')),
                ],
                selected: prefs.theme,
                onChanged: (v) => c.setPrefs(prefs.copyWith(theme: v)),
              ),
            ),
            ListItem(
              icon: Icons.palette,
              title: Text(t('Color')),
              sub: paletteName(c.preview ?? prefs.palette),
              wide: Palettes(c: c),
              wideWidth: 332,
            ),
            ListItem(
              icon: Icons.language,
              title: Text(t('Language')),
              wide: Segmented<String>(
                options: [('auto', t('Auto')), ('en', 'English'), ('ru', 'Русский')],
                selected: prefs.lang,
                onChanged: (v) => c.setPrefs(prefs.copyWith(lang: v)),
              ),
            ),
          ],
        ),
        SectionLabel(t('About')),
        ListCard(
          children: [
            ListItem(
              icon: Icons.info,
              title: const Text('AWLauncher'),
              sub: t(
                'An independent Armored Warfare launcher with FX ID and VK Play sign-in, game installation, updates and launch.',
              ),
            ),
            _UpdateItem(c: c),
          ],
        ),
      ],
    );
  }
}

/// The Proton build the game runs with on Linux.
class _ProtonItem extends StatelessWidget {
  const _ProtonItem({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final chosen = c.protons.where((p) => p.path == c.proton).firstOrNull;
    final sub = chosen != null
        ? chosen.path
        : t('Picked automatically: {name}. umu-launcher runs it when installed, otherwise Steam Linux Runtime.', {
            'name': c.protons.first.name,
          });
    return ListItem(
      icon: Icons.layers,
      title: Text('Proton${chosen != null ? ' · ${chosen.name}' : ''}'),
      sub: sub,
      trailing: [
        MenuAnchor(
          menuChildren: [
            MenuItemButton(
              leadingIcon: Icon(c.proton.isEmpty || chosen == null ? Icons.check : null),
              onPressed: () => c.setProton(''),
              child: Text(t('Automatic')),
            ),
            for (final p in c.protons)
              MenuItemButton(
                leadingIcon: Icon(p.path == c.proton ? Icons.check : null),
                onPressed: () => c.setProton(p.path),
                child: Text(p.name),
              ),
          ],
          builder: (context, menu, _) =>
              TextButton(onPressed: () => menu.isOpen ? menu.close() : menu.open(), child: Text(t('Change'))),
        ),
      ],
    );
  }
}

class _UpdateItem extends StatelessWidget {
  const _UpdateItem({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final u = c.update;
    final release = isRelease(c.version);
    var status = t('Updates come from the releases on GitHub.');
    if (c.version.isNotEmpty && !release) status = t('Development build. Updates come from the releases on GitHub.');
    if (c.checkingUpdate) {
      status = t('Checking GitHub for a newer release…');
    } else if (c.updating) {
      status = t('Downloading the update. AWLauncher restarts when it is ready.');
    } else if (u?.error != null) {
      status = t('Could not check: {error}', {'error': u!.error});
    } else if (u != null && u.available) {
      status = u.published != null
          ? t('{latest} is available, released {date}.', {'latest': u.latest, 'date': u.published})
          : t('{latest} is available.', {'latest': u.latest});
    } else if (u?.latest != null && release) {
      status = t('Up to date. The latest release is {latest}.', {'latest': u!.latest});
    } else if (u?.latest != null) {
      status = t('Development build. The latest release is {latest}.', {'latest': u!.latest});
    }
    final available = u != null && u.available;
    final blocked = available && u.canApply && c.busy;
    final label = available
        ? (u.canApply ? t('Update to {latest}', {'latest': u.latest}) : t('Download'))
        : t('Check for updates');
    void press() {
      if (available) {
        u.canApply ? c.applyUpdate() : c.openLink(u.page);
      } else {
        c.checkUpdate();
      }
    }

    final enabled = !c.checkingUpdate && !c.updating && !blocked;
    final button = available
        ? FilledButton(onPressed: enabled ? press : null, child: Text(label))
        : FilledButton.tonal(onPressed: enabled ? press : null, child: Text(label));
    return ListItem(
      icon: Icons.update,
      title: Text('${t('Version')}${release ? ' ${c.version}' : ''}'),
      sub: status,
      trailing: [
        if (available) TextButton(onPressed: () => c.openLink(u.page), child: Text(t('Release page'))),
        blocked ? Tooltip(message: t('Wait until {op} finishes', {'op': c.opsTitle}), child: button) : button,
      ],
    );
  }
}
