# Agent Setup and Configuration

The `grubstation` agent needs to be configured to communicate with your Home Assistant instance and identify itself on your local network. You can use the interactive setup wizard (recommended) or configure it manually.

## 1. Interactive Setup Wizard (Recommended)

The easiest way to get started is by running the automated setup wizard. This tool will auto-detect your system settings and guide you through the integration process.

```bash
sudo grubstation setup
```

### What the wizard handles:
- **System Detection:** Identifies your Init System (e.g., `systemd`) and Bootloader (GRUB).
- **Network Identification:** Helps you select the correct network interface and MAC address for Wake-on-LAN.
- **Home Assistant Integration:** Configures the connection URL and secure Webhook ID.
- **Service Installation:** Automatically installs and starts the background daemon.

## 2. Applying Configuration Directly

If you have an existing `config.yaml` (e.g., deployed via Ansible) or want to re-apply the system hooks without going through the survey, use the `apply` flag:

```bash
sudo grubstation setup --apply --config /etc/grubstation/config.yaml
```

## 3. The GrubStation Daemon

`grubstation` runs as a persistent background service (daemon). This allows it to:
1. **Push updates:** Report available boot entries to Home Assistant whenever they change or during system events.
2. **Handle Remote Shutdown:** Provide an API endpoint for Home Assistant to safely power off the machine.
3. **Health Monitoring:** Provide a status endpoint for monitoring the agent's health.

### API Endpoints

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/status` | `GET` | Returns the current agent status and system metadata. |
| `/shutdown` | `POST` | Triggers a system shutdown (requires Authentication). |

### Remote Shutdown Authentication
The `/shutdown` endpoint is secured via an API key.
- **TOFU (Trust On First Use):** If no key is configured, the agent generates a secure random token on startup and pushes it to Home Assistant.
- **Static Key:** You can optionally define a static `api_key` in your configuration file.

## 4. Manual Configuration Reference

The configuration file is typically located at `/etc/grubstation/config.yaml`.

```yaml
# Host identification
host:
  address: "192.168.1.50"      # This machine's IP or FQDN
  mac: "00:11:22:33:44:55"     # MAC address used for Wake-on-LAN

# Wake-on-LAN settings
wake_on_lan:
  # address: "255.255.255.255" # Optional: Custom broadcast address
  # port: 9                    # Optional: Custom UDP port

# Home Assistant connection
homeassistant:
  url: "http://ha.local:8123"  # Your Home Assistant base URL
  webhook_id: "your_id_here"   # The unique Webhook ID from the HA Integration

# Daemon behavior
daemon:
  port: 8081                   # The port the daemon listens on
  report_boot_options: true    # Enable/Disable pushing entries to HA
  # api_key: "secure_secret"   # Optional: Static API key for /shutdown

# Bootloader settings
grub:
  config_path: "/boot/grub/grub.cfg" # Path to your grub.cfg
```

## 5. Security Architecture

`grubstation` is designed with a "security-by-default" mindset:

- **Unique Webhooks:** Communication with Home Assistant is routed through a unique, non-guessable Webhook ID. This acts as a shared secret between your agent and your HA instance.
- **Secure Shutdowns:** The remote shutdown feature is protected by a 256-bit token (either pre-configured or generated via TOFU).
- **Minimal Surface:** The daemon only exposes a strictly defined API and does not require incoming connections from the public internet.

## 6. BIOS & UEFI Requirements

For remote booting and Wake-on-LAN to function, ensure the following are enabled in your BIOS/UEFI settings:

1. **Wake-on-LAN (WOL):** Often labeled as "Wake on Magic Packet" or "Power On By PCIe".
2. **Network Stack:** Required for GRUB to initialize the NIC and perform HTTP requests during boot.
3. **Disable Fast Boot:** Many "Fast Boot" implementations skip network initialization.
4. **Disable Deep Sleep (ErP):** If ErP is enabled, the NIC may lose power entirely when the PC is off, preventing it from hearing the WOL packet.
