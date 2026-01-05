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
ExecStart=%h/mcp/mcp-netutil -a 127.0.0.1 -p 20000
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

## Nginx 

```conf
server {
    location / {
        proxy_pass http://localhost:20000;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Accept-Encoding "";
        proxy_method $request_method;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```


## VSCode Configuration

If you have deployed the server on a remote machine (e.g. `yourserverip:20000`), use the following configuration to connect via SSE:

```json
{
  "mcpServers": {
    "mcp-netutil": {
      "serverUrl": "https://yourserver/sse"
    }
  }
}
```