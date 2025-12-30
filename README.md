# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This is the Plus version of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; CLIProxyAPI does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

The Plus release stays in lockstep with the mainline features.

## Differences from the Mainline

- Added GitHub Copilot support (OAuth login), provided by [em4go](https://github.com/em4go/CLIProxyAPI/tree/feature/github-copilot-auth)
- Added Kiro (AWS CodeWhisperer) support (OAuth login), provided by [fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration), [Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)

## Development

### Quick Start

**Interactive Menu (Recommended):**

Simply run the script without arguments to get an interactive menu:

```cmd
dev.bat
```

**Or use commands directly:**

```bash
dev.bat help    # Show all commands
dev.bat dev     # Start development server
dev.bat build   # Build production binary
```

### Documentation

- **[Development Scripts Guide](docs/DEV_SCRIPTS.md)** - Complete guide for using dev.bat/dev.sh
- **[Troubleshooting Guide](docs/TROUBLESHOOTING.md)** - Solutions for common issues, including the Gemini CLI "ALL" bug (FIXED)
- **[Docker Branch Build Guide](docs/DOCKER_BRANCH_BUILD.md)** - Build and use Docker images from development branches
- **[SDK Documentation](docs/sdk-usage.md)** - SDK integration guide

### Common Commands

```bash
# Install dependencies
dev.bat install

# Start development server
dev.bat dev

# Build production binary
dev.bat build

# Run tests
dev.bat test

# Login to providers
dev.bat login

# Check quotas
dev.bat quota
```

## Docker

### Using Pre-built Images

**Production (Stable Releases):**
```bash
# Latest stable release
docker pull eceasy/cli-proxy-api-plus:latest

# Specific version
docker pull eceasy/cli-proxy-api-plus:v1.2.3
```

**Development Branches:**
```bash
# Latest from CLIProxyAPIPlus-gdtiti branch (includes Gemini CLI fix)
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti

# Run the development image
docker run -d \
  --name cliproxyapi-dev \
  -p 8317:8317 \
  -v $(pwd)/config.yaml:/CLIProxyAPI/config.yaml \
  -v ~/.gemini:/root/.gemini \
  eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
```

See **[Docker Branch Build Guide](docs/DOCKER_BRANCH_BUILD.md)** for detailed instructions on using development branch images.

### Building Locally

```bash
docker build -t cliproxyapi-local .
docker run -p 8317:8317 cliproxyapi-local
```

## Recent Updates

### ✅ Gemini CLI "ALL" Selection Fix (CLIProxyAPIPlus-gdtiti branch)

Fixed critical issue where selecting "ALL" during Gemini CLI login would fail silently:
- ✅ Now skips problematic projects and continues processing
- ✅ Saves credentials for successfully activated projects
- ✅ Provides clear warning messages for failed projects
- ✅ Reports summary of successes and failures

**Try it now:**
```bash
# Using Docker
docker pull eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti
docker run -it eceasy/cli-proxy-api-plus:cliproxyapiplus-gdtiti sh
# Inside container: gemini-cli login

# Or build from source
git checkout CLIProxyAPIPlus-gdtiti
go run cmd/server/main.go
```

See [docs/GEMINI_ALL_FIX.md](docs/GEMINI_ALL_FIX.md) for technical details.

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the mainline repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
