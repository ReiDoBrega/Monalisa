# MonaLisa

> **MonaLisa Content Decryption Module for Go**

A Go library and CLI tool for decrypting IQIYI DRM License Tickets using WebAssembly-based Content Decryption Modules (CDM).

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## 📋 Overview

MonaLisa provides a secure framework for creating, managing, and using self-contained `.mld` (MonaLisa Device) files that encapsulate WebAssembly modules for DRM license processing. It supports time-based licensing with anti-tampering protection and HMAC-based signature verification.

### Key Features

- 🔐 **Secure License Processing**: Process DRM licenses and extract decryption keys
- 📦 **Self-Contained Devices**: Bundle WASM modules with metadata into portable `.mld` files
- ⏰ **Time-Based Licensing**: Support for expiring and permanent device licenses
- 🛡️ **Anti-Tampering Protection**: Advanced clock tampering detection
- 🗜️ **Compression Support**: Optional zlib compression for all components
- 📝 **HMAC Signatures**: Cryptographic verification of device integrity
- 🔍 **Device Verification**: Validate and inspect device files

## 🚀 Installation

### Prerequisites

- Go 1.21 or higher
- [WABT toolkit](https://github.com/WebAssembly/wabt) (optional, for WAT compilation)

### Install from Source

```bash
git clone https://github.com/ReiDoBrega/Monalisa.git
cd Monalisa
go build -o monalisa .
```

### Install as Go Module

```bash
go get github.com/ReiDoBrega/Monalisa
```

## 🎯 Quick Start

### Creating a Device

Create a self-contained device file from a WASM binary:

```bash
# Create a device with 30-day license
monalisa create-device --wasm module.wasm -o device.mld --valid-days 30

# Create a permanent device (no expiration)
monalisa create-device --wasm module.wasm -o device.mld --valid-days 0

# Create from WAT source with custom metadata
monalisa create-device \
  --wat module.wat \
  --js wrapper.js \
  -o device.mld \
  --version "4.0.0" \
  --name "Custom Module" \
  --description "My custom DRM module" \
  --valid-days 90
```

### Processing Licenses

Extract decryption keys from a license:

```bash
# Basic usage
monalisa license --device device.mld "AIUACgMAAAAAAAAAAAQChgACATADhwAnAgAg..."

# Save to JSON file
monalisa license --device device.mld --json keys.json "LICENSE_DATA"

# Quiet mode (keys only)
monalisa license --device device.mld --quiet "LICENSE_DATA"
```

### Checking License Information

View device license status and expiration:

```bash
monalisa license-info device.mld
```

Output example:
```
==================================================
DEVICE LICENSE INFORMATION
==================================================

  Device ID: 7f3d2a1b9c8e4f5a6d7e8f9a0b1c2d3e
  Created: 2026-01-27 10:30:00
  Expires: 2026-02-26 10:30:00
  Max Duration: 30 days

  ✓ Status: VALID
  Days since creation: 0 day(s)
  Time remaining: 30 day(s) and 0 hour(s)
```

### Verifying Device Integrity

```bash
monalisa verify-device device.mld
```

## 📚 Library Usage

### Creating and Using CDM Sessions

```go
package main

import (
    "fmt"
    "monalisa/pkg/cdm"
    "monalisa/pkg/license"
    "monalisa/pkg/module"
    "monalisa/pkg/types"
)

func main() {
    // Load device file
    mod, err := module.Load("device.mld")
    if err != nil {
        panic(err)
    }

    // Initialize CDM
    cdmInstance := cdm.FromModule(mod)

    // Open session
    sessionID, err := cdmInstance.Open()
    if err != nil {
        panic(err)
    }
    defer cdmInstance.Close(sessionID)

    // Parse license
    lic := license.New("base64_license_data")
    if err := cdmInstance.ParseLicense(sessionID, lic); err != nil {
        panic(err)
    }

    // Extract keys
    keys, err := cdmInstance.GetKeys(sessionID, types.KeyTypeContent)
    if err != nil {
        panic(err)
    }

    for _, key := range keys {
        fmt.Printf("KID: %x\n", key.KID)
        fmt.Printf("Key: %x\n", key.Key)
    }
}
```

### Building Devices Programmatically

```go
package main

import (
    "monalisa/pkg/module"
)

func main() {
    builder := module.NewBuilder()
    
    // Configure metadata
    builder.WithMetadata("1.0.0", "My Module", "Description")
    
    // Load WASM
    builder.WithWASM("module.wasm")
    
    // Optional: Add JavaScript wrapper
    builder.WithJS("wrapper.js")
    
    // Enable compression
    builder.WithCompression(true)
    
    // Set 60-day license
    builder.WithLicense(60)
    
    // Build device
    if err := builder.Build("output.mld"); err != nil {
        panic(err)
    }
}
```

## 🏗️ Architecture

### Project Structure

```
monalisa/
├── cmd/                    # CLI commands
│   ├── create_device.go   # Device creation
│   ├── license.go         # License processing
│   ├── license_info.go    # License inspection
│   ├── verify_device.go   # Device verification
│   └── root.go           # CLI root
├── pkg/
│   ├── cdm/              # Content Decryption Module
│   │   ├── cdm.go       # CDM manager
│   │   └── session.go   # WASM session handling
│   ├── license/          # License handling
│   │   └── license.go
│   ├── module/           # Module loader & builder
│   │   ├── module.go
│   │   └── signature.go  # HMAC signing
│   ├── types/            # Type definitions
│   │   └── types.go
│   └── exceptions/       # Error handling
│       └── exceptions.go
└── main.go               # Entry point
```

### Device File Format (.mld)

MonaLisa devices use a custom binary format:

```
[HEADER - 32 bytes]
  Magic:         "MLD\x00" (4 bytes)
  Version:       Major.Minor (2 bytes)
  Flags:         Compression/Signature flags (2 bytes)
  Metadata Size: (4 bytes)
  WASM Size:     (4 bytes)
  JS Size:       (4 bytes)
  Checksum:      CRC32 (4 bytes)
  Sig Size:      (4 bytes)
  Reserved:      (4 bytes)

[PAYLOAD]
  [Signature - 88 bytes, optional]
    Device ID:         32 bytes (hex)
    Creation Time:     8 bytes (obfuscated)
    Expiry Time:       8 bytes (obfuscated)
    Max Duration:      8 bytes (obfuscated)
    HMAC-SHA256:       32 bytes
  
  [Metadata - JSON, optionally compressed]
  [WASM Module - optionally compressed]
  [JavaScript - optionally compressed]
```

### Anti-Tampering Protection

The licensing system includes multiple layers of clock tampering detection:

1. **Creation Time Check**: Prevents using devices before creation date
2. **Expiry Time Check**: Enforces license expiration
3. **Maximum Duration Check**: Validates total elapsed time since creation
4. **HMAC Verification**: Ensures data integrity

## 🔧 Configuration

### Security Configuration

The signature system uses a master key for HMAC generation. **Important**: Change this in production!

```go
// In pkg/signature/signature.go
const masterKey = "your-secure-master-key-here"
const timeObfuscationKey = "your-secure-time-obfuscation-key-here"
```

### Compression Levels

Compression uses zlib with best compression (level 9) by default. This can significantly reduce file sizes:

- Metadata: ~60-80% reduction
- WASM: ~40-60% reduction
- JavaScript: ~50-70% reduction

## 📖 API Reference

### Module Functions

- `module.Load(path string)`: Load a device file
- `module.NewBuilder()`: Create a device builder

### CDM Functions

- `cdm.FromModule(mod *Module)`: Initialize CDM from module
- `cdm.Open()`: Open a new session
- `cdm.Close(sessionID)`: Close a session
- `cdm.ParseLicense(sessionID, license)`: Process a license
- `cdm.GetKeys(sessionID, keyType)`: Extract decryption keys

### License Functions

- `license.New(data string)`: Create license from base64 or raw data

## 🛠️ Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
# Build for current platform
go build -o monalisa .

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o monalisa-linux-amd64
GOOS=darwin GOARCH=arm64 go build -o monalisa-darwin-arm64
GOOS=windows GOARCH=amd64 go build -o monalisa-windows-amd64.exe
```

## ⚠️ Security Considerations

1. **Master Key**: Change the default master key in `signature.go` for production use
2. **License Duration**: Choose appropriate validity periods for your use case
3. **WASM Validation**: Ensure WASM modules are from trusted sources
4. **Key Storage**: Handle extracted keys securely in production environments

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Wasmtime-go](https://github.com/bytecodealliance/wasmtime-go) - WebAssembly runtime
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Google UUID](https://github.com/google/uuid) - UUID generation

## 📞 Support

For issues, questions, or contributions, please:

- Open an issue on [GitHub Issues](https://github.com/ReiDoBrega/Monalisa/issues)
- Visit the main repository: [github.com/ReiDoBrega/Monalisa](https://github.com/ReiDoBrega/Monalisa)

---

**Version**: 0.1.2  
**Author**: ReiDoBrega  
**Last Updated**: February 2026