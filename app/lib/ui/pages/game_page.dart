import 'package:flutter/material.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../format.dart';
import 'page_frame.dart';

class GamePage extends StatelessWidget {
  const GamePage({super.key, required this.c});

  final Controller c;

  static String clientName(Map<String, dynamic> client) => switch (client['kind']) {
    'vk' => 'VK Play',
    'fx' => 'FX ID',
    _ => 'FX ID ${client['branch'] ?? ''}',
  };

  @override
  Widget build(BuildContext context) {
    final info = c.gameInfo;
    final clients = (info?['clients'] as List? ?? const []).whereType<Map<String, dynamic>>();
    final downloads = (info?['downloads'] as num?) ?? 0;
    return PageFrame(
      title: t('Game'),
      children: [
        Section(
          title: t('Clients'),
          children: [
            if (info == null) const LinearProgressIndicator(),
            if (info != null && clients.isEmpty) ListTile(title: Text(t('Not installed'))),
            for (final client in clients)
              ListTile(
                leading: const Icon(Icons.sports_esports_outlined),
                title: Text('${clientName(client)} · ${client['version'] ?? ''}'),
                subtitle: Text('${client['dir'] ?? ''}'),
                trailing: IconButton(
                  tooltip: t('Open folder'),
                  icon: const Icon(Icons.folder_open_outlined),
                  onPressed: () =>
                      c.send({'cmd': 'openClientFolder', 'value': client['kind'], 'branch': client['branch'] ?? ''}),
                ),
              ),
          ],
        ),
        Section(
          children: [
            SwitchListTile(
              title: Text(t('Allow mods')),
              value: c.allowMods,
              onChanged: (on) => c.send({'cmd': 'allowMods', 'value': on ? 'on' : 'off'}),
            ),
            ListTile(title: Text(t('Downloaded patches')), subtitle: Text(formatBytes(downloads))),
          ],
        ),
      ],
    );
  }
}
