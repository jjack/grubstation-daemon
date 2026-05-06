# Remote Boot Agent

![GitHub](https://img.shields.io/github/license/jjack/remote-boot-agent)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/jjack/remote-boot-agent)
[![GO Tests and Coverage](https://github.com/jjack/remote-boot-agent/actions/workflows/test.yml/badge.svg)](https://github.com/jjack/remote-boot-agent/actions/workflows/test.yml)
[![CodeQL](https://github.com/jjack/remote-boot-agent/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/jjack/remote-boot-agent/actions/workflows/github-code-scanning/codeql)
[![Codecov branch](https://img.shields.io/codecov/c/github/jjack/remote-boot-agent)](https://app.codecov.io/gh/jjack/remote-boot-agent)

`remote-boot-agent` is a Go-based agent designed to manage bare-metal OS booting and selection via [Home Assistant](https://www.home-assistant.io/) and Wake-on-LAN (WOL). It helps enable a user to remotely select an operating system for a specific host, send a wake on lan packet, and have the machine dynamically boot into the chosen OS.

After installation, whenever your server shuts down, `remote-boot-agent` will read the available boot options and push them to Home Assistant through a webhook. After selecting an option in Home Assistant, you can either press the "Wake" button or just power the machine on normally. It will then boot with your newly selected options.


## Supported Systems

| Type | Supported |
| :--- | :--- |
| **Bootloaders** | GRUB | 
| **Init Systems** | systemd |

## Quick Start

**Requirements:**
- [Home Assistant](https://www.home-assistant.io/)
- [Home Assistant Remote Boot Manager](https://github.com/jjack/hass-remote-boot-manager) Integration
- Supported Bootloader and Init System (see above)

**Recommended Installation:**
1. Download the latest pre-built package for your OS from the [Releases Page](https://github.com/jjack/hass-remote-boot-manager/releases/latest).
2. Install the package (e.g., `sudo dpkg -i remote-boot-agent_*_amd64.deb`).
3. Run the automated setup wizard to auto-detect and configure your network info, home assistant info, bootloader, and init system:
   ```bash
   sudo remote-boot-agent setup
   ```

## Documentation

For advanced setups or manual configuration, please refer to the documentation:

- **Installation**
  - [Advanced Installation Methods](/docs/installation/advanced.md)
- **Configuration**
  - [Agent Setup and Configuration](/docs/configuration/setup.md)
  - [Manual Bootloader Configuration](/docs/configuration/bootloader.md)
  - [Manual Init System Configuration](/docs/configuration/init-system.md)
