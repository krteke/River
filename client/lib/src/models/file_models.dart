enum RiverFileType {
  directory,
  image,
  text,
  video,
  other;

  factory RiverFileType.fromWire(String value) {
    return RiverFileType.values.firstWhere(
      (type) => type.name == value,
      orElse: () => RiverFileType.other,
    );
  }
}

class MediaRoot {
  const MediaRoot({required this.id, required this.name});

  final String id;
  final String name;

  factory MediaRoot.fromJson(Map<String, dynamic> json) {
    return MediaRoot(id: json['id'] as String, name: json['name'] as String);
  }
}

class FileEntry {
  const FileEntry({
    required this.name,
    required this.path,
    required this.type,
    required this.size,
    required this.modifiedAt,
    this.thumbnailUrl,
  });

  final String name;
  final String path;
  final RiverFileType type;
  final int size;
  final DateTime modifiedAt;
  final String? thumbnailUrl;

  factory FileEntry.fromJson(Map<String, dynamic> json) {
    return FileEntry(
      name: json['name'] as String,
      path: json['path'] as String,
      type: RiverFileType.fromWire(json['type'] as String),
      size: (json['size'] as num).toInt(),
      modifiedAt: DateTime.fromMillisecondsSinceEpoch(
        (json['mtime'] as num).toInt() * 1000,
      ),
      thumbnailUrl: json['thumbnail_url'] as String?,
    );
  }
}

class DirectoryListing {
  const DirectoryListing({
    required this.rootId,
    required this.path,
    required this.parent,
    required this.items,
  });

  final String rootId;
  final String path;
  final String parent;
  final List<FileEntry> items;

  factory DirectoryListing.fromJson(Map<String, dynamic> json) {
    final items =
        (json['items'] as List<dynamic>)
            .cast<Map<String, dynamic>>()
            .map(FileEntry.fromJson)
            .toList()
          ..sort((left, right) {
            if (left.type == RiverFileType.directory &&
                right.type != RiverFileType.directory) {
              return -1;
            }
            if (right.type == RiverFileType.directory &&
                left.type != RiverFileType.directory) {
              return 1;
            }
            return left.name.toLowerCase().compareTo(right.name.toLowerCase());
          });

    return DirectoryListing(
      rootId: json['root_id'] as String,
      path: json['path'] as String,
      parent: json['parent'] as String,
      items: items,
    );
  }
}

class PlayResponse {
  const PlayResponse({
    required this.mode,
    required this.url,
    this.mime,
    this.sessionId,
    this.profile,
    this.startSeconds = 0,
    this.durationSeconds = 0,
  });

  final String mode;
  final String url;
  final String? mime;
  final String? sessionId;
  final String? profile;
  final double startSeconds;
  final double durationSeconds;

  bool get isHls => mode == 'hls';

  factory PlayResponse.fromJson(Map<String, dynamic> json) {
    return PlayResponse(
      mode: json['mode'] as String,
      url: json['url'] as String,
      mime: json['mime'] as String?,
      sessionId: json['session_id'] as String?,
      profile: json['profile'] as String?,
      startSeconds: (json['start_seconds'] as num?)?.toDouble() ?? 0,
      durationSeconds: (json['duration_seconds'] as num?)?.toDouble() ?? 0,
    );
  }
}

class PlaybackOption {
  const PlaybackOption({
    required this.name,
    required this.label,
    required this.direct,
    required this.isDefault,
    this.codec,
    this.resolution,
    this.bitrate,
  });

  final String name;
  final String label;
  final bool direct;
  final bool isDefault;
  final String? codec;
  final String? resolution;
  final String? bitrate;

  factory PlaybackOption.fromJson(Map<String, dynamic> json) {
    return PlaybackOption(
      name: json['name'] as String,
      label: json['label'] as String? ?? json['name'] as String,
      direct: json['direct'] as bool? ?? false,
      isDefault: json['default'] as bool? ?? false,
      codec: json['codec'] as String?,
      resolution: json['resolution'] as String?,
      bitrate: json['bitrate'] as String?,
    );
  }
}
