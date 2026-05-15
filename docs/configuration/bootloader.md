# Bootloader Configuration (GRUB)

The remote selection feature of `grubstation` works by installing a custom script into your GRUB configuration directory. This script runs early in the boot process, connects to Home Assistant, and determines which OS should be booted.

> **💡 Note:** The `sudo grubstation setup` command handles this installation automatically. Manual configuration is only recommended for advanced users or troubleshooting.

## 1. How it Works

When your computer starts, GRUB executes the scripts in `/etc/grub.d/` in numerical order. `grubstation` installs a script at `/etc/grub.d/99_grubstation`. 

This script:
1. Initializes the network interface (`insmod net`, `insmod efinet`).
2. Performs a DHCP request (`net_bootp`).
3. Fetches a small GRUB-compatible script from your Home Assistant instance.
4. If a specific OS was selected in HA, it overrides the `default` boot entry.

## 2. Manual Installation

If you need to install the hook manually, create a file at `/etc/grub.d/99_grubstation`:

```bash
#!/bin/bash
set -e

cat <<'EOF'
insmod net
insmod efinet
insmod http

# Replace with your HA URL, MAC, and Webhook ID
set boot_url="(http,192.168.1.100:8123)/api/grubstation/00:11:22:33:44:55?token=YOUR_WEBHOOK_ID"

# Wait loop for network (helpful for STP delays)
for i in 1 2 3 4 5; do
    if net_bootp; then
        if source $boot_url; then
            break
        fi
    fi
    sleep 1
done

if [ -n "$next_entry" ]; then
    set default="$next_entry"
fi
EOF
```

### Apply Changes
After creating or modifying the script, you must regenerate your GRUB configuration:

**Debian / Ubuntu:**
```bash
sudo chmod +x /etc/grub.d/99_grubstation
sudo update-grub
```

**Fedora / RHEL / Arch:**
```bash
sudo chmod +x /etc/grub.d/99_grubstation
sudo grub2-mkconfig -o /boot/grub2/grub.cfg
```

## 3. Troubleshooting GRUB Networking

GRUB's networking stack is much more limited than a full operating system. If the remote selection isn't working, consider these factors:

### Required Modules
The script attempts to load `net`, `efinet`, and `http`. Depending on your hardware (e.g., if you are using legacy BIOS instead of UEFI), you might need different modules like `pcnet` or specific vendor drivers.

### Spanning Tree Protocol (STP)
If your network switch has STP enabled, it may take 15-30 seconds for a port to transition to the "forwarding" state after the link comes up. Because GRUB initializes the link very quickly, it often fails the first few DHCP attempts.
- **Solution:** `grubstation` includes a retry loop. You can increase the `wait_time_seconds` in your config to give the switch more time.
- **Optimization:** If possible, set the switch port to "Edge Port" or "PortFast" mode.

### Wireless & Complex Networks
GRUB does **not** support Wi-Fi. The machine must be connected via Ethernet. Additionally, complex network setups (like 802.1X authentication or complex VLAN tagging) are generally not supported within the GRUB environment.
