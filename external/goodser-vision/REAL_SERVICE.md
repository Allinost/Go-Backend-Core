# 真实视觉服务部署手册（Docker Compose）

> 适用范围：`external/goodser-vision` 从 fallback 模式切换到真实 CLIP / YOLO / OCR。
> 目标：在 RTX 5060 Ti 16GB 上与 embedding / TTS 等服务共享显存，视觉服务预算 ≤ 8GB。
> 代码目录：`Go-Backend-Core/external/goodser-vision`

---

## 1. 前置条件

| 项 | 要求 |
|---|---|
| 宿主机 | Linux（推荐）或 Windows + WSL2 |
| GPU | NVIDIA，驱动 ≥ 550，支持 CUDA 12.8+（Blackwell / RTX 50 系） |
| 容器 | Docker 24+，NVIDIA Container Toolkit |
| 基础镜像 | `pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime` |
| 显存 | 16GB 共享；视觉服务 ≤ 8GB |
| 网络 | 仅内网；不把 8000 暴露公网 |
| 模型 | YOLO 检测权重、Chinese-CLIP、PaddleOCR（可选） |

验证 GPU：

```bash
nvidia-smi
docker run --rm --gpus all nvidia/cuda:12.8.0-base-ubuntu22.04 nvidia-smi
```

> RTX 5060 Ti 是 Blackwell（sm_120），必须使用 CUDA 12.8+ / PyTorch 2.7+。
> 本项目 `Dockerfile.real` 已固定为 `pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime`。
> 若使用更新驱动/镜像，可选 `2.11.0-cuda12.8-cudnn9-runtime`，需重新验证 Ultralytics/Paddle 兼容性。

---

## 2. 目录与模型

```text
external/goodser-vision/
├── models/
│   ├── yolo11s-goods.pt          # 谷子检测权重（首批 100–300 张标注后训练）
│   └── .gitkeep
├── hf-cache/                     # HuggingFace 缓存（Chinese-CLIP）
├── paddle-cache/                 # PaddleOCR 模型缓存
├── Dockerfile                    # fallback/CPU
├── Dockerfile.real               # CUDA + CLIP/YOLO/OCR
├── docker-compose.yml            # fallback/CPU
├── docker-compose.real.yml       # 真实服务/GPU
├── requirements.txt
└── requirements-real.txt
```

模型下载（示例）：

```bash
# Chinese-CLIP 首次启动会自动下载到 hf-cache
# YOLO 权重放到 models/yolo11s-goods.pt
# PaddleOCR 首次启动会自动下载到 paddle-cache
```

> 模型权重不要提交 Git；`.gitignore` 已排除 `models/*`。

---

## 3. 启动真实服务

```bash
cd external/goodser-vision
cp env.real.example env.real
# 按需修改模型路径、并发、显存比例
docker compose --env-file env.real -f docker-compose.real.yml up -d --build
```

查看状态：

```bash
docker compose -f docker-compose.real.yml ps
docker logs -f goodser-vision
curl -s http://127.0.0.1:8000/healthz
curl -s http://127.0.0.1:8000/v1/models
```

预期：

```json
{"status":"ok","embedder":"clip-v1","detector":"yolo-v1","device":"cuda"}
```

---

## 4. 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `DEVICE` | `cuda` | `cuda` / `cpu` |
| `GOODSER_VISION_EMBEDDER` | `clip` | `fallback` / `clip` |
| `GOODSER_VISION_DETECTOR` | `yolo` | `fallback` / `yolo` |
| `GOODSER_VISION_EMBED_DIM` | `512` | 与数据库向量维度一致 |
| `GOODSER_VISION_MAX_CONCURRENCY` | `1` | 真实模型建议 1–2 |
| `GOODSER_VISION_MAX_IMAGE_SIDE` | `1920` | 检测输入最长边 |
| `GOODSER_VISION_MODEL_DIR` | `/app/models` | YOLO 权重目录 |
| `HF_HOME` | `/app/hf-cache` | Chinese-CLIP 缓存 |
| `PADDLEOCR_HOME` | `/app/paddle-cache` | PaddleOCR 缓存 |
| `GOODSER_VISION_WARMUP` | `1` | 启动预热 |
| `GOODSER_VISION_IDLE_UNLOAD_SECONDS` | `0` | 不活跃卸载秒数；0=常驻不卸载 |
| `GOODSER_VISION_IDLE_CHECK_SECONDS` | `30` | 空闲扫描间隔 |
| `GOODSER_VISION_GPU_MEMORY_FRACTION` | `0.5` | 视觉服务显存上限比例（16GB → 8GB） |
| `GOODSER_VISION_GPU_HEADROOM_MB` | `1024` | 预留显存，低于阈值拒绝新请求 |
| `GOODSER_VISION_VLM_ENABLED` | `0` | 是否启用 VLM 复核 |
| `GOODSER_VISION_VLM_PROVIDER` | `disabled` | `disabled` / `openai`（OpenAI 兼容） |
| `GOODSER_VISION_VLM_BASE_URL` | — | 例如 `http://127.0.0.1:8001/v1` |
| `GOODSER_VISION_VLM_API_KEY` | — | 外部服务 Key；不要写进代码/仓库 |
| `GOODSER_VISION_VLM_MODEL` | `qwen2.5-vl-3b-instruct` | 外部服务模型名 |
| `GOODSER_VISION_VLM_MAX_IMAGE_SIDE` | `1280` | 传给 VLM 的图片最长边 |

> 显存比例通过 `torch.cuda.set_per_process_memory_fraction` 限制；实际占用仍需压测。
> VLM 默认关闭；启用时通过 OpenAI 兼容接口调用，不在本进程加载 7B 权重。

### 4.1 模型常驻、预热与空闲卸载

- 启动时加载并预热：Chinese-CLIP、YOLO、PaddleOCR。
- 常驻单例：请求复用同一份模型，不再每次 `from_pretrained` / `YOLO(...)`。
- `GOODSER_VISION_IDLE_UNLOAD_SECONDS=0`：所有已加载模型常驻，不卸载（推荐高频使用）。
- 设置为 `>0`：超过该秒数无请求时释放模型显存；下一次请求会重新加载。
- `GOODSER_VISION_MAX_CONCURRENCY=1`：连续高频时排队，避免重复分配显存。
- 不要多 worker：`uvicorn --workers 1`，否则每个 worker 复制一份模型。

### 4.2 VLM 复核

VLM = Vision Language Model，能看图 + 文本并输出判断/描述。适合：

- 识别低置信度时复核 Top-K 候选；
- 读取包装/标签文字；
- 判断同系列不同角色是否混淆。

启用方式（New API 主/备双槽位）：

```env
GOODSER_VISION_VLM_ENABLED=1

# 主
GOODSER_VISION_VLM_PRIMARY_ENABLED=1
GOODSER_VISION_VLM_PRIMARY_PROVIDER=newapi
GOODSER_VISION_VLM_PRIMARY_BASE_URL=http://new-api:3000/v1
GOODSER_VISION_VLM_PRIMARY_API_KEY=<New API Token A>
GOODSER_VISION_VLM_PRIMARY_MODEL=deepseek-v4.1-flash

# 备
GOODSER_VISION_VLM_BACKUP_ENABLED=1
GOODSER_VISION_VLM_BACKUP_PROVIDER=newapi
GOODSER_VISION_VLM_BACKUP_BASE_URL=http://new-api:3000/v1
GOODSER_VISION_VLM_BACKUP_API_KEY=<New API Token B>
GOODSER_VISION_VLM_BACKUP_MODEL=<备用视觉模型名>
```

说明：

- 主/备可以指向同一个 New API，但建议使用不同渠道/Token 或不同模型，避免同一渠道故障时一起失败。
- `goodser-vision` 只填 New API 地址、Token、模型名，不直接接触 opencode/commandcode 订阅 Token。
- `/v1/verify` 先调主；主超时/报错/无结果时自动调备；返回结果里带 `slot=primary|backup`。
- 主备都失败时返回 `503`，**不回退本地模型**。
- 兼容旧单槽位变量：`GOODSER_VISION_VLM_BASE_URL/API_KEY/MODEL` 会作为主槽位默认值。

**VLM 开关策略：**

```env
# 默认关闭
GOODSER_VISION_VLM_ENABLED=0
```

运行时配置（无需重启）：

```bash
# 查看当前主/备配置（不返回密钥明文，只返回 api_key_set）
curl -s http://127.0.0.1:8000/v1/system/vlm

# 设置主/备并启用
curl -X PUT http://127.0.0.1:8000/v1/system/vlm \
  -H 'Content-Type: application/json' \
  -d '{
    "enabled": true,
    "primary": {
      "enabled": true,
      "provider": "newapi",
      "base_url": "http://new-api:3000/v1",
      "model": "deepseek-v4.1-flash",
      "api_key": "<New API Token A>"
    },
    "backup": {
      "enabled": true,
      "provider": "newapi",
      "base_url": "http://new-api:3000/v1",
      "model": "<备用视觉模型名>",
      "api_key": "<New API Token B>"
    }
  }'

# 测试主/备；也可以加 ?slot=primary 或 ?slot=backup
curl -X POST http://127.0.0.1:8000/v1/system/vlm/test

# 临时关闭
curl -X POST http://127.0.0.1:8000/v1/system/vlm \
  -H 'Content-Type: application/json' \
  -d '{"enabled": false}'
```

- 配置持久化到 `data/vlm.json`（权限尽量 0600），密钥不会通过 API 回显。
- VLM 关闭/未配置/主备均不可用时，`/v1/verify` 返回 `503`，不会加载本地模型。
- `/v1/analyze`、`/v1/segment`、图鉴切割不依赖 VLM。
- 前端/Android 把 VLM 复核当作“可选增强”：失败时继续人工确认。
- 如果 New API 渠道不支持图片输入，保持关闭即可；主识别链路仍走 CLIP + YOLO + OCR + 人工复核。

调用：

```bash
curl -X POST http://127.0.0.1:8000/v1/verify \
  -F image=@samples/item.jpg \
  -F prompt="判断图中商品是否与候选一致" \
  -F candidates='[{"product_id":"p1","name":"徽章 A","score":0.82}]'
```

### 4.3 显存监控

```bash
curl -s http://127.0.0.1:8000/v1/system/status
curl -s http://127.0.0.1:8000/metrics
```

`/v1/system/status` 返回：

- 每个模型的 `loaded` / `last_used` / `device`
- `vram.used_bytes` / `free_bytes` / `total_bytes`
- `torch_allocated` / `torch_reserved`
- 空闲卸载配置

`/metrics` 暴露：

```text
goodser_vision_model_loaded{model="embedder"} 1
goodser_vision_model_loaded{model="detector"} 1
goodser_vision_model_loaded{model="ocr"} 1
goodser_vision_model_loaded{model="vlm"} 0
goodser_vision_gpu_memory_bytes{state="used"} ...
goodser_vision_gpu_memory_bytes{state="free"} ...
goodser_vision_gpu_memory_bytes{state="total"} ...
```

> PaddleOCR 的显存不完全计入 `torch_allocated`；请以 `used/free/total` 与 `nvidia-smi` 为准。

---

## 5. 接入 Go 后端

后端通过 `GOODSER_VISION_URL` 调用视觉服务：

```env
GOODSER_VISION_URL=http://127.0.0.1:8000
```

如果后端也跑在 Docker 同一网络：

```env
GOODSER_VISION_URL=http://goodser-vision:8000
```

验证：

```bash
curl -s http://127.0.0.1:29095/api/v1/health/modules
# 应包含 goodser-catalog
```

图鉴切割调用链：

```text
Web/Android
  -> POST /api/v1/goodser-catalog/catalog-images/:id/segment
  -> goodser-catalog 下载原图
  -> POST http://goodser-vision:8000/v1/segment
  -> 返回 boxes + crops(data URL)
  -> 上传裁剪图到 RustFS
  -> 写 zzz_goodser_catalog_items
```

> 上传原图需要 RustFS/S3；未启用时可先用 `image_url` 添加外链图片做联调。

---

## 6. GPU 共享建议（16GB）

| 服务 | 预算 |
|---|---:|
| 视觉服务（YOLO + CLIP + OCR） | 5–6GB |
| 视觉服务 VLM（可选 3B 4bit） | +3–4GB |
| embedding / TTS | 剩余 8GB |
| 显存余量 | ≥1GB |

策略：

1. `GOODSER_VISION_GPU_MEMORY_FRACTION=0.5`。
2. `GOODSER_VISION_MAX_CONCURRENCY=1`，排队而不是并发抢显存。
3. VLM 懒加载，空闲卸载。
4. 启动/推理记录 `nvidia-smi` 与 `/metrics`。
5. 压测 30 分钟显存不增长才上线。

---

## 7. 常见问题

| 现象 | 处理 |
|---|---|
| `no CUDA-capable device` | 检查 NVIDIA Container Toolkit / `--gpus all` |
| `/healthz` 正常但 `/v1/embed` 很慢 | 首次加载模型；看日志与 GPU 利用率 |
| `CUDA out of memory` | 降低并发、提高 headroom、关闭 VLM |
| 图鉴切割返回单框 | detector 仍是 fallback；确认 `GOODSER_VISION_DETECTOR=yolo` 且权重存在 |
| 上传原图失败 | RustFS/S3 未启用；先配置存储或用 `image_url` |
| 识别 Top-K 为空 | 商品/图鉴向量尚未回填；先跑索引重建 |
| 视觉服务重启后显存未释放 | 检查是否有旧容器占用 GPU；`nvidia-smi` |

---

## 8. 回滚

```bash
docker compose -f docker-compose.real.yml down
docker compose -f docker-compose.yml up -d   # fallback/CPU
```

Go 后端把 `GOODSER_VISION_URL` 指回 fallback 或直接停用图鉴切割即可；商品/图鉴数据不受影响。
