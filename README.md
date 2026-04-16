# HA-Boot-Manager

`ha-boot-manager` is a Go-based monorepo designed to manage bare-metal OS booting and selection via Home Assistant, MQTT, and Wake-on-LAN (WOL). It allows a user to remotely select an operating system for a specific server via a Home Assistant dropdown, send a WOL packet, and have the server dynamically boot into the chosen OS.

## Core Architecture

The system is built with a strictly pluggable architecture in mind. While GRUB and systemd are the default implementations, the CLI and core logic are agnostic to the underlying bootloader and init system. They should (hopefully!) be adaptable to other systems.

### `ha-remote-boot-agent`
#### Lightweight CLI that runs on each bare-metal server at shutdown time
- Parses the local boot menu to report available OS options to Home Assistant

## Repo Structure

## Getting Started

### Installation & Deployment

You can deploy the `ha-remote-boot-agent` using Ansible or via manual installation.

#### Ansible (Recommended)
An extensible set of Ansible playbooks and roles are provided in the `ansible/` directory.

To run the deployment playbook against a target node:
```bash
ansible-playbook -i your_inventory.ini ansible/deploy.yml \
  -e ha_protocol=http \
  -e ha_host=homeassistant.local:8123
```

The playbook dynamically includes separate roles for the respective `bootloader` (default: `grub`) and `initmanager` (default: `systemd`).

#### Manual Installation

If you prefer to configure your machine manually without Ansible, you can follow these steps:

**1. Copy the Binary**
Build the agent and copy the resultant binary to your system's path:
```bash
go build ./...
sudo cp ha-remote-boot-agent /usr/local/bin/
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
To run the `push` command on every system shutdown, create a systemd service file at `/etc/systemd/system/ha-remote-boot-agent.service`:

```ini
[Unit]
Description=Push remote boot state to Home Assistant on shutdown
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target network-online.target
Requires=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/ha-remote-boot-agent push --config /etc/ha-remote-boot-agent/config.yaml
TimeoutSec=10

[Install]
WantedBy=halt.target reboot.target poweroff.target
```

Enable and reload the daemon:
```bash
sudo systemctl daemon-reload
sudo systemctl enable ha-remote-boot-agent.service
