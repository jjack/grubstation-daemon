# Advanced Installation Methods

While most users should use the standard [Quick Start](/README.md#🚀-quick-start) methods, this guide covers manual installation from binaries or source code.

## 📦 Option A: Pre-built Packages

We provide native packages for major Linux distributions. Download the latest version from the [Releases Page](https://github.com/jjack/grubstation/releases/latest).

### Debian / Ubuntu (`.deb`)
```bash
sudo dpkg -i grubstation_*_amd64.deb
```

### RHEL / Fedora / CentOS (`.rpm`)
```bash
sudo rpm -i grubstation_*_amd64.rpm
```

---

## 🚀 Option B: Pre-built Binaries

If you prefer a standalone binary, download the archive for your architecture:

1. Download and extract the archive:
   ```bash
   tar -xzf grubstation_*_Linux_x86_64.tar.gz
   ```
2. Move the binary to your system PATH:
   ```bash
   sudo mv grubstation /usr/local/bin/
   sudo chmod +x /usr/local/bin/grubstation
   ```

---

## 🛠️ Option C: Building from Source

To build `grubstation` yourself, you'll need [Go](https://go.dev/dl/) 1.21 or higher installed.

1. Clone the repository:
   ```bash
   git clone https://github.com/jjack/grubstation.git
   cd grubstation
   ```
2. Build the binary:
   ```bash
   go build -o grubstation ./cmd/grubstation
   ```
3. Install:
   ```bash
   sudo mv grubstation /usr/local/bin/
   ```

### Custom Versioning
You can inject a version string at build time using Go's linker flags:
```bash
go build -ldflags="-X github.com/jjack/grubstation/internal/version.Version=1.0.0" ./cmd/grubstation
```

---

## ✅ Verification

After installation, verify that the `grubstation` command is available and working:

```bash
grubstation --version
```

You should see the version information displayed. If you get a "command not found" error, ensure that `/usr/local/bin` is in your system's `PATH`.

## Next Steps

After verifying the installation, proceed to the [Configuration Guide](/docs/configuration/setup.md) to set up your integration with Home Assistant.
