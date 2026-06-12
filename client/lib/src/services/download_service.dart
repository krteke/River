import 'dart:io';

import 'package:file_selector/file_selector.dart';
import 'package:path_provider/path_provider.dart';

import 'river_api.dart';

class DownloadService {
  Future<String?> saveFile({
    required RiverApi api,
    required String root,
    required String path,
    required String fileName,
    void Function(int received, int total)? onProgress,
  }) async {
    final destinationPath = await _destinationPath(fileName);
    if (destinationPath == null) {
      return null;
    }

    await api.downloadTo(
      root,
      path,
      destinationPath: destinationPath,
      onProgress: onProgress,
    );
    return destinationPath;
  }

  Future<String?> _destinationPath(String fileName) async {
    final safeName = fileName.replaceAll(RegExp(r'[/\\]'), '_');
    if (Platform.isAndroid || Platform.isIOS) {
      final directory = Platform.isAndroid
          ? await getExternalStorageDirectory() ??
                await getApplicationDocumentsDirectory()
          : await getApplicationDocumentsDirectory();
      return _availablePath(directory.path, safeName);
    }
    final destination = await getSaveLocation(suggestedName: safeName);
    return destination?.path;
  }

  String _availablePath(String directory, String fileName) {
    final separator = Platform.pathSeparator;
    var candidate = '$directory$separator$fileName';
    if (!File(candidate).existsSync()) {
      return candidate;
    }

    final dot = fileName.lastIndexOf('.');
    final base = dot > 0 ? fileName.substring(0, dot) : fileName;
    final extension = dot > 0 ? fileName.substring(dot) : '';
    var index = 1;
    do {
      candidate = '$directory$separator$base ($index)$extension';
      index++;
    } while (File(candidate).existsSync());
    return candidate;
  }
}
