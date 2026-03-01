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

### CLI Login

> **Note:** Google/GitHub login is not available for third-party applications due to AWS Cognito restrictions.

**AWS Builder ID** (recommended):

```bash
# Device code flow
./CLIProxyAPI --kiro-aws-login

# Authorization code flow
./CLIProxyAPI --kiro-aws-authcode
```

**Import token from Kiro IDE:**

```bash
./CLIProxyAPI --kiro-import
```

To get a token from Kiro IDE:

1. Open Kiro IDE and login with Google (or GitHub)
2. Find the token file: `~/.kiro/kiro-auth-token.json`
3. Run: `./CLIProxyAPI --kiro-import`

**AWS IAM Identity Center (IDC):**

```bash
./CLIProxyAPI --kiro-idc-login --kiro-idc-start-url https://d-xxxxxxxxxx.awsapps.com/start

# Specify region
./CLIProxyAPI --kiro-idc-login --kiro-idc-start-url https://d-xxxxxxxxxx.awsapps.com/start --kiro-idc-region us-west-2
```

**Additional flags:**

| Flag | Description |
|------|-------------|
| `--no-browser` | Don't open browser automatically, print URL instead |
| `--no-incognito` | Use existing browser session (Kiro defaults to incognito). Useful for corporate SSO that requires an authenticated browser session |
| `--kiro-idc-start-url` | IDC Start URL (required with `--kiro-idc-login`) |
| `--kiro-idc-region` | IDC region (default: `us-east-1`) |
| `--kiro-idc-flow` | IDC flow type: `authcode` (default) or `device` |

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

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Who is with us?

Those projects are based on CLIProxyAPI:

### [vibeproxy](https://github.com/automazeio/vibeproxy)

Native macOS menu bar app to use your Claude Code & ChatGPT subscriptions with AI coding tools - no API keys needed

### [Subtitle Translator](https://github.com/VjayC/SRT-Subtitle-Translator-Validator)

Browser-based tool to translate SRT subtitles using your Gemini subscription via CLIProxyAPI with automatic validation/error correction - no API keys needed

### [CCS (Claude Code Switch)](https://github.com/kaitranntt/ccs)

CLI wrapper for instant switching between multiple Claude accounts and alternative models (Gemini, Codex, Antigravity) via CLIProxyAPI OAuth - no API keys needed

### [ProxyPal](https://github.com/heyhuynhgiabuu/proxypal)

Native macOS GUI for managing CLIProxyAPI: configure providers, model mappings, and endpoints via OAuth - no API keys needed.

### [Quotio](https://github.com/nguyenphutrong/quotio)

Native macOS menu bar app that unifies Claude, Gemini, OpenAI, Qwen, and Antigravity subscriptions with real-time quota tracking and smart auto-failover for AI coding tools like Claude Code, OpenCode, and Droid - no API keys needed.

### [CodMate](https://github.com/loocor/CodMate)

Native macOS SwiftUI app for managing CLI AI sessions (Codex, Claude Code, Gemini CLI) with unified provider management, Git review, project organization, global search, and terminal integration. Integrates CLIProxyAPI to provide OAuth authentication for Codex, Claude, Gemini, Antigravity, and Qwen Code, with built-in and third-party provider rerouting through a single proxy endpoint - no API keys needed for OAuth providers.

### [ProxyPilot](https://github.com/Finesssee/ProxyPilot)

Windows-native CLIProxyAPI fork with TUI, system tray, and multi-provider OAuth for AI coding tools - no API keys needed.

### [Claude Proxy VSCode](https://github.com/uzhao/claude-proxy-vscode)

VSCode extension for quick switching between Claude Code models, featuring integrated CLIProxyAPI as its backend with automatic background lifecycle management.

### [ZeroLimit](https://github.com/0xtbug/zero-limit)

Windows desktop app built with Tauri + React for monitoring AI coding assistant quotas via CLIProxyAPI. Track usage across Gemini, Claude, OpenAI Codex, and Antigravity accounts with real-time dashboard, system tray integration, and one-click proxy control - no API keys needed.

### [CPA-XXX Panel](https://github.com/ferretgeek/CPA-X)

A lightweight web admin panel for CLIProxyAPI with health checks, resource monitoring, real-time logs, auto-update, request statistics and pricing display. Supports one-click installation and systemd service.

### [CLIProxyAPI Tray](https://github.com/kitephp/CLIProxyAPI_Tray)

A Windows tray application implemented using PowerShell scripts, without relying on any third-party libraries. The main features include: automatic creation of shortcuts, silent running, password management, channel switching (Main / Plus), and automatic downloading and updating.

> [!NOTE]  
> If you developed a project based on CLIProxyAPI, please open a PR to add it to this list.

## More choices

Those projects are ports of CLIProxyAPI or inspired by it:

### [9Router](https://github.com/decolua/9router)

A Next.js implementation inspired by CLIProxyAPI, easy to install and use, built from scratch with format translation (OpenAI/Claude/Gemini/Ollama), combo system with auto-fallback, multi-account management with exponential backoff, a Next.js web dashboard, and support for CLI tools (Cursor, Claude Code, Cline, RooCode) - no API keys needed.

> [!NOTE]  
> If you have developed a port of CLIProxyAPI or a project inspired by it, please open a PR to add it to this list.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
