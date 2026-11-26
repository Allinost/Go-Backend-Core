# Goodser Vision

Goodser V3.3 的独立视觉容器项目，负责：

- 图片向量（商品图 / 图鉴柄图）
- 多商品检测与裁剪（谷子图片）
- 图鉴单图多柄图切割
- OCR（后续）
- 可选 VLM 复核（后续）

> 代码位于 `Go-Backend-Core/external/goodser-vision`，但独立构建镜像、独立部署。
> Go 后端只通过 HTTP 调用，不共享 Python 依赖。
>
> 真实模型（CLIP/YOLO/OCR）部署见 [REAL_SERVICE.md](./REAL_SERVICE.md)。

## 快速启动

```bash
cd external/goodser-vision
python -m venv .venv
# Windows
.venv\Scripts\activate
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8000
```

或 Docker：

```bash
docker compose up -d --build
```

默认使用轻量 fallback 向量（无需下载模型），便于先联调接口。
安装 CLIP / YOLO 依赖后，通过环境变量切换模型：

```env
GOODSER_VISION_EMBEDDER=clip        # fallback | clip
GOODSER_VISION_DETECTOR=yolo        # fallback | yolo
DEVICE=cpu                          # cpu | cuda
```

## API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查与模型状态 |
| GET | `/metrics` | Prometheus 文本指标 |
| GET | `/v1/models` | 模型名称/版本/维度/设备 |
| POST | `/v1/embed` | multipart 图片 → 向量 |
| POST | `/v1/embed/batch` | 多图向量 |
| POST | `/v1/analyze` | 检测 + 每框向量（候选由 Go 查库） |
| POST | `/v1/segment` | 图鉴单图切割，返回框与裁剪图 |
| POST | `/v1/ocr` | OCR（占位） |

## 显存预算（RTX 5060 Ti 16GB 共享）

| 组件 | 预算 |
|---|---:|
| YOLO11s | 1.0–1.8GB |
| Chinese-CLIP ViT-B/16 | 0.6–1.0GB |
| PaddleOCR | 1.0–1.5GB |
| VLM 3B 4bit（可选懒加载） | 3–4GB |
| CUDA/运行时 | 1.5–2GB |
| **视觉服务上限** | **≤ 8GB** |

- 并发 1–2，超出排队。
- VLM 懒加载，空闲 5–10 分钟卸载。
- FP16/INT8；启动记录显存占用。
- 与 embedding/TTS 共存时，必须保留至少 8GB。

## 目录

```text
external/goodser-vision/
├── app/
│   ├── main.py
│   ├── runtime.py          # 模型常驻、预热、空闲卸载、显存状态
│   ├── settings.py
│   ├── schemas.py
│   └── services/
│       ├── embedder.py
│       ├── detector.py
│       ├── matcher.py
│       ├── ocr.py
│       └── vlm.py
├── tests/
├── Dockerfile
├── docker-compose.yml
├── requirements.txt
└── Makefile
```

## 下一步

1. 接入 Chinese-CLIP ONNX / PyTorch。
2. 标注首批 100–300 张谷子平铺图，训练/微调 YOLO11s。
3. 图鉴切割：检测 → NMS → 裁剪 → 向量 → 写回 Go。
4. OCR 编码/名称强匹配。
5. VLM 低置信度复核。
