import 'package:flutter/material.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../services/river_api.dart';
import '../services/video_playback_controller.dart';

class VideoPlayerScreen extends StatefulWidget {
  const VideoPlayerScreen({
    super.key,
    required this.api,
    required this.root,
    required this.path,
    required this.title,
  });

  final RiverApi api;
  final String root;
  final String path;
  final String title;

  @override
  State<VideoPlayerScreen> createState() => _VideoPlayerScreenState();
}

class _VideoPlayerScreenState extends State<VideoPlayerScreen> {
  static const double _gesturePixelsPerSecond = 8;
  static const int _gestureMaxSeconds = 120;

  late final VideoPlaybackController _controller;
  Duration? _gestureBasePosition;
  Duration _gestureOffset = Duration.zero;
  double _gestureDelta = 0;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlaybackController(
      api: widget.api,
      root: widget.root,
      path: widget.path,
    )..initialize();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _handleSeekDragStart(DragStartDetails details) {
    if (_controller.duration <= Duration.zero || _controller.seeking) {
      return;
    }
    setState(() {
      _gestureBasePosition = _controller.position;
      _gestureOffset = Duration.zero;
      _gestureDelta = 0;
    });
  }

  void _handleSeekDragUpdate(DragUpdateDetails details) {
    if (_gestureBasePosition == null) {
      return;
    }
    _gestureDelta += details.delta.dx;
    final seconds = (_gestureDelta / _gesturePixelsPerSecond).round().clamp(
      -_gestureMaxSeconds,
      _gestureMaxSeconds,
    );
    setState(() => _gestureOffset = Duration(seconds: seconds));
  }

  void _handleSeekDragEnd(DragEndDetails details) {
    final base = _gestureBasePosition;
    final offset = _gestureOffset;
    setState(() {
      _gestureBasePosition = null;
      _gestureOffset = Duration.zero;
      _gestureDelta = 0;
    });
    if (base != null && offset != Duration.zero) {
      _controller.seekTo(base + offset);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(widget.title),
      ),
      body: AnimatedBuilder(
        animation: _controller,
        builder: (context, _) {
          if (_controller.loading) {
            return const Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  CircularProgressIndicator(),
                  SizedBox(height: 16),
                  Text('正在准备视频…', style: TextStyle(color: Colors.white70)),
                ],
              ),
            );
          }
          if (_controller.errorMessage != null) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.error_outline,
                      color: Colors.white70,
                      size: 48,
                    ),
                    const SizedBox(height: 16),
                    Text(
                      _controller.errorMessage!,
                      textAlign: TextAlign.center,
                      style: const TextStyle(color: Colors.white),
                    ),
                  ],
                ),
              ),
            );
          }
          return Stack(
            fit: StackFit.expand,
            children: [
              Center(
                child: Video(
                  controller: _controller.videoController,
                  fit: BoxFit.contain,
                  controls: null,
                ),
              ),
              Positioned.fill(
                child: GestureDetector(
                  behavior: HitTestBehavior.translucent,
                  onHorizontalDragStart: _handleSeekDragStart,
                  onHorizontalDragUpdate: _handleSeekDragUpdate,
                  onHorizontalDragEnd: _handleSeekDragEnd,
                  onHorizontalDragCancel: () {
                    setState(() {
                      _gestureBasePosition = null;
                      _gestureOffset = Duration.zero;
                      _gestureDelta = 0;
                    });
                  },
                ),
              ),
              _UnifiedVideoControls(controller: _controller),
              if (_gestureBasePosition != null)
                _SeekGestureOverlay(
                  offset: _gestureOffset,
                  target: _clampDuration(
                    _gestureBasePosition! + _gestureOffset,
                    _controller.duration,
                  ),
                ),
            ],
          );
        },
      ),
    );
  }
}

class _UnifiedVideoControls extends StatefulWidget {
  const _UnifiedVideoControls({required this.controller});

  final VideoPlaybackController controller;

  @override
  State<_UnifiedVideoControls> createState() => _UnifiedVideoControlsState();
}

class _UnifiedVideoControlsState extends State<_UnifiedVideoControls> {
  double? _dragValue;

  @override
  Widget build(BuildContext context) {
    final controller = widget.controller;
    final durationMs = controller.duration.inMilliseconds;
    final positionMs =
        _dragValue ?? controller.position.inMilliseconds.toDouble();
    final enabled = durationMs > 0 && !controller.seeking;
    final sliderMax = durationMs > 0 ? durationMs.toDouble() : 1.0;
    final sliderValue = positionMs.clamp(0.0, sliderMax);

    return Positioned(
      left: 0,
      right: 0,
      bottom: 0,
      child: DecoratedBox(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.bottomCenter,
            end: Alignment.topCenter,
            colors: [Colors.black87, Colors.transparent],
          ),
        ),
        child: SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 32, 12, 8),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (controller.seeking)
                  const Padding(
                    padding: EdgeInsets.only(bottom: 8),
                    child: LinearProgressIndicator(minHeight: 2),
                  ),
                Row(
                  children: [
                    IconButton(
                      color: Colors.white,
                      tooltip: '后退 10 秒',
                      onPressed: controller.seeking
                          ? null
                          : () => controller.seekRelative(
                              const Duration(seconds: -10),
                            ),
                      icon: const Icon(Icons.replay_10_rounded),
                    ),
                    IconButton.filled(
                      tooltip: controller.playing ? '暂停' : '播放',
                      onPressed: controller.seeking
                          ? null
                          : controller.togglePlay,
                      icon: Icon(
                        controller.playing
                            ? Icons.pause_rounded
                            : Icons.play_arrow_rounded,
                      ),
                    ),
                    IconButton(
                      color: Colors.white,
                      tooltip: '快进 10 秒',
                      onPressed: controller.seeking
                          ? null
                          : () => controller.seekRelative(
                              const Duration(seconds: 10),
                            ),
                      icon: const Icon(Icons.forward_10_rounded),
                    ),
                    const Spacer(),
                    _SpeedMenu(controller: controller),
                  ],
                ),
                Row(
                  children: [
                    Text(
                      _formatDuration(
                        Duration(milliseconds: sliderValue.round()),
                      ),
                      style: const TextStyle(color: Colors.white70),
                    ),
                    Expanded(
                      child: Slider(
                        value: sliderValue,
                        max: sliderMax,
                        onChangeStart: enabled
                            ? (value) => setState(() => _dragValue = value)
                            : null,
                        onChanged: enabled
                            ? (value) => setState(() => _dragValue = value)
                            : null,
                        onChangeEnd: enabled
                            ? (value) {
                                setState(() => _dragValue = null);
                                widget.controller.seekTo(
                                  Duration(milliseconds: value.round()),
                                );
                              }
                            : null,
                      ),
                    ),
                    Text(
                      _formatDuration(controller.duration),
                      style: const TextStyle(color: Colors.white70),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _SpeedMenu extends StatelessWidget {
  const _SpeedMenu({required this.controller});

  static const speeds = [0.5, 0.75, 1.0, 1.25, 1.5, 2.0];

  final VideoPlaybackController controller;

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<double>(
      tooltip: '播放速度',
      enabled: !controller.seeking,
      initialValue: controller.playbackRate,
      onSelected: controller.setPlaybackRate,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.speed_rounded, color: Colors.white),
            const SizedBox(width: 6),
            Text(
              _formatSpeed(controller.playbackRate),
              style: const TextStyle(color: Colors.white),
            ),
          ],
        ),
      ),
      itemBuilder: (context) => [
        for (final speed in speeds)
          PopupMenuItem(value: speed, child: Text(_formatSpeed(speed))),
      ],
    );
  }
}

class _SeekGestureOverlay extends StatelessWidget {
  const _SeekGestureOverlay({required this.offset, required this.target});

  final Duration offset;
  final Duration target;

  @override
  Widget build(BuildContext context) {
    final seconds = offset.inSeconds;
    final icon = seconds >= 0 ? Icons.forward_rounded : Icons.replay_rounded;
    final label = seconds >= 0 ? '+${seconds}s' : '${seconds}s';
    return Center(
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.72),
          borderRadius: BorderRadius.circular(18),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, color: Colors.white, size: 32),
              const SizedBox(width: 10),
              Text(
                '$label  ${_formatDuration(target)}',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 18,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

String _formatDuration(Duration duration) {
  final totalSeconds = duration.inSeconds;
  final hours = totalSeconds ~/ 3600;
  final minutes = (totalSeconds % 3600) ~/ 60;
  final seconds = totalSeconds % 60;
  String twoDigits(int value) => value.toString().padLeft(2, '0');
  if (hours > 0) {
    return '$hours:${twoDigits(minutes)}:${twoDigits(seconds)}';
  }
  return '$minutes:${twoDigits(seconds)}';
}

String _formatSpeed(double speed) {
  if (speed == speed.roundToDouble()) {
    return '${speed.toStringAsFixed(0)}x';
  }
  return '${speed.toStringAsFixed(2).replaceFirst(RegExp(r'0$'), '')}x';
}

Duration _clampDuration(Duration value, Duration duration) {
  if (value < Duration.zero) {
    return Duration.zero;
  }
  if (duration > Duration.zero && value > duration) {
    return duration;
  }
  return value;
}
