# ===== 构建阶段 =====
FROM golang:1.26-alpine AS builder

WORKDIR /build

# 设置 Go 模块代理（国内镜像）
ENV GOPROXY=https://goproxy.cn,direct

# 提前复制依赖文件以利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download -x

# 编译
COPY . .

# 安装 swag CLI 并生成 API 文档
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init -g cmd/server/docs.go -o api/swagger --parseDependency --parseInternal

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server/

# ===== 运行阶段 =====
FROM alpine:3.21

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/server .
COPY config.yaml .

EXPOSE 29090 29092

CMD ["./server"]
