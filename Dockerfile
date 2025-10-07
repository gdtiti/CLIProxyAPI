# 构建参数
ARG MANAGEMENT_ASSET_VERSION=latest
ARG PREPACKAGE_MANAGEMENT_ASSET=true

# 下载管理面板资源
FROM alpine:3.22.0 AS management_asset_downloader
RUN apk add --no-cache curl

WORKDIR /tmp
ARG MANAGEMENT_ASSET_VERSION

# 下载管理面板资源
RUN if [ "$MANAGEMENT_ASSET_VERSION" = "latest" ]; then \
  MANAGEMENT_ASSET_VERSION=$(curl -s https://api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center/releases/latest | grep '"tag_name":' | cut -d'"' -f4); \
  fi && \
  echo "Downloading management asset version: ${MANAGEMENT_ASSET_VERSION}" && \
  curl -L -o management.html \
    "https://github.com/router-for-me/Cli-Proxy-API-Management-Center/releases/download/${MANAGEMENT_ASSET_VERSION}/management.html" || \
  echo "Management asset download failed, will use runtime fallback"

FROM golang:1.24-alpine AS builder

WORKDIR /app

# 配置Go代理解决网络问题
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,https://mirrors.aliyun.com/goproxy/,direct
ENV GOSUMDB=sum.golang.google.cn
ENV GO111MODULE=on

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./CLIProxyAPI ./cmd/server/

FROM alpine:3.22.0

RUN apk add --no-cache tzdata

RUN mkdir /CLIProxyAPI

COPY --from=builder ./app/CLIProxyAPI /CLIProxyAPI/CLIProxyAPI

# 预打包管理面板资源
ARG PREPACKAGE_MANAGEMENT_ASSET
RUN if [ "$PREPACKAGE_MANAGEMENT_ASSET" = "true" ]; then \
  mkdir -p /CLIProxyAPI/static && \
  if [ -f /tmp/management.html ]; then \
    cp /tmp/management.html /CLIProxyAPI/static/management.html && \
    echo "Management asset pre-packaged successfully"; \
  else \
    echo "Management asset not available, will use runtime download"; \
  fi; \
else \
  echo "Management asset pre-packaging disabled, will use runtime download"; \
fi

WORKDIR /CLIProxyAPI

EXPOSE 8317

ENV TZ=Asia/Shanghai

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./CLIProxyAPI"]