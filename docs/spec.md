# Specification

## Project Introduction

This repository contains a Go-based implementation of a Model Context Protocol (MCP) server that provides network utility tools.

User can use MCP to call the network utility tools to check the network status on their own remote server.

The program needs to provide compatibility with Linux systems, while also taking into account macOS and Windows.

This programe only runs on user mode (no root requirement) and should ensure no damage to the system.

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
    - [x] Disk Usage
- [x] System Management
    - [x] `pkill` process by CMD or PID