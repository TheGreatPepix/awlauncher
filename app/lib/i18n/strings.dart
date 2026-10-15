library;

part 'ru.dart';

/// The language the window speaks: "en" or "ru".
String currentLang = 'en';

/// The language for a preference ("auto", "en" or "ru") and the Windows language.
String resolveLang(String pref, String system) => pref == 'ru' || pref == 'en' ? pref : (system == 'ru' ? 'ru' : 'en');

/// Translates text of the window. {name} in it is replaced from [vars].
String t(String s, [Map<String, Object?>? vars]) {
  var r = currentLang == 'ru' ? (_ru[s] ?? s) : s;
  vars?.forEach((k, v) => r = r.replaceAll('{$k}', '$v'));
  return r;
}

/// Translates a message of the core, which may carry names and numbers.
String tb(String s) {
  if (currentLang != 'ru' || s.isEmpty) return s;
  final known = _ru[s];
  if (known != null) return known;
  for (final (re, fn) in _ruPatterns) {
    final m = re.firstMatch(s);
    if (m != null) return fn(m);
  }
  return s;
}
