# Docker 构建说明

## 缓存机制

Docker 构建使用了两层缓存加速：

### 1. Docker 层缓存

`go.mod` / `go.sum` 先于源码复制到镜像中并执行 `go mod download`，当这两个文件未变化时该层会被复用，避免重复下载。

### 2. BuildKit 缓存挂载

通过 `--mount=type=cache` 将以下目录持久化在宿主机，与 Docker 层缓存解耦：

| 挂载目标 | 用途 |
|---|---|
| `/go/pkg/mod/` | Go 模块包缓存 |
| `/root/.cache/go-build` | Go 编译中间产物缓存 |

即使层缓存被冲掉（如源码变更），已下载的模块包和编译产物也不会重复生成。

### 构建上下文过滤

`.dockerignore` 排除了日志、IDE 配置、本地构建产物等无关文件，减少发送给 Docker daemon 的数据量，同时避免这些文件变更导致层缓存失效。

## 清理缓存

```bash
# 清理所有 BuildKit 构建缓存
docker builder prune --all --force

# 仅清理构建缓存（保留其他缓存）
docker builder prune --filter type=build-cache --force

# 查看缓存磁盘占用
docker system df
```

## 注意

- 需使用 Docker 18.09+（BuildKit 默认启用）
- 若手动关闭了 BuildKit，设置环境变量 `DOCKER_BUILDKIT=1` 重新启用
