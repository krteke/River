class ServerProfile {
  const ServerProfile({
    required this.id,
    required this.name,
    required this.url,
  });

  final String id;
  final String name;
  final String url;

  ServerProfile copyWith({String? name, String? url}) {
    return ServerProfile(id: id, name: name ?? this.name, url: url ?? this.url);
  }

  factory ServerProfile.fromJson(Map<String, dynamic> json) {
    return ServerProfile(
      id: json['id'] as String,
      name: json['name'] as String,
      url: json['url'] as String,
    );
  }

  Map<String, dynamic> toJson() => {'id': id, 'name': name, 'url': url};
}
