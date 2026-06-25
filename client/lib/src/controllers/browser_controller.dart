import 'package:flutter/foundation.dart';

import '../models/file_models.dart';
import '../models/server_profile.dart';
import '../services/river_api.dart';
import '../services/server_store.dart';

class BrowserController extends ChangeNotifier {
  BrowserController({ServerStoreBase? store}) : _store = store ?? ServerStore();

  final ServerStoreBase _store;

  List<ServerProfile> servers = const [];
  ServerProfile? selectedServer;
  RiverApi? api;
  List<MediaRoot> roots = const [];
  MediaRoot? selectedRoot;
  DirectoryListing? listing;
  FileEntry? selectedEntry;
  String? textContent;
  String? errorMessage;
  bool loading = true;
  bool loadingPreview = false;

  bool get canOpenParentDirectory => listing != null && listing!.path != '/';

  List<FileEntry> get imageEntries =>
      listing?.items
          .where((entry) => entry.type == RiverFileType.image)
          .toList() ??
      const [];

  int get selectedImageIndex {
    final entry = selectedEntry;
    if (entry == null || entry.type != RiverFileType.image) {
      return -1;
    }
    return imageEntries.indexWhere((image) => image.path == entry.path);
  }

  bool get canSelectPreviousImage => selectedImageIndex > 0;

  bool get canSelectNextImage {
    final index = selectedImageIndex;
    return index >= 0 && index < imageEntries.length - 1;
  }

  Future<void> initialize() async {
    loading = true;
    notifyListeners();
    try {
      servers = await _store.loadServers();
      final selectedId = await _store.loadSelectedServerId();
      selectedServer = servers
          .where((server) => server.id == selectedId)
          .firstOrNull;
      selectedServer ??= servers.firstOrNull;
      if (selectedServer != null) {
        await connect(selectedServer!);
      }
    } catch (error) {
      errorMessage = _message(error);
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<void> testConnection(String url) async {
    await RiverApi(url).checkHealth();
  }

  Future<void> saveServer({
    String? id,
    required String name,
    required String url,
  }) async {
    final normalizedUrl = RiverApi.normalizeServerUrl(url);
    final profile = ServerProfile(
      id: id ?? DateTime.now().microsecondsSinceEpoch.toString(),
      name: name.trim().isEmpty ? Uri.parse(normalizedUrl).host : name.trim(),
      url: normalizedUrl,
    );
    final updated = [...servers];
    final index = updated.indexWhere((server) => server.id == profile.id);
    if (index == -1) {
      updated.add(profile);
    } else {
      updated[index] = profile;
    }
    servers = updated;
    selectedServer = profile;
    await _persist();
    notifyListeners();
    await connect(profile);
  }

  Future<void> deleteServer(ServerProfile server) async {
    servers = servers.where((item) => item.id != server.id).toList();
    if (selectedServer?.id == server.id) {
      selectedServer = servers.firstOrNull;
      _clearBrowser();
    }
    await _persist();
    notifyListeners();
    if (selectedServer != null) {
      await connect(selectedServer!);
    }
  }

  Future<void> connect(ServerProfile server) async {
    loading = true;
    errorMessage = null;
    selectedServer = server;
    _clearBrowser();
    notifyListeners();

    try {
      final nextApi = RiverApi(server.url);
      await nextApi.checkHealth();
      final nextRoots = await nextApi.getRoots();
      api = nextApi;
      roots = nextRoots;
      selectedRoot = nextRoots.firstOrNull;
      await _persist();
      if (selectedRoot != null) {
        listing = await nextApi.listDirectory(selectedRoot!.id, '/');
      }
    } catch (error) {
      api = null;
      errorMessage = _message(error);
      rethrow;
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<void> selectRoot(MediaRoot root) async {
    selectedRoot = root;
    selectedEntry = null;
    textContent = null;
    notifyListeners();
    await openDirectory('/');
  }

  Future<void> openDirectory(String path) async {
    final currentApi = api;
    final root = selectedRoot;
    if (currentApi == null || root == null) {
      return;
    }
    loading = true;
    errorMessage = null;
    notifyListeners();
    try {
      listing = await currentApi.listDirectory(root.id, path);
      selectedEntry = null;
      textContent = null;
    } catch (error) {
      errorMessage = _message(error);
      rethrow;
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<bool> openParentDirectory() async {
    final current = listing;
    if (current == null || current.path == '/') {
      return false;
    }
    await openDirectory(current.parent);
    return true;
  }

  Future<void> refresh() async {
    await openDirectory(listing?.path ?? '/');
  }

  Future<void> selectFile(FileEntry entry) async {
    selectedEntry = entry;
    textContent = null;
    notifyListeners();
    if (entry.type != RiverFileType.text) {
      return;
    }
    final currentApi = api;
    final root = selectedRoot;
    if (currentApi == null || root == null) {
      return;
    }
    loadingPreview = true;
    notifyListeners();
    try {
      textContent = await currentApi.readText(root.id, entry.path);
    } catch (error) {
      errorMessage = _message(error);
      rethrow;
    } finally {
      loadingPreview = false;
      notifyListeners();
    }
  }

  Future<void> selectPreviousImage() async {
    final index = selectedImageIndex;
    if (index <= 0) {
      return;
    }
    await selectFile(imageEntries[index - 1]);
  }

  Future<void> selectNextImage() async {
    final index = selectedImageIndex;
    final images = imageEntries;
    if (index < 0 || index >= images.length - 1) {
      return;
    }
    await selectFile(images[index + 1]);
  }

  void clearError() {
    errorMessage = null;
  }

  Future<void> _persist() {
    return _store.saveServers(servers, selectedServer?.id);
  }

  void _clearBrowser() {
    api = null;
    roots = const [];
    selectedRoot = null;
    listing = null;
    selectedEntry = null;
    textContent = null;
  }

  String _message(Object error) {
    return error is RiverApiException ? error.message : '操作失败，请稍后重试';
  }
}
