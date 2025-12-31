# wg-exchange

[![dispatch build](https://github.com/gurramsanjaya/wg-exchange/actions/workflows/dispatch.yml/badge.svg?event=workflow_dispatch)](https://github.com/gurramsanjaya/wg-exchange/actions/workflows/dispatch.yml)
![GitHub Release](https://img.shields.io/github/v/release/gurramsanjaya/wg-exchange)


wg-exchange is a Go-based tool that manages key exchange for WireGuard VPNs. It simplifies the process of generating, exchanging, and managing keys to facilitate secure and seamless VPN setup.

## Features

- Generate WireGuard key pairs
- Exchange keys securely between peers through TLS.
- Manage peer configurations
- Basic TLS cert generation in Makefile.

## Installation

To build and install wg-exchange, ensure you have Go installed:

```bash
git clone https://github.com/yourusername/wg-exchange.git
cd wg-exchange
make all
```

## Usage
Refer to the example TOML files for the format of configuration files.

**Server (may need `sudo` access for `/etc/wireguard` and system D-Bus):**
```
Usage of ./wge-server:
  -cert string
        tls server cert file, the first cert will be taken as the server cert. Any CAs in here will be considered in addition to the system CAs. (default "server.pem")
  -conf string
        server toml conf file (default "server.toml")
  -dbus
        enable dbus systemd management
  -key string
        tls server key file (default "server.key")
  -listen string
        address:port to listen on (default "127.0.0.1:7777")
  -version
        version
```
**Client:**
```
Usage of ./wge-client:
  -cert string
        tls client cert file, the first cert will be taken as the client cert. Any CAs in here will be considered in addition to the system CAs. (default "client.pem")
  -conf string
        config file name (default "client.toml")
  -endpoint string
        server endpoint (default "https://127.0.0.1:7777")
  -key string
        tls client key file (default "client.key")
  -version
        version
```

**TLS cert generation:** 

Modify the openssl.cnf and the make rules as needed
```bash
make all-tls
```

>**Note:**  In the `openssl.cnf` file, ensure that the `subjAltName` is set correctly for the server; otherwise, client authentication may fail.  
> - If your server has a Fully Qualified Domain Name (FQDN), use `DNS:<domain>`.  
> - If your server does not have an FQDN, you can use its static IP with `IP:<addr>`.  
> - If the server's IP is temporary or dynamic, you can assign a custom domain name with `DNS:<domain>`.  
  But make sure to add this domain-to-IP mapping in your client's `/etc/hosts` file.

