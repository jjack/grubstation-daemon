# Remote Boot Agent

![GitHub](https://img.shields.io/github/license/jjack/remote-boot-agent)
![GitHub Repo stars](https://img.shields.io/github/stars/jjack/remote-boot-agent)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/jjack/remote-boot-agent)
[![GO Tests and Coverage](https://github.com/jjack/remote-boot-agent/actions/workflows/test.yml/badge.svg)](https://github.com/jjack/remote-boot-agent/actions/workflows/test.yml)
[![CodeQL](https://github.com/jjack/remote-boot-agent/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/jjack/remote-boot-agent/actions/workflows/github-code-scanning/codeql)

![GitHub contributors](https://img.shields.io/github/contributors/jjack/remote-boot-agent)
![Maintenance](https://img.shields.io/maintenance/yes/2026)
![GitHub commit activity](https://img.shields.io/github/commit-activity/y/jjack/remote-boot-agent)
![GitHub last commit](https://img.shields.io/github/last-commit/jjack/remote-boot-agent/main)
![Codecov branch](https://img.shields.io/codecov/c/github/jjack/remote-boot-agent)

`remote-boot-agent` is a Go-based agent designed to manage bare-metal OS booting and selection via Home Assistant and Wake-on-LAN (WOL). It allows a user to remotely select an operating system for a specific server via a Home Assistant dropdown, send a WOL packet, and have the server dynamically boot into the chosen OS.

## Core Architecture

The system is built with a strictly pluggable architecture in mind. While `grub` and `systemd` are the default implementations, the CLI and core logic are agnostic to the underlying bootloader and init system. This _should_ be adaptable to any other bootloader that has a networking stack as well as systems other than Home Assistant.

After installation, whenever your server shuts down, `remote-boot-agent` will read the available boot options and push them to Home Assistant through a webhook. After selecting an option in Home Assistant, you can either press the "Wake" button or just power the machine on normally. It will then boot with your newly selected options.

## Requirements

- [Home Assistant](https://www.home-assistant.io/)
- [Home Assistant Remote Boot Manager](https://github.com/jjack/remote-boot-agent)
- a Linux bootloader with a networking stack (ie - `grub`)

## Getting Started

### Installation & Deployment

You can install `remote-boot-agent` using the pre-built packages or binaries from the Releases page.

**1. Install the Package**

Download the appropriate package for your system from the Releases page.

For Debian/Ubuntu:
```bash
sudo dpkg -i remote-boot-agent_*_amd64.deb
```

**2. Configure GRUB**
Create a new GRUB config file at `/etc/grub.d/99_ha_remote_boot_manager` with the following content (making sure to replace `$protocol`, `$hass_url`, and `$mac_address` with your actual Home Assistant details and the node's MAC address):

```bash
#!/bin/sh
set -e

cat << EOF
insmod net
insmod efinet
insmod http
net_bootp
source ($protocol,$hass_url)/api/remote_boot_manager/$mac_address
EOF
```

Make the script executable and regenerate your GRUB config:
```bash
sudo chmod +x /etc/grub.d/99_ha_remote_boot_manager

# On Debian/Ubuntu
sudo update-grub

# On RHEL/Fedora
sudo grub2-mkconfig -o /boot/grub2/grub.cfg
```

**3. Configure the Init Manager Shutdown Hook**
To run the `push` command on every system shutdown, create a systemd service file at `/etc/systemd/system/remote-boot-agent.service`:

```ini
[Unit]
Description=Push remote boot state to Home Assistant on shutdown
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target network-online.target
Requires=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/remote-boot-agent options push --config /etc/remote-boot-agent/config.yaml
TimeoutSec=10

[Install]
WantedBy=halt.target reboot.target poweroff.target
```

Enable and reload the daemon:
```bash
sudo systemctl daemon-reload
sudo systemctl enable remote-boot-agent.service
