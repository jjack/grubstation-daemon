# Bootloader Configuration

> **Note:** The `sudo grubstation setup` and `sudo grubstation apply` commands handle this automatically. You only need to follow these steps if you are manually configuring the system.

## Configure GRUB

> **Note:** The exact GRUB networking configuration applied by this tool may not work perfectly for every motherboard due to how finicky UEFI and network firmware can be across different hardware vendors. If your system struggles to connect to the network from within GRUB, you may need to manually troubleshoot your GRUB network settings.

Create a new GRUB config file at `/etc/grub.d/99_ha_remote_boot_manager` with the following content (making sure to replace `$hass_url`, `$mac_address`, and `$webhook_id` ith your actual Home Assistant details and the host's MAC address):

```bash
#!/bin/sh
set -e

cat << EOF
insmod net
insmod efinet
insmod http
net_bootp
source (http,$hass_url)/api/remote_boot_manager/$mac_address?$webhook_id
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