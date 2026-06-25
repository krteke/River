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
      responses: [
        const PlayResponse(
          mode: 'direct',
          url: '/api/file?root=media&path=/movie.mp4',
          startSeconds: 12.5,
          durationSeconds: 60,
        ),
      ],
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
    expect(controller.duration, const Duration(seconds: 60));
  });

  test('seeks direct videos relative to the displayed position', () async {
    final api = _FakePlaybackApi(
      responses: [
        const PlayResponse(
          mode: 'direct',
          url: '/api/file?root=media&path=/movie.mp4',
          durationSeconds: 60,
        ),
      ],
    );
    final engine = _FakePlaybackEngine();
    final controller = VideoPlaybackController(
      api: api,
      root: 'media',
      path: '/movie.mp4',
      playbackEngine: engine,
    );

    await controller.initialize();
    engine.emitPosition(const Duration(seconds: 20));
    await Future<void>.delayed(Duration.zero);
    await controller.seekRelative(const Duration(seconds: 10));
    controller.dispose();

    expect(engine.seekedPosition, const Duration(seconds: 30));
  });

  test('updates playback rate through the playback engine', () async {
    final api = _FakePlaybackApi(
      responses: [
        const PlayResponse(
          mode: 'direct',
          url: '/api/file?root=media&path=/movie.mp4',
        ),
      ],
    );
    final engine = _FakePlaybackEngine();
    final controller = VideoPlaybackController(
      api: api,
      root: 'media',
      path: '/movie.mp4',
      playbackEngine: engine,
    );

    await controller.initialize();
    await controller.setPlaybackRate(1.5);
    controller.dispose();

    expect(controller.playbackRate, 1.5);
    expect(engine.rate, 1.5);
  });

  test('does not add a client start offset for HLS sessions', () async {
    final api = _FakePlaybackApi(
      responses: [
        const PlayResponse(
          mode: 'hls',
          url: '/stream/session/master.m3u8',
          sessionId: 'session',
          startSeconds: 30,
          durationSeconds: 120,
        ),
      ],
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
    expect(controller.usesServerTimeline, isTrue);
    expect(controller.position, const Duration(seconds: 30));
    expect(controller.duration, const Duration(seconds: 120));
  });

  test('restarts HLS session when seeking on the source timeline', () async {
    final api = _FakePlaybackApi(
      responses: [
        const PlayResponse(
          mode: 'hls',
          url: '/stream/session-a/master.m3u8',
          sessionId: 'session-a',
          durationSeconds: 120,
        ),
        const PlayResponse(
          mode: 'hls',
          url: '/stream/session-b/master.m3u8',
          sessionId: 'session-b',
          startSeconds: 45,
          durationSeconds: 120,
        ),
      ],
    );
    final engine = _FakePlaybackEngine();
    final controller = VideoPlaybackController(
      api: api,
      root: 'media',
      path: '/movie.mkv',
      playbackEngine: engine,
    );

    await controller.initialize();
    await controller.seekTo(const Duration(seconds: 45));
    engine.emitPosition(const Duration(seconds: 5));
    await Future<void>.delayed(Duration.zero);
    controller.dispose();

    expect(api.calls, hasLength(2));
    expect(api.calls[1].startSeconds, 45);
    expect(api.calls[1].replaceSessionId, 'session-a');
    expect(
      engine.openedMedia?.uri,
      'http://river.test/stream/session-b/master.m3u8',
    );
    expect(controller.position, const Duration(seconds: 50));
  });
}

class _FakePlaybackApi implements VideoPlaybackApi {
  _FakePlaybackApi({required List<PlayResponse> responses})
    : _responses = List.of(responses);

  final List<PlayResponse> _responses;
  final List<_PlayCall> calls = [];
  final List<String> stoppedSessions = [];

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
    calls.add(
      _PlayCall(
        root: root,
        path: path,
        startSeconds: startSeconds,
        replaceSessionId: replaceSessionId,
      ),
    );
    return _responses.removeAt(0);
  }

  @override
  Future<void> stopSession(String sessionId) async {
    stoppedSessions.add(sessionId);
  }
}

class _FakePlaybackEngine implements PlaybackEngine {
  final _errors = StreamController<String>.broadcast();
  final _positions = StreamController<Duration>.broadcast();
  final _durations = StreamController<Duration>.broadcast();
  final _playingChanges = StreamController<bool>.broadcast();
  final calls = <String>[];

  Media? openedMedia;
  Duration? seekedPosition;
  bool _playing = true;
  double rate = 1;

  @override
  VideoController get videoController => throw UnsupportedError(
    'videoController is not used by these controller tests',
  );

  @override
  Stream<String> get errors => _errors.stream;

  @override
  Stream<Duration> get positions => _positions.stream;

  @override
  Stream<Duration> get durations => _durations.stream;

  @override
  Stream<bool> get playingChanges => _playingChanges.stream;

  @override
  bool get playing => _playing;

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
  Future<void> playOrPause() async {
    calls.add('playOrPause');
    _playing = !_playing;
    _playingChanges.add(_playing);
  }

  @override
  Future<void> seek(Duration position) async {
    calls.add('seek');
    seekedPosition = position;
  }

  @override
  Future<void> setRate(double rate) async {
    calls.add('setRate');
    this.rate = rate;
  }

  void emitPosition(Duration position) {
    _positions.add(position);
  }

  @override
  Future<void> dispose() async {
    calls.add('dispose');
    await Future.wait([
      _errors.close(),
      _positions.close(),
      _durations.close(),
      _playingChanges.close(),
    ]);
  }
}

class _PlayCall {
  const _PlayCall({
    required this.root,
    required this.path,
    required this.startSeconds,
    required this.replaceSessionId,
  });

  final String root;
  final String path;
  final double startSeconds;
  final String? replaceSessionId;
}
