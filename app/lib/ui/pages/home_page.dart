import 'package:flutter/material.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';
import '../format.dart';
import 'page_frame.dart';

class HomePage extends StatelessWidget {
  const HomePage({super.key, required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    return PageFrame(
      title: t('Play'),
      children: [
        _Hero(c: c),
        if (c.signInActive) _SignInCard(c: c),
        if (c.progress.active || c.progress.paused) _ProgressCard(c: c),
        Row(
          children: [
            Expanded(
              child: Text('${t('Accounts')} · ${c.accounts.length}', style: Theme.of(context).textTheme.titleMedium),
            ),
            TextButton.icon(
              onPressed: () => showAddAccount(context, c),
              icon: const Icon(Icons.person_add_outlined),
              label: Text(t('Add account')),
            ),
          ],
        ),
        const SizedBox(height: 8),
        if (c.accounts.isEmpty) _Empty(c: c) else for (final a in c.accounts) _AccountTile(c: c, a: a),
      ],
    );
  }
}

Future<void> showAddAccount(BuildContext context, Controller c) {
  return showDialog<void>(
    context: context,
    builder: (context) => SimpleDialog(
      title: Text(t('Add account')),
      children: [
        for (final (provider, name) in const [('vkplay', 'VK Play'), ('fxid', 'FX ID'), ('steam', 'Steam')])
          SimpleDialogOption(
            onPressed: () {
              Navigator.of(context).pop();
              c.send({'cmd': 'add', 'provider': provider});
            },
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
            child: Text(name),
          ),
      ],
    ),
  );
}

class _Hero extends StatelessWidget {
  const _Hero({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final text = Theme.of(context).textTheme;
    final a = c.selectedAccount;
    return Card.filled(
      color: scheme.primaryContainer,
      margin: const EdgeInsets.only(bottom: 20),
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    a?.name ?? t('Add an account to start'),
                    style: text.headlineSmall?.copyWith(color: scheme.onPrimaryContainer),
                  ),
                  if (a != null) Text(a.service, style: text.bodyMedium?.copyWith(color: scheme.onPrimaryContainer)),
                ],
              ),
            ),
            if (c.running)
              OutlinedButton.icon(
                onPressed: () => confirmCloseGame(context, c),
                icon: const Icon(Icons.close),
                label: Text(t('Close game')),
              )
            else
              FilledButton.icon(
                key: const Key('play'),
                style: FilledButton.styleFrom(
                  minimumSize: const Size(160, 64),
                  textStyle: text.titleLarge,
                  shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
                ),
                onPressed: c.canPlay(a) ? () => c.play(a) : null,
                icon: const Icon(Icons.play_arrow_rounded, size: 32),
                label: Text(t('Play')),
              ),
          ],
        ),
      ),
    );
  }
}

Future<void> confirmCloseGame(BuildContext context, Controller c) async {
  final close = await showDialog<bool>(
    context: context,
    builder: (context) => AlertDialog(
      icon: const Icon(Icons.close),
      title: Text(t('Close the game?')),
      content: Text(
        t(
          'Armored Warfare is asked to close; if it does not respond in 10 seconds, it is ended. If you are in a battle, you leave it.',
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(false), child: Text(t('Cancel'))),
        FilledButton(
          style: FilledButton.styleFrom(backgroundColor: Theme.of(context).colorScheme.error),
          onPressed: () => Navigator.of(context).pop(true),
          child: Text(t('Close game')),
        ),
      ],
    ),
  );
  if (close == true) c.send({'cmd': 'closeGame'});
}

class _SignInCard extends StatelessWidget {
  const _SignInCard({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    return Card.outlined(
      margin: const EdgeInsets.only(bottom: 20),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(t('Signing in to VK Play'), style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              t(
                'Continue in the browser; with a VK Play session there it returns here by itself. If the browser asks, allow vkplay.ru to access apps on this device.',
              ),
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilledButton.tonal(onPressed: () => c.send({'cmd': 'signinOpen'}), child: Text(t('Open again'))),
                TextButton(onPressed: () => c.send({'cmd': 'signinFresh'}), child: Text(t('Another account'))),
                TextButton(onPressed: () => c.send({'cmd': 'signinCancel'}), child: Text(t('Cancel'))),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ProgressCard extends StatelessWidget {
  const _ProgressCard({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final p = c.progress;
    final text = Theme.of(context).textTheme;
    final meta = [
      if (p.total > 0) formatPair(p.done, p.total),
      if (p.speed > 0) '${formatBytes(p.speed)}/s',
      if (p.speed > 0 && p.total > p.done) formatDuration((p.total - p.done) / p.speed),
    ].join(' · ');
    return Card.filled(
      margin: const EdgeInsets.only(bottom: 20),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(tb(p.title), style: text.titleMedium)),
                if (p.pausable)
                  TextButton.icon(
                    onPressed: () => c.send({'cmd': p.paused ? 'resume' : 'pause'}),
                    icon: Icon(p.paused ? Icons.play_arrow : Icons.pause),
                    label: Text(t(p.paused ? 'Resume' : 'Pause')),
                  ),
              ],
            ),
            const SizedBox(height: 12),
            LinearProgressIndicator(value: p.fraction),
            const SizedBox(height: 8),
            Text(meta, style: text.bodySmall),
          ],
        ),
      ),
    );
  }
}

class _AccountTile extends StatelessWidget {
  const _AccountTile({required this.c, required this.a});

  final Controller c;
  final Account a;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final selected = a.id == c.selected;
    final op = c.accountOp(a.id);
    return Card.filled(
      color: selected ? scheme.secondaryContainer : scheme.surfaceContainerLow,
      margin: const EdgeInsets.only(bottom: 8),
      clipBehavior: Clip.antiAlias,
      child: ListTile(
        onTap: () => c.select(a.id),
        leading: CircleAvatar(child: Text(a.name.isEmpty ? '?' : a.name.characters.first.toUpperCase())),
        title: Row(
          children: [
            Flexible(child: Text(a.name, overflow: TextOverflow.ellipsis)),
            if (a.last)
              Padding(
                padding: const EdgeInsets.only(left: 6),
                child: Icon(Icons.push_pin, size: 16, color: scheme.primary),
              ),
          ],
        ),
        subtitle: Text(op != null ? tb(op.title) : '${a.service} · ${a.login}', overflow: TextOverflow.ellipsis),
        trailing: IconButton.filledTonal(
          tooltip: t('Play'),
          onPressed: c.canPlay(a) ? () => c.play(a) : null,
          icon: const Icon(Icons.play_arrow_rounded),
        ),
      ),
    );
  }
}

class _Empty extends StatelessWidget {
  const _Empty({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    return Card.outlined(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          children: [
            Text(t('No accounts yet'), style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              t('Add a VK Play or FX ID account. Each service keeps its own game folder.'),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            FilledButton.icon(
              onPressed: () => showAddAccount(context, c),
              icon: const Icon(Icons.person_add),
              label: Text(t('Add account')),
            ),
          ],
        ),
      ),
    );
  }
}
