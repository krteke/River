import 'package:flutter/material.dart';

import '../controllers/browser_controller.dart';
import '../controllers/theme_controller.dart';
import '../models/file_models.dart';
import '../models/server_profile.dart';
import '../services/download_service.dart';
import '../services/external_playback_service.dart';
import '../services/river_api.dart';
import '../widgets/file_preview.dart';
import '../widgets/server_manager_dialog.dart';
import '../widgets/video_player_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({
    super.key,
    this.controller,
    this.themeController,
    this.externalPlaybackService,
  });

  final BrowserController? controller;
  final ThemeController? themeController;
  final ExternalPlaybackService? externalPlaybackService;

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  late final BrowserController _controller;
  late final bool _ownsController;
  late final ExternalPlaybackService _externalPlaybackService;
  final DownloadService _downloadService = DownloadService();
  double? _downloadProgress;

  @override
  void initState() {
    super.initState();
    _ownsController = widget.controller == null;
    _controller = widget.controller ?? BrowserController();
    _externalPlaybackService =
        widget.externalPlaybackService ?? ExternalPlaybackService();
    _controller.initialize();
  }

  @override
  void dispose() {
    if (_ownsController) {
      _controller.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, _) {
        final handleDirectoryBack =
            MediaQuery.sizeOf(context).width < 900 &&
            _controller.canOpenParentDirectory;
        return PopScope<void>(
          canPop: !handleDirectoryBack,
          onPopInvokedWithResult: (didPop, _) {
            if (didPop || !handleDirectoryBack) {
              return;
            }
            _run(_controller.openParentDirectory);
          },
          child: Scaffold(
            appBar: AppBar(
              titleSpacing: 20,
              title: Row(
                children: [
                  const Icon(Icons.water_rounded),
                  const SizedBox(width: 10),
                  const Text('River'),
                  if (_controller.selectedServer != null) ...[
                    const SizedBox(width: 16),
                    Flexible(
                      child: Text(
                        _controller.selectedServer!.name,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.bodyMedium,
                      ),
                    ),
                  ],
                ],
              ),
              actions: [
                if (widget.themeController != null)
                  _ThemeModeMenu(controller: widget.themeController!),
                if (_controller.api != null)
                  IconButton(
                    tooltip: '刷新',
                    onPressed: _controller.loading ? null : _refresh,
                    icon: const Icon(Icons.refresh),
                  ),
                IconButton(
                  tooltip: '服务器管理',
                  onPressed: _showServerManager,
                  icon: const Icon(Icons.dns_outlined),
                ),
                const SizedBox(width: 8),
              ],
            ),
            body: _body(context),
          ),
        );
      },
    );
  }

  Widget _body(BuildContext context) {
    if (_controller.loading && _controller.api == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_controller.servers.isEmpty) {
      return _ServerEmptyState(onAddServer: _showServerManager);
    }
    if (_controller.api == null) {
      return _ConnectionError(
        message: _controller.errorMessage ?? '尚未连接服务器',
        onRetry: () => _connect(_controller.selectedServer!),
        onManageServers: _showServerManager,
      );
    }
    if (_controller.roots.isEmpty) {
      return const Center(child: Text('服务器没有配置媒体根目录'));
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final desktop = constraints.maxWidth >= 900;
        return desktop ? _desktopLayout() : _mobileLayout();
      },
    );
  }

  Widget _desktopLayout() {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(width: 220, child: Card(child: _rootList())),
          const SizedBox(width: 12),
          Expanded(child: Card(child: _filePane())),
          const SizedBox(width: 12),
          SizedBox(
            width: 400,
            child: Card(
              child: FilePreview(
                controller: _controller,
                onDownload: _download,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _mobileLayout() {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
          child: DropdownButtonFormField<String>(
            initialValue: _controller.selectedRoot?.id,
            decoration: const InputDecoration(
              labelText: '媒体目录',
              prefixIcon: Icon(Icons.storage_outlined),
              isDense: true,
            ),
            items: _controller.roots
                .map(
                  (root) =>
                      DropdownMenuItem(value: root.id, child: Text(root.name)),
                )
                .toList(),
            onChanged: (id) {
              final root = _controller.roots.firstWhere(
                (item) => item.id == id,
              );
              _run(() => _controller.selectRoot(root));
            },
          ),
        ),
        Expanded(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
            child: Card(child: _filePane()),
          ),
        ),
      ],
    );
  }

  Widget _rootList() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 10),
          child: Text('媒体目录', style: Theme.of(context).textTheme.titleSmall),
        ),
        const Divider(height: 1),
        Expanded(
          child: ListView.builder(
            padding: const EdgeInsets.symmetric(vertical: 8),
            itemCount: _controller.roots.length,
            itemBuilder: (context, index) {
              final root = _controller.roots[index];
              return ListTile(
                selected: _controller.selectedRoot?.id == root.id,
                leading: const Icon(Icons.storage_outlined),
                title: Text(root.name),
                onTap: () => _run(() => _controller.selectRoot(root)),
              );
            },
          ),
        ),
        const Divider(height: 1),
        Padding(
          padding: const EdgeInsets.all(8),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<String>(
              isExpanded: true,
              value: _controller.selectedServer?.id,
              items: _controller.servers
                  .map(
                    (server) => DropdownMenuItem(
                      value: server.id,
                      child: Text(server.name, overflow: TextOverflow.ellipsis),
                    ),
                  )
                  .toList(),
              onChanged: (id) {
                final server = _controller.servers.firstWhere(
                  (item) => item.id == id,
                );
                _connect(server);
              },
            ),
          ),
        ),
      ],
    );
  }

  Widget _filePane() {
    final listing = _controller.listing;
    final entries = _controller.sortedEntries;
    return Column(
      children: [
        _pathBar(listing),
        const Divider(height: 1),
        if (_downloadProgress != null)
          LinearProgressIndicator(value: _downloadProgress),
        Expanded(
          child: Stack(
            children: [
              if (listing == null || entries.isEmpty)
                const Center(child: Text('此目录为空'))
              else
                ListView.separated(
                  itemCount: entries.length,
                  separatorBuilder: (_, _) =>
                      const Divider(height: 1, indent: 64),
                  itemBuilder: (context, index) {
                    final entry = entries[index];
                    return _FileTile(
                      entry: entry,
                      selected: _controller.selectedEntry?.path == entry.path,
                      api: _controller.api!,
                      onTap: () => _openEntry(entry),
                      onDownload: entry.type == RiverFileType.directory
                          ? null
                          : () => _download(entry),
                      onContextMenu: (position) =>
                          _showEntryMenu(entry, position),
                    );
                  },
                ),
              if (_controller.loading)
                Positioned.fill(
                  child: ColoredBox(
                    color: Theme.of(
                      context,
                    ).colorScheme.surface.withValues(alpha: 0.72),
                    child: const Center(child: CircularProgressIndicator()),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _pathBar(DirectoryListing? listing) {
    return SizedBox(
      height: 54,
      child: Row(
        children: [
          IconButton(
            tooltip: '上一级',
            onPressed: listing == null || listing.path == '/'
                ? null
                : () => _run(_controller.openParentDirectory),
            icon: const Icon(Icons.arrow_upward),
          ),
          IconButton(
            tooltip: '根目录',
            onPressed: listing?.path == '/'
                ? null
                : () => _run(() => _controller.openDirectory('/')),
            icon: const Icon(Icons.home_outlined),
          ),
          const SizedBox(width: 4),
          Expanded(
            child: Text(
              '${_controller.selectedRoot?.name ?? ''}${listing?.path ?? '/'}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
          PopupMenuButton<_SortChoice>(
            tooltip: '排序：${_controller.sortLabel}',
            initialValue: _SortChoice(
              _controller.sortField,
              _controller.sortAscending,
            ),
            onSelected: (choice) =>
                _controller.setSort(choice.field, choice.ascending),
            itemBuilder: (context) => const [
              PopupMenuItem(
                value: _SortChoice(FileSortField.name, true),
                child: ListTile(
                  leading: Icon(Icons.sort_by_alpha),
                  title: Text('文件名升序'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
              PopupMenuItem(
                value: _SortChoice(FileSortField.name, false),
                child: ListTile(
                  leading: Icon(Icons.sort_by_alpha),
                  title: Text('文件名降序'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
              PopupMenuItem(
                value: _SortChoice(FileSortField.size, true),
                child: ListTile(
                  leading: Icon(Icons.data_usage_outlined),
                  title: Text('文件大小升序'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
              PopupMenuItem(
                value: _SortChoice(FileSortField.size, false),
                child: ListTile(
                  leading: Icon(Icons.data_usage_outlined),
                  title: Text('文件大小降序'),
                  contentPadding: EdgeInsets.zero,
                ),
              ),
            ],
            icon: Icon(
              _controller.sortAscending
                  ? Icons.arrow_upward
                  : Icons.arrow_downward,
            ),
          ),
          const SizedBox(width: 8),
        ],
      ),
    );
  }

  Future<void> _openEntry(FileEntry entry) async {
    if (entry.type == RiverFileType.directory) {
      await _run(() => _controller.openDirectory(entry.path));
      return;
    }
    if (entry.type == RiverFileType.video) {
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => VideoPlayerScreen(
            api: _controller.api!,
            root: _controller.selectedRoot!.id,
            path: entry.path,
            title: entry.name,
          ),
        ),
      );
      return;
    }
    await _run(() => _controller.selectFile(entry));
    if (!mounted || MediaQuery.sizeOf(context).width >= 900) {
      return;
    }
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) =>
            MobilePreviewScreen(controller: _controller, onDownload: _download),
      ),
    );
  }

  Future<void> _download(FileEntry entry) async {
    final api = _controller.api;
    final root = _controller.selectedRoot;
    if (api == null || root == null) {
      return;
    }
    try {
      setState(() => _downloadProgress = 0);
      final destination = await _downloadService.saveFile(
        api: api,
        root: root.id,
        path: entry.path,
        fileName: entry.name,
        onProgress: (received, total) {
          if (mounted) {
            setState(() {
              _downloadProgress = total > 0 ? received / total : null;
            });
          }
        },
      );
      if (mounted && destination != null) {
        _showMessage('文件已保存到 $destination');
      }
    } catch (error) {
      if (mounted) {
        _showError(error);
      }
    } finally {
      if (mounted) {
        setState(() => _downloadProgress = null);
      }
    }
  }

  Future<void> _showEntryMenu(FileEntry entry, Offset globalPosition) async {
    if (!mounted) {
      return;
    }
    final overlay = Overlay.of(context).context.findRenderObject();
    if (overlay is! RenderBox) {
      return;
    }
    final action = await showMenu<_FileAction>(
      context: context,
      position: RelativeRect.fromRect(
        Rect.fromLTWH(globalPosition.dx, globalPosition.dy, 1, 1),
        Offset.zero & overlay.size,
      ),
      items: [
        if (entry.type != RiverFileType.directory)
          const PopupMenuItem(
            value: _FileAction.download,
            child: ListTile(
              leading: Icon(Icons.download_outlined),
              title: Text('下载'),
              contentPadding: EdgeInsets.zero,
            ),
          ),
        const PopupMenuItem(
          value: _FileAction.open,
          child: ListTile(
            leading: Icon(Icons.open_in_new_rounded),
            title: Text('打开'),
            contentPadding: EdgeInsets.zero,
          ),
        ),
        if (entry.type == RiverFileType.video)
          const PopupMenuItem(
            value: _FileAction.openExternally,
            child: ListTile(
              leading: Icon(Icons.play_circle_outline_rounded),
              title: Text('使用外部播放器播放'),
              contentPadding: EdgeInsets.zero,
            ),
          ),
      ],
    );
    if (action == null || !mounted) {
      return;
    }
    switch (action) {
      case _FileAction.download:
        await _download(entry);
        break;
      case _FileAction.open:
        await _openEntry(entry);
        break;
      case _FileAction.openExternally:
        await _openExternally(entry);
        break;
    }
  }

  Future<void> _openExternally(FileEntry entry) async {
    final api = _controller.api;
    final root = _controller.selectedRoot;
    if (api == null || root == null || entry.type != RiverFileType.video) {
      return;
    }
    try {
      await _externalPlaybackService.openOriginal(
        url: api.originalFileUrl(root.id, entry.path),
        headers: api.authHeaders ?? const {},
      );
    } on ExternalPlaybackException catch (error) {
      if (mounted) {
        _showMessage(error.message);
      }
    } catch (_) {
      if (mounted) {
        _showMessage('无法打开外部播放器');
      }
    }
  }

  Future<void> _refresh() => _run(_controller.refresh);

  Future<void> _connect(ServerProfile server) {
    return _run(() => _controller.connect(server));
  }

  Future<void> _run(Future<void> Function() action) async {
    try {
      await action();
    } catch (error) {
      if (mounted) {
        _showError(error);
      }
    }
  }

  void _showServerManager() {
    showDialog<void>(
      context: context,
      builder: (_) => ServerManagerDialog(controller: _controller),
    );
  }

  void _showError(Object error) {
    final message = error is RiverApiException ? error.message : '操作失败';
    _showMessage(message);
  }

  void _showMessage(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }
}

class _SortChoice {
  const _SortChoice(this.field, this.ascending);

  final FileSortField field;
  final bool ascending;

  @override
  bool operator ==(Object other) =>
      other is _SortChoice &&
      other.field == field &&
      other.ascending == ascending;

  @override
  int get hashCode => Object.hash(field, ascending);
}

enum _FileAction { download, open, openExternally }

class _ThemeModeMenu extends StatelessWidget {
  const _ThemeModeMenu({required this.controller});

  final ThemeController controller;

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<ThemeMode>(
      tooltip: '主题',
      initialValue: controller.mode,
      onSelected: controller.setMode,
      icon: Icon(_modeIcon(controller.mode)),
      itemBuilder: (context) => const [
        PopupMenuItem(
          value: ThemeMode.system,
          child: ListTile(
            leading: Icon(Icons.brightness_auto_outlined),
            title: Text('跟随系统'),
            contentPadding: EdgeInsets.zero,
          ),
        ),
        PopupMenuItem(
          value: ThemeMode.light,
          child: ListTile(
            leading: Icon(Icons.light_mode_outlined),
            title: Text('浅色'),
            contentPadding: EdgeInsets.zero,
          ),
        ),
        PopupMenuItem(
          value: ThemeMode.dark,
          child: ListTile(
            leading: Icon(Icons.dark_mode_outlined),
            title: Text('深色'),
            contentPadding: EdgeInsets.zero,
          ),
        ),
      ],
    );
  }

  IconData _modeIcon(ThemeMode mode) {
    return switch (mode) {
      ThemeMode.system => Icons.brightness_auto_outlined,
      ThemeMode.light => Icons.light_mode_outlined,
      ThemeMode.dark => Icons.dark_mode_outlined,
    };
  }
}

class _FileTile extends StatelessWidget {
  const _FileTile({
    required this.entry,
    required this.selected,
    required this.api,
    required this.onTap,
    required this.onContextMenu,
    this.onDownload,
  });

  final FileEntry entry;
  final bool selected;
  final RiverApi api;
  final VoidCallback onTap;
  final ValueChanged<Offset> onContextMenu;
  final VoidCallback? onDownload;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onSecondaryTapDown: (details) => onContextMenu(details.globalPosition),
      onLongPressStart: (details) => onContextMenu(details.globalPosition),
      child: ListTile(
        selected: selected,
        leading: _leading(context),
        title: Text(entry.name, maxLines: 1, overflow: TextOverflow.ellipsis),
        subtitle: entry.type == RiverFileType.directory
            ? const Text('文件夹')
            : Text(
                '${formatBytes(entry.size)} · ${formatDate(entry.modifiedAt)}',
              ),
        trailing: onDownload == null
            ? const Icon(Icons.chevron_right)
            : IconButton(
                tooltip: '下载',
                onPressed: onDownload,
                icon: const Icon(Icons.download_outlined),
              ),
        onTap: onTap,
      ),
    );
  }

  Widget _leading(BuildContext context) {
    final thumbnailUrl = entry.thumbnailUrl;
    if (thumbnailUrl == null || thumbnailUrl.isEmpty) {
      return _iconAvatar(context);
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular(10),
      child: SizedBox(
        width: 48,
        height: 48,
        child: Image.network(
          api.absoluteUrl(thumbnailUrl),
          headers: api.authHeaders,
          fit: BoxFit.cover,
          errorBuilder: (_, _, _) => _iconAvatar(context),
        ),
      ),
    );
  }

  Widget _iconAvatar(BuildContext context) {
    return CircleAvatar(
      backgroundColor: _color(context).withValues(alpha: 0.12),
      foregroundColor: _color(context),
      child: Icon(_icon),
    );
  }

  IconData get _icon => switch (entry.type) {
    RiverFileType.directory => Icons.folder_outlined,
    RiverFileType.image => Icons.image_outlined,
    RiverFileType.text => Icons.description_outlined,
    RiverFileType.video => Icons.movie_outlined,
    RiverFileType.other => Icons.insert_drive_file_outlined,
  };

  Color _color(BuildContext context) => switch (entry.type) {
    RiverFileType.directory => const Color(0xFFE59A19),
    RiverFileType.image => const Color(0xFF2E9D72),
    RiverFileType.text => const Color(0xFF6576CC),
    RiverFileType.video => const Color(0xFFD35368),
    RiverFileType.other => Theme.of(context).colorScheme.outline,
  };
}

class _ServerEmptyState extends StatelessWidget {
  const _ServerEmptyState({required this.onAddServer});

  final VoidCallback onAddServer;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.dns_outlined,
              size: 72,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 20),
            Text(
              '连接你的 River 服务器',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            const Text('添加服务器地址后，即可浏览文件并播放视频。', textAlign: TextAlign.center),
            const SizedBox(height: 24),
            FilledButton.icon(
              onPressed: onAddServer,
              icon: const Icon(Icons.add),
              label: const Text('添加服务器'),
            ),
          ],
        ),
      ),
    );
  }
}

class _ConnectionError extends StatelessWidget {
  const _ConnectionError({
    required this.message,
    required this.onRetry,
    required this.onManageServers,
  });

  final String message;
  final VoidCallback onRetry;
  final VoidCallback onManageServers;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.cloud_off_outlined,
              size: 64,
              color: Theme.of(context).colorScheme.error,
            ),
            const SizedBox(height: 16),
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 20),
            Wrap(
              spacing: 12,
              children: [
                OutlinedButton(
                  onPressed: onManageServers,
                  child: const Text('服务器管理'),
                ),
                FilledButton.icon(
                  onPressed: onRetry,
                  icon: const Icon(Icons.refresh),
                  label: const Text('重新连接'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
