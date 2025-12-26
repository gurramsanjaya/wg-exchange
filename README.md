# wg-exchange

[![dispatch build](https://github.com/gurramsanjaya/wg-exchange/actions/workflows/dispatch.yml/badge.svg?event=workflow_dispatch)](https://github.com/gurramsanjaya/wg-exchange/actions/workflows/dispatch.yml)
![GitHub Release](https://img.shields.io/github/v/release/gurramsanjaya/wg-exchange)


wg-exchange is a Go-based tool that manages key exchange for WireGuard VPNs. It simplifies the process of generating, exchanging, and managing keys to facilitate secure and seamless VPN setup.

## Features

- Generate WireGuard key pairs
- Exchange keys securely between peers through tls (not completed)
- Manage peer configurations

## Installation

To build and install wg-exchange, ensure you have Go installed:

```bash
git clone https://github.com/yourusername/wg-exchange.git
cd wg-exchange
make all
```

## Usage
Refer to the example tomls for the format of conf files <br>
server:
```
Usage of ./server:
  -conf string
        server toml conf file (default "server.toml")
  -dbus
        enable dbus systemd management
  -version
        version
```
client:
```
Usage of ./client:
  -conf string
        config file name (default "client.toml")
  -version
        version
```
