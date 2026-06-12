import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import '../models/server_profile.dart';

abstract interface class ServerStoreBase {
  Future<List<ServerProfile>> loadServers();

  Future<String?> loadSelectedServerId();

  Future<void> saveServers(
    List<ServerProfile> servers,
    String? selectedServerId,
  );
}

class ServerStore implements ServerStoreBase {
  static const _serversKey = 'river.servers';
  static const _selectedServerKey = 'river.selectedServer';

  final SharedPreferencesAsync _preferences = SharedPreferencesAsync();

  @override
  Future<List<ServerProfile>> loadServers() async {
    final raw = await _preferences.getString(_serversKey);
    if (raw == null || raw.isEmpty) {
      return const [];
    }
    final list = jsonDecode(raw) as List<dynamic>;
    return list
        .cast<Map<String, dynamic>>()
        .map(ServerProfile.fromJson)
        .toList();
  }

  @override
  Future<String?> loadSelectedServerId() {
    return _preferences.getString(_selectedServerKey);
  }

  @override
  Future<void> saveServers(
    List<ServerProfile> servers,
    String? selectedServerId,
  ) async {
    await _preferences.setString(
      _serversKey,
      jsonEncode(servers.map((server) => server.toJson()).toList()),
    );
    if (selectedServerId == null) {
      await _preferences.remove(_selectedServerKey);
    } else {
      await _preferences.setString(_selectedServerKey, selectedServerId);
    }
  }
}
