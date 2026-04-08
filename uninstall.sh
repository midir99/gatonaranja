#!/usr/bin/env bash

set -Eeuo pipefail

APP_NAME="gatonaranja"

BIN_PATH="${HOME}/.local/bin/${APP_NAME}"
DATA_DIR="${HOME}/.local/share/${APP_NAME}"
CONFIG_DIR="${HOME}/.config/${APP_NAME}"
ENV_PATH="${CONFIG_DIR}/${APP_NAME}.env"
SYSTEMD_USER_DIR="${HOME}/.config/systemd/user"
SERVICE_PATH="${SYSTEMD_USER_DIR}/${APP_NAME}.service"

REMOVE_DATA="false"
REMOVE_CONFIG="false"

log() {
    printf '%s\n' "$*"
}

warn() {
    printf 'warning: %s\n' "$*" >&2
}

usage() {
    cat <<EOF
Uninstall ${APP_NAME} from the current user account.

Usage:
  ./uninstall.sh [--remove-data] [--remove-config]

Options:
  --remove-data    Also remove ${DATA_DIR}
  --remove-config  Also remove ${ENV_PATH} and ${CONFIG_DIR} if empty
  -h, --help       Show this help message

By default, this script removes the binary and systemd user service, but keeps
the working directory and configuration file to avoid accidental data loss.
EOF
}

parse_args() {
    while (($# > 0)); do
        case "$1" in
        --remove-data)
            REMOVE_DATA="true"
            shift
            ;;
        --remove-config)
            REMOVE_CONFIG="true"
            shift
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        *)
            printf 'error: unknown argument: %s\n' "$1" >&2
            exit 1
            ;;
        esac
    done
}

has_systemctl() {
    command -v systemctl >/dev/null 2>&1
}

disable_service() {
    if ! has_systemctl; then
        warn "systemctl not found; skipping service disable/stop"
        return
    fi

    if [[ -f "$SERVICE_PATH" ]]; then
        systemctl --user disable --now "$APP_NAME" >/dev/null 2>&1 || true
        systemctl --user daemon-reload >/dev/null 2>&1 || true
        log "Disabled and stopped ${APP_NAME} user service"
    fi
}

remove_service_file() {
    if [[ -f "$SERVICE_PATH" ]]; then
        rm -f "$SERVICE_PATH"
        log "Removed service file ${SERVICE_PATH}"
    else
        log "Service file not found, skipping: ${SERVICE_PATH}"
    fi
}

remove_binary() {
    if [[ -f "$BIN_PATH" ]]; then
        rm -f "$BIN_PATH"
        log "Removed binary ${BIN_PATH}"
    else
        log "Binary not found, skipping: ${BIN_PATH}"
    fi
}

remove_data() {
    if [[ "$REMOVE_DATA" != "true" ]]; then
        log "Keeping working directory ${DATA_DIR}"
        return
    fi

    if [[ -d "$DATA_DIR" ]]; then
        rm -rf "$DATA_DIR"
        log "Removed working directory ${DATA_DIR}"
    else
        log "Working directory not found, skipping: ${DATA_DIR}"
    fi
}

remove_config() {
    if [[ "$REMOVE_CONFIG" != "true" ]]; then
        log "Keeping configuration file ${ENV_PATH}"
        return
    fi

    if [[ -f "$ENV_PATH" ]]; then
        rm -f "$ENV_PATH"
        log "Removed configuration file ${ENV_PATH}"
    else
        log "Configuration file not found, skipping: ${ENV_PATH}"
    fi

    if [[ -d "$CONFIG_DIR" ]]; then
        rmdir "$CONFIG_DIR" >/dev/null 2>&1 || true
    fi
}

main() {
    parse_args "$@"
    disable_service
    remove_service_file
    remove_binary
    remove_data
    remove_config

    cat <<EOF

Uninstall complete.

Removed:
  Binary:  ${BIN_PATH}
  Service: ${SERVICE_PATH}

Kept by default:
  Working dir: ${DATA_DIR}
  Config file: ${ENV_PATH}

Use --remove-data and/or --remove-config if you also want to delete those.
EOF
}

main "$@"
