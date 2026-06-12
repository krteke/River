import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:river_client/src/controllers/browser_controller.dart';
import 'package:river_client/src/models/server_profile.dart';
import 'package:river_client/src/screens/home_screen.dart';
import 'package:river_client/src/services/server_store.dart';

void main() {
  testWidgets('shows server setup when no server is saved', (tester) async {
    final controller = BrowserController(store: _MemoryServerStore());

    await tester.pumpWidget(
      MaterialApp(home: HomeScreen(controller: controller)),
    );
    await tester.pumpAndSettle();

    expect(find.text('连接你的 River 服务器'), findsOneWidget);
    expect(find.text('添加服务器'), findsOneWidget);
  });
}

class _MemoryServerStore implements ServerStoreBase {
  @override
  Future<List<ServerProfile>> loadServers() async => const [];

  @override
  Future<String?> loadSelectedServerId() async => null;

  @override
  Future<void> saveServers(
    List<ServerProfile> servers,
    String? selectedServerId,
  ) async {}
}
