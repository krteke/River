import 'package:flutter_test/flutter_test.dart';
import 'package:river_client/src/models/file_models.dart';
import 'package:river_client/src/services/river_api.dart';

void main() {
  test('normalizes server URL', () {
    expect(
      RiverApi.normalizeServerUrl('192.168.1.20:8080/'),
      'http://192.168.1.20:8080',
    );
    expect(
      RiverApi.normalizeServerUrl('https://example.com///'),
      'https://example.com',
    );
  });

  test('sorts directories before files', () {
    final listing = DirectoryListing.fromJson({
      'root_id': 'media',
      'path': '/',
      'parent': '',
      'items': [
        {
          'name': 'movie.mp4',
          'path': '/movie.mp4',
          'type': 'video',
          'size': 10,
          'mtime': 1,
        },
        {
          'name': 'Folder',
          'path': '/Folder',
          'type': 'directory',
          'size': 0,
          'mtime': 1,
        },
      ],
    });

    expect(listing.items.first.type, RiverFileType.directory);
    expect(listing.items.last.type, RiverFileType.video);
  });

  test('parses HLS play response', () {
    final response = PlayResponse.fromJson({
      'mode': 'hls',
      'url': '/stream/session/master.m3u8',
      'session_id': 'session',
      'profile': '1080p_8m',
    });

    expect(response.isHls, isTrue);
    expect(response.sessionId, 'session');
  });
}
