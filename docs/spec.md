# Specification

## Project Introduction

This repository contains a Go-based implementation of a Model Context Protocol (MCP) server that provides network utility tools.

User can use MCP to call the network utility tools to check the network status on their own remote server.

The program only supports Linux systems. macOS and Windows support has been removed.

This program requires root privileges to run (e.g., must be run with `sudo`) to ensure full functionality (like `ss` port scanning and `pkill`).

## Current Features

- [x] `cache`
    - [x] Query Records: Read records from `cache.db` based on time or time range provided by user, when user start query `timestamp` is required items, `tool_name` is optional.
- [x] `letency`
    - [x] Ping
- [x] `port`
    - [x] Port usage status (via `ss` command)
- [x] `system`
    - [x] System Stats
        - [x] System Info
        - [x] CPU Usage
        - [x] PID of most CPU usage process (highest top10 CPU usage over a 5-second interval)
        - [x] Memory Usage
        - [x] PID of most memory usage process (highest top10)
        - [x] Network Interface Usage
        - [x] Disk Usage
    - [x] System Control
        - [x] `pkill` process by PID (Name resolution via Agent)
- [x] `systemd`
    - [x] Manage systemctl, providing several control options including enable, disable, stop, start, status, restart, and reload.
    - [x] View the journalctl logs for a specific systemctl process, user can optionally specify the number of recent log entries to display; the default is 100.entries for analysis.
    - [x] List Server
        - [x] View all services that have been loaded into memory (Loaded Units) by systemd in the current system.
        - [x] View "all" installed services (Installed Files) by systemd in the current system.
- [x] `traceroute`
    - [x] Traceroute
- [x] `diagnostics`
    - [x] System Diagnostics
        - [x] View the last 100 error entries in journalctl
        - [x] View the last 100 error entries in /var/log/syslog
        - [x] View the last 50 entries in dmesg (kernel ring buffer)
        - [x] last / lastb: View recent user login history and failed login attempts (possible brute-force attacks), last 10 entries
## Cache

When the -D flag is used to define the cache path, the caching feature is enabled. Every MCP tool output is automatically saved to the cache file. The cache content is stored in the `cache.db` file in the defined path. The structure of `cache.db` is as follows: 

- Table `records`, Columns:
    - `timestamp`
    - `tool_name` MCP tool name
    - `mcp_output` (JSON structured text, utilizing the MCP output text directly)