# 真实视觉服务部署手册（Docker Compose�?
> 适用范围：`external/goodser-vision` �?fallback 模式切换到真�?CLIP / YOLO / OCR�?> 目标：在 RTX 5060 Ti 16GB 上与 embedding / TTS 等服务共享显存，视觉服务预算 �?8GB�?> 代码目录：`Go-Backend-Core/external/goodser-vision`

---

## 1. 前置条件

| �?| 要求 |
|---|---|
| 宿主�?| Linux（推荐）�?Windows + WSL2 |
| GPU | NVIDIA，驱�?�?550，支�?CUDA 12.x |
| 容器 | Docker 24+，NVIDIA Container Toolkit |
| 显存 | 16GB 共享；视觉服�?�?8GB |
| 网络 | 仅内网；不把 8000 暴露公网 |
| 模型 | YOLO 检测权重、Chinese-CLIP、PaddleOCR（可选） |

验证 GPU�?
```bash
nvidia-smi
docker run --rm --gpus all nvidia/cuda:12.4.1-base-ubuntu22.04 nvidia-smi
```

---

## 2. 目录与模�?
```text
external/goodser-vision/
├── models/
�?  ├── yolo11s-goods.pt          # 谷子检测权重（首批 100�?00 张标注后训练�?�?  └── .gitkeep
├── hf-cache/                     # HuggingFace 缓存（Chinese-CLIP�?├── paddle-cache/                 # PaddleOCR 模型缓存
├── Dockerfile                    # fallback/CPU
├── Dockerfile.real               # CUDA + CLIP/YOLO/OCR
├── docker-compose.yml            # fallback/CPU
├── docker-compose.real.yml       # 真实服务/GPU
├── requirements.txt
└── requirements-real.txt
```

模型下载（示例）�?
```bash
# Chinese-CLIP 首次启动会自动下载到 hf-cache
# YOLO 权重放到 models/yolo11s-goods.pt
# PaddleOCR 首次启动会自动下载到 paddle-cache
```

> 模型权重不要提交 Git；`.gitignore` 已排�?`models/*`�?
---

## 3. 启动真实服务

```bash
cd external/goodser-vision
cp env.real.example env.real
# 按需修改模型路径、并发、显存比�?docker compose --env-file env.real -f docker-compose.real.yml up -d --build
```

查看状态：

```bash
docker compose -f docker-compose.real.yml ps
docker logs -f goodser-vision
curl -s http://127.0.0.1:8000/healthz
curl -s http://127.0.0.1:8000/v1/models
```

预期�?
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
| `GOODSER_VISION_EMBED_DIM` | `512` | 与数据库向量维度一�?|
| `GOODSER_VISION_MAX_CONCURRENCY` | `1` | 真实模型建议 1�? |
| `GOODSER_VISION_MAX_IMAGE_SIDE` | `1920` | 检测输入最长边 |
| `GOODSER_VISION_MODEL_DIR` | `/app/models` | YOLO 权重目录 |
| `HF_HOME` | `/app/hf-cache` | Chinese-CLIP 缓存 |
| `PADDLEOCR_HOME` | `/app/paddle-cache` | PaddleOCR 缓存 |
| `GOODSER_VISION_GPU_MEMORY_FRACTION` | `0.5` | 视觉服务显存上限比例�?6GB �?8GB�?|
| `GOODSER_VISION_GPU_HEADROOM_MB` | `1024` | 预留显存，低于阈值拒绝新请求 |

> 显存比例通过 `torch.cuda.set_per_process_memory_fraction` 限制；实际占用仍需压测�?> VLM 复核默认关闭；启用时单独加载，空�?5�?0 分钟卸载�?
---

## 5. 接入 Go 后端

后端通过 `GOODSER_VISION_URL` 调用视觉服务�?
```env
GOODSER_VISION_URL=http://127.0.0.1:8000
```

如果后端也跑�?Docker 同一网络�?
```env
GOODSER_VISION_URL=http://goodser-vision:8000
```

验证�?
```bash
curl -s http://127.0.0.1:29095/api/v1/health/modules
# 应包�?goodser-catalog
```

图鉴切割调用链：

```text
Web/Android
  -> POST /api/v1/goodser-catalog/catalog-images/:id/segment
  -> goodser-catalog 下载原图
  -> POST http://goodser-vision:8000/v1/segment
  -> 返回 boxes + crops(data URL)
  -> 上传裁剪图到 RustFS
  -> �?zzz_goodser_catalog_items
```

> 上传原图需�?RustFS/S3；未启用时可先用 `image_url` 添加外链图片做联调�?
---

## 6. GPU 共享建议�?6GB�?
| 服务 | 预算 |
|---|---:|
| 视觉服务（YOLO + CLIP + OCR�?| 5�?GB |
| 视觉服务 VLM（可�?3B 4bit�?| +3�?GB |
| embedding / TTS | 剩余 8GB |
| 显存余量 | �?GB |

策略�?
1. `GOODSER_VISION_GPU_MEMORY_FRACTION=0.5`�?2. `GOODSER_VISION_MAX_CONCURRENCY=1`，排队而不是并发抢显存�?3. VLM 懒加载，空闲卸载�?4. 启动/推理记录 `nvidia-smi` �?`/metrics`�?5. 压测 30 分钟显存不增长才上线�?
---

## 7. 常见问题

| 现象 | 处理 |
|---|---|
| `no CUDA-capable device` | 检�?NVIDIA Container Toolkit / `--gpus all` |
| `/healthz` 正常�?`/v1/embed` 很慢 | 首次加载模型；看日志�?GPU 利用�?|
| `CUDA out of memory` | 降低并发、提�?headroom、关�?VLM |
| 图鉴切割返回单框 | detector 仍是 fallback；确�?`GOODSER_VISION_DETECTOR=yolo` 且权重存�?|
| 上传原图失败 | RustFS/S3 未启用；先配置存储或�?`image_url` |
| 识别 Top-K 为空 | 商品/图鉴向量尚未回填；先跑索引重�?|
| 视觉服务重启后显存未释放 | 检查是否有旧容器占�?GPU；`nvidia-smi` |

---

## 8. 回滚

```bash
docker compose -f docker-compose.real.yml down
docker compose -f docker-compose.yml up -d   # fallback/CPU
```

Go 后端�?`GOODSER_VISION_URL` 指回 fallback 或直接停用图鉴切割即可；商品/图鉴数据不受影响�?