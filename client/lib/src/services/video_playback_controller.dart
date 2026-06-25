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
  }

  final VideoPlaybackApi api;
  final String root;
  final String path;

  late final PlaybackEngine _playbackEngine;
  late final StreamSubscription<String> _errorSubscription;

  PlayResponse? playResponse;
  String? errorMessage;
  bool loading = true;
  bool _disposed = false;

  VideoController get videoController => _playbackEngine.videoController;

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
      playResponse = response;
      await _playbackEngine.forceSeekable();
      await _playbackEngine.open(_mediaFor(response));
    } catch (error) {
      errorMessage = error is RiverApiException ? error.message : '播放器初始化失败';
    } finally {
      loading = false;
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
    unawaited(_playbackEngine.dispose());
    super.dispose();
  }

  Media _mediaFor(PlayResponse response) {
    final start = !response.isHls && response.startSeconds > 0
        ? Duration(milliseconds: (response.startSeconds * 1000).round())
        : null;
    return Media(api.absoluteUrl(response.url), start: start);
  }
}

abstract interface class PlaybackEngine {
  VideoController get videoController;

  Stream<String> get errors;

  Future<void> forceSeekable();

  Future<void> open(Media media);

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
  Future<void> dispose() {
    return _player.dispose();
  }
}
