# UPKGT - Universal Package Tool

[![Go Report Card](https://goreportcard.com/badge/github.com/NurOS-Linux/upkgt)](https://goreportcard.com/report/github.com/NurOS-Linux/upkgt)
[![License](https://img.shields.io/github/license/NurOS-Linux/upkgt)](https://github.com/NurOS-Linux/upkgt/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/NurOS-Linux/upkgt)](https://github.com/NurOS-Linux/upkgt/releases/latest)
[![Build Status](https://github.com/NurOS-Linux/upkgt/workflows/Build/badge.svg)](https://github.com/NurOS-Linux/upkgt/actions)

UPKGT is a universal package management tool that provides a unified interface for working with different package formats including:

- DEB (Debian, Ubuntu)
- RPM (Red Hat, Fedora, SUSE)
- EOPKG (Solus)
- Pacman (Arch Linux)
- APK (Alpine Linux)

## Features

- 📦 Support for multiple package formats
- 🔒 Secure package validation and verification
- 🔄 Automatic dependency resolution
- 💾 Backup creation before operations
- 📝 Detailed logging
- 🚀 Easy installation and removal
- 🔍 Package information querying
- 🛡️ Root privilege management

## Installation

### From Binary Releases

Download the latest release for your platform from the [releases page](https://github.com/NurOS-Linux/upkgt/releases).

### From Source

Requires Go 1.21 or higher.

```bash
git clone https://github.com/NurOS-Linux/upkgt.git
cd upkgt
./build.sh build
sudo mv bin/upkgt /usr/local/bin/
```

## Usage

### Basic Commands

```bash
# Install a package
sudo upkgt install package.deb
sudo upkgt install package.rpm
sudo upkgt install package.eopkg
sudo upkgt install package.pkg.tar.xz
sudo upkgt install package.apk

# Remove a package
sudo upkgt remove packagename
sudo upkgt remove --purge packagename

# Get package info
upkgt info package.deb
```

### Advanced Usage

```bash
# Install with force option
sudo upkgt install --force package.deb

# Create backup before operation
sudo upkgt install --backup package.rpm

# Show package contents
upkgt list package.eopkg

# Verify package signature
upkgt verify package.pkg.tar.xz

# Show package dependencies
upkgt deps package.apk
```

## Building

The project includes a build script that supports various operations:

```bash
# Clean build artifacts
./build.sh clean

# Run tests
./build.sh test

# Build for current platform
./build.sh build

# Build for all platforms
./build.sh build-all

# Create distribution packages
./build.sh packages

# Do everything
./build.sh all
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Requirements

- Go 1.21 or higher
- For building packages:
  - dpkg (for DEB packages)
  - rpm (for RPM packages)
  - eopkg (for EOPKG packages)
  - pacman (for Pacman packages)
  - apk (for APK packages)

## Project Structure

```
.
├── cmd/            # Command line interface
├── internal/       # Internal packages
│   ├── apk.go     # APK package implementation
│   ├── deb.go     # DEB package implementation
│   ├── eopkg.go   # EOPKG package implementation
│   ├── pacman.go  # Pacman package implementation
│   ├── rpm.go     # RPM package implementation
│   ├── package.go # Common package interface
│   └── utils.go   # Utility functions
├── build.sh       # Build script
├── go.mod         # Go modules file
├── go.sum         # Go modules checksum
└── README.md      # This file
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

Created and maintained by [NurOS-Linux](https://github.com/NurOS-Linux).

## Acknowledgments

- Thanks to all package manager developers for their documentation
- Special thanks to the Go community for great tools and libraries

## Version History

### 1.0.0 (2025-02-01)
- Initial release
- Support for DEB, RPM, EOPKG, Pacman, and APK packages
- Basic package management operations
- Backup functionality
- Logging system