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
echo   CLIProxyAPIPlus 开发菜单
echo ========================================
echo.
echo 开发命令:
echo   1. 安装依赖
echo   2. 启动开发服务器
echo   3. 构建生产版本
echo   4. 运行测试
echo   5. 运行代码检查
echo   6. 格式化代码
echo   7. 清理构建产物
echo.
echo 后端管理:
echo   8. 启动后端
echo   9. 停止后端
echo   10. 重启后端
echo   11. 检查后端状态
echo.
echo 认证登录:
echo   12. 登录菜单
echo   13. 检查配额
echo   14. 验证凭证
echo.
echo 其他:
echo   15. 显示帮助
echo   0. 退出
echo.
set /p "choice=请输入选项 (0-15): "

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
echo 无效选项，请重试...
timeout /t 2 /nobreak >nul
goto :show_menu

:menu_end
echo.
echo 按任意键返回菜单，或按 Ctrl+C 退出...
pause >nul
goto :show_menu
exit /b 0

:cmd_install
echo 正在安装依赖...
go mod download
if %ERRORLEVEL% neq 0 (
    echo 安装依赖失败
    exit /b 1
)
echo 依赖安装成功
exit /b 0

:cmd_dev
echo 正在启动开发服务器...
go run cmd/server/main.go
exit /b 0

:cmd_build
echo 正在构建生产版本...
go build -o bin/cliproxyapi.exe cmd/server/main.go
if %ERRORLEVEL% neq 0 (
    echo 构建失败
    exit /b 1
)
echo 构建完成: bin/cliproxyapi.exe
exit /b 0

:cmd_test
echo 正在运行测试...
go test ./...
exit /b 0

:cmd_lint
echo 正在运行代码检查...
where golangci-lint >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo golangci-lint 未找到。请从以下地址安装: https://golangci-lint.run/usage/install/
    exit /b 1
)
golangci-lint run
exit /b 0

:cmd_format
echo 正在格式化代码...
go fmt ./...
echo 代码格式化完成
exit /b 0

:cmd_clean
echo 正在清理构建产物...
if exist bin rmdir /s /q bin
if exist dist rmdir /s /q dist
if exist %PID_FILE% del /f %PID_FILE%
echo 清理完成
exit /b 0

:cmd_start
echo 正在启动后端服务器...
if exist %PID_FILE% (
    echo 后端已在运行 (PID 文件存在)
    call :cmd_status
    exit /b 0
)
start /b "" %BACKEND_CMD%
timeout /t 2 /nobreak >nul
echo 后端已启动
exit /b 0

:cmd_stop
echo 正在停止后端服务器...
if not exist %PID_FILE% (
    echo 后端未运行 (无 PID 文件)
    exit /b 0
)
taskkill /f /im go.exe >nul 2>&1
del /f %PID_FILE% >nul 2>&1
echo 后端已停止
exit /b 0

:cmd_restart
echo 正在重启后端服务器...
call :cmd_stop
timeout /t 1 /nobreak >nul
call :cmd_start
exit /b 0

:cmd_status
echo 正在检查后端状态...
tasklist /fi "imagename eq go.exe" 2>nul | find /i "go.exe" >nul
if %ERRORLEVEL% equ 0 (
    echo 后端正在运行
) else (
    echo 后端未运行
)
exit /b 0

:cmd_login_menu
echo.
echo === 认证登录菜单 ===
echo.
echo 选择提供商:
echo   1. Kiro (AWS CodeWhisperer)
echo   2. Codex CLI
echo   3. Gemini CLI
echo   4. Antigravity
echo   5. 退出
echo.
set /p "choice=请输入选项 (1-5): "

if "%choice%"=="1" call :cmd_login_kiro & exit /b 0
if "%choice%"=="2" call :cmd_login_codex & exit /b 0
if "%choice%"=="3" call :cmd_login_gemini & exit /b 0
if "%choice%"=="4" call :cmd_login_antigravity & exit /b 0
if "%choice%"=="5" exit /b 0

echo 无效选项
exit /b 1

:cmd_login_kiro
echo.
echo === Kiro (AWS CodeWhisperer) 登录 ===
echo.
echo 选择认证方式:
echo   1. Builder ID (推荐)
echo   2. Google 账号
echo   3. GitHub 账号
echo   4. 取消
echo.
set /p "method=请输入选项 (1-4): "

if "%method%"=="1" (
    echo 正在使用 Builder ID 登录...
    kiro login --method builder-id
) else if "%method%"=="2" (
    echo 正在使用 Google 登录...
    kiro login --method google
) else if "%method%"=="3" (
    echo 正在使用 GitHub 登录...
    kiro login --method github
) else if "%method%"=="4" (
    exit /b 0
) else (
    echo 无效选项
    exit /b 1
)

if %ERRORLEVEL% equ 0 (
    echo Kiro 登录成功
) else (
    echo Kiro 登录失败
    exit /b 1
)
exit /b 0

:cmd_login_codex
echo 正在登录 Codex CLI...
codex login
if %ERRORLEVEL% equ 0 (
    echo Codex 登录成功
) else (
    echo Codex 登录失败
    exit /b 1
)
exit /b 0

:cmd_login_gemini
echo.
echo === Gemini CLI 登录 ===
echo.
echo 重要提示: 关于 'ALL' 选项的已知问题
echo.
echo 当提示选择项目时:
echo   - 建议: 选择单个特定的项目 ID
echo   - 不要: 选择 "ALL" (可能导致静默失败)
echo.
echo 推荐方法:
echo   1. 选择你经常使用的项目
echo   2. 避免测试/临时项目 (例如 csp-cli-*)
echo   3. 确保已启用 Generative Language API
echo.
echo 详情请参阅 docs/TROUBLESHOOTING.md
echo.
pause
echo.
echo 正在登录 Gemini CLI...
gemini-cli login

if %ERRORLEVEL% equ 0 (
    echo.
    echo Gemini 登录成功
    echo.
    echo 正在验证凭证文件...
    call :verify_gemini_creds
) else (
    echo Gemini 登录失败
    echo.
    echo 故障排除提示:
    echo   1. 检查是否选择了单个项目 (而非 ALL)
    echo   2. 验证项目是否已启用所需的 API
    echo   3. 详情请参阅 docs/TROUBLESHOOTING.md
    exit /b 1
)
exit /b 0

:verify_gemini_creds
if exist "%USERPROFILE%\.gemini\credentials.json" (
    echo √ 找到凭证文件
    echo   位置: %USERPROFILE%\.gemini\credentials.json
) else if exist "%USERPROFILE%\.gemini\config.json" (
    echo √ 找到配置文件
    echo   位置: %USERPROFILE%\.gemini\config.json
) else (
    echo × 未找到凭证文件
    echo.
    echo 这可能表示发生了 'ALL' 错误。
    echo 请尝试使用单个项目重新登录。
    echo 详情请参阅 docs/TROUBLESHOOTING.md
    exit /b 1
)
exit /b 0

:cmd_login_antigravity
echo 正在登录 Antigravity...
antigravity login
if %ERRORLEVEL% equ 0 (
    echo Antigravity 登录成功
) else (
    echo Antigravity 登录失败
    exit /b 1
)
exit /b 0

:cmd_quota
echo 正在检查提供商配额...
echo.

echo === Kiro (AWS CodeWhisperer) ===
kiro quota 2>nul
if %ERRORLEVEL% neq 0 echo 未登录或命令不可用
echo.

echo === Codex CLI ===
codex quota 2>nul
if %ERRORLEVEL% neq 0 echo 未登录或命令不可用
echo.

echo === Gemini CLI ===
gemini-cli quota 2>nul
if %ERRORLEVEL% neq 0 echo 未登录或命令不可用
echo.

echo === Antigravity ===
antigravity quota 2>nul
if %ERRORLEVEL% neq 0 echo 未登录或命令不可用

exit /b 0

:cmd_verify_creds
echo 正在验证凭证...
echo.

echo 检查 Kiro 凭证...
if exist "%USERPROFILE%\.kiro\credentials" (
    echo √ 找到 Kiro 凭证
) else (
    echo × 未找到 Kiro 凭证
)

echo.
echo 检查 Codex 凭证...
if exist "%USERPROFILE%\.codex\credentials.json" (
    echo √ 找到 Codex 凭证
) else (
    echo × 未找到 Codex 凭证
)

echo.
echo 检查 Gemini 凭证...
if exist "%USERPROFILE%\.gemini\credentials.json" (
    echo √ 找到 Gemini 凭证
) else if exist "%USERPROFILE%\.gemini\config.json" (
    echo √ 找到 Gemini 配置
) else (
    echo × 未找到 Gemini 凭证
)

echo.
echo 检查 Antigravity 凭证...
if exist "%USERPROFILE%\.antigravity\credentials" (
    echo √ 找到 Antigravity 凭证
) else (
    echo × 未找到 Antigravity 凭证
)

exit /b 0
