# mcp-netutil

## Run

```
./mcp-netutil [FLAG] [PARAMETER]
```

FLAG
- `-a` listen address
- `-p` listen port
- `-D` Enable cache and define cache directory


## Deploy on your server

Recommend runing under user mode and use systemd user mode to control it

```bash
mkdir -p ~/mcp/share/mcp-netutil/ && \
cd ~/mcp/ && \
wget -O mcp-netutil https://github.com/ashton2914/mcp-netutil/releases/latest/download/mcp-netutil-linux-amd64 && \
chmod +x mcp-netutil && \
cd -
```

```bash
mkdir -p ~/.config/systemd/user/ && \
nano ~/.config/systemd/user/mcp-netutil.service
```

paste below content

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

Please use https if you deploy on public server. Recommended Nginx config

```conf
server {
    location / {
        proxy_pass http://localhost:20000;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_cache off;
        chunked_transfer_encoding on;
        tcp_nopush on;
        tcp_nodelay on;
        keepalive_timeout 300;
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