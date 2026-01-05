# mcp-netutil

## How to use

```bash
mkdir -p ~/.config/systemd/user/ && \
nano ~/.config/systemd/user/mcp-netutil.service
```

```ini
[Unit]
Description=mcp-netutil(User Mode)
After=network.target

[Service]
# Ensure the path is absolute
ExecStart=%h/mcp/mcp-netutil -a 0.0.0.0 -p 20000
Restart=always
RestartSec=3

[Install]
WantedBy=default.target
```

```bash
sudo loginctl enable-linger $USER
```

```
systemctl --user daemon-reload
systemctl --user start mcp-netutil
systemctl --user enable mcp-netutil
systemctl --user status mcp-netutil
journalctl --user -u mcp-netutil -f
```