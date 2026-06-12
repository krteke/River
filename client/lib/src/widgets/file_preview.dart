import 'package:flutter/material.dart';

import '../controllers/browser_controller.dart';
import '../models/file_models.dart';

class FilePreview extends StatelessWidget {
  const FilePreview({
    super.key,
    required this.controller,
    required this.onDownload,
  });

  final BrowserController controller;
  final Future<void> Function(FileEntry entry) onDownload;

  @override
  Widget build(BuildContext context) {
    final entry = controller.selectedEntry;
    if (entry == null) {
      return const _EmptyPreview();
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(20, 18, 12, 12),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      entry.name,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '${formatBytes(entry.size)} · ${formatDate(entry.modifiedAt)}',
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ],
                ),
              ),
              IconButton(
                tooltip: '下载',
                onPressed: () => onDownload(entry),
                icon: const Icon(Icons.download_outlined),
              ),
            ],
          ),
        ),
        const Divider(height: 1),
        Expanded(child: _content(context, entry)),
      ],
    );
  }

  Widget _content(BuildContext context, FileEntry entry) {
    return switch (entry.type) {
      RiverFileType.image => InteractiveViewer(
        minScale: 0.5,
        maxScale: 5,
        child: Center(
          child: Image.network(
            controller.api!.fileUrl(controller.selectedRoot!.id, entry.path),
            fit: BoxFit.contain,
            errorBuilder: (_, _, _) => const _PreviewMessage(
              icon: Icons.broken_image_outlined,
              message: '图片加载失败',
            ),
            loadingBuilder: (_, child, progress) {
              return progress == null
                  ? child
                  : const Center(child: CircularProgressIndicator());
            },
          ),
        ),
      ),
      RiverFileType.text =>
        controller.loadingPreview
            ? const Center(child: CircularProgressIndicator())
            : Scrollbar(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(20),
                  child: SelectableText(
                    controller.textContent ?? '',
                    style: const TextStyle(
                      fontFamily: 'monospace',
                      height: 1.5,
                    ),
                  ),
                ),
              ),
      RiverFileType.video => const _PreviewMessage(
        icon: Icons.play_circle_outline,
        message: '双击文件开始播放',
      ),
      RiverFileType.other => const _PreviewMessage(
        icon: Icons.insert_drive_file_outlined,
        message: '此文件仅支持下载',
      ),
      RiverFileType.directory => const SizedBox.shrink(),
    };
  }
}

class MobilePreviewScreen extends StatelessWidget {
  const MobilePreviewScreen({
    super.key,
    required this.controller,
    required this.onDownload,
  });

  final BrowserController controller;
  final Future<void> Function(FileEntry entry) onDownload;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(controller.selectedEntry?.name ?? '预览')),
      body: FilePreview(controller: controller, onDownload: onDownload),
    );
  }
}

class _EmptyPreview extends StatelessWidget {
  const _EmptyPreview();

  @override
  Widget build(BuildContext context) {
    return const _PreviewMessage(
      icon: Icons.preview_outlined,
      message: '选择文件以查看详情',
    );
  }
}

class _PreviewMessage extends StatelessWidget {
  const _PreviewMessage({required this.icon, required this.message});

  final IconData icon;
  final String message;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 52, color: Theme.of(context).colorScheme.outline),
          const SizedBox(height: 12),
          Text(
            message,
            style: TextStyle(color: Theme.of(context).colorScheme.outline),
          ),
        ],
      ),
    );
  }
}

String formatBytes(int bytes) {
  if (bytes < 1024) {
    return '$bytes B';
  }
  if (bytes < 1024 * 1024) {
    return '${(bytes / 1024).toStringAsFixed(1)} KB';
  }
  if (bytes < 1024 * 1024 * 1024) {
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }
  return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
}

String formatDate(DateTime date) {
  final local = date.toLocal();
  String two(int value) => value.toString().padLeft(2, '0');
  return '${local.year}-${two(local.month)}-${two(local.day)} '
      '${two(local.hour)}:${two(local.minute)}';
}
