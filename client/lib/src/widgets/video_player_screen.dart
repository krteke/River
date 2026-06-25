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
  late final VideoPlaybackController _controller;

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
          final useTimelineControls = _controller.usesServerTimeline;
          return Stack(
            fit: StackFit.expand,
            children: [
              Center(
                child: Video(
                  controller: _controller.videoController,
                  fit: BoxFit.contain,
                  controls: useTimelineControls ? null : AdaptiveVideoControls,
                ),
              ),
              if (useTimelineControls)
                _SourceTimelineControls(controller: _controller),
            ],
          );
        },
      ),
    );
  }
}

class _SourceTimelineControls extends StatefulWidget {
  const _SourceTimelineControls({required this.controller});

  final VideoPlaybackController controller;

  @override
  State<_SourceTimelineControls> createState() =>
      _SourceTimelineControlsState();
}

class _SourceTimelineControlsState extends State<_SourceTimelineControls> {
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
                      tooltip: controller.playing ? '暂停' : '播放',
                      onPressed: controller.togglePlay,
                      icon: Icon(
                        controller.playing
                            ? Icons.pause_rounded
                            : Icons.play_arrow_rounded,
                      ),
                    ),
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
