# Goodser Vision 最终模型名单

> 版本：v1.0（2026-11-26）
> 适用：`external/goodser-vision` + New API VLM 主/备
> 硬件：RTX 5060 Ti 16GB（Blackwell，sm_120）

---

## 1. 生产模型清单

| 环节 | 模型 | 版本/权重 | 运行方式 | 显存 |
|---|---|---|---|---:|
| 目标检测 | **YOLO11s 谷子专用** | `models/yolo11s-goods.pt`（自训练） | PyTorch FP16 / TensorRT | 0.7–1.2GB |
| 图像向量 | **Chinese-CLIP ViT-B/16** | `OFA-Sys/chinese-clip-vit-base-patch16`，512 维 | PyTorch FP16 | 1.0–1.4GB（仅 vision tower 0.6–0.8GB） |
| OCR | **PaddleOCR 中文** | PP-OCRv4/v5 det + rec + cls | Paddle Inference（建议 CPU） | GPU 0.8–1.3GB / CPU 0 |
| VLM 主 | **DS V4.1 Flash** | New API 渠道 A 暴露的模型 ID | New API 外部调用 | 0（不占本机） |
| VLM 备 | **DS V4.1 Flash 或备用视觉模型** | New API 渠道 B 暴露的模型 ID | New API 外部调用 | 0（不占本机） |
| 相似检索 | **pgvector HNSW / FAISS** | 无神经网络权重 | PostgreSQL / 视觉服务 | 0.2–0.5GB（索引） |
| 导出 | **Noto Sans SC** | 字体子集 | Go 渲染器 | 0 |

> VLM 不加载本地 7B 权重。`goodser-vision` 只通过 New API 调用主/备槽位。

---

## 2. 显存预算（常驻）

### 不含 VLM（推荐）

| 组件 | 预算 |
|---|---:|
| CUDA / PyTorch / Paddle 运行时 | 1.2–1.8GB |
| Chinese-CLIP | 1.0–1.4GB |
| YOLO11s | 0.7–1.2GB |
| PaddleOCR（GPU） | 0.8–1.3GB |
| workspace / 队列 / 碎片余量 | 0.5–1.0GB |
| **合计** | **4.5–6.5GB** |

- 建议按 **7GB** 预留
- `GOODSER_VISION_GPU_MEMORY_FRACTION=0.5`（16GB → 8GB）
- `GOODSER_VISION_GPU_HEADROOM_MB=1024`
- 并发 1–2

### 含 VLM

- VLM 走 New API：**不占本地显存**
- 只有网络延迟和调用成本
- 如果改用本地 vLLM：3B 4bit 额外 +3.5–4.5GB；7B 4bit 额外 +7–9GB（不推荐与 TTS/embedding 共享）

---

## 3. 软件版本

| 组件 | 版本 |
|---|---|
| CUDA | 12.8+ |
| PyTorch | **2.7.1**（Blackwell 最低要求） |
| 基础镜像 | `pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime` |
| Python | 3.11（镜像内置） |
| Ultralytics | 8.3.x |
| Transformers | 4.46.x |
| PaddleOCR | 2.9.x |
| PaddlePaddle | 2.6.x（CPU 推荐；GPU 需验证 sm_120） |
| pgvector | 0.8+（如使用） |

> RTX 5060 Ti 必须 CUDA 12.8+ / PyTorch 2.7+。旧的 2.4.1-cuda12.1 镜像不支持 sm_120。

---

## 4. New API VLM 配置

### 主槽位

```env
GOODSER_VISION_VLM_PRIMARY_ENABLED=1
GOODSER_VISION_VLM_PRIMARY_PROVIDER=newapi
GOODSER_VISION_VLM_PRIMARY_BASE_URL=http://new-api:3000/v1
GOODSER_VISION_VLM_PRIMARY_API_KEY=<New API Token A>
GOODSER_VISION_VLM_PRIMARY_MODEL=deepseek-v4.1-flash
```

### 备槽位

```env
GOODSER_VISION_VLM_BACKUP_ENABLED=1
GOODSER_VISION_VLM_BACKUP_PROVIDER=newapi
GOODSER_VISION_VLM_BACKUP_BASE_URL=http://new-api:3000/v1
GOODSER_VISION_VLM_BACKUP_API_KEY=<New API Token B>
GOODSER_VISION_VLM_BACKUP_MODEL=<备用视觉模型 ID>
```

### 全局

```env
GOODSER_VISION_VLM_ENABLED=1
GOODSER_VISION_VLM_CONFIG_FILE=data/vlm.json
```

规则：

- 主失败/超时/空结果 → 自动尝试备
- 主备都失败 → `/v1/verify` 返回 503
- 主备都关闭 → 不回退本地模型
- 配置持久化 `data/vlm.json`，密钥不回显

---

## 5. 模型文件放置

```text
external/goodser-vision/
├── models/
│   └── yolo11s-goods.pt
├── hf-cache/                 # Chinese-CLIP 自动下载
├── paddle-cache/             # PaddleOCR 自动下载
└── data/
    └── vlm.json              # New API 主/备配置（含密钥，勿提交）
```

---

## 6. 启动与验收

```bash
cd external/goodser-vision
cp env.real.example env.real
docker compose --env-file env.real -f docker-compose.real.yml up -d --build
```

验收：

```bash
curl -s http://127.0.0.1:8000/healthz
curl -s http://127.0.0.1:8000/v1/system/status
curl -s http://127.0.0.1:8000/v1/models
curl -X POST http://127.0.0.1:8000/v1/system/vlm/test
```

通过标准：

- `healthz.status=ok`
- `models` 显示 embedder/detector/ocr 的 loaded 状态
- `/v1/system/status` 显示 `vram.used/free/total`
- `/v1/system/vlm` 主/备 `configured=true`（如果启用）
- `/v1/segment` 返回多个 boxes（YOLO 权重就绪后）
- `/v1/verify` 返回 `slot=primary` 或 `backup`
- 连续压测 30 分钟显存不增长

---

## 7. 不使用的模型

- 不在 `goodser-vision` 内加载 Qwen2.5-VL-7B/3B 本地权重
- 不把 opencode / commandcode 订阅 Token 直接写入配置
- 不使用 CUDA 12.1 / PyTorch 2.4 镜像
- 不使用 fallback 网格向量作为生产识别模型（仅联调）
