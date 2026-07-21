import 'dart:io';

import 'package:url_launcher/url_launcher.dart';

class ExternalPlaybackException implements Exception {
  const ExternalPlaybackException(this.message);

  final String message;

  @override
  String toString() => message;
}

abstract interface class ExternalPlaybackBackend {
  bool get isDesktop;

  Future<void> startMpv(List<String> arguments);

  Future<bool> launchExternalUrl(Uri uri);
}

class ExternalPlaybackService {
  ExternalPlaybackService({ExternalPlaybackBackend? backend})
    : _backend = backend ?? _DefaultExternalPlaybackBackend();

  final ExternalPlaybackBackend _backend;

  Future<void> openOriginal({
    required String url,
    Map<String, String> headers = const {},
  }) async {
    if (_backend.isDesktop) {
      try {
        await _backend.startMpv([
          '--no-terminal',
          '--force-window=yes',
          for (final header in headers.entries)
            '--http-header-fields=${header.key}: ${header.value}',
          url,
        ]);
        return;
      } on ProcessException {
        if (headers.isNotEmpty) {
          throw const ExternalPlaybackException('未找到 mpv。密码服务器的外部播放需要桌面端 mpv。');
        }
      }
    }

    if (headers.isNotEmpty) {
      throw const ExternalPlaybackException('密码服务器无法安全地交给外部播放器，请在桌面端安装 mpv。');
    }
    if (!await _backend.launchExternalUrl(Uri.parse(url))) {
      throw const ExternalPlaybackException('无法打开外部播放器');
    }
  }
}

class _DefaultExternalPlaybackBackend implements ExternalPlaybackBackend {
  @override
  bool get isDesktop =>
      Platform.isLinux || Platform.isMacOS || Platform.isWindows;

  @override
  Future<void> startMpv(List<String> arguments) async {
    await Process.start('mpv', arguments, mode: ProcessStartMode.detached);
  }

  @override
  Future<bool> launchExternalUrl(Uri uri) {
    return launchUrl(uri, mode: LaunchMode.externalApplication);
  }
}
