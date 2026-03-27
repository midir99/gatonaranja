# gatonaranja

`gatonaranja` is a Telegram bot for downloading YouTube videos, clips, and audio.

It is designed to be simple to run as a standalone binary and easy to deploy as a `systemd` service.

## Features

- Download a full YouTube video
- Download only a clip from a YouTube video
- Download audio only
- Download only an audio clip
- Restrict bot usage to specific Telegram user IDs
- Run as a simple CLI program
- Easy to integrate with `systemd`

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

This creates the `gatonaranja` binary in the project directory.

You can also build it directly with Go:

```bash
go build -o gatonaranja
```

## Usage

Run the bot with:

```bash
./gatonaranja -telegram-bot-token "<YOUR_TELEGRAM_BOT_TOKEN>"
```

Optionally restrict which Telegram users can use the bot and tune download concurrency and timeout:

```bash
./gatonaranja \
  -telegram-bot-token "<YOUR_TELEGRAM_BOT_TOKEN>" \
  -authorized-users "123456789,987654321" \
  -max-concurrent-downloads 5 \
  -download-timeout 5m
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

- `-download-timeout`
  Maximum time allowed for a single download before it is canceled.
  Accepts Go duration strings such as `30s`, `2m`, or `5m`.
  Defaults to `5m`.
  Can also be set with `DOWNLOAD_TIMEOUT`.

### Environment Variables

You can provide configuration through environment variables instead of flags:

- `TELEGRAM_BOT_TOKEN`
- `AUTHORIZED_USERS`
- `MAX_CONCURRENT_DOWNLOADS`
- `DOWNLOAD_TIMEOUT`

Example:

```bash
export TELEGRAM_BOT_TOKEN="<YOUR_TELEGRAM_BOT_TOKEN>"
export AUTHORIZED_USERS="123456789,987654321"
export MAX_CONCURRENT_DOWNLOADS="5"
export DOWNLOAD_TIMEOUT="5m"

./gatonaranja
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

## Running With systemd

`gatonaranja` is intended to work well as a `systemd` service.

A typical deployment flow is:

1. Build the binary
2. Copy the binary to a directory such as `/usr/local/bin`
3. Create a dedicated service user
4. Configure the Telegram token and authorized users
5. Install and enable a `systemd` service unit

Example service usage will be documented once the service installation flow is finalized.

## Logging

When run under `systemd`, logs can be viewed with:

```bash
journalctl -u gatonaranja
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
