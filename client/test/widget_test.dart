import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:river_client/src/controllers/browser_controller.dart';
import 'package:river_client/src/models/file_models.dart';
import 'package:river_client/src/models/server_profile.dart';
import 'package:river_client/src/screens/home_screen.dart';
import 'package:river_client/src/services/external_playback_service.dart';
import 'package:river_client/src/services/river_api.dart';
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

  testWidgets('shows all video actions in the file context menu', (
    tester,
  ) async {
    final controller = _ReadyBrowserController([_entry('movie.mkv', 'video')]);

    await tester.pumpWidget(
      MaterialApp(home: HomeScreen(controller: controller)),
    );
    await tester.pump();
    await tester.tap(find.text('movie.mkv'), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();

    expect(find.text('下载'), findsOneWidget);
    expect(find.text('复制下载链接'), findsOneWidget);
    expect(find.text('打开'), findsOneWidget);
    expect(find.text('使用外部播放器播放'), findsOneWidget);
    expect(
      tester.getCenter(find.text('下载')).dy,
      lessThan(tester.getCenter(find.text('复制下载链接')).dy),
    );
    expect(
      tester.getCenter(find.text('复制下载链接')).dy,
      lessThan(tester.getCenter(find.text('打开')).dy),
    );
    expect(
      tester.getCenter(find.text('打开')).dy,
      lessThan(tester.getCenter(find.text('使用外部播放器播放')).dy),
    );
  });

  testWidgets('omits external playback for non-video files', (tester) async {
    final controller = _ReadyBrowserController([_entry('notes.txt', 'text')]);

    await tester.pumpWidget(
      MaterialApp(home: HomeScreen(controller: controller)),
    );
    await tester.pump();
    await tester.tap(find.text('notes.txt'), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();

    expect(find.text('下载'), findsOneWidget);
    expect(find.text('复制下载链接'), findsOneWidget);
    expect(find.text('打开'), findsOneWidget);
    expect(find.text('使用外部播放器播放'), findsNothing);
  });

  testWidgets('copies the encoded download URL from the file menu', (
    tester,
  ) async {
    final controller = _ReadyBrowserController([
      _entry('movie.mkv', 'video'),
    ], password: 'secret');
    String? clipboardText;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          clipboardText =
              (call.arguments as Map<Object?, Object?>)['text'] as String?;
        }
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );

    await tester.pumpWidget(
      MaterialApp(home: HomeScreen(controller: controller)),
    );
    await tester.pump();
    await tester.tap(find.text('movie.mkv'), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    await tester.tap(find.text('复制下载链接'));
    await tester.pumpAndSettle();

    expect(
      clipboardText,
      'http://river.test/api/download?root=media&path=%2Fmovie.mkv',
    );
    expect(clipboardText, isNot(contains('secret')));
    expect(find.text('下载链接已复制'), findsOneWidget);
  });

  testWidgets('opens the original video externally from the file menu', (
    tester,
  ) async {
    final controller = _ReadyBrowserController([
      _entry('movie.mkv', 'video'),
    ], password: 'secret');
    final backend = _RecordingExternalPlaybackBackend();

    await tester.pumpWidget(
      MaterialApp(
        home: HomeScreen(
          controller: controller,
          externalPlaybackService: ExternalPlaybackService(backend: backend),
        ),
      ),
    );
    await tester.pump();
    await tester.tap(find.text('movie.mkv'), buttons: kSecondaryMouseButton);
    await tester.pumpAndSettle();
    await tester.tap(find.text('使用外部播放器播放'));
    await tester.pumpAndSettle();

    expect(backend.mpvArguments, [
      '--no-terminal',
      '--force-window=yes',
      '--http-header-fields=X-River-Password: secret',
      'http://river.test/api/file?root=media&path=%2Fmovie.mkv',
    ]);
    expect(find.text('movie.mkv'), findsOneWidget);
  });
}

Map<String, Object> _entry(String name, String type) => {
  'name': name,
  'path': '/$name',
  'type': type,
  'size': 1024,
  'mtime': 1,
};

class _ReadyBrowserController extends BrowserController {
  _ReadyBrowserController(
    List<Map<String, Object>> entries, {
    String password = '',
  }) : super(store: _MemoryServerStore()) {
    final server = ServerProfile(
      id: 'server',
      name: 'Server',
      url: 'http://river.test',
      password: password,
    );
    const root = MediaRoot(id: 'media', name: 'Media');
    servers = [server];
    selectedServer = server;
    api = RiverApi(server.url, password: server.password);
    roots = const [root];
    selectedRoot = root;
    listing = DirectoryListing.fromJson({
      'root_id': root.id,
      'path': '/',
      'parent': '',
      'items': entries,
    });
    loading = false;
  }

  @override
  Future<void> initialize() async {}
}

class _RecordingExternalPlaybackBackend implements ExternalPlaybackBackend {
  List<String>? mpvArguments;

  @override
  bool get isDesktop => true;

  @override
  Future<bool> launchExternalUrl(Uri uri) async => true;

  @override
  Future<void> startMpv(List<String> arguments) async {
    mpvArguments = arguments;
  }
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
