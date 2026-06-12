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
          return Center(
            child: Video(
              controller: _controller.videoController,
              fit: BoxFit.contain,
            ),
          );
        },
      ),
    );
  }
}
