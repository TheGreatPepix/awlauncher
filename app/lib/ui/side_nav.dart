import 'package:flutter/material.dart';

import '../i18n/strings.dart';
import '../state/controller.dart';

/// The navigation drawer on the left: the brand, the pages and the status.
class SideNav extends StatelessWidget {
  const SideNav({super.key, required this.c, required this.width});

  final Controller c;
  final double width;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    Widget dot(bool on) => Badge(isLabelVisible: on, smallSize: 8, child: const SizedBox.shrink());
    final pages = [
      (AppPage.home, Icons.play_circle_outline, Icons.play_circle, t('Play'), false),
      (AppPage.game, Icons.sports_esports_outlined, Icons.sports_esports, t('Game'), false),
      (AppPage.activity, Icons.receipt_long_outlined, Icons.receipt_long, t('Logs'), c.logUnseen),
      (AppPage.settings, Icons.settings_outlined, Icons.settings, t('Settings'), c.updateUnseen),
    ];
    return SizedBox(
      width: width,
      child: NavigationDrawer(
        backgroundColor: scheme.surfaceContainerLow,
        selectedIndex: c.page.index,
        onDestinationSelected: (i) => c.showPage(AppPage.values[i]),
        footer: Padding(
          padding: const EdgeInsets.fromLTRB(28, 8, 16, 20),
          child: Row(
            children: [
              Icon(Icons.circle, size: 10, color: c.busy ? scheme.tertiary : scheme.primary),
              const SizedBox(width: 10),
              Expanded(
                child: Text(
                  c.status,
                  key: const Key('status'),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: text.bodySmall,
                ),
              ),
            ],
          ),
        ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 20, 16, 20),
            child: Row(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(10),
                  child: Image.asset('assets/images/logo.png', width: 40, height: 40),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('AWLauncher', style: text.titleMedium),
                      Text('Armored Warfare', style: text.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          for (final (_, icon, selectedIcon, label, unseen) in pages)
            NavigationDrawerDestination(
              icon: Icon(icon),
              selectedIcon: Icon(selectedIcon),
              label: Row(children: [Text(label), const SizedBox(width: 8), dot(unseen)]),
            ),
        ],
      ),
    );
  }
}
