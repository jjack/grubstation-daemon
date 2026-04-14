# HA-Boot-Manager

`ha-boot-manager` is a Go-based monorepo designed to manage bare-metal OS booting and selection via Home Assistant, MQTT, and Wake-on-LAN (WOL). It allows a user to remotely select an operating system for a specific server via a Home Assistant dropdown, send a WOL packet, and have the server dynamically boot into the chosen OS.

## Core Architecture

The system is built with a strictly pluggable architecture in mind. While GRUB and systemd are the default implementations, the CLI and core logic are agnostic to the underlying bootloader and init system. They should (hopefully!) be adaptable to other systems.

### `remote-boot-agent`
#### Lightweight CLI that runs on each bare-metal server at shutdown time
- Parses the local boot menu to report available OS options to Home Assistant

## Repo Structure

## Getting Started

*(Instructions for building, configuring, and deploying to be added.)*