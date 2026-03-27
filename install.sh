#!/usr/bin/env bash

set -Eeuo pipefail

APP_NAME="gatonaranja"
GITHUB_REPO="midir99/gatonaranja"
DEFAULT_VERSION="latest"

BIN_DIR="${HOME}/.local/bin"
DATA_DIR="${HOME}/.local/share/${APP_NAME}"
CONFIG_DIR="${HOME}/.config/${APP_NAME}"
SYSTEMD_USER_DIR="${HOME}/.config/systemd/user"

BIN_PATH="${BIN_DIR}/${APP_NAME}"
ENV_PATH="${CONFIG_DIR}/${APP_NAME}.env"
SERVICE_PATH="${SYSTEMD_USER_DIR}/${APP_NAME}.service"

DOWNLOAD_URL=""
DOWNLOAD_FILE=""
SKIP_ENABLE="false"
VERSION="${DEFAULT_VERSION}"

SERVICE_ENABLED="false"

log() {
	printf '%s\n' "$*"
}

warn() {
	printf 'warning: %s\n' "$*" >&2
}

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

usage() {
	cat <<EOF
Install ${APP_NAME} as a user-scoped systemd service.

Usage:
  ./install.sh [--version <tag>] [--skip-enable]

Options:
  --version <tag>  Install a specific GitHub release tag (default: latest)
  --skip-enable    Install files but do not enable/start the user service
  -h, --help       Show this help message

This installer expects GitHub release assets named like:
  ${APP_NAME}_linux_amd64.tar.gz
  ${APP_NAME}_linux_arm64.tar.gz
EOF
}

parse_args() {
	while (($# > 0)); do
		case "$1" in
		--version)
			(($# >= 2)) || fail "--version requires a value"
			VERSION="$2"
			shift 2
			;;
		--skip-enable)
			SKIP_ENABLE="true"
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1"
			;;
		esac
	done
}

detect_platform() {
	local os arch
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(uname -m)"

	case "$os" in
	linux) ;;
	*)
		fail "unsupported operating system: ${os}; this installer currently supports Linux only"
		;;
	esac

	case "$arch" in
	x86_64 | amd64)
		arch="amd64"
		;;
	aarch64 | arm64)
		arch="arm64"
		;;
	*)
		fail "unsupported architecture: ${arch}"
		;;
	esac

	printf '%s %s\n' "$os" "$arch"
}

download() {
	local url="$1"
	local output="$2"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$output"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$output" "$url"
	else
		fail "curl or wget is required to download releases"
	fi
}

resolve_release_asset() {
	local os="$1"
	local arch="$2"
	local base_url
	local asset_tar="${APP_NAME}_${os}_${arch}.tar.gz"

	if [[ "$VERSION" == "latest" ]]; then
		base_url="https://github.com/${GITHUB_REPO}/releases/latest/download"
	else
		base_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}"
	fi

	DOWNLOAD_URL="${base_url}/${asset_tar}"
	DOWNLOAD_FILE="${asset_tar}"
}

ensure_directories() {
	mkdir -p "$BIN_DIR" "$DATA_DIR" "$CONFIG_DIR" "$SYSTEMD_USER_DIR"
}

install_binary() {
	local tmpdir extracted_bin cleanup_cmd
	tmpdir="$(mktemp -d)"
	printf -v cleanup_cmd 'rm -rf -- %q' "$tmpdir"
	trap "$cleanup_cmd" EXIT

    log "Downloading ${APP_NAME} release asset..."
    if ! download "$DOWNLOAD_URL" "${tmpdir}/${DOWNLOAD_FILE}"; then
    	fail "failed to download release asset from GitHub: ${DOWNLOAD_URL}"
    fi

    need_cmd tar
    tar -xzf "${tmpdir}/${DOWNLOAD_FILE}" -C "$tmpdir"

    if [[ -x "${tmpdir}/${APP_NAME}" ]]; then
    	extracted_bin="${tmpdir}/${APP_NAME}"
    elif [[ -f "${tmpdir}/${APP_NAME}" ]]; then
        extracted_bin="${tmpdir}/${APP_NAME}"
    else
        fail "release archive did not contain ${APP_NAME}"
    fi

	install -m 755 "$extracted_bin" "$BIN_PATH"
	rm -rf -- "$tmpdir"
	trap - EXIT
}

create_env_file() {
    if [[ -e "$ENV_PATH" ]]; then
    	if [[ ! -f "$ENV_PATH" ]]; then
    		fail "expected ${ENV_PATH} to be a regular file"
    	fi
    	if ! chmod 600 "$ENV_PATH"; then
    		warn "could not set permissions to 600 on ${ENV_PATH}"
    	fi
    	log "Keeping existing environment file at ${ENV_PATH}"
    	return
    fi

	cat >"$ENV_PATH" <<EOF
AUTHORIZED_USERS=
TELEGRAM_BOT_TOKEN=
MAX_CONCURRENT_DOWNLOADS=5
DOWNLOAD_TIMEOUT=5m
EOF
	chmod 600 "$ENV_PATH"
	log "Created environment file at ${ENV_PATH}"
}

service_contents() {
	cat <<EOF
[Unit]
Description=gatonaranja Telegram bot

[Service]
Type=simple
WorkingDirectory=%h/.local/share/gatonaranja
EnvironmentFile=%h/.config/gatonaranja/gatonaranja.env
ExecStart=%h/.local/bin/gatonaranja
Restart=on-failure
RestartSec=5
TimeoutStopSec=60
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gatonaranja
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=default.target
EOF
}

install_service() {
	local tmp_service
	tmp_service="$(mktemp)"
	service_contents >"$tmp_service"

    if [[ -e "$SERVICE_PATH" ]]; then
    	if [[ ! -f "$SERVICE_PATH" ]]; then
    		fail "expected ${SERVICE_PATH} to be a regular file"
    	fi
    	if ! cmp -s "$tmp_service" "$SERVICE_PATH"; then
    		local backup_path="${SERVICE_PATH}.bak.$(date +%Y%m%d%H%M%S)"
    		cp "$SERVICE_PATH" "$backup_path"
    		warn "backed up existing service file to ${backup_path}"
    	fi
    fi

	install -m 644 "$tmp_service" "$SERVICE_PATH"
	rm -f "$tmp_service"
	log "Installed service file at ${SERVICE_PATH}"
}

check_runtime_dependencies() {
	if ! command -v ffmpeg >/dev/null 2>&1; then
		warn "ffmpeg is not installed or not in PATH"
	fi
	if ! command -v yt-dlp >/dev/null 2>&1; then
		warn "yt-dlp is not installed or not in PATH"
	fi
}

enable_service() {
	if ! command -v systemctl >/dev/null 2>&1; then
		warn "systemctl not found; skipping automatic service enable/start"
		return 1
	fi

	if ! systemctl --user daemon-reload; then
		warn "failed to reload the user systemd daemon"
		return 1
	fi

	if ! systemctl --user enable --now "$APP_NAME"; then
		warn "failed to enable or start the user service automatically"
		return 1
	fi
}

print_next_steps() {
	cat <<EOF

Installation complete.

Installed files:
  Binary:      ${BIN_PATH}
  Working dir: ${DATA_DIR}
  Env file:    ${ENV_PATH}
  Service:     ${SERVICE_PATH}

Next steps:
  1. Edit ${ENV_PATH} and set TELEGRAM_BOT_TOKEN.
  2. Optionally set AUTHORIZED_USERS, MAX_CONCURRENT_DOWNLOADS, and DOWNLOAD_TIMEOUT.
EOF

	if [[ "$SERVICE_ENABLED" == "true" ]]; then
		cat <<EOF
  3. Restart the service after changes:
     systemctl --user restart ${APP_NAME}
  4. View logs:
     journalctl --user -u ${APP_NAME} -f
EOF
	else
		cat <<EOF
  3. Enable and start the service manually:
     systemctl --user daemon-reload
     systemctl --user enable --now ${APP_NAME}
  4. View logs:
     journalctl --user -u ${APP_NAME} -f
EOF
	fi

	cat <<EOF

If you want the user service to keep running after logout, consider enabling linger:
  loginctl enable-linger "${USER}"
EOF
}

main() {
	local os arch

	parse_args "$@"
	check_runtime_dependencies
	read -r os arch < <(detect_platform)
	resolve_release_asset "$os" "$arch"
	ensure_directories
	install_binary
	create_env_file
	install_service

	if [[ "$SKIP_ENABLE" == "true" ]]; then
		log "Skipping service enable/start as requested"
	else
	    if enable_service; then
	    	SERVICE_ENABLED="true"
	    else
	    	warn "you can try enabling the service manually with: systemctl --user enable --now ${APP_NAME}"
	    fi
	fi

	print_next_steps
}

main "$@"
