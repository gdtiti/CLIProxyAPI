# HF Space Dockerfile - 从环境变量下载预编译二进制并运行
FROM alpine:3.22

# 安装必要工具
RUN apk add --no-cache curl tzdata bash

# 设置时区
ENV TZ=Asia/Shanghai
RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

# 创建工作目录
RUN mkdir /app
WORKDIR /app

# 环境变量说明：
# - DOWNLOAD_URL: 预编译二进制下载链接（必填）
# - MANAGEMENT_PASSWORD: 管理面板密码
# - PGSTORE_DS: PostgreSQL 数据源连接字符串

# 下载链接（必填）
ARG DOWNLOAD_URL
ENV DOWNLOAD_URL=${DOWNLOAD_URL:-}

# 运行时环境变量
ARG MANAGEMENT_PASSWORD
ENV MANAGEMENT_PASSWORD=${MANAGEMENT_PASSWORD:-}

ARG PGSTORE_DS
ENV PGSTORE_DS=${PGSTORE_DS:-}

RUN echo "Downloading from: ${DOWNLOAD_URL}" && \
    curl -L -o cli.tar.gz "${DOWNLOAD_URL}" && \
    tar -xzf cli.tar.gz && \
    rm cli.tar.gz && \
    # 压缩包内的文件名是 cli-proxy-api-plus
    chmod +x cli-proxy-api-plus && \
    mv cli-proxy-api-plus CLIProxyAPIPlus && \
    ls -la

# 复制配置文件
COPY config.example.yaml /app/config.example.yaml

# 默认端口 (HF Space)
EXPOSE 7860

# 启动脚本
COPY run.sh /app/run.sh
RUN chmod +x /app/run.sh

# 默认命令
CMD ["/app/run.sh"]
