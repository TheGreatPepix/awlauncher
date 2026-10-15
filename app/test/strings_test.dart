import 'package:awlauncher/i18n/strings.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  tearDown(() => currentLang = 'en');

  test('English text stays as it is', () {
    expect(t('Play'), 'Play');
    expect(tb('Starting Tanker'), 'Starting Tanker');
  });

  test('Russian text and variables', () {
    currentLang = 'ru';
    expect(t('Play'), 'Играть');
    expect(t('AWLauncher {version} is up to date', {'version': 'v1.2.3'}), 'AWLauncher v1.2.3 — последняя версия');
    expect(t('Not in the table'), 'Not in the table');
  });

  test("the core's messages are translated with their names and numbers", () {
    currentLang = 'ru';
    expect(tb('Starting Tanker'), 'Запуск: Tanker');
    expect(tb('Update VK Play from build 442 to 443 (1 patches)?'), 'Обновить VK Play с билда 442 до 443 (патчей: 1)?');
    expect(tb('Remove VK Play and FX ID from D:\\Games?'), 'Удалить VK Play и FX ID из D:\\Games?');
    expect(tb('Wait until “Checking VK Play” finishes'), 'Дождитесь окончания: «Проверка: VK Play»');
  });

  test('the language follows Windows on auto', () {
    expect(resolveLang('auto', 'ru'), 'ru');
    expect(resolveLang('auto', 'de'), 'en');
    expect(resolveLang('en', 'ru'), 'en');
  });
}
