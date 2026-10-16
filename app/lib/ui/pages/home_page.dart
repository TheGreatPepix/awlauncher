import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../i18n/strings.dart';
import '../../state/controller.dart';
import '../../state/models.dart';
import '../account_dialogs.dart';
import '../play_button.dart';
import '../progress_block.dart';
import '../widgets.dart';
import 'page_frame.dart';

class HomePage extends StatelessWidget {
  const HomePage({super.key, required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    return PageFrame(
      children: [
        _Hero(c: c),
        if (c.signInActive) _SignInCard(c: c),
        ProgressBlock(c: c),
        _AccountsCard(c: c),
      ],
    );
  }
}

class _Hero extends StatelessWidget {
  const _Hero({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final a = c.selectedAccount;
    final own = a == null ? null : c.accountOp(a.id);
    final game = c.gameOp;
    final blocker = own ?? game;
    var chip = t('Add an account to start');
    if (own != null) {
      chip = '${tb(own.title)}…';
    } else if (a != null && game != null) {
      chip = t('{name} · {service} · waits for {op}', {'name': a.name, 'service': a.service, 'op': tb(game.title)});
    } else if (a != null) {
      chip = '${a.name} · ${a.service}';
    }
    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 8, 0, 22),
      child: Column(
        children: [
          PlayButton(
            onPressed: c.canPlay(a) ? () => c.play(a) : null,
            working: own != null && own.game,
            paused: c.paused,
            tooltip: a == null || c.canPlay(a) || blocker == null
                ? t('Play')
                : t('Wait until {op} finishes', {'op': tb(blocker.title)}),
          ),
          const SizedBox(height: 18),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Wrap(
              alignment: WrapAlignment.center,
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 10,
              runSpacing: 10,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 8),
                  decoration: BoxDecoration(color: scheme.secondaryContainer, borderRadius: BorderRadius.circular(18)),
                  child: Text(
                    chip,
                    key: const Key('hero-chip'),
                    textAlign: TextAlign.center,
                    style: TextStyle(fontSize: 14, color: scheme.onSecondaryContainer).weight(500),
                  ),
                ),
                if (c.running)
                  FilledButton.icon(
                    style: FilledButton.styleFrom(
                      backgroundColor: scheme.errorContainer,
                      foregroundColor: scheme.onErrorContainer,
                      minimumSize: const Size(0, 36),
                      padding: const EdgeInsets.fromLTRB(14, 0, 16, 0),
                    ),
                    onPressed: () => confirmCloseGame(context, c),
                    icon: const Icon(Icons.close, size: 18),
                    label: Text(t('Close game')),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SignInCard extends StatelessWidget {
  const _SignInCard({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final fg = scheme.onTertiaryContainer;
    final text = TextButton.styleFrom(foregroundColor: fg, padding: const EdgeInsets.symmetric(horizontal: 12));
    return Container(
      margin: const EdgeInsets.only(bottom: 20),
      padding: const EdgeInsets.fromLTRB(16, 14, 14, 14),
      decoration: BoxDecoration(color: scheme.tertiaryContainer, borderRadius: BorderRadius.circular(20)),
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        alignment: WrapAlignment.end,
        spacing: 14,
        runSpacing: 10,
        children: [
          LayoutBuilder(
            builder: (context, box) => ConstrainedBox(
              constraints: BoxConstraints(minWidth: math.min(240 + 54, box.maxWidth), maxWidth: box.maxWidth),
              child: IntrinsicWidth(
                child: Row(
                  children: [
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: fg.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Icon(Icons.language, color: fg),
                    ),
                    const SizedBox(width: 14),
                    Flexible(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(t('Signing in to VK Play'), style: TextStyle(color: fg, fontSize: 14).weight(600)),
                          const SizedBox(height: 2),
                          Text(
                            t(
                              'Continue in the browser; with a VK Play session there it returns here by itself. If the browser asks, allow vkplay.ru to access apps on this device.',
                            ),
                            style: TextStyle(color: fg.withValues(alpha: 0.85), fontSize: 13, height: 18 / 13),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          Wrap(
            spacing: 4,
            runSpacing: 4,
            children: [
              Tooltip(
                message: t('Open the sign-in in the browser again'),
                child: TextButton(
                  style: text,
                  onPressed: () => c.send({'cmd': 'signinOpen'}),
                  child: Text(t('Open again')),
                ),
              ),
              Tooltip(
                message: t('Sign in with another VK Play account in the browser'),
                child: TextButton(
                  style: text,
                  onPressed: () => c.send({'cmd': 'signinFresh'}),
                  child: Text(t('Another account')),
                ),
              ),
              FilledButton.tonal(onPressed: () => c.send({'cmd': 'signinCancel'}), child: Text(t('Cancel'))),
            ],
          ),
        ],
      ),
    );
  }
}

class _AccountsCard extends StatelessWidget {
  const _AccountsCard({required this.c});

  final Controller c;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(color: scheme.surfaceContainerLow, borderRadius: BorderRadius.circular(24)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(14, 8, 8, 10),
            child: Row(
              children: [
                Icon(Icons.group, size: 20, color: scheme.onSurfaceVariant),
                const SizedBox(width: 10),
                Text(t('Accounts'), style: TextStyle(fontSize: 14, color: scheme.onSurface).weight(500)),
                const SizedBox(width: 10),
                Container(
                  constraints: const BoxConstraints(minWidth: 26),
                  height: 22,
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  alignment: Alignment.center,
                  decoration: BoxDecoration(color: scheme.secondaryContainer, borderRadius: BorderRadius.circular(11)),
                  child: Text(
                    '${c.accounts.length}',
                    key: const Key('account-count'),
                    style: TextStyle(fontSize: 12, color: scheme.onSecondaryContainer).weight(600),
                  ),
                ),
                const Spacer(),
                IconButton(
                  tooltip: t('Add account'),
                  color: scheme.onSurfaceVariant,
                  onPressed: () => showAddAccount(context, c),
                  icon: const Icon(Icons.person_add),
                ),
              ],
            ),
          ),
          if (c.accounts.isEmpty)
            _Empty(c: c)
          else
            LayoutBuilder(
              builder: (context, box) {
                const gap = 4.0, min = 300.0;
                final cols = math.max(1, ((box.maxWidth + gap) / (min + gap)).floor());
                final w = (box.maxWidth - gap * (cols - 1)) / cols;
                return Wrap(
                  spacing: gap,
                  runSpacing: gap,
                  children: [
                    for (var i = 0; i < c.accounts.length; i++)
                      SizedBox(
                        width: w,
                        child: AccountTile(key: ValueKey(c.accounts[i].id), c: c, a: c.accounts[i], first: i == 0),
                      ),
                  ],
                );
              },
            ),
        ],
      ),
    );
  }
}

/// An account: its avatar, name, service and state, with Play and a menu.
/// A click selects it, a double click plays.
class AccountTile extends StatefulWidget {
  const AccountTile({super.key, required this.c, required this.a, this.first = false});

  final Controller c;
  final Account a;
  final bool first;

  @override
  State<AccountTile> createState() => _AccountTileState();
}

class _AccountTileState extends State<AccountTile> {
  var _hover = false, _pressed = false;
  DateTime? _lastTap;

  Controller get c => widget.c;
  Account get a => widget.a;

  void _tap() {
    final now = DateTime.now();
    final double = _lastTap != null && now.difference(_lastTap!) < const Duration(milliseconds: 400);
    _lastTap = double ? null : now;
    c.select(a.id);
    if (double) c.play(a);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final on = a.id == c.selected;
    final op = c.accountOp(a.id);
    final taken = op != null;
    final fg = on ? scheme.onPrimaryContainer : scheme.onSurface;
    final r = _pressed
        ? 12.0
        : on
        ? 22.0
        : 16.0;
    final radius = BorderRadius.only(
      topLeft: Radius.circular(widget.first && !_pressed && !on ? 20 : r),
      topRight: Radius.circular(r),
      bottomLeft: Radius.circular(r),
      bottomRight: Radius.circular(r),
    );
    final canPlay = c.canPlay(a);
    final showPlay = canPlay && (on || _hover);
    final service = serviceColor(context, a.isFX);
    final tags = <Widget>[
      Tag(a.providerName, color: service),
      if (a.branch != null && a.branch!.isNotEmpty) Tag(a.branch!),
      if (a.language != null && a.language!.isNotEmpty) Tag(a.language!, tooltip: t('Game language')),
    ];
    final Widget? state = op != null
        ? Expanded(
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Pulse(color: on ? fg : scheme.primary, size: 6),
                const SizedBox(width: 6),
                Flexible(
                  child: Text(
                    tb(op.title),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 12.5, color: on ? fg : scheme.primary),
                  ),
                ),
              ],
            ),
          )
        : a.login != a.name
        ? Expanded(
            child: Text(
              a.login,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 12.5, color: on ? fg.withValues(alpha: 0.8) : scheme.onSurfaceVariant),
            ),
          )
        : null;
    return MouseRegion(
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: AnimatedContainer(
        duration: motion,
        curve: spring,
        constraints: const BoxConstraints(minHeight: 72),
        decoration: BoxDecoration(
          color: on ? scheme.primaryContainer : scheme.surfaceContainerHighest,
          borderRadius: radius,
        ),
        child: Material(
          type: MaterialType.transparency,
          child: InkWell(
            borderRadius: radius,
            onTap: _tap,
            onHighlightChanged: (v) => setState(() => _pressed = v),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(14, 12, 8, 12),
              child: Row(
                children: [
                  CookieAvatar(letter: a.name.trim().isEmpty ? '?' : a.name.trim()[0].toUpperCase(), color: service),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Row(
                          children: [
                            if (a.last)
                              Padding(
                                padding: const EdgeInsets.only(right: 4),
                                child: Icon(Icons.push_pin, size: 15, color: fg.withValues(alpha: 0.8)),
                              ),
                            Flexible(
                              child: Text(
                                a.name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(fontSize: 15, height: 21 / 15, color: fg).weight(500),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 3),
                        // The tags keep their size and are cut at the tile's
                        // edge; the operation or login takes what is left.
                        LayoutBuilder(
                          builder: (context, box) => Row(
                            children: [
                              ConstrainedBox(
                                constraints: BoxConstraints(maxWidth: box.maxWidth),
                                child: UnconstrainedBox(
                                  alignment: Alignment.centerLeft,
                                  constrainedAxis: Axis.vertical,
                                  clipBehavior: Clip.hardEdge,
                                  child: Row(
                                    mainAxisSize: MainAxisSize.min,
                                    children: [
                                      for (final tag in tags)
                                        Padding(padding: const EdgeInsets.only(right: 8), child: tag),
                                    ],
                                  ),
                                ),
                              ),
                              ?state,
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),
                  IgnorePointer(
                    ignoring: !showPlay,
                    child: AnimatedOpacity(
                      opacity: showPlay ? 1 : 0,
                      duration: effects,
                      curve: effectsCurve,
                      child: AnimatedScale(
                        scale: showPlay ? 1 : 0.7,
                        duration: motionFast,
                        curve: spring,
                        child: _TilePlay(
                          tooltip: t('Play as {name}', {'name': a.name}),
                          onPressed: () {
                            c.select(a.id);
                            c.play(a);
                          },
                        ),
                      ),
                    ),
                  ),
                  MoreMenu(
                    tooltip: t('More'),
                    color: fg.withValues(alpha: 0.85),
                    entries: [
                      MenuEntry(Icons.play_arrow_rounded, t('Play'), () {
                        c.select(a.id);
                        c.play(a);
                      }, enabled: canPlay),
                      if (!a.last)
                        MenuEntry(Icons.push_pin, t('Make main'), () {
                          c.select(a.id);
                          c.send({'cmd': 'pin', 'account': a.id});
                        }),
                      if (a.isFX) ...[
                        MenuEntry(
                          Icons.alt_route,
                          t('Client branches'),
                          () => c.send({'cmd': 'branches', 'account': a.id}),
                          enabled: !taken,
                        ),
                        MenuEntry(
                          Icons.key,
                          t('Activate key'),
                          () => c.send({'cmd': 'key', 'account': a.id}),
                          enabled: !taken,
                        ),
                      ],
                      MenuEntry(Icons.language, t('Game language'), () => showGameLanguage(context, c, a)),
                      MenuEntry(Icons.edit, t('Rename'), () => showRename(context, c, a)),
                      null,
                      MenuEntry(
                        Icons.delete,
                        t('Remove'),
                        () => confirmRemove(context, c, a),
                        danger: true,
                        enabled: !taken,
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _TilePlay extends StatefulWidget {
  const _TilePlay({required this.tooltip, required this.onPressed});

  final String tooltip;
  final VoidCallback onPressed;

  @override
  State<_TilePlay> createState() => _TilePlayState();
}

class _TilePlayState extends State<_TilePlay> {
  var _hover = false, _pressed = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final radius = BorderRadius.circular(_pressed ? 8 : 18);
    return Tooltip(
      message: widget.tooltip,
      child: MouseRegion(
        onEnter: (_) => setState(() => _hover = true),
        onExit: (_) => setState(() => _hover = false),
        child: AnimatedContainer(
          duration: motionFast,
          curve: spring,
          width: 36,
          height: 36,
          decoration: BoxDecoration(
            color: _hover ? scheme.primary : scheme.primary.withValues(alpha: 0.18),
            borderRadius: radius,
          ),
          child: Material(
            type: MaterialType.transparency,
            child: InkWell(
              borderRadius: radius,
              onTap: widget.onPressed,
              onHighlightChanged: (v) => setState(() => _pressed = v),
              child: Icon(Icons.play_arrow_rounded, size: 22, color: _hover ? scheme.onPrimary : scheme.primary),
            ),
          ),
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
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(24, 26, 24, 30),
      child: Column(
        children: [
          Container(
            width: 76,
            height: 76,
            margin: const EdgeInsets.only(bottom: 14),
            decoration: BoxDecoration(color: scheme.tertiaryContainer, borderRadius: BorderRadius.circular(26)),
            child: Icon(Icons.person_add, size: 32, color: scheme.onTertiaryContainer),
          ),
          Text(t('No accounts yet'), style: TextStyle(fontSize: 18, color: scheme.onSurface).weight(500)),
          const SizedBox(height: 8),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Text(
              t('Add a VK Play or FX ID account. Each service keeps its own game folder.'),
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 14, color: scheme.onSurfaceVariant),
            ),
          ),
          const SizedBox(height: 16),
          FilledButton.tonalIcon(
            onPressed: () => showAddAccount(context, c),
            icon: const Icon(Icons.add, size: 18),
            label: Text(t('Add account')),
          ),
        ],
      ),
    );
  }
}
