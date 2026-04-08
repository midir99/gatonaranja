# gatonaranja

`gatonaranja` is a Telegram bot for downloading YouTube videos, clips, and audio.

It is designed to be simple to run as a standalone binary and easy to deploy as a `systemd` service.

## Index

- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Telegram Request Format](#telegram-request-format)
- [Supported Timestamp Formats](#supported-timestamp-formats)
- [Make Targets](#make-targets)
- [Maintainer Release Flow](#maintainer-release-flow)
- [Running With systemd](#running-with-systemd)
- [Logging](#logging)
- [Security Notes](#security-notes)
- [Development](#development)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Features

- Download a full YouTube video
- Download only a clip from a YouTube video
- Download audio only
- Download only an audio clip
- Restrict bot usage to specific Telegram user IDs
- Process downloads with a bounded queue and worker pool
- Run as a simple CLI program
- Easy to install as a user-scoped `systemd` service

## Architecture

```text
Telegram
   |
   v
RunTelegramBot
   |
   v
DownloadRequestHandler
   |
   +--> authorize user
   +--> parse request
   +--> send immediate reply
   |
   v
bounded download queue
   |
   v
download workers
   |
   +--> yt-dlp
   +--> ffmpeg
   |
   v
send audio/video back to Telegram
```

## Requirements

`gatonaranja` depends on these external tools:

- `yt-dlp`
- `ffmpeg`

They must be installed and available in your `PATH`.

## Installation

Build the binary from source:

```bash
make build
```

This creates the `gatonaranja` binary at:

```bash
bin/gatonaranja
```

You can also build it directly with Go:

```bash
go build -o gatonaranja
```

### Install From GitHub Releases

You can install `gatonaranja` as a user-scoped service with the provided
installer script published with each release.

For a quick install of the latest release:

```bash
curl -fsSL https://github.com/midir99/gatonaranja/releases/latest/download/install-systemd-user.sh | bash -s
```

For a safer and reproducible install, download the installer for a specific
release, inspect it if you want, and run it with the same version:

```bash
curl -fsSLO https://github.com/midir99/gatonaranja/releases/download/vX.Y.Z/install-systemd-user.sh
chmod +x install-systemd-user.sh
./install-systemd-user.sh --version vX.Y.Z
```

This installs:

- `~/.local/bin/gatonaranja`
- `~/.local/share/gatonaranja`
- `~/.config/gatonaranja/gatonaranja.env`
- `~/.config/systemd/user/gatonaranja.service`

By default, the installer also enables and starts the user service.

You can install without enabling the service yet:

```bash
./install-systemd-user.sh --skip-enable
```

After installation, edit:

```bash
~/.config/gatonaranja/gatonaranja.env
```

and set at least:

```bash
TELEGRAM_BOT_TOKEN=...
```

Then restart the service:

```bash
systemctl --user restart gatonaranja
```

## Usage

Run the bot with:

```bash
./bin/gatonaranja -telegram-bot-token "<YOUR_TELEGRAM_BOT_TOKEN>"
```

Optionally restrict which Telegram users can use the bot and tune download concurrency, queue size, timeout and yt-dlp configuration:

```bash
./bin/gatonaranja \
  -telegram-bot-token "<YOUR_TELEGRAM_BOT_TOKEN>" \
  -authorized-users "123456789,987654321" \
  -max-concurrent-downloads 5 \
  -max-queued-downloads 5 \
  -download-timeout 5m \
  -ytdlp-config ~/.config/gatonaranja/yt-dlp.conf
```

### Flags

- `-telegram-bot-token`
  Telegram bot token used to authenticate the bot.
  Defaults to `TELEGRAM_BOT_TOKEN`.

- `-authorized-users`
  Comma-separated list of Telegram user IDs allowed to use the bot.
  Defaults to `AUTHORIZED_USERS`.
  If no IDs are specified, everyone can use the bot.

- `-max-concurrent-downloads`
  Maximum number of downloads that can run at the same time.
  Defaults to `5`.
  Can also be set with `MAX_CONCURRENT_DOWNLOADS`.

- `-max-queued-downloads`
  Maximum number of accepted download requests waiting in the queue.
  Defaults to `5`.
  Can also be set with `MAX_QUEUED_DOWNLOADS`.

- `-download-timeout`
  Maximum time allowed for a single download before it is canceled.
  Accepts Go duration strings such as `30s`, `2m`, or `5m`.
  Defaults to `5m`.
  Can also be set with `DOWNLOAD_TIMEOUT`.

- `-ytdlp-config`
  Path to an optional yt-dlp configuration file with extra operator-controlled
  options such as proxy or cookies.
  Can also be set with `YTDLP_CONFIG`.
  When omitted, gatonaranja runs yt-dlp with `--ignore-config` so ambient
  system or user yt-dlp configs do not affect the bot.
  Use `YTDLP_CONFIG` only for advanced operator settings such as cookies, proxy,
  authentication, or extractor/network workarounds.
  Avoid setting options that change how gatonaranja obtains or locates the final
  downloaded file, such as `--print`, `--output`, or `--paths`. These can interfere
  with the bot's download flow and cause requests to fail.

- `-version`
  Print the application version and exit.

### Environment Variables

You can provide configuration through environment variables instead of flags:

- `TELEGRAM_BOT_TOKEN`
- `AUTHORIZED_USERS`
- `MAX_CONCURRENT_DOWNLOADS`
- `MAX_QUEUED_DOWNLOADS`
- `DOWNLOAD_TIMEOUT`
- `YTDLP_CONFIG`

Example:

```bash
export TELEGRAM_BOT_TOKEN="<YOUR_TELEGRAM_BOT_TOKEN>"
export AUTHORIZED_USERS="123456789,987654321"
export MAX_CONCURRENT_DOWNLOADS="5"
export MAX_QUEUED_DOWNLOADS="5"
export DOWNLOAD_TIMEOUT="5m"
export YTDLP_CONFIG="$HOME/.config/gatonaranja/yt-dlp.conf"

./bin/gatonaranja
```

## Telegram Request Format

The bot accepts requests in this format:

```text
URL [TIMESTAMP_RANGE] [audio]
```

Supported examples:

### Download a Video

```text
https://www.youtube.com/watch?v=AqjB8DGt85U
```

### Download a Video Clip

```text
https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05
```

### Download Audio Only

```text
https://www.youtube.com/watch?v=AqjB8DGt85U audio
```

### Download an Audio Clip

```text
https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio
```

### Use `start` or `end`

```text
https://www.youtube.com/watch?v=AqjB8DGt85U start-0:10
https://www.youtube.com/watch?v=AqjB8DGt85U 0:10-end
```

## Supported Timestamp Formats

A timestamp can be:

- `MM:SS`
- `HH:MM:SS`
- `start`
- `end`

A timestamp range can be:

- `1:00-1:05`
- `start-0:10`
- `0:10-end`

## Make Targets

Useful development commands:

```bash
make help
make build
make fmt
make test
make coverage
make coverage-html
make vet
make lint
```

## Maintainer Release Flow

Releases are tag-driven.

To publish a new release:

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

That tag triggers the GitHub Actions workflow at
`.github/workflows/release.yml`, which:

- checks out the tagged commit
- runs `go test ./...`
- runs GoReleaser using `.goreleaser.yaml`
- publishes GitHub release assets

The release pipeline currently builds:

- `gatonaranja_linux_amd64.tar.gz`
- `gatonaranja_linux_arm64.tar.gz`
- `checksums.txt`
- `install-systemd-user.sh`

The binary version printed by `-version` is injected at build time from the
Git tag into `main.Version`.

## Running With systemd

`gatonaranja` is intended to work well as a user-scoped `systemd` service.

Recommended paths for the user service setup are:

- binary: `~/.local/bin/gatonaranja`
- working directory: `~/.local/share/gatonaranja`
- env file: `~/.config/gatonaranja/gatonaranja.env`
- service unit: `~/.config/systemd/user/gatonaranja.service`

The easiest way to install this layout is:

```bash
curl -fsSL https://github.com/midir99/gatonaranja/releases/latest/download/install-systemd-user.sh | bash -s
```

If you are installing manually, reload and enable the user service with:

```bash
systemctl --user daemon-reload
systemctl --user enable --now gatonaranja
```

If you want the service to keep running after logout, enable linger:

```bash
loginctl enable-linger "$USER"
```

## Logging

When run under `systemd`, logs can be viewed with:

```bash
journalctl --user-unit=gatonaranja.service -f
```

## Security Notes

- Only YouTube URLs are accepted
- The bot executes `yt-dlp` using argument slices through Go's `exec.Command`, avoiding shell command interpolation
- You can restrict access to specific Telegram user IDs with `-authorized-users`

## Development

Format, lint, and test the project with:

```bash
make fmt
make lint
make test
```

## Acknowledgements

Special thanks to the developers and maintainers of:

- Telegram
- yt-dlp
- ffmpeg

## License

See [LICENSE](LICENSE).
