#!/usr/bin/env bash
set -euo pipefail

# Install fox-control as a systemd service.
# Usage: sudo ./install.sh /path/to/fox-control

BINARY="${1:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ -z "$BINARY" ]; then
    echo "Usage: sudo $0 /path/to/fox-control" >&2
    exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
    echo "Error: must run as root (use sudo)" >&2
    exit 1
fi

if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found: $BINARY" >&2
    exit 1
fi

echo "Installing fox-control..."

# Create system user (no login shell, no home)
if ! id fox-control &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin fox-control
    echo "  Created user: fox-control"
fi

# Add fox-control user to docker group
if getent group docker &>/dev/null; then
    usermod -aG docker fox-control
    echo "  Added fox-control to docker group"
else
    echo "  Warning: docker group not found — create it or install Docker first" >&2
fi

# Install binary
install -m 0755 "$BINARY" /usr/local/bin/fox-control
echo "  Installed binary: /usr/local/bin/fox-control"

# Create config directory
install -d -m 0750 -o fox-control -g fox-control /etc/fox-control

# Install config (don't overwrite existing)
if [ ! -f /etc/fox-control/fox-control.toml ]; then
    install -m 0640 -o fox-control -g fox-control "$SCRIPT_DIR/fox-control.toml" /etc/fox-control/fox-control.toml
    echo "  Installed config: /etc/fox-control/fox-control.toml"
else
    echo "  Config exists, skipping: /etc/fox-control/fox-control.toml"
fi

# Install env file (don't overwrite existing)
if [ ! -f /etc/fox-control/env ]; then
    install -m 0600 -o fox-control -g fox-control "$SCRIPT_DIR/env.example" /etc/fox-control/env
    echo "  Installed env file: /etc/fox-control/env"
    echo "  >>> Edit /etc/fox-control/env and set real secrets before starting <<<"
else
    echo "  Env file exists, skipping: /etc/fox-control/env"
fi

# Create data directory
install -d -m 0750 -o fox-control -g fox-control /var/lib/fox-control
echo "  Created data dir: /var/lib/fox-control"

# Install systemd unit
install -m 0644 "$SCRIPT_DIR/fox-control.service" /etc/systemd/system/fox-control.service
systemctl daemon-reload
echo "  Installed systemd unit: fox-control.service"

echo ""
echo "Done. Next steps:"
echo "  1. Edit /etc/fox-control/env   (set FOX_ADMIN_SECRET and FOX_INSTANCE_PASSWORD)"
echo "  2. Edit /etc/fox-control/fox-control.toml   (adjust listen address, image, etc.)"
echo "  3. systemctl enable --now fox-control"
echo "  4. journalctl -u fox-control -f"
