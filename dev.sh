#!/bin/bash

# CLIProxyAPIPlus Development Utility Script
# Version: 1.1.0
# Enhanced with credential validation and error handling

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
RESET='\033[0m'

# Configuration
BACKEND_DIR="."
BACKEND_CMD="go run cmd/server/main.go"
PID_FILE=".backend.pid"

# Helper functions
print_error() {
    echo -e "${RED}$1${RESET}"
}

print_success() {
    echo -e "${GREEN}$1${RESET}"
}

print_warning() {
    echo -e "${YELLOW}$1${RESET}"
}

print_info() {
    echo -e "${BLUE}$1${RESET}"
}

print_header() {
    echo -e "${CYAN}$1${RESET}"
}

# Show help
show_help() {
    print_header "CLIProxyAPIPlus Development Utility"
    echo
    print_warning "Usage:"
    echo "  ./dev.sh [command]"
    echo
    print_warning "Development Commands:"
    echo "  install, i          Install dependencies"
    echo "  dev, d              Start development server"
    echo "  build, b            Build for production"
    echo "  test, t             Run tests"
    echo "  lint, l             Run linter"
    echo "  format, fmt         Format code"
    echo "  clean, c            Clean build artifacts"
    echo
    print_warning "Backend Management:"
    echo "  start               Start backend server"
    echo "  stop                Stop backend server"
    echo "  restart             Restart backend server"
    echo "  status              Check backend status"
    echo
    print_warning "Authentication:"
    echo "  login               Interactive login menu"
    echo "  login-kiro          Login to Kiro (AWS CodeWhisperer)"
    echo "  login-codex         Login to Codex CLI"
    echo "  login-gemini        Login to Gemini CLI"
    echo "  login-antigravity   Login to Antigravity"
    echo
    print_warning "Utilities:"
    echo "  quota, q            Check provider quotas"
    echo "  verify-creds        Verify all credentials"
    echo "  help, h             Show this help message"
}

# Show interactive menu
show_menu() {
    while true; do
        clear
        echo
        print_header "========================================"
        print_header "  CLIProxyAPIPlus Development Menu"
        print_header "========================================"
        echo
        print_warning "Development:"
        echo -e "  ${GREEN}1${RESET}. Install dependencies"
        echo -e "  ${GREEN}2${RESET}. Start development server"
        echo -e "  ${GREEN}3${RESET}. Build for production"
        echo -e "  ${GREEN}4${RESET}. Run tests"
        echo -e "  ${GREEN}5${RESET}. Run linter"
        echo -e "  ${GREEN}6${RESET}. Format code"
        echo -e "  ${GREEN}7${RESET}. Clean build artifacts"
        echo
        print_warning "Backend:"
        echo -e "  ${GREEN}8${RESET}. Start backend"
        echo -e "  ${GREEN}9${RESET}. Stop backend"
        echo -e "  ${GREEN}10${RESET}. Restart backend"
        echo -e "  ${GREEN}11${RESET}. Check backend status"
        echo
        print_warning "Authentication:"
        echo -e "  ${GREEN}12${RESET}. Login menu"
        echo -e "  ${GREEN}13${RESET}. Check quotas"
        echo -e "  ${GREEN}14${RESET}. Verify credentials"
        echo
        print_warning "Other:"
        echo -e "  ${GREEN}15${RESET}. Show help"
        echo -e "  ${GREEN}0${RESET}. Exit"
        echo
        echo -ne "${CYAN}Enter your choice (0-15):${RESET} "
        read choice

        case $choice in
            1) cmd_install; menu_pause ;;
            2) cmd_dev; menu_pause ;;
            3) cmd_build; menu_pause ;;
            4) cmd_test; menu_pause ;;
            5) cmd_lint; menu_pause ;;
            6) cmd_format; menu_pause ;;
            7) cmd_clean; menu_pause ;;
            8) cmd_start; menu_pause ;;
            9) cmd_stop; menu_pause ;;
            10) cmd_restart; menu_pause ;;
            11) cmd_status; menu_pause ;;
            12) cmd_login_menu; menu_pause ;;
            13) cmd_quota; menu_pause ;;
            14) cmd_verify_creds; menu_pause ;;
            15) show_help; menu_pause ;;
            0) exit 0 ;;
            *)
                echo
                print_error "Invalid choice. Please try again."
                sleep 2
                ;;
        esac
    done
}

# Pause and wait for user input
menu_pause() {
    echo
    print_info "Press Enter to return to menu or Ctrl+C to exit..."
    read
}

# Install dependencies
cmd_install() {
    print_info "Installing dependencies..."
    go mod download
    if [ $? -eq 0 ]; then
        print_success "Dependencies installed successfully"
    else
        print_error "Failed to install dependencies"
        exit 1
    fi
}

# Start development server
cmd_dev() {
    print_info "Starting development server..."
    go run cmd/server/main.go
}

# Build for production
cmd_build() {
    print_info "Building for production..."
    go build -o bin/cliproxyapi cmd/server/main.go
    if [ $? -eq 0 ]; then
        print_success "Build completed: bin/cliproxyapi"
    else
        print_error "Build failed"
        exit 1
    fi
}

# Run tests
cmd_test() {
    print_info "Running tests..."
    go test ./...
}

# Run linter
cmd_lint() {
    print_info "Running linter..."
    if ! command -v golangci-lint &> /dev/null; then
        print_warning "golangci-lint not found. Install it from: https://golangci-lint.run/usage/install/"
        exit 1
    fi
    golangci-lint run
}

# Format code
cmd_format() {
    print_info "Formatting code..."
    go fmt ./...
    print_success "Code formatted"
}

# Clean build artifacts
cmd_clean() {
    print_info "Cleaning build artifacts..."
    rm -rf bin dist "$PID_FILE"
    print_success "Clean completed"
}

# Start backend server
cmd_start() {
    print_info "Starting backend server..."
    if [ -f "$PID_FILE" ]; then
        print_warning "Backend is already running (PID file exists)"
        cmd_status
        return 0
    fi

    nohup $BACKEND_CMD > backend.log 2>&1 &
    echo $! > "$PID_FILE"
    sleep 2
    print_success "Backend started (PID: $(cat $PID_FILE))"
}

# Stop backend server
cmd_stop() {
    print_info "Stopping backend server..."
    if [ ! -f "$PID_FILE" ]; then
        print_warning "Backend is not running (no PID file)"
        return 0
    fi

    PID=$(cat "$PID_FILE")
    kill "$PID" 2>/dev/null
    rm -f "$PID_FILE"
    print_success "Backend stopped"
}

# Restart backend server
cmd_restart() {
    print_info "Restarting backend server..."
    cmd_stop
    sleep 1
    cmd_start
}

# Check backend status
cmd_status() {
    print_info "Checking backend status..."
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            print_success "Backend is running (PID: $PID)"
        else
            print_warning "Backend PID file exists but process is not running"
            rm -f "$PID_FILE"
        fi
    else
        print_warning "Backend is not running"
    fi
}

# Login menu
cmd_login_menu() {
    echo
    print_header "=== Authentication Login Menu ==="
    echo
    print_warning "Select a provider:"
    echo "  1. Kiro (AWS CodeWhisperer)"
    echo "  2. Codex CLI"
    echo "  3. Gemini CLI"
    echo "  4. Antigravity"
    echo "  5. Exit"
    echo
    read -p "Enter your choice (1-5): " choice

    case $choice in
        1) cmd_login_kiro ;;
        2) cmd_login_codex ;;
        3) cmd_login_gemini ;;
        4) cmd_login_antigravity ;;
        5) exit 0 ;;
        *) print_error "Invalid choice"; exit 1 ;;
    esac
}

# Login to Kiro
cmd_login_kiro() {
    echo
    print_header "=== Kiro (AWS CodeWhisperer) Login ==="
    echo
    print_warning "Select authentication method:"
    echo "  1. Builder ID (Recommended)"
    echo "  2. Google Account"
    echo "  3. GitHub Account"
    echo "  4. Cancel"
    echo
    read -p "Enter your choice (1-4): " method

    case $method in
        1)
            print_info "Logging in with Builder ID..."
            kiro login --method builder-id
            ;;
        2)
            print_info "Logging in with Google..."
            kiro login --method google
            ;;
        3)
            print_info "Logging in with GitHub..."
            kiro login --method github
            ;;
        4)
            return 0
            ;;
        *)
            print_error "Invalid choice"
            exit 1
            ;;
    esac

    if [ $? -eq 0 ]; then
        print_success "Kiro login successful"
    else
        print_error "Kiro login failed"
        exit 1
    fi
}

# Login to Codex
cmd_login_codex() {
    print_info "Logging in to Codex CLI..."
    codex login
    if [ $? -eq 0 ]; then
        print_success "Codex login successful"
    else
        print_error "Codex login failed"
        exit 1
    fi
}

# Login to Gemini
cmd_login_gemini() {
    echo
    print_header "=== Gemini CLI Login ==="
    echo
    print_warning "IMPORTANT: Known Issue with 'ALL' Selection"
    echo
    echo "When prompted to select projects:"
    echo -e "  - ${GREEN}DO:${RESET} Select a single, specific project ID"
    echo -e "  - ${RED}DON'T:${RESET} Select \"ALL\" (may cause silent failure)"
    echo
    print_warning "Recommended approach:"
    echo "  1. Choose a project you actively use"
    echo "  2. Avoid test/temporary projects (e.g., csp-cli-*)"
    echo "  3. Ensure the Generative Language API is enabled"
    echo
    echo "See docs/TROUBLESHOOTING.md for details"
    echo
    read -p "Press Enter to continue..."
    echo

    print_info "Logging in to Gemini CLI..."
    gemini-cli login

    if [ $? -eq 0 ]; then
        echo
        print_success "Gemini login successful"
        echo
        print_info "Verifying credential file..."
        verify_gemini_creds
    else
        print_error "Gemini login failed"
        echo
        print_warning "Troubleshooting tips:"
        echo "  1. Check if you selected a single project (not ALL)"
        echo "  2. Verify the project has required APIs enabled"
        echo "  3. See docs/TROUBLESHOOTING.md for more help"
        exit 1
    fi
}

# Verify Gemini credentials
verify_gemini_creds() {
    if [ -f "$HOME/.gemini/credentials.json" ]; then
        print_success "✓ Credential file found"
        echo "  Location: $HOME/.gemini/credentials.json"
    elif [ -f "$HOME/.gemini/config.json" ]; then
        print_success "✓ Config file found"
        echo "  Location: $HOME/.gemini/config.json"
    else
        print_error "✗ No credential file found"
        echo
        print_warning "This may indicate the 'ALL' bug occurred."
        echo "Please try logging in again with a single project."
        echo "See docs/TROUBLESHOOTING.md for details."
        exit 1
    fi
}

# Login to Antigravity
cmd_login_antigravity() {
    print_info "Logging in to Antigravity..."
    antigravity login
    if [ $? -eq 0 ]; then
        print_success "Antigravity login successful"
    else
        print_error "Antigravity login failed"
        exit 1
    fi
}

# Check quotas
cmd_quota() {
    print_info "Checking provider quotas..."
    echo

    print_header "=== Kiro (AWS CodeWhisperer) ==="
    kiro quota 2>/dev/null || print_warning "Not logged in or command not available"
    echo

    print_header "=== Codex CLI ==="
    codex quota 2>/dev/null || print_warning "Not logged in or command not available"
    echo

    print_header "=== Gemini CLI ==="
    gemini-cli quota 2>/dev/null || print_warning "Not logged in or command not available"
    echo

    print_header "=== Antigravity ==="
    antigravity quota 2>/dev/null || print_warning "Not logged in or command not available"
}

# Verify all credentials
cmd_verify_creds() {
    print_info "Verifying credentials..."
    echo

    print_header "Checking Kiro credentials..."
    if [ -f "$HOME/.kiro/credentials" ]; then
        print_success "✓ Kiro credentials found"
    else
        print_warning "✗ Kiro credentials not found"
    fi

    echo
    print_header "Checking Codex credentials..."
    if [ -f "$HOME/.codex/credentials.json" ]; then
        print_success "✓ Codex credentials found"
    else
        print_warning "✗ Codex credentials not found"
    fi

    echo
    print_header "Checking Gemini credentials..."
    if [ -f "$HOME/.gemini/credentials.json" ] || [ -f "$HOME/.gemini/config.json" ]; then
        print_success "✓ Gemini credentials found"
    else
        print_warning "✗ Gemini credentials not found"
    fi

    echo
    print_header "Checking Antigravity credentials..."
    if [ -f "$HOME/.antigravity/credentials" ]; then
        print_success "✓ Antigravity credentials found"
    else
        print_warning "✗ Antigravity credentials not found"
    fi
}

# Main command router
CMD="${1}"

# If no command provided, show interactive menu
if [ -z "$CMD" ]; then
    show_menu
    exit 0
fi

case $CMD in
    help|h)
        show_help
        ;;
    install|i)
        cmd_install
        ;;
    dev|d)
        cmd_dev
        ;;
    build|b)
        cmd_build
        ;;
    test|t)
        cmd_test
        ;;
    lint|l)
        cmd_lint
        ;;
    format|fmt)
        cmd_format
        ;;
    clean|c)
        cmd_clean
        ;;
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    status)
        cmd_status
        ;;
    login)
        cmd_login_menu
        ;;
    login-kiro)
        cmd_login_kiro
        ;;
    login-codex)
        cmd_login_codex
        ;;
    login-gemini)
        cmd_login_gemini
        ;;
    login-antigravity)
        cmd_login_antigravity
        ;;
    quota|q)
        cmd_quota
        ;;
    verify-creds)
        cmd_verify_creds
        ;;
    *)
        print_error "Unknown command: $CMD"
        echo "Run './dev.sh help' for usage information"
        exit 1
        ;;
esac
