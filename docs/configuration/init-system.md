# Init System Configuration

`grubstation` runs as a persistent background service (daemon) to ensure that boot options are always up-to-date in Home Assistant and to handle remote shutdown requests.

> **💡 Note:** The `sudo grubstation setup` command handles service installation automatically for both Linux and Windows. Manual configuration is typically not required.

## 1. Linux (systemd)

On most Linux distributions, `grubstation` is managed as a `systemd` service. The service file is located at `/etc/systemd/system/grubstation.service`.

### Service Definition
The service is configured to start after the network is online and restart automatically if it fails.

```ini
[Unit]
Description=GrubStation Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/grubstation daemon --config /etc/grubstation/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Management Commands

**Start the service:**
```bash
sudo systemctl start grubstation
```

**Check status and logs:**
```bash
sudo systemctl status grubstation
journalctl -u grubstation -f
```

**Stop the service:**
```bash
sudo systemctl stop grubstation
```

---

## 2. Windows (Service Control Manager)

On Windows, `grubstation` runs as a standard Windows Service. 

### Management Commands

You can manage the service using the **Services** desktop app (`services.msc`) or via the command line using `sc`:

**Check status:**
```powershell
sc query grubstation
```

**Start the service:**
```powershell
sc start grubstation
```

**Stop the service:**
```powershell
sc stop grubstation
```

---

## 3. Why run as a Daemon?

Running as a persistent service instead of a one-off script provides several benefits:
- **Graceful Shutdowns:** The daemon catches termination signals (SIGTERM) to perform a final "push" of your boot entries to Home Assistant before the system powers off.
- **Remote Power Management:** It listens for shutdown commands from Home Assistant, allowing you to remotely turn off the machine safely.
- **Reliability:** The init system (systemd or SCM) will automatically restart the agent if it crashes, ensuring your remote boot setup is always available.
