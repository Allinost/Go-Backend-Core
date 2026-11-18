# Goodser 图片识别多商品与库存匹配（规划稿）

> 状态：规划中，待确认硬件、调用量与入库方式后进入 P0 验证。
> 后续：纳入 goodser V3 重构（V3.3），作为“目标识别 / 相似商品检查”需求。

## 0. 需求原文

> 根据拍照/上传的图片（图片中包含多个商品），识别并查找库中有的商品及数量，这个用免费的API或者本地部署GPU模型，帮我规划一下

## 1. 目标与边界

### 1.1 目标

1. 上传一张包含多个商品的照片，自动识别商品个数与位置。
2. 将每个商品与当前库存目录中的商品匹配，输出商品、数量、置信度。
3. 支持人工复核、修正数量与匹配结果。
4. 复核后一键生成入库单（或出库/盘点草稿，待确认）。
5. 优先本地 GPU 部署；无 GPU 时可退化到免费云端视觉 API。

### 1.2 非目标（首期）

- 不做视频流实时识别。
- 不自动确认库存变更，必须人工确认后落单。
- 不保证首次识别 100% 准确，按“候选 + 人工复核”设计。

## 2. 关键难点

- 商品是徽章、卡片、立牌等收藏品，通用检测模型不一定认识具体 SKU。
- 同系列不同角色外观接近，纯图像相似度容易混淆。
- 一张图可能有重复商品，需要检测框去重与同款聚类计数。
- 商品图片可能缺失、背景复杂、反光、遮挡。
- 免费云端 API 有速率/额度限制，且图片会离开本地。

## 3. 方案对比

| 方案 | 组成 | 优点 | 缺点 | 适用 |
|---|---|---|---|---|
| A. 免费云端 VLM | Gemini Flash 免费层 / GLM-4V-Flash / Qwen-VL 免费额度 | 无需 GPU、上线快 | 限流、隐私、框选与计数不稳定、目录大时上下文不足 | 先验证可行性 |
| B. 本地检测 + 向量检索 | YOLO11 / Grounding DINO + SAM2 + Chinese-CLIP/DINOv2 + PaddleOCR + FAISS | 隐私好、单次成本低、可离线 | 需要 GPU、需要调参与建索引 | 长期主力 |
| C. 混合（推荐） | 本地检测/OCR/向量召回 + 可选本地/云端 VLM 复核 | 准确率与成本平衡，可降级 | 组件较多，首期工程量最大 | 正式方案 |

**推荐：先做 C 的最小闭环，P0 用 A 做对照基线，最终以 B/C 为主。**

## 4. 推荐架构

```text
Web/小程序
  │ 上传图片
  ▼
Go goodser 模块
  ├─ 图片上传（复用 /legacy/uploadImage，得到公开 URL）
  ├─ 创建识别任务（recognition_jobs）
  ├─ 调用视觉服务 HTTP API
  └─ 复核后生成入库单
        │
        ▼
Python 视觉服务（goodser-vision）
  ├─ 目标检测：YOLO11 / Grounding DINO + SAM2
  ├─ 文本识别：PaddleOCR（商品名/编码/系列）
  ├─ 图像向量：Chinese-CLIP / DINOv2
  ├─ 商品索引：FAISS（按 inventory_id 分索引）
  ├─ 匹配打分：图像相似度 + 文本相似度 + 编码精确匹配
  └─ 可选 VLM 复核：Qwen2.5-VL / InternVL（本地）或免费云端
```

### 4.1 离线建索引

1. Go 侧导出目录商品快照：`id/name/code/series/character/type/image_url/images`。
2. 视觉服务下载商品图，计算图像向量；名称/编码/系列计算文本向量。
3. 按 `inventory_id` 建 FAISS 索引，保存商品元数据。
4. 商品新增、图片修改、目录切换后触发增量重建；提供全量重建接口。

### 4.2 在线识别

1. 图片上传后创建异步任务，状态 `pending → processing → succeeded/failed`。
2. 检测模型输出框；NMS 去重。
3. 每个框裁剪后：
   - 计算图像向量，FAISS 召回 Top-K；
   - OCR 识别编码/名称，命中编码时强加分；
   - 文本向量与商品名/系列/角色匹配；
   - 可选 VLM 对 Top-5 候选做最终判定。
4. 同一 `product_id` 的框聚合计数；低置信度保留候选列表。
5. 返回：框坐标、裁剪图、匹配商品、数量、置信度、Top-3 候选。

### 4.3 人工复核与落单

- Web 展示原图 + 框选 + 识别清单。
- 支持改商品、改数量、删除误识别、手动补商品。
- 确认后调用现有入库批量接口生成入库单；库存变更仍走现有事务与幂等逻辑。

## 5. 模型与硬件

| 能力 | 首选 | 备选 | 资源 |
|---|---|---|---|
| 目标检测 | YOLO11n/s（需少量标注） | Grounding DINO + SAM2 零样本 | 4–8GB GPU |
| 图像向量 | Chinese-CLIP ViT-B/16 | DINOv2 / SigLIP | 4–8GB GPU |
| OCR | PaddleOCR 中文模型 | 云端 OCR | CPU 可跑 |
| 向量检索 | FAISS | Qdrant/Milvus | 内存 |
| VLM 复核 | Qwen2.5-VL-7B 4bit | Gemini Flash / GLM-4V-Flash | 16GB GPU 或云端 |

- **8GB GPU**：检测 + CLIP + OCR + FAISS，可跑 MVP。
- **16GB GPU**：增加 7B VLM 4bit 复核。
- **无 GPU**：先用免费云端 VLM + 本地 OCR/文本检索验证；正式使用建议补 GPU 或购买云端推理额度。

## 6. 数据模型与接口

### 6.1 表

- `zzz_goodser_recognition_jobs`
  - `id`、`inventory_id`、`image_url`、`status`、`result JSON`、`error`、`created_by`、`created_at`、`finished_at`
- `zzz_goodser_recognition_items`（可选，便于复核与统计）
  - `job_id`、`box JSON`、`product_id`、`quantity`、`confidence`、`matched_by`、`alternatives JSON`

商品向量不落 MySQL，由视觉服务维护 FAISS 索引；MySQL 只存任务与复核结果。

### 6.2 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/zzz-goodser/inventories/:id/recognitions` | 创建识别任务，body 传 `image_url` 或 multipart |
| GET | `/zzz-goodser/recognitions/:rid` | 查询任务与识别结果 |
| POST | `/zzz-goodser/recognitions/:rid/apply` | 按复核结果生成入库单 |
| POST | `/zzz-goodser/vision/index/rebuild` | 重建目录商品索引（管理员） |

Go 侧配置：

```yaml
vision:
  provider: local          # local | gemini | glm | qwen
  base_url: "http://goodser-vision:8000"
  api_key: ""
  timeout_seconds: 60
  auto_apply_threshold: 0.90
  review_threshold: 0.60
```

## 7. 分阶段计划

### P0：可行性验证（1–2 天）

- 收集 30–50 张真实商品照片（平铺、叠放、弱光各若干）。
- 对比三条路线：
  1. 免费 VLM 零样本（Gemini Flash / GLM-4V-Flash）；
  2. 本地 Grounding DINO + SAM2 + CLIP 检索；
  3. 本地检测 + CLIP + OCR 混合。
- 指标：Top-1 / Top-3 命中率、数量误差、单张耗时、单张成本。
- 产出：选型结论与阈值建议。

### P1：MVP 闭环（3–5 天）

- Python 视觉服务：检测 + CLIP + FAISS + OCR。
- Go 识别任务表、创建/查询接口、索引重建接口。
- Web「拍照识别」入口：上传、轮询、框选预览、清单复核。
- 复核后生成入库单（不自动确认）。
- 验收：常见平铺照片 Top-3 ≥ 80%，数量误差 ≤ 1。

### P2：准确率与体验（2–4 天）

- 引入编码 OCR 强匹配、同款聚类、VLM 复核。
- 人工修正数据留档，形成再训练样本。
- 置信度分档：自动通过 / 建议复核 / 必须人工。
- 任务队列、超时重试、失败告警。

### P3：扩展（按需）

- 小程序拍照识别。
- 细粒度检测模型微调。
- 盘点模式（识别结果与现有库存对比）。
- 批量多图/视频抽帧。

## 8. 验收标准

- 上传图片后 10 秒内返回识别结果（本地 GPU）。
- 返回每个商品的框、匹配商品、数量、置信度、Top-3 候选。
- 人工可改商品与数量；确认后生成的入库单商品与数量正确。
- 低置信度不自动落库；库存变更仍由现有事务保证。
- 识别服务不可用时，任务失败并给出明确提示，不影响现有入库流程。

## 9. 风险与对策

| 风险 | 对策 |
|---|---|
| 商品图缺失/质量差 | 文本/编码匹配兜底，提示补图 |
| 同系列外观接近 | 图像 + OCR + 文本融合，Top-K 人工确认 |
| 免费 API 限流 | 本地模型优先；云端只做兜底 |
| GPU 资源不足 | 8GB 小模型 MVP；或云端推理 |
| 误识别导致库存错误 | 强制人工确认，不自动落库 |
| 图片隐私 | 本地部署优先，云端传输前告知并限制保留 |

## 10. 待确认

1. 服务器是否有 NVIDIA GPU，显存多少？
2. 预计每天识别多少张图片、每张大概多少个商品？
3. 识别结果只用于入库，还是也要支持出库/盘点？
4. 是否接受把商品照片发送到免费云端 API 做首轮验证？
