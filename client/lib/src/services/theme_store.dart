import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

abstract interface class ThemeStoreBase {
  Future<ThemeMode> load();

  Future<void> save(ThemeMode mode);
}

class ThemeStore implements ThemeStoreBase {
  static const _themeModeKey = 'river.themeMode';

  final SharedPreferencesAsync _preferences = SharedPreferencesAsync();

  @override
  Future<ThemeMode> load() async {
    final value = await _preferences.getString(_themeModeKey);
    return ThemeMode.values.firstWhere(
      (mode) => mode.name == value,
      orElse: () => ThemeMode.system,
    );
  }

  @override
  Future<void> save(ThemeMode mode) {
    return _preferences.setString(_themeModeKey, mode.name);
  }
}
