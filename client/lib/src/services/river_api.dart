import 'package:dio/dio.dart';

import '../models/file_models.dart';

class RiverApiException implements Exception {
  const RiverApiException(this.message, {this.code});

  final String message;
  final String? code;

  @override
  String toString() => message;
}

class RiverApi {
  RiverApi(String serverUrl)
    : baseUrl = normalizeServerUrl(serverUrl),
      _dio = Dio(
        BaseOptions(
          baseUrl: normalizeServerUrl(serverUrl),
          connectTimeout: const Duration(seconds: 8),
          receiveTimeout: const Duration(seconds: 30),
        ),
      );

  final String baseUrl;
  final Dio _dio;

  static String normalizeServerUrl(String value) {
    var url = value.trim();
    if (url.isEmpty) {
      throw const RiverApiException('请输入服务器地址');
    }
    if (!url.contains('://')) {
      url = 'http://$url';
    }
    final uri = Uri.tryParse(url);
    if (uri == null || !uri.hasScheme || uri.host.isEmpty) {
      throw const RiverApiException('服务器地址格式无效');
    }
    return url.replaceFirst(RegExp(r'/+$'), '');
  }

  String absoluteUrl(String path) {
    if (path.startsWith('http://') || path.startsWith('https://')) {
      return path;
    }
    return '$baseUrl${path.startsWith('/') ? path : '/$path'}';
  }

  String fileUrl(String root, String path) {
    return Uri.parse(
      '$baseUrl/api/file',
    ).replace(queryParameters: {'root': root, 'path': path}).toString();
  }

  Future<void> checkHealth() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/api/health');
      if (response.data?['ok'] != true) {
        throw const RiverApiException('服务器健康检查响应无效');
      }
    } catch (error) {
      throw _mapError(error, fallback: '无法连接到服务器');
    }
  }

  Future<List<MediaRoot>> getRoots() async {
    try {
      final response = await _dio.get<List<dynamic>>('/api/roots');
      return (response.data ?? const [])
          .cast<Map<String, dynamic>>()
          .map(MediaRoot.fromJson)
          .toList();
    } catch (error) {
      throw _mapError(error, fallback: '无法读取媒体根目录');
    }
  }

  Future<DirectoryListing> listDirectory(String root, String path) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/api/list',
        queryParameters: {'root': root, 'path': path},
      );
      return DirectoryListing.fromJson(response.data!);
    } catch (error) {
      throw _mapError(error, fallback: '无法读取目录');
    }
  }

  Future<String> readText(String root, String path) async {
    try {
      final response = await _dio.get<String>(
        '/api/file',
        queryParameters: {'root': root, 'path': path},
        options: Options(responseType: ResponseType.plain),
      );
      return response.data ?? '';
    } catch (error) {
      throw _mapError(error, fallback: '无法读取文本文件');
    }
  }

  Future<void> downloadTo(
    String root,
    String path, {
    required String destinationPath,
    void Function(int received, int total)? onProgress,
  }) async {
    try {
      await _dio.download(
        '/api/download',
        destinationPath,
        queryParameters: {'root': root, 'path': path},
        onReceiveProgress: onProgress,
      );
    } catch (error) {
      throw _mapError(error, fallback: '文件下载失败');
    }
  }

  Future<PlayResponse> playVideo(
    String root,
    String path, {
    double startSeconds = 0,
    String? replaceSessionId,
  }) async {
    try {
      final queryParameters = <String, dynamic>{
        'root': root,
        'path': path,
        if (startSeconds > 0) 'start_seconds': startSeconds,
      };
      if (replaceSessionId case final sessionId?) {
        queryParameters['replace_session_id'] = sessionId;
      }
      final response = await _dio.get<Map<String, dynamic>>(
        '/api/video/play',
        queryParameters: queryParameters,
        options: Options(receiveTimeout: const Duration(seconds: 45)),
      );
      return PlayResponse.fromJson(response.data!);
    } catch (error) {
      throw _mapError(error, fallback: '无法开始视频播放');
    }
  }

  Future<void> stopSession(String sessionId) async {
    try {
      await _dio.delete<void>('/api/video/session/$sessionId');
    } on DioException catch (error) {
      if (error.response?.statusCode != 404) {
        throw _mapError(error, fallback: '停止转码会话失败');
      }
    }
  }

  RiverApiException _mapError(Object error, {required String fallback}) {
    if (error is RiverApiException) {
      return error;
    }
    if (error is DioException) {
      final data = error.response?.data;
      if (data is Map<String, dynamic>) {
        final code = data['error'] as String?;
        final serverMessage = data['message'] as String?;
        return RiverApiException(
          _friendlyMessage(code, serverMessage ?? fallback),
          code: code,
        );
      }
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.receiveTimeout:
          return const RiverApiException('连接服务器超时，请检查地址和网络');
        case DioExceptionType.connectionError:
          return const RiverApiException('无法连接服务器，请确认服务已启动');
        default:
          return RiverApiException(fallback);
      }
    }
    return RiverApiException(fallback);
  }

  String _friendlyMessage(String? code, String fallback) {
    return switch (code) {
      'bad_request' => '请求参数无效',
      'path_forbidden' => '没有权限访问该路径',
      'not_found' => '文件或目录不存在',
      'text_file_too_large' => '文本文件过大，无法在线显示',
      'unsupported_file_type' => '不支持此文件类型',
      'transcode_queue_full' => '转码任务已满，请稍后再试',
      'ffmpeg_not_available' => '服务器未正确安装 FFmpeg',
      'internal_error' => '服务器内部错误',
      _ => fallback,
    };
  }
}
