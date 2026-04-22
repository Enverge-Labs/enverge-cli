#!/bin/bash
set -euo pipefail

# Enverge Agent installer for DGX Spark (linux/arm64)
# Usage: curl -fsSL https://raw.githubusercontent.com/Enverge-Labs/enverge-cli/main/install-agent.sh | sudo bash -s -- <bootstrap-token> [api-url]

BOOTSTRAP_TOKEN="${1:-}"
API_URL="${2:-https://enverge-control-plane-production.up.railway.app}"

if [ -z "$BOOTSTRAP_TOKEN" ]; then
  echo "Usage: curl -fsSL <url> | sudo bash -s -- <bootstrap-token> [api-url]"
  echo "  bootstrap-token: required"
  echo "  api-url: optional (default: https://enverge-control-plane-production.up.railway.app)"
  exit 1
fi

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/enverge"
BINARY_URL="https://github.com/Enverge-Labs/enverge-cli/releases/download/v0.1.0-agent/enverge-agent-linux-arm64"

echo "==> Installing cloudflared..."
if ! command -v cloudflared &>/dev/null; then
  ARCH=$(uname -m)
  case "$ARCH" in
    aarch64|arm64) CF_ARCH="arm64" ;;
    x86_64)        CF_ARCH="amd64" ;;
    *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
  esac
  CF_URL="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CF_ARCH}"
  curl -fsSL -o "${INSTALL_DIR}/cloudflared" "$CF_URL"
  chmod +x "${INSTALL_DIR}/cloudflared"
  echo "    cloudflared installed at ${INSTALL_DIR}/cloudflared"
else
  echo "    cloudflared already installed at $(which cloudflared)"
fi

echo "==> Downloading enverge-agent..."
curl -fsSL -o "${INSTALL_DIR}/enverge-agent" "$BINARY_URL"
chmod +x "${INSTALL_DIR}/enverge-agent"

echo "==> Creating config directory..."
mkdir -p "$CONFIG_DIR"

echo "==> Writing environment file..."
cat > "${CONFIG_DIR}/agent.env" <<EOF
ENVERGE_API_URL=${API_URL}
ENVERGE_BOOTSTRAP_TOKEN=${BOOTSTRAP_TOKEN}
ENVERGE_DRY_RUN=false
EOF
chmod 600 "${CONFIG_DIR}/agent.env"

echo "==> Installing systemd service..."
cat > /etc/systemd/system/enverge-agent.service <<EOF
[Unit]
Description=Enverge Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${CONFIG_DIR}/agent.env
ExecStart=${INSTALL_DIR}/enverge-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable enverge-agent

echo ""
echo "==> Installed successfully!"
echo ""
echo "  Binary:      ${INSTALL_DIR}/enverge-agent"
echo "  Cloudflared: $(which cloudflared)"
echo "  Config:      ${CONFIG_DIR}/agent.env"
echo "  Service:     enverge-agent.service"
echo ""
echo "  Next steps:"
echo "    1. Start the agent:  sudo systemctl start enverge-agent"
echo "    2. The agent will register and wait for admin approval"
echo "    3. Once approved, the CF tunnel starts automatically"
echo "    4. Check logs:       sudo journalctl -u enverge-agent -f"
echo ""
