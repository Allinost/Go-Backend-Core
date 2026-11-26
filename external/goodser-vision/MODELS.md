# Goodser Vision 最终模型清单

> 版本：v1.1（2026-11-26）
> 适用：`external/goodser-vision` + New API VLM 主/备
> 硬件：RTX 5060 Ti 16GB（Blackwell，sm_120）
> 说明：下表“常驻显存”为 FP16/INT8、batch=1、图片最长边 1920 的**估算区间**；
> 当前本机未启动视觉容器，因此“当前实测”均为“未启动/待实测”，不是 0 占用的结论。

---

## 1. 最终模型清单

| 环节 | 模型 / 版本 | 参数 / 文件 | 输入 → 输出 | 用途 | 常驻显存（估算） | 当前状态 |
|---|---|---|---|---:|---|---|
| 目标检测 | **YOLO11s 谷子专用** | ~9.4M 参数；`models/yolo11s-goods.pt`（FP16 约 19MB） | 图片 → 检测框 + 置信度 | 找出图中每个谷子/柄图，图鉴切割与多商品识别 | **0.7–1.2GB** | 权重未提供；缺失时回退整图单框 |
| 图像向量 | **Chinese-CLIP ViT-B/16** | ~188M 参数；`OFA-Sys/chinese-clip-vit-base-patch16`；FP16 约 0.38GB | 图片 → 512 维向量 | 商品图/柄图相似召回 | **1.0–1.4GB**（仅 vision tower 0.6–0.8GB） | 代码已接；真实模式自动下载 |
| OCR | **PaddleOCR 中文** | PP-OCRv4/v5：det ~4.7M + rec ~10M + cls ~0.6M；权重约 15–60MB | 图片 → 文本 + 框 + 分数 | 编码/名称/包装文字强匹配 | **0.8–1.3GB（GPU）** / **0（CPU）** | 默认 GPU；Paddle GPU 不可用时自动回退 CPU |
| VLM 主 | **DS V4.1 Flash**（New API 渠道 A） | 外部 API，无本地权重 | 图片 + 提示词 + 候选 → 文本判断 | 低置信度复核、同系列角色判断 | **0（不占本机）** | 默认关闭；配置后可启用 |
| VLM 备 | **DS V4.1 Flash 或备用视觉模型**（New API 渠道 B） | 外部 API，无本地权重 | 同上 | 主 VLM 失败时自动切换 | **0（不占本机）** | 默认关闭；与主槽位独立配置 |
| 相似检索 | **pgvector HNSW / FAISS** | 512 维 float32；1 万条约 20MB 原始向量 | 向量 → Top-K | 商品/图鉴召回 | **0.1–0.5GB**（按 1–10 万条估算） | pgvector 表已建；识别 Top-K 待接 |
| 导出 | **Noto Sans SC** | 字体子集约 10–20MB | 文本 → 图片 | 平铺图中文名称/价格 | **0** | 待接入 Go 渲染器 |
| 联调兜底 | **16×16 RGB 网格向量** | 无权重 | 图片 → 512 维颜色特征 | 仅接口联调，不可用于生产 | 0（CPU） | 默认 fallback |

> VLM 不加载本地 7B/3B 权重；`goodser-vision` 只通过 New API 调用主/备槽位。
> 如果未来改成本地 vLLM：3B 4bit 额外 +3.5–4.5GB；7B 4bit 额外 +7–9GB。

---

## 2. 当前实际运行状态

当前开发机：

- `goodser-vision` 容器**未启动**
- 本机没有 Python/Docker 运行环境
- 因此没有“当前模型实测显存”
- 真实 GPU 主机启动后，用下面命令读取实测值

当前代码行为：

| 条件 | 行为 |
|---|---|
| `GOODSER_VISION_EMBEDDER=fallback` | 16×16 RGB 网格向量，不加载 CLIP |
| `GOODSER_VISION_EMBEDDER=clip` | 启动时加载 Chinese-CLIP 并常驻 |
| `GOODSER_VISION_DETECTOR=fallback` | 整图单框 |
| `GOODSER_VISION_DETECTOR=yolo` 且权重存在 | 加载 YOLO11s 并常驻 |
| `GOODSER_VISION_DETECTOR=yolo` 但权重缺失 | 回退整图单框 |
| `GOODSER_VISION_OCR_ENABLED=1` | 启动时加载 PaddleOCR 并常驻 |
| `GOODSER_VISION_OCR_DEVICE=gpu` | 先尝试 GPU；初始化失败自动回退 CPU |
| `GOODSER_VISION_VLM_ENABLED=0` | 不调用 VLM，`/v1/verify` 返回 503 |
| `GOODSER_VISION_VLM_ENABLED=1` | 按主/备顺序调用 New API |

---

## 3. 显存总账

### 3.1 不含 VLM（推荐常驻）

| 组件 | 预算 |
|---|---:|
| CUDA / PyTorch / Paddle 运行时 | 1.5–2.5GB |
| Chinese-CLIP | 1.0–1.4GB |
| YOLO11s | 0.7–1.2GB |
| PaddleOCR（GPU） | 0.8–1.3GB |
| workspace / 队列 / 碎片余量 | 0.5–1.0GB |
| **小计** | **4.5–7.4GB** |

建议：

- 按 **7–8GB** 预留
- `GOODSER_VISION_GPU_MEMORY_FRACTION=0.5`（16GB → 8GB）
- `GOODSER_VISION_GPU_HEADROOM_MB=1024`
- `GOODSER_VISION_MAX_CONCURRENCY=1–2`

### 3.2 OCR 用 CPU

| 变化 | 显存 |
|---|---:|
| 去掉 OCR GPU | **-0.8–1.3GB** |
| 总预算 | **约 3.7–6.1GB** |

CPU OCR 会慢一些，但省显存；适合和 TTS/embedding 紧共享。

### 3.3 加 VLM

| 方案 | 额外显存 | 总占用 |
|---|---:|---:|
| New API 外部 VLM（主/备） | **0** | 4.5–7.4GB |
| 本地 vLLM 3B 4bit | +3.5–4.5GB | 8–11.9GB |
| 本地 vLLM 7B 4bit | +7–9GB | 11.5–16.4GB（不推荐共享） |

### 3.4 加向量索引

| 数据量 | 512 维原始向量 | HNSW 估算 |
|---|---:|---:|
| 1 万条 | ~20MB | ~40–60MB |
| 5 万条 | ~100MB | ~200–300MB |
| 10 万条 | ~200MB | ~400–600MB |

> 实际以 `pg_relation_size()`、`pg_indexes_size()` 和 `nvidia-smi` 为准。

---

## 4. 软件版本

| 组件 | 版本 |
|---|---|
| CUDA | **12.8+** |
| PyTorch | **2.7.1** |
| 基础镜像 | `pytorch/pytorch:2.7.1-cuda12.8-cudnn9-runtime` |
| Python | 3.11（镜像内置） |
| Ultralytics | `>=8.3.40` |
| Transformers | `>=4.46.3` |
| PaddleOCR | `>=2.9.1` |
| PaddlePaddle | GPU：`paddlepaddle-gpu`；CPU 回退：`2.6.2` |
| pgvector | 0.8+（如启用） |

> RTX 5060 Ti 是 Blackwell（sm_120），必须 CUDA 12.8+ / PyTorch 2.7+。
> 旧镜像 `2.4.1-cuda12.1` 不支持，已替换。

---

## 5. New API VLM 主/备

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
GOODSER_VISION_VLM_MODE=auto          # auto | primary | backup
GOODSER_VISION_VLM_COOLDOWN_SECONDS=60
GOODSER_VISION_VLM_CONFIG_FILE=data/vlm.json
```

切换：

```bash
# 自动：主健康走主，主失败/冷却走备
curl -X POST http://127.0.0.1:8000/v1/system/vlm/switch \
  -H 'Content-Type: application/json' -d '{"slot":"auto"}'

# 强制主
curl -X POST http://127.0.0.1:8000/v1/system/vlm/switch \
  -H 'Content-Type: application/json' -d '{"slot":"primary"}'

# 强制备
curl -X POST http://127.0.0.1:8000/v1/system/vlm/switch \
  -H 'Content-Type: application/json' -d '{"slot":"backup"}'
```

规则：

- `auto`：主健康 → 主；主失败/超时 → 标记冷却 60s → 备
- `primary`：强制主，不自动切备（维护/测试用）
- `backup`：强制备
- 主备都失败 → `/v1/verify` 返回 503
- 不回退本地模型

---

## 6. 实测命令

在 GPU 主机启动容器后：

```bash
# 容器/进程显存
nvidia-smi

# 模型加载状态 + 显存快照
curl -s http://127.0.0.1:8000/v1/system/status

# Prometheus 指标
curl -s http://127.0.0.1:8000/metrics

# 模型清单与加载状态
curl -s http://127.0.0.1:8000/v1/models
```

`/v1/system/status` 会返回：

```json
{
  "models": {
    "embedder": {"loaded": true, "last_used": 0},
    "detector": {"loaded": true, "last_used": 0},
    "ocr": {"loaded": true, "device": "gpu"},
    "vlm": {
      "mode": "auto",
      "active_slot": "primary",
      "primary": {"healthy": true, "cooldown_remaining": 0},
      "backup": {"healthy": true, "cooldown_remaining": 0}
    }
  },
  "vram": {
    "used_bytes": 0,
    "free_bytes": 0,
    "total_bytes": 0,
    "allocated_bytes": 0,
    "reserved_bytes": 0
  }
}
```

> 实际数值以启动后的返回为准；上表的总账是预算，不是实测。

---

## 7. 不使用 / 不推荐

- 不在 `goodser-vision` 内加载 Qwen2.5-VL-7B/3B 本地权重
- 不把 opencode/commandcode 订阅 Token 直接写入代码
- 不使用 CUDA 12.1 / PyTorch 2.4 镜像
- 不把 fallback 16×16 网格向量当生产识别模型
- 不与 TTS/embedding 抢显存：VLM 用 New API 外部调用
