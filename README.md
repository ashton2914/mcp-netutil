# mcp-traceroute

## How to use

```bash
mkdir -p ~/.config/systemd/user/ && \
nano ~/.config/systemd/user/mcp-traceroute.service
```

```ini
[Unit]
Description=mcp-traceroute(User Mode)
After=network.target

[Service]
# Ensure the path is absolute
ExecStart=%h/mcp/mcp-traceroute -a 0.0.0.0 -p 20000
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
systemctl --user start mcp-traceroute
systemctl --user enable mcp-traceroute
systemctl --user status mcp-traceroute
journalctl --user -u mcp-traceroute -f
```