import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../models/file_models.dart';
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
  bool _fullscreen = false;

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
    unawaited(_exitFullscreen());
    _controller.dispose();
    super.dispose();
  }

  Future<void> _toggleFullscreen() async {
    if (_fullscreen) {
      await _exitFullscreen();
      return;
    }
    setState(() => _fullscreen = true);
    await SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    await SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
    ]);
  }

  Future<void> _exitFullscreen() async {
    if (!_fullscreen) {
      return;
    }
    if (mounted) {
      setState(() => _fullscreen = false);
    } else {
      _fullscreen = false;
    }
    await SystemChrome.setPreferredOrientations(DeviceOrientation.values);
    await SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
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
            return _PlaybackErrorView(
              message: _controller.errorMessage!,
              onBack: () => Navigator.of(context).maybePop(),
              onRetry: _controller.retry,
            );
          }
          return Stack(
            fit: StackFit.expand,
            children: [
              Center(
                child: Video(
                  key: const ValueKey('river-video-no-controls'),
                  controller: _controller.videoController,
                  fit: BoxFit.contain,
                  controls: _emptyVideoControls,
                ),
              ),
              Positioned.fill(
                child: GestureDetector(
                  behavior: HitTestBehavior.translucent,
                  onTap: () {},
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
              _UnifiedVideoControls(
                controller: _controller,
                fullscreen: _fullscreen,
                onBack: () => Navigator.of(context).maybePop(),
                onToggleFullscreen: _toggleFullscreen,
              ),
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

class _PlaybackErrorView extends StatelessWidget {
  const _PlaybackErrorView({
    required this.message,
    required this.onBack,
    required this.onRetry,
  });

  final String message;
  final VoidCallback onBack;
  final Future<void> Function() onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, color: Colors.white70, size: 48),
            const SizedBox(height: 16),
            Text(
              message,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.white),
            ),
            const SizedBox(height: 24),
            Wrap(
              spacing: 12,
              runSpacing: 12,
              alignment: WrapAlignment.center,
              children: [
                OutlinedButton.icon(
                  onPressed: onBack,
                  icon: const Icon(Icons.arrow_back_rounded),
                  label: const Text('返回文件列表'),
                ),
                FilledButton.icon(
                  onPressed: () => unawaited(onRetry()),
                  icon: const Icon(Icons.refresh_rounded),
                  label: const Text('重试'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _UnifiedVideoControls extends StatefulWidget {
  const _UnifiedVideoControls({
    required this.controller,
    required this.fullscreen,
    required this.onBack,
    required this.onToggleFullscreen,
  });

  final VideoPlaybackController controller;
  final bool fullscreen;
  final VoidCallback onBack;
  final VoidCallback onToggleFullscreen;

  @override
  State<_UnifiedVideoControls> createState() => _UnifiedVideoControlsState();
}

Widget _emptyVideoControls(VideoState state) => const SizedBox.shrink();

class _UnifiedVideoControlsState extends State<_UnifiedVideoControls> {
  static const _hideDelay = Duration(seconds: 3);

  double? _dragValue;
  bool _visible = true;
  Timer? _hideTimer;

  @override
  void initState() {
    super.initState();
    _scheduleHide();
  }

  @override
  void didUpdateWidget(covariant _UnifiedVideoControls oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.controller.seeking || _dragValue != null) {
      _showControls(scheduleHide: false);
    } else if (_visible) {
      _scheduleHide();
    }
  }

  @override
  void dispose() {
    _hideTimer?.cancel();
    super.dispose();
  }

  void _toggleControls() {
    if (_visible) {
      _hideTimer?.cancel();
      setState(() => _visible = false);
      return;
    }
    _showControls();
  }

  void _showControls({bool scheduleHide = true}) {
    if (!_visible) {
      setState(() => _visible = true);
    }
    if (scheduleHide) {
      _scheduleHide();
    } else {
      _hideTimer?.cancel();
    }
  }

  void _scheduleHide() {
    _hideTimer?.cancel();
    if (widget.controller.seeking || _dragValue != null) {
      return;
    }
    _hideTimer = Timer(_hideDelay, () {
      if (mounted) {
        setState(() => _visible = false);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final controller = widget.controller;
    final durationMs = controller.duration.inMilliseconds;
    final positionMs =
        _dragValue ?? controller.position.inMilliseconds.toDouble();
    final enabled = durationMs > 0 && !controller.seeking;
    final sliderMax = durationMs > 0 ? durationMs.toDouble() : 1.0;
    final sliderValue = positionMs.clamp(0.0, sliderMax);

    return Positioned.fill(
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onTap: _toggleControls,
        child: Stack(
          children: [
            IgnorePointer(
              ignoring: !_visible,
              child: AnimatedOpacity(
                opacity: _visible ? 1 : 0,
                duration: const Duration(milliseconds: 180),
                child: Stack(
                  children: [
                    Positioned.fill(
                      child: GestureDetector(
                        behavior: HitTestBehavior.translucent,
                        onTap: _toggleControls,
                      ),
                    ),
                    const Positioned.fill(
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          gradient: LinearGradient(
                            begin: Alignment.bottomCenter,
                            end: Alignment.topCenter,
                            colors: [Colors.black87, Colors.transparent],
                            stops: [0, 1],
                          ),
                        ),
                      ),
                    ),
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: SafeArea(
                        top: false,
                        bottom: false,
                        child: Padding(
                          padding: const EdgeInsets.fromLTRB(12, 32, 12, 10),
                          child: GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onTap: _scheduleHide,
                            child: _ControlPanel(
                              controller: controller,
                              enabled: enabled,
                              sliderValue: sliderValue,
                              sliderMax: sliderMax,
                              onBack: widget.onBack,
                              onShowControls: _showControls,
                              onDragStart: (value) {
                                setState(() => _dragValue = value);
                                _showControls(scheduleHide: false);
                              },
                              onDragUpdate: (value) {
                                setState(() => _dragValue = value);
                              },
                              onDragEnd: (value) {
                                setState(() => _dragValue = null);
                                widget.controller.seekTo(
                                  Duration(milliseconds: value.round()),
                                );
                                _scheduleHide();
                              },
                              onToggleFullscreen: widget.onToggleFullscreen,
                              fullscreen: widget.fullscreen,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            if (!_visible)
              Positioned(
                left: 0,
                right: 0,
                bottom: 0,
                child: _BottomProgressBar(
                  value: sliderValue,
                  max: sliderMax,
                  seeking: controller.seeking,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _ControlPanel extends StatelessWidget {
  const _ControlPanel({
    required this.controller,
    required this.enabled,
    required this.sliderValue,
    required this.sliderMax,
    required this.onBack,
    required this.onShowControls,
    required this.onDragStart,
    required this.onDragUpdate,
    required this.onDragEnd,
    required this.onToggleFullscreen,
    required this.fullscreen,
  });

  final VideoPlaybackController controller;
  final bool enabled;
  final double sliderValue;
  final double sliderMax;
  final VoidCallback onBack;
  final VoidCallback onShowControls;
  final ValueChanged<double> onDragStart;
  final ValueChanged<double> onDragUpdate;
  final ValueChanged<double> onDragEnd;
  final VoidCallback onToggleFullscreen;
  final bool fullscreen;

  @override
  Widget build(BuildContext context) {
    final bottomInset = MediaQuery.paddingOf(context).bottom;
    final compact = MediaQuery.sizeOf(context).width < 500;
    return IconTheme(
      data: const IconThemeData(color: Colors.white),
      child: DefaultTextStyle(
        style: const TextStyle(color: Colors.white70, fontSize: 12),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (controller.seeking)
              const Padding(
                padding: EdgeInsets.only(bottom: 8),
                child: LinearProgressIndicator(minHeight: 2),
              ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              child: Row(
                children: [
                  IconButton(
                    tooltip: '返回',
                    color: Colors.white,
                    onPressed: onBack,
                    icon: const Icon(Icons.arrow_back_rounded),
                  ),
                  const SizedBox(width: 8),
                  Text(
                    _formatDuration(
                      Duration(milliseconds: sliderValue.round()),
                    ),
                  ),
                  const SizedBox(width: 4),
                  Text('/ ${_formatDuration(controller.duration)}'),
                  const Spacer(),
                  _PlaybackOptionButton(
                    controller: controller,
                    onSelected: onShowControls,
                  ),
                  if (!compact && controller.hasSubtitleChoices) ...[
                    const SizedBox(width: 4),
                    _SubtitleButton(
                      controller: controller,
                      onSelected: onShowControls,
                    ),
                  ],
                  const SizedBox(width: 8),
                  _SpeedMenu(
                    controller: controller,
                    onSelected: onShowControls,
                  ),
                  const SizedBox(width: 12),
                  IconButton(
                    tooltip: fullscreen ? '退出全屏' : '全屏',
                    color: Colors.white,
                    onPressed: onToggleFullscreen,
                    icon: Icon(
                      fullscreen
                          ? Icons.fullscreen_exit_rounded
                          : Icons.fullscreen_rounded,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 2),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                _ControlButton(
                  tooltip: '后退 10 秒',
                  onPressed: controller.seeking
                      ? null
                      : () {
                          onShowControls();
                          controller.seekRelative(const Duration(seconds: -10));
                        },
                  icon: Icons.replay_10_rounded,
                ),
                const SizedBox(width: 20),
                IconButton.filled(
                  tooltip: controller.playing ? '暂停' : '播放',
                  iconSize: 36,
                  onPressed: controller.seeking
                      ? null
                      : () {
                          onShowControls();
                          controller.togglePlay();
                        },
                  icon: Icon(
                    controller.playing
                        ? Icons.pause_rounded
                        : Icons.play_arrow_rounded,
                  ),
                ),
                const SizedBox(width: 20),
                _ControlButton(
                  tooltip: '快进 10 秒',
                  onPressed: controller.seeking
                      ? null
                      : () {
                          onShowControls();
                          controller.seekRelative(const Duration(seconds: 10));
                        },
                  icon: Icons.forward_10_rounded,
                ),
                if (compact && controller.hasSubtitleChoices) ...[
                  const SizedBox(width: 8),
                  _SubtitleButton(
                    controller: controller,
                    onSelected: onShowControls,
                  ),
                ],
              ],
            ),
            const SizedBox(height: 4),
            SliderTheme(
              data: SliderTheme.of(context).copyWith(
                trackHeight: 4,
                thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 6),
                overlayShape: const RoundSliderOverlayShape(overlayRadius: 14),
                activeTrackColor: Colors.white,
                inactiveTrackColor: Colors.white30,
                thumbColor: Colors.white,
                overlayColor: Colors.white24,
                disabledActiveTrackColor: Colors.white54,
                disabledInactiveTrackColor: Colors.white24,
                disabledThumbColor: Colors.white54,
              ),
              child: Slider(
                value: sliderValue,
                max: sliderMax,
                onChangeStart: enabled ? onDragStart : null,
                onChanged: enabled ? onDragUpdate : null,
                onChangeEnd: enabled ? onDragEnd : null,
              ),
            ),
            SizedBox(height: bottomInset),
          ],
        ),
      ),
    );
  }
}

class _PlaybackOptionButton extends StatelessWidget {
  const _PlaybackOptionButton({
    required this.controller,
    required this.onSelected,
  });

  final VideoPlaybackController controller;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      tooltip: '播放参数',
      color: Colors.white,
      onPressed: controller.seeking || controller.playbackOptions.isEmpty
          ? null
          : () {
              onSelected();
              showModalBottomSheet<void>(
                context: context,
                showDragHandle: true,
                isScrollControlled: true,
                builder: (context) => _PlaybackOptionSheet(
                  controller: controller,
                  onSelected: onSelected,
                ),
              );
            },
      icon: const Icon(Icons.tune_rounded),
    );
  }
}

class _SubtitleButton extends StatelessWidget {
  const _SubtitleButton({required this.controller, required this.onSelected});

  final VideoPlaybackController controller;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      tooltip: '字幕',
      color: Colors.white,
      onPressed: controller.seeking || controller.subtitleLoading
          ? null
          : () {
              onSelected();
              showModalBottomSheet<void>(
                context: context,
                showDragHandle: true,
                isScrollControlled: true,
                builder: (context) => controller.usesNativeSubtitleTracks
                    ? _NativeSubtitleSheet(
                        controller: controller,
                        onSelected: onSelected,
                      )
                    : _SubtitleSheet(
                        controller: controller,
                        onSelected: onSelected,
                      ),
              );
            },
      icon: Icon(
        (controller.usesNativeSubtitleTracks
                ? controller.selectedNativeSubtitle?.id == 'no'
                : controller.selectedSubtitle == null)
            ? Icons.subtitles_outlined
            : Icons.subtitles_rounded,
      ),
    );
  }
}

class _SubtitleSheet extends StatelessWidget {
  const _SubtitleSheet({required this.controller, required this.onSelected});

  final VideoPlaybackController controller;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        return SafeArea(
          top: false,
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: MediaQuery.sizeOf(context).height * 0.75,
            ),
            child: SingleChildScrollView(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text('字幕'),
                      subtitle: Text('文本字幕会转换为 WebVTT 后显示'),
                    ),
                    if (controller.subtitleMessage case final message?)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Text(
                          message,
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                          ),
                        ),
                      ),
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(
                        controller.selectedSubtitle == null
                            ? Icons.check_rounded
                            : null,
                      ),
                      title: const Text('关闭字幕'),
                      onTap: controller.seeking || controller.subtitleLoading
                          ? null
                          : () async {
                              await controller.selectSubtitle(null);
                              onSelected();
                              if (context.mounted) {
                                Navigator.of(context).pop();
                              }
                            },
                    ),
                    for (final subtitle in controller.subtitles)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        enabled:
                            subtitle.text &&
                            !controller.seeking &&
                            !controller.subtitleLoading,
                        leading: Icon(
                          controller.selectedSubtitle?.index == subtitle.index
                              ? Icons.check_rounded
                              : subtitle.text
                              ? Icons.subtitles_outlined
                              : Icons.image_not_supported_outlined,
                        ),
                        title: Text(subtitle.label),
                        subtitle: subtitle.text
                            ? Text(subtitle.codec)
                            : Text('${subtitle.codec} 图形字幕，暂不支持'),
                        onTap:
                            subtitle.text &&
                                !controller.seeking &&
                                !controller.subtitleLoading
                            ? () async {
                                await controller.selectSubtitle(subtitle);
                                onSelected();
                                if (controller.subtitleMessage == null &&
                                    context.mounted) {
                                  Navigator.of(context).pop();
                                }
                              }
                            : null,
                      ),
                    if (controller.subtitleLoading)
                      const Padding(
                        padding: EdgeInsets.only(top: 8),
                        child: LinearProgressIndicator(),
                      ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _NativeSubtitleSheet extends StatelessWidget {
  const _NativeSubtitleSheet({
    required this.controller,
    required this.onSelected,
  });

  final VideoPlaybackController controller;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        return SafeArea(
          top: false,
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: MediaQuery.sizeOf(context).height * 0.75,
            ),
            child: SingleChildScrollView(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text('内嵌字幕'),
                      subtitle: Text('原画直连，保留字幕格式与特效'),
                    ),
                    _nativeSubtitleTile(
                      context,
                      title: '自动选择',
                      trackId: 'auto',
                    ),
                    _nativeSubtitleTile(context, title: '关闭字幕', trackId: 'no'),
                    for (final track in controller.nativeSubtitles)
                      _nativeSubtitleTile(
                        context,
                        title: _nativeSubtitleTitle(track),
                        subtitle: track.codec,
                        track: track,
                      ),
                    if (controller.subtitleLoading)
                      const Padding(
                        padding: EdgeInsets.only(top: 8),
                        child: LinearProgressIndicator(),
                      ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _nativeSubtitleTile(
    BuildContext context, {
    required String title,
    String? subtitle,
    String? trackId,
    SubtitleTrack? track,
  }) {
    final selected =
        controller.selectedNativeSubtitle?.id == (trackId ?? track!.id);
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: Icon(selected ? Icons.check_rounded : Icons.subtitles_outlined),
      title: Text(title),
      subtitle: subtitle == null || subtitle.isEmpty ? null : Text(subtitle),
      enabled: !controller.subtitleLoading,
      onTap: controller.subtitleLoading
          ? null
          : () async {
              await controller.selectNativeSubtitle(
                track ??
                    (trackId == 'no'
                        ? const SubtitleTrack('no', null, null)
                        : const SubtitleTrack('auto', null, null)),
              );
              onSelected();
              if (context.mounted) {
                Navigator.of(context).pop();
              }
            },
    );
  }

  String _nativeSubtitleTitle(SubtitleTrack track) {
    final parts = <String>[
      if (track.title != null && track.title!.isNotEmpty) track.title!,
      if (track.language != null && track.language!.isNotEmpty) track.language!,
      if ((track.title == null || track.title!.isEmpty) &&
          (track.language == null || track.language!.isEmpty))
        '字幕 ${track.id}',
    ];
    return parts.join(' · ');
  }
}

class _PlaybackOptionSheet extends StatefulWidget {
  const _PlaybackOptionSheet({
    required this.controller,
    required this.onSelected,
  });

  final VideoPlaybackController controller;
  final VoidCallback onSelected;

  @override
  State<_PlaybackOptionSheet> createState() => _PlaybackOptionSheetState();
}

class _PlaybackOptionSheetState extends State<_PlaybackOptionSheet> {
  static const _originalResolution = '原画';

  late String _resolution;
  String? _codec;
  String? _bitrate;

  List<PlaybackOption> get _options => widget.controller.playbackOptions;

  @override
  void initState() {
    super.initState();
    final selected =
        widget.controller.selectedPlaybackOption ??
        _firstDefaultOption() ??
        _options.first;
    _resolution = _resolutionOf(selected);
    _codec = selected.codec;
    _bitrate = selected.bitrate;
  }

  @override
  Widget build(BuildContext context) {
    final selected = _selectedOption();
    final isOriginal = _resolution == _originalResolution;
    final codecs = _uniqueStrings(
      _options
          .where(
            (option) => !option.direct && _resolutionOf(option) == _resolution,
          )
          .map((option) => option.codec),
    );
    final bitrates = _uniqueStrings(
      _options
          .where(
            (option) =>
                !option.direct &&
                _resolutionOf(option) == _resolution &&
                option.codec == _codec,
          )
          .map((option) => option.bitrate),
    );

    final maxHeight = MediaQuery.sizeOf(context).height * 0.82;
    return SafeArea(
      child: ConstrainedBox(
        constraints: BoxConstraints(maxHeight: maxHeight),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('播放参数', style: Theme.of(context).textTheme.titleSmall),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                key: ValueKey('resolution-$_resolution'),
                initialValue: _resolution,
                decoration: const InputDecoration(
                  labelText: '分辨率',
                  isDense: true,
                ),
                items: [
                  for (final resolution in _resolutions())
                    DropdownMenuItem(
                      value: resolution,
                      child: Text(resolution),
                    ),
                ],
                onChanged: (value) {
                  if (value == null) {
                    return;
                  }
                  setState(() {
                    _resolution = value;
                    final option = _firstOptionForResolution(value);
                    _codec = option?.codec;
                    _bitrate = option?.bitrate;
                  });
                },
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                key: ValueKey('codec-$isOriginal-$_resolution-$_codec'),
                initialValue: isOriginal ? null : _codec,
                decoration: const InputDecoration(
                  labelText: '编码',
                  isDense: true,
                ),
                items: [
                  for (final codec in codecs)
                    DropdownMenuItem(value: codec, child: Text(codec)),
                ],
                onChanged: isOriginal
                    ? null
                    : (value) {
                        if (value == null) {
                          return;
                        }
                        setState(() {
                          _codec = value;
                          _bitrate = _firstOptionForCodec(value)?.bitrate;
                        });
                      },
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                key: ValueKey(
                  'bitrate-$isOriginal-$_resolution-$_codec-$_bitrate',
                ),
                initialValue: isOriginal ? null : _bitrate,
                decoration: const InputDecoration(
                  labelText: '码率',
                  isDense: true,
                ),
                items: [
                  for (final bitrate in bitrates)
                    DropdownMenuItem(value: bitrate, child: Text(bitrate)),
                ],
                onChanged: isOriginal
                    ? null
                    : (value) => setState(() => _bitrate = value),
              ),
              const SizedBox(height: 12),
              FilledButton(
                onPressed:
                    selected == null ||
                        selected.name ==
                            widget.controller.selectedPlaybackOption?.name
                    ? null
                    : () {
                        Navigator.of(context).pop();
                        widget.onSelected();
                        widget.controller.selectPlaybackOption(selected);
                      },
                child: Text(selected == null ? '没有匹配的预设' : '应用'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  List<String> _resolutions() {
    return _uniqueStrings(_options.map(_resolutionOf));
  }

  PlaybackOption? _selectedOption() {
    for (final option in _options) {
      if (option.direct) {
        if (_resolution == _originalResolution) {
          return option;
        }
        continue;
      }
      if (_resolutionOf(option) == _resolution &&
          option.codec == _codec &&
          option.bitrate == _bitrate) {
        return option;
      }
    }
    return null;
  }

  PlaybackOption? _firstDefaultOption() {
    for (final option in _options) {
      if (option.isDefault) {
        return option;
      }
    }
    return null;
  }

  PlaybackOption? _firstOptionForResolution(String resolution) {
    for (final option in _options) {
      if (_resolutionOf(option) == resolution) {
        return option;
      }
    }
    return null;
  }

  PlaybackOption? _firstOptionForCodec(String codec) {
    for (final option in _options) {
      if (!option.direct &&
          _resolutionOf(option) == _resolution &&
          option.codec == codec) {
        return option;
      }
    }
    return null;
  }

  static String _resolutionOf(PlaybackOption option) {
    return option.direct
        ? _originalResolution
        : option.resolution ?? option.name;
  }

  static List<String> _uniqueStrings(Iterable<String?> values) {
    final seen = <String>{};
    final result = <String>[];
    for (final value in values) {
      if (value == null || value.isEmpty || !seen.add(value)) {
        continue;
      }
      result.add(value);
    }
    return result;
  }
}

class _ControlButton extends StatelessWidget {
  const _ControlButton({
    required this.tooltip,
    required this.onPressed,
    required this.icon,
  });

  final String tooltip;
  final VoidCallback? onPressed;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      tooltip: tooltip,
      iconSize: 30,
      color: Colors.white,
      onPressed: onPressed,
      icon: Icon(icon),
    );
  }
}

class _BottomProgressBar extends StatelessWidget {
  const _BottomProgressBar({
    required this.value,
    required this.max,
    required this.seeking,
  });

  final double value;
  final double max;
  final bool seeking;

  @override
  Widget build(BuildContext context) {
    final progress = max <= 0 ? 0.0 : (value / max).clamp(0.0, 1.0);
    return LinearProgressIndicator(
      minHeight: 3,
      value: seeking ? null : progress,
      backgroundColor: Colors.white24,
      valueColor: const AlwaysStoppedAnimation<Color>(Colors.white),
    );
  }
}

class _SpeedMenu extends StatelessWidget {
  const _SpeedMenu({required this.controller, required this.onSelected});

  static const speeds = [0.5, 0.75, 1.0, 1.25, 1.5, 2.0];

  final VideoPlaybackController controller;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<double>(
      tooltip: '播放速度',
      enabled: !controller.seeking,
      initialValue: controller.playbackRate,
      onSelected: (speed) {
        onSelected();
        controller.setPlaybackRate(speed);
      },
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
