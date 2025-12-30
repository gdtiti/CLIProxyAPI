@echo off
setlocal enabledelayedexpansion

:: CLIProxyAPIPlus Development Utility Script
:: Version: 1.1.0
:: Enhanced with credential validation and error handling

:: Color codes for Windows
set "RED=[91m"
set "GREEN=[92m"
set "YELLOW=[93m"
set "BLUE=[94m"
set "MAGENTA=[95m"
set "CYAN=[96m"
set "RESET=[0m"

:: Configuration
set "BACKEND_DIR=."
set "BACKEND_CMD=go run cmd/server/main.go"
set "PID_FILE=.backend.pid"

:: Parse command
set "CMD=%~1"
if "%CMD%"=="" (
    call :show_menu
    exit /b 0
)

:: Command routing
if /i "%CMD%"=="help" call :show_help & exit /b 0
if /i "%CMD%"=="h" call :show_help & exit /b 0

if /i "%CMD%"=="install" call :cmd_install & exit /b 0
if /i "%CMD%"=="i" call :cmd_install & exit /b 0

if /i "%CMD%"=="dev" call :cmd_dev & exit /b 0
if /i "%CMD%"=="d" call :cmd_dev & exit /b 0

if /i "%CMD%"=="build" call :cmd_build & exit /b 0
if /i "%CMD%"=="b" call :cmd_build & exit /b 0

if /i "%CMD%"=="test" call :cmd_test & exit /b 0
if /i "%CMD%"=="t" call :cmd_test & exit /b 0

if /i "%CMD%"=="lint" call :cmd_lint & exit /b 0
if /i "%CMD%"=="l" call :cmd_lint & exit /b 0

if /i "%CMD%"=="format" call :cmd_format & exit /b 0
if /i "%CMD%"=="fmt" call :cmd_format & exit /b 0

if /i "%CMD%"=="clean" call :cmd_clean & exit /b 0
if /i "%CMD%"=="c" call :cmd_clean & exit /b 0

if /i "%CMD%"=="start" call :cmd_start & exit /b 0
if /i "%CMD%"=="stop" call :cmd_stop & exit /b 0
if /i "%CMD%"=="restart" call :cmd_restart & exit /b 0
if /i "%CMD%"=="status" call :cmd_status & exit /b 0

if /i "%CMD%"=="login" call :cmd_login_menu & exit /b 0
if /i "%CMD%"=="login-kiro" call :cmd_login_kiro & exit /b 0
if /i "%CMD%"=="login-codex" call :cmd_login_codex & exit /b 0
if /i "%CMD%"=="login-gemini" call :cmd_login_gemini & exit /b 0
if /i "%CMD%"=="login-antigravity" call :cmd_login_antigravity & exit /b 0

if /i "%CMD%"=="quota" call :cmd_quota & exit /b 0
if /i "%CMD%"=="q" call :cmd_quota & exit /b 0

if /i "%CMD%"=="verify-creds" call :cmd_verify_creds & exit /b 0

echo %RED%Unknown command: %CMD%%RESET%
echo Run 'dev.bat help' for usage information
exit /b 1

:: ============================================================================
:: Command Implementations
:: ============================================================================

:show_help
echo %CYAN%CLIProxyAPIPlus Development Utility%RESET%
echo.
echo %YELLOW%Usage:%RESET% dev.bat [command]
echo.
echo %YELLOW%Development Commands:%RESET%
echo   install, i          Install dependencies
echo   dev, d              Start development server
echo   build, b            Build for production
echo   test, t             Run tests
echo   lint, l             Run linter
echo   format, fmt         Format code
echo   clean, c            Clean build artifacts
echo.
echo %YELLOW%Backend Management:%RESET%
echo   start               Start backend server
echo   stop                Stop backend server
echo   restart             Restart backend server
echo   status              Check backend status
echo.
echo %YELLOW%Authentication:%RESET%
echo   login               Interactive login menu
echo   login-kiro          Login to Kiro (AWS CodeWhisperer)
echo   login-codex         Login to Codex CLI
echo   login-gemini        Login to Gemini CLI
echo   login-antigravity   Login to Antigravity
echo.
echo %YELLOW%Utilities:%RESET%
echo   quota, q            Check provider quotas
echo   verify-creds        Verify all credentials
echo   help, h             Show this help message
exit /b 0

:show_menu
cls
echo.
echo ========================================
echo   CLIProxyAPIPlus Development Menu
echo ========================================
echo.
echo Development:
echo   1. Install dependencies
echo   2. Start development server
echo   3. Build for production
echo   4. Run tests
echo   5. Run linter
echo   6. Format code
echo   7. Clean build artifacts
echo.
echo Backend:
echo   8. Start backend
echo   9. Stop backend
echo   10. Restart backend
echo   11. Check backend status
echo.
echo Authentication:
echo   12. Login menu
echo   13. Check quotas
echo   14. Verify credentials
echo.
echo Other:
echo   15. Show help
echo   0. Exit
echo.
set /p "choice=Enter your choice (0-15): "

if "%choice%"=="1" call :cmd_install & goto :menu_end
if "%choice%"=="2" call :cmd_dev & goto :menu_end
if "%choice%"=="3" call :cmd_build & goto :menu_end
if "%choice%"=="4" call :cmd_test & goto :menu_end
if "%choice%"=="5" call :cmd_lint & goto :menu_end
if "%choice%"=="6" call :cmd_format & goto :menu_end
if "%choice%"=="7" call :cmd_clean & goto :menu_end
if "%choice%"=="8" call :cmd_start & goto :menu_end
if "%choice%"=="9" call :cmd_stop & goto :menu_end
if "%choice%"=="10" call :cmd_restart & goto :menu_end
if "%choice%"=="11" call :cmd_status & goto :menu_end
if "%choice%"=="12" call :cmd_login_menu & goto :menu_end
if "%choice%"=="13" call :cmd_quota & goto :menu_end
if "%choice%"=="14" call :cmd_verify_creds & goto :menu_end
if "%choice%"=="15" call :show_help & goto :menu_end
if "%choice%"=="0" exit /b 0

echo.
echo %RED%Invalid choice. Please try again.%RESET%
timeout /t 2 /nobreak >nul
goto :show_menu

:menu_end
echo.
echo Press any key to return to menu or Ctrl+C to exit...
pause >nul
goto :show_menu
exit /b 0

:cmd_install
echo %BLUE%Installing dependencies...%RESET%
go mod download
if %ERRORLEVEL% neq 0 (
    echo %RED%Failed to install dependencies%RESET%
    exit /b 1
)
echo %GREEN%Dependencies installed successfully%RESET%
exit /b 0

:cmd_dev
echo %BLUE%Starting development server...%RESET%
go run cmd/server/main.go
exit /b 0

:cmd_build
echo %BLUE%Building for production...%RESET%
go build -o bin/cliproxyapi.exe cmd/server/main.go
if %ERRORLEVEL% neq 0 (
    echo %RED%Build failed%RESET%
    exit /b 1
)
echo %GREEN%Build completed: bin/cliproxyapi.exe%RESET%
exit /b 0

:cmd_test
echo %BLUE%Running tests...%RESET%
go test ./...
exit /b 0

:cmd_lint
echo %BLUE%Running linter...%RESET%
where golangci-lint >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo %YELLOW%golangci-lint not found. Install it from: https://golangci-lint.run/usage/install/%RESET%
    exit /b 1
)
golangci-lint run
exit /b 0

:cmd_format
echo %BLUE%Formatting code...%RESET%
go fmt ./...
echo %GREEN%Code formatted%RESET%
exit /b 0

:cmd_clean
echo %BLUE%Cleaning build artifacts...%RESET%
if exist bin rmdir /s /q bin
if exist dist rmdir /s /q dist
if exist %PID_FILE% del /f %PID_FILE%
echo %GREEN%Clean completed%RESET%
exit /b 0

:cmd_start
echo %BLUE%Starting backend server...%RESET%
if exist %PID_FILE% (
    echo %YELLOW%Backend is already running (PID file exists)%RESET%
    call :cmd_status
    exit /b 0
)
start /b "" %BACKEND_CMD%
timeout /t 2 /nobreak >nul
echo %GREEN%Backend started%RESET%
exit /b 0

:cmd_stop
echo %BLUE%Stopping backend server...%RESET%
if not exist %PID_FILE% (
    echo %YELLOW%Backend is not running (no PID file)%RESET%
    exit /b 0
)
taskkill /f /im go.exe >nul 2>&1
del /f %PID_FILE% >nul 2>&1
echo %GREEN%Backend stopped%RESET%
exit /b 0

:cmd_restart
echo %BLUE%Restarting backend server...%RESET%
call :cmd_stop
timeout /t 1 /nobreak >nul
call :cmd_start
exit /b 0

:cmd_status
echo %BLUE%Checking backend status...%RESET%
tasklist /fi "imagename eq go.exe" 2>nul | find /i "go.exe" >nul
if %ERRORLEVEL% equ 0 (
    echo %GREEN%Backend is running%RESET%
) else (
    echo %YELLOW%Backend is not running%RESET%
)
exit /b 0

:cmd_login_menu
echo.
echo %CYAN%=== Authentication Login Menu ===%RESET%
echo.
echo %YELLOW%Select a provider:%RESET%
echo   1. Kiro (AWS CodeWhisperer)
echo   2. Codex CLI
echo   3. Gemini CLI
echo   4. Antigravity
echo   5. Exit
echo.
set /p "choice=Enter your choice (1-5): "

if "%choice%"=="1" call :cmd_login_kiro & exit /b 0
if "%choice%"=="2" call :cmd_login_codex & exit /b 0
if "%choice%"=="3" call :cmd_login_gemini & exit /b 0
if "%choice%"=="4" call :cmd_login_antigravity & exit /b 0
if "%choice%"=="5" exit /b 0

echo %RED%Invalid choice%RESET%
exit /b 1

:cmd_login_kiro
echo.
echo %CYAN%=== Kiro (AWS CodeWhisperer) Login ===%RESET%
echo.
echo %YELLOW%Select authentication method:%RESET%
echo   1. Builder ID (Recommended)
echo   2. Google Account
echo   3. GitHub Account
echo   4. Cancel
echo.
set /p "method=Enter your choice (1-4): "

if "%method%"=="1" (
    echo %BLUE%Logging in with Builder ID...%RESET%
    kiro login --method builder-id
) else if "%method%"=="2" (
    echo %BLUE%Logging in with Google...%RESET%
    kiro login --method google
) else if "%method%"=="3" (
    echo %BLUE%Logging in with GitHub...%RESET%
    kiro login --method github
) else if "%method%"=="4" (
    exit /b 0
) else (
    echo %RED%Invalid choice%RESET%
    exit /b 1
)

if %ERRORLEVEL% equ 0 (
    echo %GREEN%Kiro login successful%RESET%
) else (
    echo %RED%Kiro login failed%RESET%
    exit /b 1
)
exit /b 0

:cmd_login_codex
echo %BLUE%Logging in to Codex CLI...%RESET%
codex login
if %ERRORLEVEL% equ 0 (
    echo %GREEN%Codex login successful%RESET%
) else (
    echo %RED%Codex login failed%RESET%
    exit /b 1
)
exit /b 0

:cmd_login_gemini
echo.
echo %CYAN%=== Gemini CLI Login ===%RESET%
echo.
echo %YELLOW%IMPORTANT: Known Issue with 'ALL' Selection%RESET%
echo.
echo When prompted to select projects:
echo   - %GREEN%DO:%RESET% Select a single, specific project ID
echo   - %RED%DON'T:%RESET% Select "ALL" (may cause silent failure)
echo.
echo %YELLOW%Recommended approach:%RESET%
echo   1. Choose a project you actively use
echo   2. Avoid test/temporary projects (e.g., csp-cli-*)
echo   3. Ensure the Generative Language API is enabled
echo.
echo See docs/TROUBLESHOOTING.md for details
echo.
pause
echo.
echo %BLUE%Logging in to Gemini CLI...%RESET%
gemini-cli login

if %ERRORLEVEL% equ 0 (
    echo.
    echo %GREEN%Gemini login successful%RESET%
    echo.
    echo %BLUE%Verifying credential file...%RESET%
    call :verify_gemini_creds
) else (
    echo %RED%Gemini login failed%RESET%
    echo.
    echo %YELLOW%Troubleshooting tips:%RESET%
    echo   1. Check if you selected a single project (not ALL)
    echo   2. Verify the project has required APIs enabled
    echo   3. See docs/TROUBLESHOOTING.md for more help
    exit /b 1
)
exit /b 0

:verify_gemini_creds
if exist "%USERPROFILE%\.gemini\credentials.json" (
    echo %GREEN%✓ Credential file found%RESET%
    echo   Location: %USERPROFILE%\.gemini\credentials.json
) else if exist "%USERPROFILE%\.gemini\config.json" (
    echo %GREEN%✓ Config file found%RESET%
    echo   Location: %USERPROFILE%\.gemini\config.json
) else (
    echo %RED%✗ No credential file found%RESET%
    echo.
    echo %YELLOW%This may indicate the 'ALL' bug occurred.%RESET%
    echo Please try logging in again with a single project.
    echo See docs/TROUBLESHOOTING.md for details.
    exit /b 1
)
exit /b 0

:cmd_login_antigravity
echo %BLUE%Logging in to Antigravity...%RESET%
antigravity login
if %ERRORLEVEL% equ 0 (
    echo %GREEN%Antigravity login successful%RESET%
) else (
    echo %RED%Antigravity login failed%RESET%
    exit /b 1
)
exit /b 0

:cmd_quota
echo %BLUE%Checking provider quotas...%RESET%
echo.

echo %CYAN%=== Kiro (AWS CodeWhisperer) ===%RESET%
kiro quota 2>nul
if %ERRORLEVEL% neq 0 echo %YELLOW%Not logged in or command not available%RESET%
echo.

echo %CYAN%=== Codex CLI ===%RESET%
codex quota 2>nul
if %ERRORLEVEL% neq 0 echo %YELLOW%Not logged in or command not available%RESET%
echo.

echo %CYAN%=== Gemini CLI ===%RESET%
gemini-cli quota 2>nul
if %ERRORLEVEL% neq 0 echo %YELLOW%Not logged in or command not available%RESET%
echo.

echo %CYAN%=== Antigravity ===%RESET%
antigravity quota 2>nul
if %ERRORLEVEL% neq 0 echo %YELLOW%Not logged in or command not available%RESET%

exit /b 0

:cmd_verify_creds
echo %BLUE%Verifying credentials...%RESET%
echo.

echo %CYAN%Checking Kiro credentials...%RESET%
if exist "%USERPROFILE%\.kiro\credentials" (
    echo %GREEN%✓ Kiro credentials found%RESET%
) else (
    echo %YELLOW%✗ Kiro credentials not found%RESET%
)

echo.
echo %CYAN%Checking Codex credentials...%RESET%
if exist "%USERPROFILE%\.codex\credentials.json" (
    echo %GREEN%✓ Codex credentials found%RESET%
) else (
    echo %YELLOW%✗ Codex credentials not found%RESET%
)

echo.
echo %CYAN%Checking Gemini credentials...%RESET%
if exist "%USERPROFILE%\.gemini\credentials.json" (
    echo %GREEN%✓ Gemini credentials found%RESET%
) else if exist "%USERPROFILE%\.gemini\config.json" (
    echo %GREEN%✓ Gemini config found%RESET%
) else (
    echo %YELLOW%✗ Gemini credentials not found%RESET%
)

echo.
echo %CYAN%Checking Antigravity credentials...%RESET%
if exist "%USERPROFILE%\.antigravity\credentials" (
    echo %GREEN%✓ Antigravity credentials found%RESET%
) else (
    echo %YELLOW%✗ Antigravity credentials not found%RESET%
)

exit /b 0
