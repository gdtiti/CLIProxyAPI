#!/bin/bash
# HF Space 启动脚本

set -e

cd /app

# 检查 CLI 是否存在
if [ ! -f "./CLIProxyAPIPlus" ]; then
    echo "Error: CLIProxyAPIPlus not found!"
    exit 1
fi

# 如果 config.yaml 不存在，从 example 复制
if [ ! -f "./config.yaml" ]; then
    echo "No config.yaml found, copying from example..."
    cp config.example.yaml config.yaml
    # 修改默认端口为 7860 (HF Space 默认端口)
    sed -i 's/^port:.*/port: 7860/' config.yaml
fi

# 打印环境变量配置
echo "========================================"
echo "CLIProxyAPIPlus Environment Configuration"
echo "========================================"
echo "Download URL:   ${DOWNLOAD_URL:+[SET]}"
echo "Management Password: ${MANAGEMENT_PASSWORD:+[SET]}"
echo "PGSTORE_DS:     ${PGSTORE_DS:+[SET]}"
echo "========================================"

# 允许通过环境变量传递额外参数
# CLI_PROXY_ARGS: 传递给 CLIProxyAPIPlus 的额外参数
EXTRA_ARGS="${CLI_PROXY_ARGS:-}"

echo "Starting CLIProxyAPIPlus..."
echo "Extra args: ${EXTRA_ARGS}"

# 启动服务（环境变量会自动传递给进程）
exec ./CLIProxyAPIPlus ${EXTRA_ARGS}
