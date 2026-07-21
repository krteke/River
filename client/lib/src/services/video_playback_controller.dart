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
  List<PlaybackOption> playbackOptions = const [];
  PlaybackOption? selectedPlaybackOption;
  List<EmbeddedSubtitle> subtitles = const [];
  EmbeddedSubtitle? selectedSubtitle;
  String? subtitleMessage;
  bool subtitleLoading = false;
  bool _disposed = false;
  bool _subtitleSelectionInitialized = false;
  Duration _hlsBaseOffset = Duration.zero;

  VideoController get videoController => _playbackEngine.videoController;
  bool get usesServerTimeline =>
      playResponse?.isHls == true && (playResponse?.durationSeconds ?? 0) > 0;

  Future<void> initialize() async {
    loading = true;
    notifyListeners();
    try {
      playbackOptions = await api.getPlaybackOptions();
      selectedPlaybackOption = _initialPlaybackOption(playbackOptions);
      final response = await api.playVideo(
        root,
        path,
        profile: selectedPlaybackOption?.name,
      );
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

  Future<void> selectSubtitle(EmbeddedSubtitle? subtitle) async {
    if (subtitleLoading || subtitle == selectedSubtitle) {
      return;
    }
    if (subtitle != null && !subtitle.text) {
      subtitleMessage = '该字幕是图形字幕，当前无法作为独立字幕播放';
      notifyListeners();
      return;
    }

    final previous = selectedSubtitle;
    subtitleLoading = true;
    subtitleMessage = null;
    notifyListeners();
    try {
      if (subtitle == null) {
        await _playbackEngine.clearSubtitle();
      } else {
        await _loadSubtitle(subtitle);
      }
      selectedSubtitle = subtitle;
    } catch (error) {
      selectedSubtitle = previous;
      subtitleMessage = error is RiverApiException ? error.message : '无法加载字幕';
    } finally {
      subtitleLoading = false;
      if (!_disposed) {
        notifyListeners();
      }
    }
  }

  Future<void> selectPlaybackOption(PlaybackOption option) async {
    if (selectedPlaybackOption?.name == option.name || seeking) {
      return;
    }
    final previousResponse = playResponse;
    final previousSessionId = previousResponse?.sessionId;
    selectedPlaybackOption = option;
    seeking = true;
    errorMessage = null;
    notifyListeners();
    try {
      final next = await api.playVideo(
        root,
        path,
        startSeconds: position.inMilliseconds / 1000,
        profile: option.name,
        replaceSessionId: previousSessionId,
      );
      if (_disposed) {
        if (next.sessionId case final sessionId?) {
          await api.stopSession(sessionId);
        }
        return;
      }
      await _openResponse(next);
      final nextSessionId = next.sessionId;
      if (previousSessionId != null && nextSessionId != previousSessionId) {
        await api.stopSession(previousSessionId);
      }
    } catch (error) {
      selectedPlaybackOption = _optionForResponse(previousResponse);
      errorMessage = error is RiverApiException ? error.message : '切换播放参数失败';
    } finally {
      seeking = false;
      if (!_disposed) {
        notifyListeners();
      }
    }
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
        profile: selectedPlaybackOption?.name,
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
    selectedPlaybackOption = _optionForResponse(response);
    _syncSubtitles(response.subtitles);
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
    await _restoreSubtitle();
  }

  void _syncSubtitles(List<EmbeddedSubtitle> nextSubtitles) {
    subtitles = nextSubtitles;
    if (!_subtitleSelectionInitialized) {
      for (final track in nextSubtitles) {
        if (track.text && track.isDefault) {
          selectedSubtitle = track;
          break;
        }
      }
      for (final track in nextSubtitles) {
        if (selectedSubtitle == null && track.text && track.forced) {
          selectedSubtitle = track;
          break;
        }
      }
      _subtitleSelectionInitialized = true;
      return;
    }
    final selected = selectedSubtitle;
    if (selected == null) {
      return;
    }
    for (final track in nextSubtitles) {
      if (track.index == selected.index && track.text) {
        selectedSubtitle = track;
        return;
      }
    }
    selectedSubtitle = null;
  }

  Future<void> _restoreSubtitle() async {
    await _playbackEngine.clearSubtitle();
    final subtitle = selectedSubtitle;
    if (subtitle == null) {
      return;
    }
    try {
      await _loadSubtitle(subtitle);
    } catch (error) {
      selectedSubtitle = null;
      subtitleMessage = error is RiverApiException ? error.message : '无法加载字幕';
      if (!_disposed) {
        notifyListeners();
      }
    }
  }

  Future<void> _loadSubtitle(EmbeddedSubtitle subtitle) async {
    final content = await api.getSubtitle(root, path, subtitle.index);
    if (content.isEmpty) {
      throw const RiverApiException('服务器返回了空字幕');
    }
    if (_disposed) {
      return;
    }
    await _playbackEngine.setSubtitle(
      content,
      title: subtitle.title,
      language: subtitle.language,
    );
  }

  PlaybackOption? _initialPlaybackOption(List<PlaybackOption> options) {
    if (options.isEmpty) {
      return null;
    }
    for (final option in options) {
      if (option.isDefault) {
        return option;
      }
    }
    return options.first;
  }

  PlaybackOption? _optionForResponse(PlayResponse? response) {
    if (response == null || playbackOptions.isEmpty) {
      return selectedPlaybackOption;
    }
    if (response.profile case final profile?) {
      for (final option in playbackOptions) {
        if (option.name == profile) {
          return option;
        }
      }
    }
    if (!response.isHls) {
      for (final option in playbackOptions) {
        if (option.direct) {
          return option;
        }
      }
    }
    return selectedPlaybackOption;
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

  Future<void> setSubtitle(String content, {String? title, String? language});

  Future<void> clearSubtitle();

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
  Future<void> setSubtitle(String content, {String? title, String? language}) {
    return _player.setSubtitleTrack(
      SubtitleTrack.data(content, title: title, language: language),
    );
  }

  @override
  Future<void> clearSubtitle() {
    return _player.setSubtitleTrack(SubtitleTrack.no());
  }

  @override
  Future<void> dispose() {
    return _player.dispose();
  }
}
