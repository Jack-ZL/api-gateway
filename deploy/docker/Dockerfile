# Dockerfile for api-gateway

# --- Builder Stage ---
FROM golang:1.22-alpine AS builder

# 设置工作目录在容器内部
WORKDIR /app

# 复制 go.mod 和 go.sum 文件，用于下载依赖
COPY go.mod go.sum ./

# 下载 Go 模块依赖 (利用 Docker Layer Caching 加速构建)
RUN go mod download

# 复制项目源代码到工作目录
COPY . .

# 构建 api-gateway 可执行文件
RUN go build -o gateway cmd/gateway/main.go

# --- Runner Stage ---
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从 builder stage 复制构建好的可执行文件
COPY --from=builder /app/gateway ./gateway

# 复制配置文件目录
COPY config ./config

# 声明暴露端口 (网关默认监听 8000 端口)
EXPOSE 8000

# 定义容器启动时执行的命令
CMD ["./gateway"]