# River Client

Flutter client for the River media server.

## Features

- Manage and validate multiple server addresses.
- Browse configured media roots and directories.
- Preview images and text files.
- Download files with progress reporting.
- Play direct video or server-generated HLS through a shared player wrapper.
- Responsive desktop and mobile layouts.

## Run

```bash
flutter pub get
flutter run -d linux
```

Replace `linux` with an available Windows, macOS, Android, or iOS device.

The client accepts HTTP server addresses for local-network use. Production
deployments exposed outside a trusted network should use HTTPS.
