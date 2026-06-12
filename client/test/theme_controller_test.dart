import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:river_client/src/controllers/theme_controller.dart';
import 'package:river_client/src/services/theme_store.dart';

void main() {
  test('defaults to the system theme', () {
    final controller = ThemeController(store: _MemoryThemeStore());

    expect(controller.mode, ThemeMode.system);
  });

  test('loads and persists the selected theme', () async {
    final store = _MemoryThemeStore(initialMode: ThemeMode.dark);
    final controller = ThemeController(store: store);

    await controller.initialize();
    expect(controller.mode, ThemeMode.dark);

    await controller.setMode(ThemeMode.light);
    expect(controller.mode, ThemeMode.light);
    expect(store.savedMode, ThemeMode.light);
  });
}

class _MemoryThemeStore implements ThemeStoreBase {
  _MemoryThemeStore({this.initialMode = ThemeMode.system});

  final ThemeMode initialMode;
  ThemeMode? savedMode;

  @override
  Future<ThemeMode> load() async => initialMode;

  @override
  Future<void> save(ThemeMode mode) async {
    savedMode = mode;
  }
}
