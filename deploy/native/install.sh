#!/usr/bin/env bash
# Install (or remove) RkTopNG as a native systemd service — no Docker needed.
#
#   sudo ./install.sh [path/to/rktopng-binary]     install / upgrade
#   sudo ./install.sh --uninstall                  remove the service (keeps /etc/rktopng)
#
# The binary defaults to ./rktopng next to this script, then to ../../dist/rktopng-linux-arm64
# (what `make build-arm64` produces).
set -euo pipefail

BIN_DST=/usr/local/bin/rktopng
UNIT_DST=/etc/systemd/system/rktopng.service
CONF_DIR=/etc/rktopng
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ $EUID -ne 0 ]]; then
  echo "Run as root: sudo $0 $*" >&2
  exit 1
fi

if [[ "${1:-}" == "--uninstall" ]]; then
  systemctl disable --now rktopng 2>/dev/null || true
  rm -f "$UNIT_DST" "$BIN_DST"
  systemctl daemon-reload
  echo "RkTopNG removed (configuration kept in $CONF_DIR)."
  exit 0
fi

SRC="${1:-}"
if [[ -z "$SRC" ]]; then
  for c in "$HERE/rktopng" "$HERE/../../dist/rktopng-linux-arm64"; do
    [[ -f "$c" ]] && SRC="$c" && break
  done
fi
if [[ -z "$SRC" || ! -f "$SRC" ]]; then
  echo "Binary not found. Pass its path, or build it first: make build-arm64" >&2
  exit 1
fi

install -m 0755 "$SRC" "$BIN_DST"
install -d -m 0755 "$CONF_DIR"
# Never overwrite an existing configuration on upgrade.
[[ -f "$CONF_DIR/rktopng.env" ]] || install -m 0600 "$HERE/rktopng.env.example" "$CONF_DIR/rktopng.env"
# It may hold the password (RKTOP_AUTH_PASSWORD): root only. systemd reads it as root.
chmod 0600 "$CONF_DIR/rktopng.env"
install -m 0644 "$HERE/rktopng.service" "$UNIT_DST"

systemctl daemon-reload
systemctl enable rktopng >/dev/null
systemctl restart rktopng

if ! command -v smartctl >/dev/null 2>&1; then
  echo "Note: smartctl not found - SATA disk temperature/health will be missing."
  echo "      Install it with:  sudo apt install smartmontools   (then: sudo systemctl restart rktopng)"
fi

port="$(grep -E '^RKTOP_PORT=' "$CONF_DIR/rktopng.env" 2>/dev/null | tail -1 | cut -d= -f2 || true)"
echo "RkTopNG installed and running on port ${port:-9888}."
echo "  status: systemctl status rktopng      logs: journalctl -u rktopng -f"
