# Advanced Installation Methods

You can install `grub-os-reporter` using the pre-built packages, binaries, or from source. The pre-built packages are recommended.

## Option A: Pre-built Packages (Recommended)

Download the appropriate package for your OS from the Releases Page.

For Debian/Ubuntu:
```bash
sudo dpkg -i grub-os-reporter_*_amd64.deb
```

## Option B: Pre-built Binaries

Download the binary archive for your architecture from the Releases Page.

```bash
tar -xzf grub-os-reporter_*_Linux_x86_64.tar.gz
sudo mv grub-os-reporter /usr/local/bin/
```

## Option C: From Source

Ensure you have Go installed on your system.

```bash
git clone https://github.com/jjack/grub-os-reporter.git
cd grub-os-reporter
go build -o grub-os-reporter .
sudo mv grub-os-reporter /usr/local/bin/
```

### Build-time Versioning
When building from source, the version defaults to `dev`. You can inject a specific version string during the build process using Go linker flags:

```bash
go build -ldflags="-X github.com/jjack/grub-os-reporter/internal/version.Version=1.0.0" -o grub-os-reporter .
```

This version will be reported in the Home Assistant webhook payload and visible in the daemon's logs.

## Next Steps

Once installed, run the automated setup wizard to configure the agent and install the necessary system hooks:
`sudo grub-os-reporter setup`
