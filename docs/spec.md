# Specification

## Project Introduction

This repository contains a Go-based implementation of a Model Context Protocol (MCP) server that provides network utility tools.

User can use MCP to call the network utility tools to check the network status on their own remote server.

## Current Features

- [x] Ping
- [x] Traceroute
- [x] System Stats
    - [x] System Info
    - [x] CPU Usage
    - [ ] PID of most CPU usage process (avg in 5s)
    - [x] Memory Usage
    - [ ] PID of most memory usage process
    - [ ] Network Interface Usage
    - [x] Disk Usage
- [ ] System Management
    - [ ] `pkill` process by CMD or PID