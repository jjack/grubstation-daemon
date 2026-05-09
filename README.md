# GrubStation CLI

![GitHub](https://img.shields.io/github/license/jjack/grubstation-cli)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/jjack/grubstation-cli)
[![GO Tests and Coverage](https://github.com/jjack/grubstation-cli/actions/workflows/test.yml/badge.svg)](https://github.com/jjack/grubstation-cli/actions/workflows/test.yml)
[![CodeQL](https://github.com/jjack/grubstation-cli/actions/workflows/github-code-scanning/codeql/badge.svg)](https://github.com/jjack/grubstation-cli/actions/workflows/github-code-scanning/codeql)
[![Codecov branch](https://img.shields.io/codecov/c/github/jjack/grubstation-cli)](https://app.codecov.io/gh/jjack/grubstation-cli)

`grubstation` is a Go-based agent designed to manage bare-metal OS booting and selection via [Home Assistant](https://www.home-assistant.io/) and Wake-on-LAN (WOL). It helps enable a user to remotely select an operating system for a specific host, send a wake on lan packet, and have the machine dynamically boot into the chosen OS.

After installation, whenever your server shuts down, `grubstation` will read the available boot options and push them to Home Assistant through a webhook. After selecting an option in Home Assistant, you can either press the "Wake" button or just power the machine on normally. It will then boot with your newly selected options.


## Supported Systems

| Type | Supported |
| :--- | :--- |
| **Bootloaders** | GRUB | 
| **Init Systems** | systemd |

## Quick Start

**Requirements:**
- [Home Assistant](https://www.home-assistant.io/)
- [Home Assistant Grubstation](https://github.com/jjack/ha-grubstation) Integration
- Supported Bootloader and Init System (see above)

**Recommended Installation:**
1. Download the latest pre-built package for your OS from the [Releases Page](https://github.com/jjack/ha-grubstation/releases/latest).
2. Install the package (e.g., `sudo dpkg -i grubstation_*_amd64.deb`).
3. Run the automated setup wizard to auto-detect and configure your network info, home assistant info, bootloader, and init system:
   ```bash
   sudo grubstation setup
   ```

## Documentation

For advanced setups or manual configuration, please refer to the documentation:

- **Installation**
  - [Advanced Installation Methods](/docs/installation/advanced.md)
- **Configuration**
  - [Agent Setup and Configuration](/docs/configuration/setup.md)
  - [Manual Bootloader Configuration](/docs/configuration/bootloader.md)
  - [Manual Init System Configuration](/docs/configuration/init-system.md)
