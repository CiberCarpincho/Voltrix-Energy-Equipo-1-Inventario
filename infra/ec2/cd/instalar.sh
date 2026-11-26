#!/usr/bin/env bash
set -euo pipefail
project_dir=$(realpath "${1:?Indica la ruta absoluta del clon desplegado}")
source_dir=$(cd "$(dirname "$0")" && pwd)
test -d "$project_dir/.git"
test -f "$project_dir/deploy/compose/.env"
install -d -m 0755 /usr/local/lib/voltrix-cd
install -m 0755 "$source_dir/desplegar.py" /usr/local/lib/voltrix-cd/desplegar.py
cat > /etc/systemd/system/voltrix-cd.service <<EOF
[Unit]
Description=Desplegar main de Inventario y verificar PostgreSQL
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=oneshot
WorkingDirectory="$project_dir"
StateDirectory=voltrix-cd
Environment=GIT_TERMINAL_PROMPT=0
ExecStart=/usr/bin/python3 -u /usr/local/lib/voltrix-cd/desplegar.py
EOF
install -m 0644 "$source_dir/voltrix-cd.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now voltrix-cd.timer
systemctl start voltrix-cd.service
systemctl list-timers voltrix-cd.timer --no-pager
