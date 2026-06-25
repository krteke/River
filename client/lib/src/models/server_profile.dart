class ServerProfile {
  const ServerProfile({
    required this.id,
    required this.name,
    required this.url,
    this.password = '',
  });

  final String id;
  final String name;
  final String url;
  final String password;

  ServerProfile copyWith({String? name, String? url, String? password}) {
    return ServerProfile(
      id: id,
      name: name ?? this.name,
      url: url ?? this.url,
      password: password ?? this.password,
    );
  }

  factory ServerProfile.fromJson(Map<String, dynamic> json) {
    return ServerProfile(
      id: json['id'] as String,
      name: json['name'] as String,
      url: json['url'] as String,
      password: json['password'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'name': name,
    'url': url,
    'password': password,
  };
}
