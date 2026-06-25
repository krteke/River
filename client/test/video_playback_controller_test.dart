import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import 'package:river_client/src/models/file_models.dart';
import 'package:river_client/src/services/river_api.dart';
import 'package:river_client/src/services/video_playback_controller.dart';

void main() {
  test('forces seekable playback before opening direct video', () async {
    final api = _FakePlaybackApi(
      const PlayResponse(
        mode: 'direct',
        url: '/api/file?root=media&path=/movie.mp4',
        startSeconds: 12.5,
      ),
    );
    final engine = _FakePlaybackEngine();
    final controller = VideoPlaybackController(
      api: api,
      root: 'media',
      path: '/movie.mp4',
      playbackEngine: engine,
    );

    await controller.initialize();
    controller.dispose();

    expect(engine.calls.take(2), ['forceSeekable', 'open']);
    expect(
      engine.openedMedia?.uri,
      'http://river.test/api/file?root=media&path=/movie.mp4',
    );
    expect(engine.openedMedia?.start, const Duration(milliseconds: 12500));
  });

  test('does not add a client start offset for HLS sessions', () async {
    final api = _FakePlaybackApi(
      const PlayResponse(
        mode: 'hls',
        url: '/stream/session/master.m3u8',
        sessionId: 'session',
        startSeconds: 30,
      ),
    );
    final engine = _FakePlaybackEngine();
    final controller = VideoPlaybackController(
      api: api,
      root: 'media',
      path: '/movie.mkv',
      playbackEngine: engine,
    );

    await controller.initialize();
    controller.dispose();

    expect(engine.calls.take(2), ['forceSeekable', 'open']);
    expect(
      engine.openedMedia?.uri,
      'http://river.test/stream/session/master.m3u8',
    );
    expect(engine.openedMedia?.start, isNull);
  });
}

class _FakePlaybackApi implements VideoPlaybackApi {
  const _FakePlaybackApi(this.response);

  final PlayResponse response;

  @override
  String absoluteUrl(String path) {
    if (path.startsWith('http://') || path.startsWith('https://')) {
      return path;
    }
    return 'http://river.test${path.startsWith('/') ? path : '/$path'}';
  }

  @override
  Future<PlayResponse> playVideo(
    String root,
    String path, {
    double startSeconds = 0,
    String? replaceSessionId,
  }) async {
    return response;
  }

  @override
  Future<void> stopSession(String sessionId) async {}
}

class _FakePlaybackEngine implements PlaybackEngine {
  final _errors = StreamController<String>.broadcast();
  final calls = <String>[];

  Media? openedMedia;

  @override
  VideoController get videoController => throw UnsupportedError(
    'videoController is not used by these controller tests',
  );

  @override
  Stream<String> get errors => _errors.stream;

  @override
  Future<void> forceSeekable() async {
    calls.add('forceSeekable');
  }

  @override
  Future<void> open(Media media) async {
    calls.add('open');
    openedMedia = media;
  }

  @override
  Future<void> dispose() async {
    calls.add('dispose');
    await _errors.close();
  }
}
