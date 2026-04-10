# Development Scripts Usage Guide

Complete guide for using the CLIProxyAPIPlus development utility scripts.

## Quick Start

### Interactive Menu (Recommended)

Simply run the script without any arguments to launch an interactive menu:

**Windows:**
```cmd
dev.bat
```

**Linux/macOS:**
```bash
./dev.sh
```

This will display a color-coded menu with all available options. Just enter the number of your choice!

### Command Line Usage

You can also run commands directly:

**Windows:**
```cmd
dev.bat help
```

**Linux/macOS:**
```bash
chmod +x dev.sh
./dev.sh help
```

---

## Installation

No installation required. The scripts are standalone and use the tools already installed on your system.

**Prerequisites:**
- Go 1.21 or higher
- Git
- CLI tools for authentication (kiro, codex, gemini-cli, antigravity)

---

## Command Reference

### Development Commands

#### `install` / `i`
Install Go dependencies.

```bash
# Windows
dev.bat install

# Linux/macOS
./dev.sh install
```

**What it does:**
- Runs `go mod download`
- Downloads all required Go modules
- Verifies dependency integrity

---

#### `dev` / `d`
Start the development server with hot reload.

```bash
# Windows
dev.bat dev

# Linux/macOS
./dev.sh dev
```

**What it does:**
- Runs `go run cmd/server/main.go`
- Starts the API server in development mode
- Watches for file changes (if configured)

**Default port:** 8080 (configurable in config.yaml)

---

#### `build` / `b`
Build the production binary.

```bash
# Windows
dev.bat build

# Linux/macOS
./dev.sh build
```

**Output:**
- Windows: `bin/cliproxyapi.exe`
- Linux/macOS: `bin/cliproxyapi`

**What it does:**
- Compiles optimized production binary
- Strips debug symbols
- Creates standalone executable
- Source: `cmd/server/main.go`

---

#### `test` / `t`
Run all tests.

```bash
# Windows
dev.bat test

# Linux/macOS
./dev.sh test
```

**What it does:**
- Runs `go test ./...`
- Executes all unit tests
- Reports coverage (if configured)

---

#### `lint` / `l`
Run code linter.

```bash
# Windows
dev.bat lint

# Linux/macOS
./dev.sh lint
```

**Requirements:**
- golangci-lint must be installed
- Install from: https://golangci-lint.run/usage/install/

**What it does:**
- Checks code quality
- Identifies potential bugs
- Enforces coding standards

---

#### `format` / `fmt`
Format all Go code.

```bash
# Windows
dev.bat format

# Linux/macOS
./dev.sh format
```

**What it does:**
- Runs `go fmt ./...`
- Applies standard Go formatting
- Ensures consistent code style

---

#### `clean` / `c`
Clean build artifacts.

```bash
# Windows
dev.bat clean

# Linux/macOS
./dev.sh clean
```

**What it does:**
- Removes `bin/` directory
- Removes `dist/` directory
- Cleans PID files
- Resets build state

---

### Backend Management

#### `start`
Start the backend server in the background.

```bash
# Windows
dev.bat start

# Linux/macOS
./dev.sh start
```

**What it does:**
- Starts server as background process
- Creates PID file for tracking
- Logs output to `backend.log` (Linux/macOS)

---

#### `stop`
Stop the running backend server.

```bash
# Windows
dev.bat stop

# Linux/macOS
./dev.sh stop
```

**What it does:**
- Gracefully stops the server
- Removes PID file
- Cleans up resources

---

#### `restart`
Restart the backend server.

```bash
# Windows
dev.bat restart

# Linux/macOS
./dev.sh restart
```

**What it does:**
- Stops the current server
- Waits 1 second
- Starts a new instance

---

#### `status`
Check if the backend is running.

```bash
# Windows
dev.bat status

# Linux/macOS
./dev.sh status
```

**Output:**
- ✓ Backend is running (PID: 12345)
- ✗ Backend is not running

---

### Authentication Commands

#### `login`
Interactive login menu.

```bash
# Windows
dev.bat login

# Linux/macOS
./dev.sh login
```

**Menu options:**
1. Kiro (AWS CodeWhisperer)
2. Codex CLI
3. Gemini CLI
4. Antigravity
5. Exit

---

#### `login-kiro`
Login to Kiro (AWS CodeWhisperer).

```bash
# Windows
dev.bat login-kiro

# Linux/macOS
./dev.sh login-kiro
```

**Authentication methods:**
1. **Builder ID** (Recommended)
   - Free tier available
   - No AWS account required
   - Best for individual developers

2. **Google Account**
   - OAuth-based authentication
   - Quick setup

3. **GitHub Account**
   - OAuth-based authentication
   - Integrates with GitHub profile

**Credential location:**
- Windows: `%USERPROFILE%\.kiro\credentials`
- Linux/macOS: `~/.kiro/credentials`

---

#### `login-codex`
Login to Codex CLI.

```bash
# Windows
dev.bat login-codex

# Linux/macOS
./dev.sh login-codex
```

**What it does:**
- Opens browser for OAuth
- Authenticates with Codex service
- Saves credentials locally

**Credential location:**
- Windows: `%USERPROFILE%\.codex\credentials.json`
- Linux/macOS: `~/.codex/credentials.json`

---

#### `login-gemini`
Login to Gemini CLI with enhanced error handling.

```bash
# Windows
dev.bat login-gemini

# Linux/macOS
./dev.sh login-gemini
```

**⚠️ IMPORTANT: Known Issue**

When prompted to select projects:
- ✅ **DO:** Select a single, specific project ID
- ❌ **DON'T:** Select "ALL" (may cause silent failure)

**Recommended approach:**
1. Choose a project you actively use
2. Avoid test/temporary projects (e.g., `csp-cli-*`)
3. Ensure the Generative Language API is enabled
4. Verify the project is active and accessible

**What the script does:**
- Displays warning about the "ALL" bug
- Guides you through safe authentication
- Verifies credential file creation
- Provides troubleshooting tips on failure

**Credential location:**
- Windows: `%USERPROFILE%\.gemini\credentials.json`
- Linux/macOS: `~/.gemini/credentials.json`

**See also:** [docs/TROUBLESHOOTING.md](TROUBLESHOOTING.md) for detailed bug analysis

---

#### `login-antigravity`
Login to Antigravity.

```bash
# Windows
dev.bat login-antigravity

# Linux/macOS
./dev.sh login-antigravity
```

**Credential location:**
- Windows: `%USERPROFILE%\.antigravity\credentials`
- Linux/macOS: `~/.antigravity/credentials`

---

### Utility Commands

#### `quota` / `q`
Check quotas for all providers.

```bash
# Windows
dev.bat quota

# Linux/macOS
./dev.sh quota
```

**What it does:**
- Queries Kiro quota
- Queries Codex quota
- Queries Gemini quota
- Queries Antigravity quota
- Displays usage limits and remaining capacity

**Example output:**
```
=== Kiro (AWS CodeWhisperer) ===
Monthly quota: 500 requests
Used: 127 requests
Remaining: 373 requests

=== Codex CLI ===
Not logged in or command not available

=== Gemini CLI ===
Daily quota: 1000 requests
Used: 45 requests
Remaining: 955 requests
```

---

#### `verify-creds`
Verify all credential files exist.

```bash
# Windows
dev.bat verify-creds

# Linux/macOS
./dev.sh verify-creds
```

**What it does:**
- Checks for Kiro credentials
- Checks for Codex credentials
- Checks for Gemini credentials
- Checks for Antigravity credentials
- Reports which providers are configured

**Example output:**
```
Checking Kiro credentials...
✓ Kiro credentials found

Checking Codex credentials...
✗ Codex credentials not found

Checking Gemini credentials...
✓ Gemini credentials found

Checking Antigravity credentials...
✗ Antigravity credentials not found
```

---

## Common Workflows

### Initial Setup

```bash
# 1. Install dependencies
dev.bat install

# 2. Verify credentials
dev.bat verify-creds

# 3. Login to required providers
dev.bat login

# 4. Check quotas
dev.bat quota

# 5. Start development
dev.bat dev
```

---

### Daily Development

```bash
# Start backend
dev.bat start

# Check status
dev.bat status

# Run tests
dev.bat test

# Format code
dev.bat format

# Stop backend
dev.bat stop
```

---

### Production Build

```bash
# Clean previous builds
dev.bat clean

# Run tests
dev.bat test

# Run linter
dev.bat lint

# Build production binary
dev.bat build

# Binary is now in bin/cliproxyapi.exe
```

---

### Troubleshooting Authentication

```bash
# 1. Verify which credentials exist
dev.bat verify-creds

# 2. Check current quotas
dev.bat quota

# 3. Re-authenticate if needed
dev.bat login-gemini

# 4. Verify credential file was created
dev.bat verify-creds
```

---

## Gemini CLI "ALL" Bug Workaround

### The Problem

When logging into Gemini CLI and selecting "ALL" to onboard all projects:
- Authentication succeeds
- Project list is displayed
- Script hangs or exits silently
- No credential file is created
- No error messages shown

### The Solution

**Always select a single project instead of "ALL".**

### Step-by-Step Workaround

1. **Run the login command:**
   ```bash
   dev.bat login-gemini
   ```

2. **Read the warning message carefully**

3. **When prompted for project selection:**
   ```
   Enter project ID [...] or ALL:
   ```

4. **Choose ONE project from the list:**
   - Look for a project you actively use
   - Avoid projects with names like `csp-cli-*`
   - Choose a project where you know the API is enabled

5. **Enter the project ID:**
   ```
   your-active-project-id
   ```

6. **Verify success:**
   - Script will check for credential file
   - Green checkmark indicates success
   - Red X indicates failure (try again with different project)

### If You Still Have Issues

See [docs/TROUBLESHOOTING.md](TROUBLESHOOTING.md) for:
- Detailed root cause analysis
- Alternative workarounds
- Manual credential configuration
- API enablement instructions
- How to report the bug

---

## Advanced Usage

### Custom Backend Command

Edit the script to change the backend command:

**Windows (dev.bat):**
```batch
set "BACKEND_CMD=go run cmd/server/main.go --port 9000"
```

**Linux/macOS (dev.sh):**
```bash
BACKEND_CMD="go run cmd/server/main.go --port 9000"
```

---

### Environment Variables

Set environment variables before running commands:

**Windows:**
```cmd
set PORT=9000
dev.bat dev
```

**Linux/macOS:**
```bash
PORT=9000 ./dev.sh dev
```

---

### Chaining Commands

**Windows:**
```cmd
dev.bat clean && dev.bat build && dev.bat start
```

**Linux/macOS:**
```bash
./dev.sh clean && ./dev.sh build && ./dev.sh start
```

---

## Troubleshooting

### Script Won't Run (Linux/macOS)

**Problem:** `Permission denied`

**Solution:**
```bash
chmod +x dev.sh
```

---

### Backend Won't Start

**Problem:** Port already in use

**Solution:**
1. Check what's using the port:
   ```bash
   # Windows
   netstat -ano | findstr :8080

   # Linux/macOS
   lsof -i :8080
   ```

2. Stop the conflicting process or change the port in config.yaml

---

### Credential File Not Found

**Problem:** Login succeeds but credentials not saved

**Solution:**
1. Check the credential directory exists:
   ```bash
   # Windows
   dir %USERPROFILE%\.gemini

   # Linux/macOS
   ls -la ~/.gemini/
   ```

2. If directory doesn't exist, create it:
   ```bash
   # Windows
   mkdir %USERPROFILE%\.gemini

   # Linux/macOS
   mkdir -p ~/.gemini/
   ```

3. Try logging in again with a single project (not "ALL")

---

### Quota Command Shows Nothing

**Problem:** `quota` command returns no data

**Possible causes:**
1. Not logged in to the provider
2. Provider CLI tool not installed
3. Provider doesn't support quota command

**Solution:**
1. Verify you're logged in: `dev.bat verify-creds`
2. Check CLI tool is installed: `kiro --version`
3. Try logging in again: `dev.bat login-kiro`

---

## Getting Help

### Documentation
- [README.md](../README.md) - Project overview
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Detailed troubleshooting
- [SDK Documentation](sdk-usage.md) - SDK integration guide

### Support
- GitHub Issues: Report bugs and request features
- Project maintainers: Contact for urgent issues

---

## Script Maintenance

### Updating the Scripts

The scripts are version-controlled. To update:

```bash
git pull origin main
```

### Customizing for Your Team

1. Fork the repository
2. Modify `dev.bat` and `dev.sh` as needed
3. Update this documentation
4. Commit your changes
5. Share with your team

---

## Version History

### v1.1.0 (2025-12-31)
- Added Gemini CLI "ALL" bug warning and validation
- Enhanced credential verification
- Added `verify-creds` command
- Improved error messages
- Added comprehensive troubleshooting guide

### v1.0.0 (Initial Release)
- Basic development commands
- Backend management
- Authentication support
- Quota checking
- Cross-platform support (Windows, Linux, macOS)
