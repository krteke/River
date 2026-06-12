import 'package:flutter/material.dart';

import 'controllers/theme_controller.dart';
import 'screens/home_screen.dart';

class RiverApp extends StatefulWidget {
  const RiverApp({super.key});

  @override
  State<RiverApp> createState() => _RiverAppState();
}

class _RiverAppState extends State<RiverApp> {
  late final ThemeController _themeController;

  @override
  void initState() {
    super.initState();
    _themeController = ThemeController()..initialize();
  }

  @override
  void dispose() {
    _themeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _themeController,
      builder: (context, _) => MaterialApp(
        title: 'River',
        debugShowCheckedModeBanner: false,
        theme: _theme(Brightness.light),
        darkTheme: _theme(Brightness.dark),
        themeMode: _themeController.mode,
        home: HomeScreen(themeController: _themeController),
      ),
    );
  }

  ThemeData _theme(Brightness brightness) {
    final colorScheme = ColorScheme.fromSeed(
      seedColor: const Color(0xFF3B6FD8),
      brightness: brightness,
    );
    return ThemeData(
      colorScheme: colorScheme,
      brightness: brightness,
      useMaterial3: true,
      scaffoldBackgroundColor: colorScheme.surface,
      cardTheme: const CardThemeData(
        margin: EdgeInsets.zero,
        elevation: 0,
        clipBehavior: Clip.antiAlias,
      ),
      inputDecorationTheme: const InputDecorationTheme(
        border: OutlineInputBorder(),
      ),
    );
  }
}
