import '../theme/palette.dart';

List<Map<String, dynamic>> _maps(Object? list) =>
    list is List ? list.whereType<Map<String, dynamic>>().toList() : const [];

List<String> _strings(Object? list) => list is List ? list.map((e) => '$e').toList() : const [];

int _int(Object? v) => v is num ? v.toInt() : 0;

class Account {
  Account.fromJson(Map<String, dynamic> j)
    : id = '${j['id'] ?? ''}',
      name = '${j['name'] ?? ''}',
      login = '${j['login'] ?? ''}',
      service = '${j['service'] ?? ''}',
      provider = '${j['provider'] ?? ''}',
      branch = j['branch'] as String?,
      language = j['language'] as String?,
      autoLanguage = '${j['autoLanguage'] ?? ''}',
      languages = _strings(j['languages']),
      last = j['last'] == true;

  final String id, name, login, service, provider, autoLanguage;
  final String? branch, language;
  final List<String> languages;
  final bool last;

  /// FX ID and Steam accounts both play the FX ID client.
  bool get isFX => provider == 'fxid' || provider == 'steam';

  String get providerName => switch (provider) {
    'steam' => 'Steam',
    'fxid' => 'FX ID',
    _ => 'VK Play',
  };
}

class Operation {
  Operation.fromJson(Map<String, dynamic> j)
    : id = _int(j['id']),
      title = '${j['title'] ?? ''}',
      game = j['game'] == true,
      account = j['account'] as String?,
      cancel = j['cancel'] as String?;

  final int id;
  final String title;
  final bool game;
  final String? account;

  /// How the person can cancel this download: "install" deletes the new
  /// client, "update" asks whether to delete it.
  final String? cancel;
}

class TaskProgress {
  TaskProgress.fromJson(Map<String, dynamic> j)
    : label = '${j['label'] ?? ''}',
      done = _int(j['done']),
      total = _int(j['total']);

  final String label;
  final int done, total;
}

class Progress {
  const Progress.idle()
    : active = false,
      paused = false,
      pausable = false,
      files = false,
      title = '',
      done = 0,
      total = 0,
      speed = 0,
      tasks = const [];

  Progress.fromJson(Map<String, dynamic> j)
    : active = j['active'] == true,
      paused = j['paused'] == true,
      pausable = j['pausable'] == true,
      files = j['files'] == true,
      title = '${j['title'] ?? ''}',
      done = _int(j['done']),
      total = _int(j['total']),
      speed = (j['speed'] as num?)?.toDouble() ?? 0,
      tasks = _maps(j['tasks']).map(TaskProgress.fromJson).toList();

  final bool active, paused, pausable, files;
  final String title;
  final int done, total;
  final double speed;
  final List<TaskProgress> tasks;

  double? get fraction => total > 0 ? (done / total).clamp(0, 1).toDouble() : null;
}

enum ThemeChoice { system, light, dark }

class Prefs {
  const Prefs({
    this.theme = ThemeChoice.system,
    this.palette = const Palette(defaultHue, PaletteStyle.tonal),
    this.palettes = const [],
    this.lang = 'auto',
  });

  factory Prefs.fromJson(Map<String, dynamic> j) => Prefs(
    theme: ThemeChoice.values.firstWhere((t) => t.name == j['theme'], orElse: () => ThemeChoice.system),
    palette: Palette.fromJson(j),
    palettes: _maps(j['palettes']).map(Palette.fromJson).toList(),
    lang: j['lang'] == 'en' || j['lang'] == 'ru' ? j['lang'] as String : 'auto',
  );

  final ThemeChoice theme;
  final Palette palette;
  final List<Palette> palettes;
  final String lang;

  Prefs copyWith({ThemeChoice? theme, Palette? palette, List<Palette>? palettes, String? lang}) => Prefs(
    theme: theme ?? this.theme,
    palette: palette ?? this.palette,
    palettes: palettes ?? this.palettes,
    lang: lang ?? this.lang,
  );

  /// The "prefs" command that saves these preferences into ui.json.
  Map<String, Object?> toCommand() => {
    'cmd': 'prefs',
    'theme': theme.name,
    'hue': palette.hue,
    'style': palette.style.name,
    'palettes': [for (final p in palettes) p.toJson()],
    'lang': lang,
  };
}

class Choice {
  Choice.fromJson(Map<String, dynamic> j)
    : value = '${j['value'] ?? ''}',
      label = '${j['label'] ?? ''}',
      detail = j['detail'] as String?,
      current = j['current'] == true;

  final String value, label;
  final String? detail;
  final bool current;
}

/// A question an operation of the core waits on.
class PromptRequest {
  PromptRequest.fromJson(Map<String, dynamic> j)
    : id = _int(j['id']),
      op = '${j['op'] ?? ''}',
      kind = '${j['kind'] ?? 'confirm'}',
      question = '${j['question'] ?? ''}',
      folder = j['folder'] as String?,
      branch = j['branch'] as String?,
      suggest = j['suggest'] as String?,
      isDefault = j['default'] == true,
      context = _strings(j['context']),
      options = _maps(j['options']).map(Choice.fromJson).toList();

  final int id;
  final String op, kind, question;
  final String? folder, branch, suggest;
  final bool isDefault;
  final List<String> context;
  final List<Choice> options;
}

class UpdateInfo {
  UpdateInfo.fromJson(Map<String, dynamic> j)
    : current = '${j['current'] ?? ''}',
      latest = j['latest'] as String?,
      page = '${j['page'] ?? ''}',
      notes = j['notes'] as String?,
      published = j['published'] as String?,
      available = j['available'] == true,
      canApply = j['canApply'] == true,
      quiet = j['quiet'] == true,
      error = j['error'] as String?;

  final String current, page;
  final String? latest, notes, published, error;
  final bool available, canApply, quiet;
}

/// A short message for a snackbar: a notice or the end of an operation.
class Notice {
  const Notice(this.message, {this.kind = NoticeKind.info, this.title, this.showLogs = false});

  final String message;
  final NoticeKind kind;
  final String? title;

  /// Whether the snackbar offers to open the logs.
  final bool showLogs;
}

enum NoticeKind { info, ok, error }

/// An installed game client.
class Client {
  Client.fromJson(Map<String, dynamic> j)
    : kind = '${j['kind'] ?? ''}',
      branch = '${j['branch'] ?? ''}',
      dir = '${j['dir'] ?? ''}',
      version = '${j['version'] ?? ''}',
      downloads = _int(j['downloads']);

  final String kind, branch, dir, version;
  final int downloads;
}

/// The installed clients and the space they take.
class GameInfo {
  GameInfo.fromJson(Map<String, dynamic> j)
    : dir = '${j['dir'] ?? ''}',
      clients = _maps(j['clients']).map(Client.fromJson).toList(),
      downloads = _int(j['downloads']),
      free = _int(j['free']),
      branchPaths = {for (final e in ((j['branchPaths'] as Map?) ?? const {}).entries) '${e.key}': '${e.value}'};

  final String dir;
  final List<Client> clients;
  final int downloads, free;
  final Map<String, String> branchPaths;
}

/// A client the accounts can download.
class AvailableClient {
  AvailableClient.fromJson(Map<String, dynamic> j)
    : kind = '${j['kind'] ?? ''}',
      branch = '${j['branch'] ?? ''}',
      account = '${j['account'] ?? ''}',
      version = '${j['version'] ?? ''}';

  final String kind, branch, account, version;
}

/// What the core found in a folder the game folder prompt offers.
class FolderInfo {
  FolderInfo.fromJson(Map<String, dynamic> j)
    : prompt = _int(j['prompt']),
      path = '${j['path'] ?? ''}',
      valid = j['valid'] == true,
      free = _int(j['free']),
      drive = '${j['drive'] ?? ''}',
      install = j['install'] == true,
      branch = '${j['branch'] ?? ''}',
      used = j['used'] == true;

  final int prompt, free;
  final String path, drive, branch;
  final bool valid, install, used;
}
