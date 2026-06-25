import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../controllers/browser_controller.dart';
import '../models/server_profile.dart';
import '../services/river_api.dart';

class ServerManagerDialog extends StatelessWidget {
  const ServerManagerDialog({super.key, required this.controller});

  final BrowserController controller;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('服务器管理'),
      content: AnimatedBuilder(
        animation: controller,
        builder: (context, _) {
          if (controller.servers.isEmpty) {
            return const SizedBox(
              width: 560,
              height: 120,
              child: Center(child: Text('尚未添加服务器')),
            );
          }
          return SizedBox(
            width: 560,
            height: math.min(400, controller.servers.length * 72).toDouble(),
            child: ListView.separated(
              itemCount: controller.servers.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, index) {
                final server = controller.servers[index];
                final selected = controller.selectedServer?.id == server.id;
                return ListTile(
                  leading: Icon(
                    selected ? Icons.dns_rounded : Icons.dns_outlined,
                    color: selected
                        ? Theme.of(context).colorScheme.primary
                        : null,
                  ),
                  title: Text(server.name),
                  subtitle: Row(
                    children: [
                      Expanded(
                        child: Text(
                          server.url,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      if (server.password.isNotEmpty) ...[
                        const SizedBox(width: 8),
                        const Icon(Icons.lock_outline, size: 16),
                      ],
                    ],
                  ),
                  onTap: () async {
                    try {
                      await controller.connect(server);
                      if (context.mounted) {
                        Navigator.pop(context);
                      }
                    } catch (error) {
                      if (context.mounted) {
                        _showError(context, error);
                      }
                    }
                  },
                  trailing: Wrap(
                    children: [
                      IconButton(
                        tooltip: '编辑',
                        onPressed: () => _edit(context, server),
                        icon: const Icon(Icons.edit_outlined),
                      ),
                      IconButton(
                        tooltip: '删除',
                        onPressed: () => _delete(context, server),
                        icon: const Icon(Icons.delete_outline),
                      ),
                    ],
                  ),
                );
              },
            ),
          );
        },
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('关闭'),
        ),
        FilledButton.icon(
          onPressed: () => _edit(context, null),
          icon: const Icon(Icons.add),
          label: const Text('添加服务器'),
        ),
      ],
    );
  }

  Future<void> _edit(BuildContext context, ServerProfile? server) async {
    final result = await showDialog<_ServerFormValue>(
      context: context,
      builder: (_) =>
          ServerEditorDialog(controller: controller, server: server),
    );
    if (result == null || !context.mounted) {
      return;
    }
    try {
      await controller.saveServer(
        id: server?.id,
        name: result.name,
        url: result.url,
        password: result.password,
      );
      if (context.mounted) {
        Navigator.pop(context);
      }
    } catch (error) {
      if (context.mounted) {
        _showError(context, error);
      }
    }
  }

  Future<void> _delete(BuildContext context, ServerProfile server) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('删除服务器'),
        content: Text('确定删除“${server.name}”吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      await controller.deleteServer(server);
    }
  }

  void _showError(BuildContext context, Object error) {
    final message = error is RiverApiException ? error.message : '操作失败';
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }
}

class ServerEditorDialog extends StatefulWidget {
  const ServerEditorDialog({super.key, required this.controller, this.server});

  final BrowserController controller;
  final ServerProfile? server;

  @override
  State<ServerEditorDialog> createState() => _ServerEditorDialogState();
}

class _ServerEditorDialogState extends State<ServerEditorDialog> {
  late final TextEditingController _nameController;
  late final TextEditingController _urlController;
  late final TextEditingController _passwordController;
  final _formKey = GlobalKey<FormState>();
  bool _testing = false;
  bool _connectionPassed = false;
  bool _obscurePassword = true;
  String? _testMessage;

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.server?.name);
    _urlController = TextEditingController(
      text: widget.server?.url ?? 'http://',
    );
    _passwordController = TextEditingController(
      text: widget.server?.password ?? '',
    );
  }

  @override
  void dispose() {
    _nameController.dispose();
    _urlController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.server == null ? '添加服务器' : '编辑服务器'),
      content: SizedBox(
        width: 440,
        child: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                controller: _nameController,
                decoration: const InputDecoration(
                  labelText: '名称',
                  hintText: '家庭媒体服务器',
                  prefixIcon: Icon(Icons.label_outline),
                ),
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _urlController,
                keyboardType: TextInputType.url,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: '服务器地址',
                  hintText: 'http://192.168.1.10:8080',
                  prefixIcon: Icon(Icons.link),
                ),
                validator: (value) {
                  try {
                    RiverApi.normalizeServerUrl(value ?? '');
                    return null;
                  } on RiverApiException catch (error) {
                    return error.message;
                  }
                },
                onChanged: (_) {
                  setState(() {
                    _connectionPassed = false;
                    _testMessage = null;
                  });
                },
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _passwordController,
                obscureText: _obscurePassword,
                autocorrect: false,
                enableSuggestions: false,
                decoration: InputDecoration(
                  labelText: '访问密码',
                  hintText: '服务器未配置密码可留空',
                  prefixIcon: const Icon(Icons.password_outlined),
                  suffixIcon: IconButton(
                    tooltip: _obscurePassword ? '显示密码' : '隐藏密码',
                    onPressed: () =>
                        setState(() => _obscurePassword = !_obscurePassword),
                    icon: Icon(
                      _obscurePassword
                          ? Icons.visibility_outlined
                          : Icons.visibility_off_outlined,
                    ),
                  ),
                ),
                onChanged: (_) {
                  setState(() {
                    _connectionPassed = false;
                    _testMessage = null;
                  });
                },
              ),
              if (_testMessage != null) ...[
                const SizedBox(height: 12),
                Row(
                  children: [
                    Icon(
                      _connectionPassed
                          ? Icons.check_circle
                          : Icons.error_outline,
                      size: 18,
                      color: _connectionPassed
                          ? Colors.green
                          : Theme.of(context).colorScheme.error,
                    ),
                    const SizedBox(width: 8),
                    Expanded(child: Text(_testMessage!)),
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        OutlinedButton.icon(
          onPressed: _testing ? null : _testConnection,
          icon: _testing
              ? const SizedBox.square(
                  dimension: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.monitor_heart_outlined),
          label: const Text('测试连接'),
        ),
        FilledButton(
          onPressed: () {
            if (_formKey.currentState!.validate()) {
              Navigator.pop(
                context,
                _ServerFormValue(
                  name: _nameController.text,
                  url: _urlController.text,
                  password: _passwordController.text,
                ),
              );
            }
          },
          child: const Text('保存并连接'),
        ),
      ],
    );
  }

  Future<void> _testConnection() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }
    setState(() {
      _testing = true;
      _testMessage = null;
    });
    try {
      await widget.controller.testConnectionWithPassword(
        _urlController.text,
        _passwordController.text,
      );
      setState(() {
        _connectionPassed = true;
        _testMessage = '连接成功';
      });
    } catch (error) {
      setState(() {
        _connectionPassed = false;
        _testMessage = error is RiverApiException ? error.message : '连接失败';
      });
    } finally {
      if (mounted) {
        setState(() => _testing = false);
      }
    }
  }
}

class _ServerFormValue {
  const _ServerFormValue({
    required this.name,
    required this.url,
    required this.password,
  });

  final String name;
  final String url;
  final String password;
}
