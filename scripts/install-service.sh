#!/usr/bin/env bash
# install-service.sh — install x-extract-server as a macOS LaunchAgent
# Usage: ./scripts/install-service.sh [--uninstall]
set -euo pipefail

LABEL="com.x-extract.server"
PLIST_SRC="$(cd "$(dirname "$0")/.." && pwd)/launchd/${LABEL}.plist"
PLIST_DST="$HOME/Library/LaunchAgents/${LABEL}.plist"
BINARY="$HOME/bin/x-extract-server"
LOGS_DIR="$HOME/Downloads/x-download/logs"

# ── Uninstall ──────────────────────────────────────────────────────────────────
if [[ "${1:-}" == "--uninstall" ]]; then
  echo "Stopping and unloading LaunchAgent..."
  launchctl bootout "gui/$(id -u)/${LABEL}" 2>/dev/null || true
  rm -f "$PLIST_DST"
  echo "Uninstalled. To remove the binary: rm $BINARY"
  exit 0
fi

# ── Pre-flight checks ──────────────────────────────────────────────────────────
if [[ "$(uname)" != "Darwin" ]]; then
  echo "Error: this script is macOS-only (launchd)." >&2
  exit 1
fi

if [[ ! -f "$BINARY" ]]; then
  echo "Error: binary not found at $BINARY — run 'make deploy' first." >&2
  exit 1
fi

# ── Write plist ────────────────────────────────────────────────────────────────
mkdir -p "$HOME/Library/LaunchAgents"
mkdir -p "$LOGS_DIR"

sed \
  -e "s|__BINARY_PATH__|$BINARY|g" \
  -e "s|__LOGS_DIR__|$LOGS_DIR|g" \
  "$PLIST_SRC" > "$PLIST_DST"

echo "Wrote $PLIST_DST"

# ── Load (or reload) ───────────────────────────────────────────────────────────
# Unload existing instance quietly before reloading
launchctl bootout "gui/$(id -u)/${LABEL}" 2>/dev/null || true
sleep 1
launchctl bootstrap "gui/$(id -u)" "$PLIST_DST"

echo ""
echo "✓ x-extract-server is now running as a LaunchAgent."
echo "  It will start automatically on every login and restart on crash."
echo ""
echo "  Status:    launchctl print gui/\$(id -u)/${LABEL}"
echo "  Logs:      tail -f ${LOGS_DIR}/launchd-stderr.log"
echo "  Uninstall: make uninstall-service"
