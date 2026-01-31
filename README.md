# CLIProxyAPI Plus

English | [Chinese](README_CN.md)

This is the Plus version of [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI), adding support for third-party providers on top of the mainline project.

All third-party provider support is maintained by community contributors; CLIProxyAPI does not provide technical support. Please contact the corresponding community maintainer if you need assistance.

The Plus release stays in lockstep with the mainline features.

## Differences from the Mainline

- Added GitHub Copilot support (OAuth login), provided by [em4go](https://github.com/em4go/CLIProxyAPI/tree/feature/github-copilot-auth)
- Added Kiro (AWS CodeWhisperer) support (OAuth login), provided by [fuko2935](https://github.com/fuko2935/CLIProxyAPI/tree/feature/kiro-integration), [Ravens2121](https://github.com/Ravens2121/CLIProxyAPIPlus/)

## New Features (Plus Enhanced)

- **OAuth Web Authentication**: Browser-based OAuth login for Kiro with beautiful web UI
- **Rate Limiter**: Built-in request rate limiting to prevent API abuse
- **Background Token Refresh**: Automatic token refresh 10 minutes before expiration
- **Metrics & Monitoring**: Request metrics collection for monitoring and debugging
- **Device Fingerprint**: Device fingerprint generation for enhanced security
- **Cooldown Management**: Smart cooldown mechanism for API rate limits
- **Usage Checker**: Real-time usage monitoring and quota management
- **Model Converter**: Unified model name conversion across providers
- **UTF-8 Stream Processing**: Improved streaming response handling

## Kiro Authentication

### Web-based OAuth Login

Access the Kiro OAuth web interface at:

```
http://your-server:8080/v0/oauth/kiro
```

This provides a browser-based OAuth flow for Kiro (AWS CodeWhisperer) authentication with:
- AWS Builder ID login
- AWS Identity Center (IDC) login
- Token import from Kiro IDE

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

## Quick Deployment with Docker

### One-Command Deployment

```bash
# Create deployment directory
mkdir -p ~/cli-proxy && cd ~/cli-proxy

# Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
services:
  cli-proxy-api:
    image: eceasy/cli-proxy-api-plus:latest
    container_name: cli-proxy-api-plus
    ports:
      - "8317:8317"
    volumes:
      - ./config.yaml:/CLIProxyAPI/config.yaml
      - ./auths:/root/.cli-proxy-api
      - ./logs:/CLIProxyAPI/logs
    restart: unless-stopped
EOF

# Download example config
curl -o config.yaml https://raw.githubusercontent.com/router-for-me/CLIProxyAPIPlus/main/config.example.yaml

# Pull and start
docker compose pull && docker compose up -d
```

### Using Pre-built Images

**Production (Stable Releases):**
```bash
# Latest stable release
docker pull ghcr.io/your-org/cliproxyapiplus:latest

# Specific version
docker pull ghcr.io/your-org/cliproxyapiplus:v1.2.3
```

**Development Branches:**
```bash
# Latest from CLIProxyAPIPlus-gdtiti branch (includes Gemini CLI fix)
docker pull ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti

# Run the development image
docker run -d \
  --name cliproxyapi-dev \
  -p 8317:8317 \
  -v $(pwd)/config.yaml:/CLIProxyAPI/config.yaml \
  -v ~/.gemini:/root/.gemini \
  ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
```

See **[Docker Branch Build Guide](docs/DOCKER_BRANCH_BUILD.md)** for detailed instructions on using development branch images.

### Building Locally

```bash
docker build -t cliproxyapi-local .
docker run -p 8317:8317 cliproxyapi-local
```

### Configuration

Edit `config.yaml` before starting:

```yaml
# Basic configuration example
server:
  port: 8317

# Add your provider configurations here
```

### Update to Latest Version

```bash
cd ~/cli-proxy
docker compose pull && docker compose up -d
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
docker pull ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti
docker run -it ghcr.io/your-org/cliproxyapiplus:cliproxyapiplus-gdtiti sh
# Inside container: gemini-cli login

# Or build from source
git checkout CLIProxyAPIPlus-gdtiti
go run cmd/server/main.go
```

See [docs/GEMINI_ALL_FIX.md](docs/GEMINI_ALL_FIX.md) for technical details.

## Contributing

This project only accepts pull requests that relate to third-party provider support. Any pull requests unrelated to third-party provider support will be rejected.

If you need to submit any non-third-party provider changes, please open them against the [mainline](https://github.com/router-for-me/CLIProxyAPI) repository.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
