import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:river_client/src/services/external_playback_service.dart';

void main() {
  test('starts desktop mpv with the server authentication header', () async {
    final backend = _FakeExternalPlaybackBackend(isDesktop: true);
    final service = ExternalPlaybackService(backend: backend);

    await service.openOriginal(
      url: 'http://river.test/api/file?root=media&path=/movie.mkv',
      headers: const {'X-River-Password': 'secret'},
    );

    expect(backend.mpvArguments, [
      '--force-window=yes',
      '--http-header-fields=X-River-Password: secret',
      'http://river.test/api/file?root=media&path=/movie.mkv',
    ]);
    expect(backend.launchedUrl, isNull);
  });

  test(
    'does not expose password-protected URLs to a generic launcher',
    () async {
      final backend = _FakeExternalPlaybackBackend(
        isDesktop: true,
        mpvError: ProcessException('mpv', const []),
      );
      final service = ExternalPlaybackService(backend: backend);

      await expectLater(
        service.openOriginal(
          url: 'http://river.test/api/file?root=media&path=/movie.mkv',
          headers: const {'X-River-Password': 'secret'},
        ),
        throwsA(isA<ExternalPlaybackException>()),
      );
      expect(backend.launchedUrl, isNull);
    },
  );

  test('uses the platform launcher for an unprotected mobile server', () async {
    final backend = _FakeExternalPlaybackBackend(isDesktop: false);
    final service = ExternalPlaybackService(backend: backend);

    await service.openOriginal(
      url: 'http://river.test/api/file?root=media&path=/movie.mkv',
    );

    expect(
      backend.launchedUrl,
      Uri.parse('http://river.test/api/file?root=media&path=/movie.mkv'),
    );
  });
}

class _FakeExternalPlaybackBackend implements ExternalPlaybackBackend {
  _FakeExternalPlaybackBackend({required this.isDesktop, this.mpvError});

  @override
  final bool isDesktop;
  final ProcessException? mpvError;
  List<String>? mpvArguments;
  Uri? launchedUrl;

  @override
  Future<void> startMpv(List<String> arguments) async {
    if (mpvError case final error?) {
      throw error;
    }
    mpvArguments = arguments;
  }

  @override
  Future<bool> launchExternalUrl(Uri uri) async {
    launchedUrl = uri;
    return true;
  }
}
