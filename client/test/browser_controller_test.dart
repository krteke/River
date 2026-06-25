import 'package:flutter_test/flutter_test.dart';
import 'package:river_client/src/controllers/browser_controller.dart';
import 'package:river_client/src/models/file_models.dart';
import 'package:river_client/src/models/server_profile.dart';
import 'package:river_client/src/services/server_store.dart';

void main() {
  test('navigates images using the sorted directory listing order', () async {
    final controller = BrowserController(store: _MemoryServerStore());
    controller.listing = DirectoryListing.fromJson({
      'root_id': 'media',
      'path': '/photos',
      'parent': '/',
      'items': [
        {
          'name': 'zeta.jpg',
          'path': '/photos/zeta.jpg',
          'type': 'image',
          'size': 1,
          'mtime': 1,
        },
        {
          'name': 'notes.txt',
          'path': '/photos/notes.txt',
          'type': 'text',
          'size': 1,
          'mtime': 1,
        },
        {
          'name': 'alpha.jpg',
          'path': '/photos/alpha.jpg',
          'type': 'image',
          'size': 1,
          'mtime': 1,
        },
      ],
    });

    await controller.selectFile(controller.imageEntries.first);
    expect(controller.selectedEntry?.path, '/photos/alpha.jpg');
    expect(controller.canSelectPreviousImage, isFalse);
    expect(controller.canSelectNextImage, isTrue);

    await controller.selectNextImage();
    expect(controller.selectedEntry?.path, '/photos/zeta.jpg');
    expect(controller.canSelectPreviousImage, isTrue);
    expect(controller.canSelectNextImage, isFalse);

    await controller.selectPreviousImage();
    expect(controller.selectedEntry?.path, '/photos/alpha.jpg');
  });

  test('reports when browser can navigate to parent directory', () {
    final controller = BrowserController(store: _MemoryServerStore());
    controller.listing = DirectoryListing.fromJson({
      'root_id': 'media',
      'path': '/photos',
      'parent': '/',
      'items': [],
    });

    expect(controller.canOpenParentDirectory, isTrue);

    controller.listing = DirectoryListing.fromJson({
      'root_id': 'media',
      'path': '/',
      'parent': '',
      'items': [],
    });

    expect(controller.canOpenParentDirectory, isFalse);
  });

  test('sorts entries by name and size in both directions', () {
    final controller = BrowserController(store: _MemoryServerStore());
    controller.listing = DirectoryListing.fromJson({
      'root_id': 'media',
      'path': '/',
      'parent': '',
      'items': [
        {
          'name': 'b.mp4',
          'path': '/b.mp4',
          'type': 'video',
          'size': 200,
          'mtime': 1,
        },
        {
          'name': 'Folder',
          'path': '/Folder',
          'type': 'directory',
          'size': 0,
          'mtime': 1,
        },
        {
          'name': 'a.mp4',
          'path': '/a.mp4',
          'type': 'video',
          'size': 300,
          'mtime': 1,
        },
      ],
    });

    expect(controller.sortedEntries.map((entry) => entry.name), [
      'Folder',
      'a.mp4',
      'b.mp4',
    ]);

    controller.setSort(FileSortField.name, false);
    expect(controller.sortedEntries.map((entry) => entry.name), [
      'Folder',
      'b.mp4',
      'a.mp4',
    ]);

    controller.setSort(FileSortField.size, true);
    expect(controller.sortedEntries.map((entry) => entry.name), [
      'Folder',
      'b.mp4',
      'a.mp4',
    ]);

    controller.setSort(FileSortField.size, false);
    expect(controller.sortedEntries.map((entry) => entry.name), [
      'Folder',
      'a.mp4',
      'b.mp4',
    ]);
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
