import 'package:flutter/material.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';
import '../format.dart';
import '../widgets.dart';
import 'page_frame.dart';

/// A client on the Game page: installed, or one an account can download.
class ClientRow {
  ClientRow(this.kind, this.branch);

  final String kind;
  String branch;
  Client? installed;
  String? available;
  String? account;

  String get name => switch (kind) {
    'vkplay' => t('VK Play'),
    'fxid' => t('FX ID main branch'),
    _ => 'FX ID $branch',
  };

  /// The kind and branch the core's client commands take.
  Map<String, Object?> get target => {'value': kind, 'branch': kind == 'fxid' ? '' : branch};
}

/// The clients to show: one for each service and branch the accounts use,
/// the ones they can download, and the ones on disk.
List<ClientRow> clientRows(Controller c) {
  final rows = <ClientRow>[];
  void add(String kind, {String branch = '', String dir = '', String version = '', String? account, Client? client}) {
    final b = branch.isNotEmpty ? branch : (kind == 'fxid' ? 'default' : '');
    var row = rows
        .where((r) => r.kind == kind && (kind != 'branch' || r.branch.toLowerCase() == b.toLowerCase()))
        .firstOrNull;
    if (row == null) rows.add(row = ClientRow(kind, b));
    if (dir.isNotEmpty) {
      row.installed = client;
    } else if (version.isNotEmpty) {
      row.available = version;
    }
    if (account != null && account.isNotEmpty && row.account == null) row.account = account;
  }

  final vk = c.accounts.where((a) => a.provider == 'vkplay').firstOrNull;
  final fx = c.accounts.where((a) => a.isFX).toList();
  if (vk != null) add('vkplay', account: vk.id);
  if (fx.isNotEmpty) add('fxid', account: fx.first.id);
  for (final a in fx) {
    final b = a.branch ?? '';
    if (b.isNotEmpty && b.toLowerCase() != 'default') add('branch', branch: b, account: a.id);
  }
  for (final a in c.availableClients) {
    if (c.accounts.any((x) => x.id == a.account)) {
      add(a.kind, branch: a.branch, version: a.version, account: a.account);
    }
  }
  for (final cl in c.gameInfo?.clients ?? const <Client>[]) {
    add(cl.kind, branch: cl.branch, dir: cl.dir, version: cl.version, client: cl);
  }
  return rows;
}

class GamePage extends StatelessWidget {
  const GamePage({super.key, required this.c});

  final Controller c;

  String _folder(ClientRow r) {
    if (r.installed != null) return r.installed!.dir;
    if (r.kind == 'vkplay') return c.game.isNotEmpty ? c.game : c.suggestedGame;
    if (r.kind == 'fxid') return c.fxGame;
    return c.gameInfo?.branchPaths[r.branch.toLowerCase()] ?? (c.fxGame.isNotEmpty ? '${c.fxGame} ${r.branch}' : '');
  }

  void _chooseFolder(ClientRow r) {
    if (r.kind == 'vkplay') {
      c.send({'cmd': 'gameFolder'});
    } else if (r.kind == 'fxid') {
      c.send({'cmd': 'fxFolder'});
    } else {
      c.send({'cmd': 'branchFolder', 'branch': r.branch});
    }
  }

  @override
  Widget build(BuildContext context) {
    final busy = c.gameOp != null;
    final rows = clientRows(c);
    Widget note(String text, {String? title, VoidCallback? retry}) => ListItem(
      icon: Icons.info,
      title: title == null ? const SizedBox.shrink() : Text(title),
      sub: text,
      trailing: [if (retry != null) TextButton(onPressed: retry, child: Text(t('Retry')))],
    );
    void retry() => c.send({'cmd': 'availableClients'});
    return PageFrame(
      title: t('Game'),
      children: [
        SectionLabel(t('Clients'), first: true),
        ListCard(
          children: [
            for (final r in rows) _clientItem(context, r, busy),
            if (c.availableClientsFailed) note(t('Could not load some FX ID branches.'), retry: retry),
            if (c.availableVKFailed) note(t('Could not load the VK Play catalog.'), retry: retry),
            if (rows.isEmpty)
              note(t('Add a VK Play or FX ID account to see its client here.'), title: t('No clients yet')),
          ],
        ),
        SectionLabel(t('Game files')),
        ListCard(
          children: [
            ListItem(
              icon: Icons.verified_user,
              title: Text(t('Allow mods')),
              sub: t(
                'Keep existing modified files when starting the game. Missing files are still downloaded; Check files restores originals. Game updates may replace mods.',
              ),
              wide: _onOff(c.allowMods, (v) => c.send({'cmd': 'allowMods', 'value': v})),
            ),
            ListItem(
              icon: Icons.history,
              title: Text(t('Keep patch backups')),
              sub: t(
                r'VK Play patches save the files they replace to -gup-\awlauncher-cache until the next update. When off, they are deleted as soon as a patch is installed.',
              ),
              wide: _onOff(c.patchBackups, (v) => c.send({'cmd': 'patchBackups', 'value': v})),
            ),
          ],
        ),
      ],
    );
  }

  Widget _clientItem(BuildContext context, ClientRow r, bool busy) {
    final cl = r.installed;
    final sub = cl != null
        ? [
            cl.version,
            cl.dir,
            if (cl.downloads > 0) t('patches {size}', {'size': formatBytes(cl.downloads)}).replaceAll(' ', ' '),
          ]
        : [t('Not installed'), r.available ?? '', _folder(r)];
    return ListItem(
      icon: r.kind == 'branch' ? Icons.alt_route : Icons.language,
      title: Text(r.name),
      sub: sub.where((s) => s.isNotEmpty).join(' · '),
      trailing: [
        if (cl == null) TextButton(onPressed: busy ? null : () => _chooseFolder(r), child: Text(t('Folder…'))),
        if (cl != null)
          Tooltip(
            message: t('Check every file and download the ones that differ'),
            child: FilledButton.tonalIcon(
              onPressed: busy ? null : () => c.send({'cmd': 'verify', ...r.target}),
              icon: const Icon(Icons.verified_user, size: 18),
              label: Text(t('Check files')),
            ),
          )
        else
          FilledButton.tonal(
            onPressed: busy || r.account == null
                ? null
                : () => c.send({'cmd': 'downloadClient', 'account': r.account, 'value': r.kind, 'branch': r.branch}),
            child: Text(t('Download')),
          ),
        if (cl != null)
          MoreMenu(
            tooltip: t('More'),
            entries: [
              MenuEntry(
                Icons.update,
                t('Check for updates'),
                () => c.send({'cmd': 'updateClient', ...r.target}),
                enabled: !busy,
              ),
              MenuEntry(Icons.folder, t('Open folder'), () => c.send({'cmd': 'openClientFolder', ...r.target})),
              MenuEntry(Icons.edit, t('Change folder…'), () => _chooseFolder(r), enabled: !busy),
              if (cl.downloads > 0)
                MenuEntry(
                  Icons.clear_all,
                  t('Delete downloaded patches ({size})', {'size': formatBytes(cl.downloads)}),
                  () => c.send({'cmd': 'clearDownloads', ...r.target}),
                  enabled: !busy,
                ),
              null,
              MenuEntry(
                Icons.delete,
                t('Remove client'),
                () => c.send({'cmd': r.kind == 'branch' ? 'removeBranch' : 'removeMainClient', ...r.target}),
                danger: true,
                enabled: !busy,
              ),
            ],
          ),
      ],
    );
  }
}

Widget _onOff(bool on, ValueChanged<String> set) => Segmented<bool>(
  options: [(false, t('Off')), (true, t('On'))],
  selected: on,
  onChanged: (v) => set(v ? 'on' : 'off'),
);
