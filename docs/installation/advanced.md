# Advanced Installation Methods

You can install `grubstation` using the pre-built packages, binaries, or from source. The pre-built packages are recommended.

## Option A: Pre-built Packages (Recommended)

Download the appropriate package for your OS from the Releases Page.

For Debian/Ubuntu:
```bash
sudo dpkg -i grubstation_*_amd64.deb
```

## Option B: Pre-built Binaries

Download the binary archive for your architecture from the Releases Page.

```bash
tar -xzf grubstation_*_Linux_x86_64.tar.gz
sudo mv grubstation /usr/local/bin/
```

## Option C: From Source

Ensure you have Go installed on your system.

```bash
git clone https://github.com/jjack/grubstation-cli.git
cd grubstation
go build -o grubstation .
sudo mv grubstation /usr/local/bin/
```

### Build-time Versioning
When building from source, the version defaults to `dev`. You can inject a specific version string during the build process using Go linker flags:

```bash
go build -ldflags="-X github.com/jjack/grubstation-cli/internal/version.Version=1.0.0" -o grubstation .
```

This version will be reported in the Home Assistant webhook payload and visible in the daemon's logs.

## Next Steps

Once installed, run the automated setup wizard to configure the agent and install the necessary system hooks:
`sudo grubstation setup`
