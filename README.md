# mcp-netutil

## Run

**Note: This program requires root privileges.**

```bash
sudo ./mcp-netutil [FLAG] [PARAMETER]
```

use `./mcp-netutil --help` for help


## Deploy on your server

Recommend running under systemd.

```bash
sudo mkdir -p /opt/mcp-netutil/ && \
cd /opt/mcp-netutil/ && \
sudo wget -O mcp-netutil https://github.com/ashton2914/mcp-netutil/releases/latest/download/mcp-netutil-linux-amd64 && \
sudo chmod +x mcp-netutil && \
cd -
```

```bash
sudo nano /etc/systemd/system/mcp-netutil.service
```

paste below content

```ini
[Unit]
Description=mcp-netutil
Documentation=https://github.com/ashton2914/mcp-netutil
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/mcp-netutil
ExecStart=/opt/mcp-netutil/mcp-netutil -a 127.0.0.1 -p 20000 -D /opt/mcp-netutil -o "you_api_key"
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl start mcp-netutil
sudo systemctl enable mcp-netutil
sudo systemctl status mcp-netutil
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

## Clinet Configuration

```json
{
  "mcpServers": {
    "mcp-netutil": {
      "serverUrl": "https://yourserver/sse/you_api_key"
    }
  }
}
```