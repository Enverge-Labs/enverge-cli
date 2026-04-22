#!/bin/bash
set -euo pipefail

# Enverge Agent installer for DGX Spark (linux/arm64)
# Usage: curl -fsSL https://raw.githubusercontent.com/Enverge-Labs/enverge-cli/main/install-agent.sh | sudo bash -s -- <bootstrap-token> <api-url>

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

echo "==> Downloading enverge-agent..."
curl -fsSL -o "${INSTALL_DIR}/enverge-agent" "$BINARY_URL"
chmod +x "${INSTALL_DIR}/enverge-agent"

echo "==> Creating config directory..."
mkdir -p "$CONFIG_DIR"

echo "==> Writing environment file..."
cat > "${CONFIG_DIR}/agent.env" <<EOF
ENVERGE_API_URL=${API_URL}
ENVERGE_BOOTSTRAP_TOKEN=${BOOTSTRAP_TOKEN}
ENVERGE_DRY_RUN=true
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
echo "  Binary:  ${INSTALL_DIR}/enverge-agent"
echo "  Config:  ${CONFIG_DIR}/agent.env"
echo "  Service: enverge-agent.service"
echo ""
echo "  IMPORTANT: dry-run mode is ON by default."
echo "  To start the agent:  sudo systemctl start enverge-agent"
echo "  To check logs:       sudo journalctl -u enverge-agent -f"
echo "  To enable real mode: edit ${CONFIG_DIR}/agent.env and set ENVERGE_DRY_RUN=false"
echo ""
