# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.3.1] - 2026-09-25

### Fixed

- Terminate the full `yt-dlp` process group on timeout so child processes such
  as `ffmpeg` do not keep running after a canceled download.
- Clean up temporary download directories, including partial files and merge
  artifacts, after failed or completed downloads.

### Changed

- Store each download in its own temporary directory and keep the final output
  filename as title plus extension.
- Log `duration_seconds` when a worker finishes processing a download.
- Allow a second `Ctrl+C` to force shutdown while gatonaranja is waiting for
  active downloads to finish.

## [1.3.0] - 2026-09-15

### Added

- Log the running gatonaranja version when the Telegram bot starts.
- Document the video download flow, including DASH stream merging through `ffmpeg`.

### Changed

- Prefer DASH-friendly video downloads by selecting separate MP4 video and M4A audio streams when needed, then merging them with `ffmpeg`.
- Prefer video formats close to 480p before optimizing for smaller file size.
- Update GitHub Actions workflows to use `actions/checkout@v7` and `actions/setup-go@v7`.

## [1.2.0] - 2026-09-08

### Changed

- Improve yt-dlp video format selection to reduce download failures.
- Send audio downloads as Telegram-friendly M4A files when possible.
- Make yt-dlp output filenames more collision-resistant by including the video ID.

## [1.1.0] - 2026-04-13

### Added

- Support YouTube `/live/<VIDEO_ID>` URLs.

## [1.0.0] - 2026-04-09

### Added

- First stable release of gatonaranja.
- Download full YouTube videos, video clips, audio-only media, and audio clips from Telegram messages.
- Restrict bot usage to configured Telegram user IDs.
- Process downloads with a bounded queue and worker pool.
- Configure download concurrency, queue size, timeout, and optional yt-dlp config file.
- Run as a standalone CLI with `-version` support.
- Install and uninstall as a user-scoped `systemd` service.
- Publish Linux `amd64` and `arm64` release assets with GoReleaser.

### Changed

- Use a first-party stdlib Telegram Bot API client.
- Stream Telegram media uploads instead of loading whole files into memory.
- Improve validation and user-facing errors for YouTube URLs, timestamp ranges, and download requests.

## Pre-1.0.0

### Added

- Initial bot implementation.
- YouTube URL validation.
- Timestamp parsing.
- yt-dlp integration.
- Graceful shutdown.
- Unit tests, coverage reporting, and release automation.

[1.3.1]: https://github.com/midir99/gatonaranja/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/midir99/gatonaranja/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/midir99/gatonaranja/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/midir99/gatonaranja/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/midir99/gatonaranja/releases/tag/v1.0.0
