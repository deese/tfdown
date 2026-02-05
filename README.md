# tfdown

**tfdown** is a cross-platform tool written in Go that automatically downloads the latest version of Terraform available for your operating system and architecture.

## Features

- ✨ **Automatic download** of the latest stable Terraform version
- 🎯 **Cross-platform**: Windows, Linux, macOS, OpenBSD, Solaris
- 🏗️ **Multiple architectures**: amd64, 386, arm64, arm
- ⚙️ **Optional automatic installation**
- 💾 **Configuration management** to maintain automatic updates
- 🔄 **Smart updates** - only downloads if there's a new version
- 🚀 **Force download** with `-f` flag to re-download even if up to date

## Installation

### From Releases

Download the pre-compiled binary for your platform from [GitHub Releases](https://github.com/yourusername/tfdown/releases):

```bash
# Linux/macOS
wget https://github.com/yourusername/tfdown/releases/latest/download/tfdown-linux-amd64.tar.gz
tar -xzf tfdown-linux-amd64.tar.gz
sudo mv tfdown-linux-amd64 /usr/local/bin/tfdown

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/yourusername/tfdown/releases/latest/download/tfdown-windows-amd64.zip" -OutFile "tfdown.zip"
Expand-Archive tfdown.zip
Move-Item tfdown\tfdown-windows-amd64.exe C:\Windows\System32\tfdown.exe
```

### From Source

```bash
git clone https://github.com/yourusername/tfdown.git
cd tfdown
make build
# Or on Windows: go build -o tfdown.exe .
```

## Usage

### Basic Examples

```bash
# Download the latest Terraform version for your current platform
tfdown

# Download and configure automatic installation
tfdown --install --install-path /usr/local/bin

# Force re-download even if already up to date
tfdown -f

# Download a specific version
tfdown --ver 1.7.0

# Download for a different platform
tfdown --os linux --arch arm64

# Download specific version for Windows
tfdown --ver 1.6.5 --os windows --arch amd64
```

### Available Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `--os` | Target operating system (linux, windows, darwin, openbsd, solaris) | Current OS |
| `--arch` | Target architecture (amd64, 386, arm64, arm) | Current architecture |
| `--ver` | Terraform version to download (e.g., 1.7.0) | Latest stable version |
| `--install` | Enable automatic installation | false |
| `--install-path` | Path to install Terraform | - |
| `-f, --force` | Force download and install even if already up to date | false |
| `-q, --quiet` | Quiet mode, disable progress bar | false |
| `--version` | Show tfdown version | - |
| `--help` | Show help message | - |

## Automatic Configuration

tfdown saves the configuration in `~/.tfdown.conf` with the following format:

```ini
# tfdown configuration file
version=1.7.0
install=true
install_path=/usr/local/bin
```

When you run `tfdown` without arguments:
- If `install=true` and `install_path` exists, it will automatically download and install new versions
- If you already have the latest version, it won't do anything
- Updates the configuration file with each download

Use the `-f` or `--force` flag to bypass the version check and force a re-download and installation.

## Typical Workflow

### First Time

```bash
# Configure automatic installation
tfdown --install --install-path /usr/local/bin
```

This will:
1. Download the latest Terraform version
2. Extract and install it to `/usr/local/bin`
3. Save the configuration

### Future Updates

```bash
# Simply run without arguments
tfdown
```

This will:
1. Check if there's a new version
2. If it exists, download and install it automatically
3. If you're already up to date, it won't do anything

## Development

### Requirements

- Go 1.21 or higher
- Make (optional, to use the Makefile)

### Build

```bash
# For your current platform
make build

# For all platforms
make build-all

# Linux only
make build-linux

# Windows only
make build-windows

# macOS only
make build-darwin
```

### Project Structure

```
tfdown/
├── src/
│   ├── main.go          # Main application and CLI handling
│   ├── config.go        # Configuration management
│   ├── downloader.go    # Terraform download logic
│   └── go.mod           # Go dependencies
├── Makefile             # Build commands
├── .github/
│   └── workflows/
│       └── release.yml  # GitHub Actions for automatic releases
└── README.md            # This documentation
```

## Creating a Release

1. Create a tag:
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

2. Create a release on GitHub
3. GitHub Actions will automatically build for all platforms
4. Binaries will be uploaded to the release

## License

MIT

## Contributing

Contributions are welcome! Please open an issue or pull request.

## Author

Your name
