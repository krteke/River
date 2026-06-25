import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../models/file_models.dart';
import 'river_api.dart';

class VideoPlaybackController extends ChangeNotifier {
  VideoPlaybackController({
    required this.api,
    required this.root,
    required this.path,
    PlaybackEngine? playbackEngine,
  }) {
    _playbackEngine = playbackEngine ?? MediaKitPlaybackEngine();
    _errorSubscription = _playbackEngine.errors.listen((message) {
      errorMessage = message;
      notifyListeners();
    });
    _positionSubscription = _playbackEngine.positions.listen((value) {
      position = _displayPosition(value);
      notifyListeners();
    });
    _durationSubscription = _playbackEngine.durations.listen((value) {
      if (playResponse?.isHls != true && value > Duration.zero) {
        duration = value;
        notifyListeners();
      }
    });
    _playingSubscription = _playbackEngine.playingChanges.listen((value) {
      playing = value;
      notifyListeners();
    });
  }

  final VideoPlaybackApi api;
  final String root;
  final String path;

  late final PlaybackEngine _playbackEngine;
  late final StreamSubscription<String> _errorSubscription;
  late final StreamSubscription<Duration> _positionSubscription;
  late final StreamSubscription<Duration> _durationSubscription;
  late final StreamSubscription<bool> _playingSubscription;

  PlayResponse? playResponse;
  String? errorMessage;
  bool loading = true;
  bool seeking = false;
  bool playing = false;
  double playbackRate = 1;
  Duration position = Duration.zero;
  Duration duration = Duration.zero;
  bool _disposed = false;
  Duration _hlsBaseOffset = Duration.zero;

  VideoController get videoController => _playbackEngine.videoController;
  bool get usesServerTimeline =>
      playResponse?.isHls == true && (playResponse?.durationSeconds ?? 0) > 0;

  Future<void> initialize() async {
    loading = true;
    notifyListeners();
    try {
      final response = await api.playVideo(root, path);
      if (_disposed) {
        if (response.sessionId case final sessionId?) {
          await api.stopSession(sessionId);
        }
        return;
      }
      await _openResponse(response);
    } catch (error) {
      errorMessage = error is RiverApiException ? error.message : '播放器初始化失败';
    } finally {
      loading = false;
      if (!_disposed) {
        notifyListeners();
      }
    }
  }

  Future<void> togglePlay() {
    return _playbackEngine.playOrPause();
  }

  Future<void> setPlaybackRate(double rate) async {
    if (rate <= 0) {
      return;
    }
    playbackRate = rate;
    notifyListeners();
    await _playbackEngine.setRate(rate);
  }

  Future<void> seekRelative(Duration offset) {
    return seekTo(position + offset);
  }

  Future<void> seekTo(Duration target) async {
    final response = playResponse;
    if (response == null) {
      return;
    }
    final clamped = _clampToDuration(target);
    position = clamped;
    notifyListeners();

    if (!response.isHls) {
      await _playbackEngine.seek(clamped);
      return;
    }

    seeking = true;
    errorMessage = null;
    notifyListeners();
    try {
      final next = await api.playVideo(
        root,
        path,
        startSeconds: clamped.inMilliseconds / 1000,
        replaceSessionId: response.sessionId,
      );
      if (_disposed) {
        if (next.sessionId case final sessionId?) {
          await api.stopSession(sessionId);
        }
        return;
      }
      await _openResponse(next);
    } catch (error) {
      errorMessage = error is RiverApiException ? error.message : '跳转播放位置失败';
    } finally {
      seeking = false;
      if (!_disposed) {
        notifyListeners();
      }
    }
  }

  @override
  void dispose() {
    _disposed = true;
    final sessionId = playResponse?.sessionId;
    if (sessionId != null) {
      unawaited(api.stopSession(sessionId));
    }
    unawaited(_errorSubscription.cancel());
    unawaited(_positionSubscription.cancel());
    unawaited(_durationSubscription.cancel());
    unawaited(_playingSubscription.cancel());
    unawaited(_playbackEngine.dispose());
    super.dispose();
  }

  Future<void> _openResponse(PlayResponse response) async {
    playResponse = response;
    _hlsBaseOffset = response.isHls
        ? _durationFromSeconds(response.startSeconds)
        : Duration.zero;
    duration = _durationFromSeconds(response.durationSeconds);
    position = response.isHls
        ? _hlsBaseOffset
        : _durationFromSeconds(response.startSeconds);
    playing = _playbackEngine.playing;
    notifyListeners();

    await _playbackEngine.forceSeekable();
    await _playbackEngine.open(_mediaFor(response));
    if (playbackRate != 1) {
      await _playbackEngine.setRate(playbackRate);
    }
  }

  Media _mediaFor(PlayResponse response) {
    final start = !response.isHls && response.startSeconds > 0
        ? Duration(milliseconds: (response.startSeconds * 1000).round())
        : null;
    return Media(
      api.absoluteUrl(response.url),
      start: start,
      httpHeaders: api.authHeaders,
    );
  }

  Duration _displayPosition(Duration enginePosition) {
    final value = playResponse?.isHls == true
        ? _hlsBaseOffset + enginePosition
        : enginePosition;
    return _clampToDuration(value);
  }

  Duration _clampToDuration(Duration value) {
    if (value < Duration.zero) {
      return Duration.zero;
    }
    if (duration > Duration.zero && value > duration) {
      return duration;
    }
    return value;
  }
}

Duration _durationFromSeconds(double seconds) {
  if (seconds <= 0) {
    return Duration.zero;
  }
  return Duration(milliseconds: (seconds * 1000).round());
}

abstract interface class PlaybackEngine {
  VideoController get videoController;

  Stream<String> get errors;

  Stream<Duration> get positions;

  Stream<Duration> get durations;

  Stream<bool> get playingChanges;

  bool get playing;

  Future<void> forceSeekable();

  Future<void> open(Media media);

  Future<void> playOrPause();

  Future<void> seek(Duration position);

  Future<void> setRate(double rate);

  Future<void> dispose();
}

class MediaKitPlaybackEngine implements PlaybackEngine {
  MediaKitPlaybackEngine({Player? player}) : _player = player ?? Player() {
    videoController = VideoController(_player);
  }

  final Player _player;

  @override
  late final VideoController videoController;

  @override
  Stream<String> get errors => _player.stream.error;

  @override
  Stream<Duration> get positions => _player.stream.position;

  @override
  Stream<Duration> get durations => _player.stream.duration;

  @override
  Stream<bool> get playingChanges => _player.stream.playing;

  @override
  bool get playing => _player.state.playing;

  @override
  Future<void> forceSeekable() async {
    final platform = _player.platform;
    if (platform is NativePlayer) {
      await platform.setProperty('force-seekable', 'yes');
    }
  }

  @override
  Future<void> open(Media media) {
    return _player.open(media, play: true);
  }

  @override
  Future<void> playOrPause() {
    return _player.playOrPause();
  }

  @override
  Future<void> seek(Duration position) {
    return _player.seek(position);
  }

  @override
  Future<void> setRate(double rate) {
    return _player.setRate(rate);
  }

  @override
  Future<void> dispose() {
    return _player.dispose();
  }
}
