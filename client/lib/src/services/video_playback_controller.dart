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
  }) {
    player = Player();
    videoController = VideoController(player);
    _errorSubscription = player.stream.error.listen((message) {
      errorMessage = message;
      notifyListeners();
    });
  }

  final RiverApi api;
  final String root;
  final String path;

  late final Player player;
  late final VideoController videoController;
  late final StreamSubscription<String> _errorSubscription;

  PlayResponse? playResponse;
  String? errorMessage;
  bool loading = true;
  bool _disposed = false;

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
      await player.open(Media(api.absoluteUrl(response.url)), play: true);
      if (!response.isHls && response.startSeconds > 0) {
        await player.seek(
          Duration(milliseconds: (response.startSeconds * 1000).round()),
        );
      }
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
    unawaited(player.dispose());
    super.dispose();
  }
}
