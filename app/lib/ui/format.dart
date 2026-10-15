import '../i18n/strings.dart';

const _kib = 1 << 10, _mib = 1 << 20, _gib = 1 << 30;

String formatBytes(num n) {
  if (n >= _gib) return '${(n / _gib).toStringAsFixed(1)} GiB';
  if (n >= _mib) return '${(n / _mib).toStringAsFixed(1)} MiB';
  if (n >= _kib) return '${(n / _kib).round()} KiB';
  return '$n B';
}

String formatPair(num done, num total) {
  if (total >= _gib) return '${(done / _gib).toStringAsFixed(1)} / ${(total / _gib).toStringAsFixed(1)} GiB';
  if (total >= _mib) return '${(done / _mib).toStringAsFixed(1)} / ${(total / _mib).toStringAsFixed(1)} MiB';
  if (total >= _kib) return '${(done / _kib).round()} / ${(total / _kib).round()} KiB';
  return '$done / $total B';
}

String formatDuration(num seconds) {
  final sec = seconds.round();
  final h = sec ~/ 3600, m = sec ~/ 60 % 60, s = sec % 60;
  final (uh, um, us) = currentLang == 'ru' ? (' ч', ' мин', ' с') : ('h', 'm', 's');
  String two(int v) => v.toString().padLeft(2, '0');
  if (h > 0) return '$h$uh ${two(m)}$um';
  if (m > 0) return '$m$um ${two(s)}$us';
  return '$s$us';
}
