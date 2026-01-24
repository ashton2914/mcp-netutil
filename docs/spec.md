# Specification

## Project Introduction

This repository contains a Go-based implementation of a Model Context Protocol (MCP) server that provides network utility tools.

User can use MCP to call the network utility tools to check the network status on their own remote server.

The program only supports Linux systems. macOS and Windows support has been removed.

This program requires root privileges to run (e.g., must be run with `sudo`) to ensure full functionality (like `ss` port scanning and `pkill`).

## Current Features

- [x] Ping
- [x] Traceroute
- [x] System Stats
    - [x] System Info
    - [x] CPU Usage
    - [x] PID of most CPU usage process (highest top10 CPU usage over a 5-second interval)
    - [x] Memory Usage
    - [x] PID of most memory usage process (highest top10)
    - [x] Network Interface Usage
    - [x] Port usage status (via `ss` command)
    - [x] Disk Usage
- [x] System Management
    - [x] `pkill` process by PID (Name resolution via Agent)
- [ ] Query Records: Read records from `cache.db` based on time or time range provided by user, when user start query `timestamp` is required items, `tool_name` is optional.

## Cache

When the -D flag is used to define the cache path, the caching feature is enabled. Every MCP tool output is automatically saved to the cache file. The cache content is stored in the `cache.db` file in the defined path. The structure of `cache.db` is as follows: 

- Table `records`, Columns:
    - `timestamp`
    - `tool_name` MCP tool name
    - `mcp_output` (JSON structured text, utilizing the MCP output text directly)