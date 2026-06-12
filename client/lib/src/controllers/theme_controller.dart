import 'package:flutter/material.dart';

import '../services/theme_store.dart';

class ThemeController extends ChangeNotifier {
  ThemeController({ThemeStoreBase? store}) : _store = store ?? ThemeStore();

  final ThemeStoreBase _store;

  ThemeMode mode = ThemeMode.system;

  Future<void> initialize() async {
    mode = await _store.load();
    notifyListeners();
  }

  Future<void> setMode(ThemeMode nextMode) async {
    if (mode == nextMode) {
      return;
    }
    mode = nextMode;
    notifyListeners();
    await _store.save(nextMode);
  }
}
