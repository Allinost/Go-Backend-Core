# Goodser V3 迁移与增强规划

> 版本：v0.1（规划稿）
> 日期：2026-11-26
> 分支建议：`v3/goodser-plan` → `feature-V3`
> 关联文档：
> - [V3 PostgreSQL 重构规划](./V3-postgres-refactor-2026-11-18.md)
> - [V3 SOP](../projects/go-backend-core/v3/SOP.md)
> - [Goodser 图片识别旧规划](./goodser-image-recognition-plan-2026-11-03.md)
> - [Goodser UI 优化](./goodser-ui-improvement.md)
>
> 目标：把 Goodser 从 V2（Rust + MySQL + 小程序 + Android）迁入 V3（Go + PostgreSQL + Web + Android），
> 并新增「Android 拍照/扫描识别」与「平铺图拼接导出」两项能力。

---

## 0. 一页纸结论

### 0.1 推荐路线

采用 **「现有 V3 Go 模块直接升级 + PG 化 + 数据迁移 + 按模块切路由」**，不重写业务逻辑：

1. **保留** `/api/v1/zzz-goodser` 与 `/legacy/*` 契约，Web / Android / 小程序可不改代码先切换。
2. **PG-only**：19 张 `zzz_goodser_*` 表迁到 PostgreSQL，运行时不再依赖 MySQL。
3. **独立开关**：新增 `modules.goodser.enabled`（或 `APP_MODULES_GOODSER_ENABLED`），不再借用 `LegacyModulesEnabled`。
4. **数据迁移**：V2 MySQL 快照 → `cmd/v3migrate` allowlist → PG，行数/主键/JSON/金额/图片 URL 全量校验。
5. **识别 MVP 先做单商品相似检索**：Android 拍照 → Top-K 相似商品 → 人工确认；多商品检测/计数作为第二阶段。
6. **平铺图导出走服务端渲染**：Web/Android 只提交商品与数量，Go 端合成 PNG/JPEG，保证两端一致。

### 0.2 关键判断

| 事项 | 结论 |
|---|---|
| V2 是否需要保留 | 保留运行，V3 切换稳定 7 天后再停 V2 Goodser |
| V3 数据库 | PostgreSQL 16 + `pgvector`（不可用时退 FAISS） |
| 路由前缀 | 不改，继续 `/api/v1/zzz-goodser` |
| Web 前端 | 已有 V3 Goodser 页面，切换 API 层即可，不重做 |
| Android | 升级现有 Compose App，保留 legacy 调用兼容 |
| 小程序 | 保留 legacy/REST 兼容，不强制首期改版 |
| 识别服务 | 独立 Docker `goodser-vision`，本地部署，默认 CPU |
| 平铺导出 | 服务端 Go 渲染，异步任务，结果进 RustFS |
| 迁移工期 | 核心迁移约 13 人日；识别 + 导出约 13 人日；建议 5–7 周并行完成 |

### 0.3 与 V3.0 的关系

V3.0 规划明确「业务模块暂不迁移，V3.3 再做 Goodser」。因此：

- **可以现在启动**：需求冻结、数据映射、迁移脚本、视觉服务 POC、Android 相机原型，这些不阻塞 V3.0。
- **生产切路由建议在 V3.0 稳定后**（12.15 之后），避免与公网/反代/认证上线叠加风险。
- 如业务上必须提前使用 Goodser，可临时打开 V2 MySQL 模块，但需明确这是过渡态，不作为 V3 最终形态。

---

## 1. 现状盘点

### 1.1 已有资产

| 端 | 现状 | 位置 |
|---|---|---|
| V2 后端 | Rust / Axum / MySQL / RustFS，action 风格 API | `Goodser/backend` |
| V2 小程序 | 微信小程序，支持云开发 / Go 后端两种模式 | `Goodser/miniprogram` |
| V2 Android | Kotlin + Jetpack Compose，Retrofit | `Goodser/Goodser/android` |
| V3 后端 | Go 模块，19 张表、REST + legacy 共 91 个路由 | `zzz/goodser` |
| V3 Web | 已拆分为 10 个 Tab 的 Goodser 页面 | `Go-Frontend-Core/src/views/modules/zzz/goodser` |
| V3 迁移工具 | `cmd/v3migrate` 支持 allowlist/dump/load/verify | `cmd/v3migrate` |
| 图片识别旧规划 | 多商品检测 + CLIP + FAISS + OCR 方案 | `docs/plans/goodser-image-recognition-plan-2026-11-03.md` |

### 1.2 关键阻塞

1. **V3 当前不注册 Goodser**：`cmd/server/main.go` 只有 `cfg.LegacyModulesEnabled=true` 才注册 `zzz/goodser`；V3 新配置默认 `false`，因此 Web/Android 调 `/zzz-goodser/...` 返回 404。
2. **V3 Goodser 仍连 MySQL**：`zzz/goodser/module.go` 使用 `database.DB.MySQL["goodser"]`，与 V3 PG-only 冲突。
3. **V3 数据模型没有 owner 字段**：V2 `inventories/products/tags/status_codes` 有 `owner_openid`；V3 Go 模型未保留，存在多用户数据归属差异。
4. **客户端契约不统一**：Web 基本走 `/legacy/*`；Android 走 `/legacy/*` + V3 `auth/login`；小程序混合 REST + legacy。
5. **图片 URL 历史问题已修**：`image_url.go` 会把预签名 URL 归一为公开 URL，迁移时沿用即可。

### 1.3 数据表清单（19 张）

| 分组 | 表 |
|---|---|
| 基础（8） | `zzz_goodser_inventories`、`products`、`recycled_seq_numbers`、`seq_counters`、`outbound_orders`、`inbound_logs`、`status_codes`、`tags` |
| P0（2） | `stock_movements`、`returns` |
| 维度/组合（4） | `dictionaries`、`product_characters`、`combos`、`combo_items` |
| 捆（3） | `bundles`、`bundle_main_products`、`bundle_sub_products` |
| 捆物池（2） | `bundle_pools`、`bundle_pool_items` |

新增功能表（规划）：

- `zzz_goodser_product_embeddings`
- `zzz_goodser_recognition_jobs`
- `zzz_goodser_recognition_items`
- `zzz_goodser_recognition_feedback`（可选）
- `zzz_goodser_export_jobs`
- `zzz_goodser_export_templates`

---

## 2. 范围与目标

### 2.1 目标

1. Goodser 在 V3 新机器上 **PG-only** 运行，不连接 MySQL。
2. 现有 Web / Android / 小程序功能不回归；legacy 契约保持。
3. 完成 V2 数据迁移与回滚演练。
4. Android 增加拍照/相册扫描，识别「是否与库中商品相似」。
5. Web + Android 增加平铺图拼接导出（商品图 + 数量）。
6. 所有新能力遵守 V3 SOP：迁移可回滚、PG 集成测试、权限/审计、文档与台账同步。

### 2.2 非目标

- 不做视频流实时识别。
- 不做自动改库存：识别结果必须人工确认后才入库/出库。
- 不重写库存编码、预留、退货等核心业务语义。
- 不在 V2 上新增功能。
- 不追求首期 100% 识别准确率，按「候选 + 人工复核」设计。
- 不做多租户完整方案（首期单 owner，schema 预留归属字段）。

---

## 3. 目标架构

```text
┌──────────────────────────────┐
│ Web / Android / 微信小程序    │
└──────────────┬───────────────┘
               │ /api/v1/zzz-goodser/*
               ▼
┌──────────────────────────────┐        ┌─────────────────────────┐
│ V3 Go Goodser 模块            │───────▶│ RustFS / S3（商品图）    │
│ - REST + legacy 兼容          │        └─────────────────────────┘
│ - 识别任务 / 导出任务          │
│ - 权限 / 审计 / 幂等           │        ┌─────────────────────────┐
└───────┬──────────────┬───────┘───────▶│ goodser-vision（Docker） │
        │              │                 │ FastAPI + ONNX          │
        ▼              ▼                 │ CLIP / OCR / 检测        │
┌──────────────┐  ┌──────────────┐      └─────────────────────────┘
│ PostgreSQL   │  │ 导出渲染器    │
│ 19 业务表     │  │ Go image/gg  │
│ + pgvector   │  │ 异步 worker  │
└──────────────┘  └──────────────┘
```

### 3.1 服务边界

| 服务 | 职责 | 部署 |
|---|---|---|
| V3 Go Goodser | 业务 CRUD、权限、任务编排、导出、相似检索查询 | V3 主进程 |
| `goodser-vision` | 图片向量、目标检测、OCR（可选） | 独立 Docker，仅内网 |
| PostgreSQL | 业务数据 + 向量 + 任务 | V3 主库 |
| RustFS/S3 | 原图、裁剪图、导出图 | 现有对象存储 |
| 导出 worker | 图片下载、排版、合成、上传 | Go 协程池（首期进程内） |

---

## 4. V2 → V3 迁移方案

### 4.1 迁移原则

1. **契约优先**：`/api/v1/zzz-goodser` 不变；新增端点只增不改。
2. **按模块整体切换**：不做同域随机分流，不做双写。
3. **先结构后数据**：PG 迁移先建表，再灌数据，再切路由。
4. **可重复执行**：迁移脚本清空目标表后可重跑。
5. **可回滚**：V2 快照保留 7 天；路由切回 V2 不超过 30 分钟。
6. **PG 方言**：`$n`、`jsonb`、`boolean`、`timestamptz`、`numeric`；生产禁用 AutoMigrate。

### 4.2 阶段 A：冻结与盘点

- [ ] 冻结 V2 Goodser 写入（维护窗口）。
- [ ] 备份 V2 MySQL：结构 + 数据 + 行数 + checksum。
- [ ] 导出 V2 商品图片清单（RustFS key / URL），确认无缺失。
- [ ] 确认 V2 使用方：Web、Android、小程序、API Key/第三方调用。
- [ ] 记录所有 legacy action 调用量（按 action 统计），确定兼容优先级。

**产出**：`docs/migrations/goodser-v2-baseline.md`（行数、checksum、表清单、调用量）。

### 4.3 阶段 B：PG Schema

新增 `migrations/postgres/008_goodser.sql`（可拆为 008–012），包含：

1. 19 张业务表，表名保持 `zzz_goodser_*`。
2. 类型映射：

| MySQL | PostgreSQL |
|---|---|
| `VARCHAR(n)` | `varchar(n)` |
| `CHAR(1)` | `char(1)` |
| `INT` / `BIGINT AUTO_INCREMENT` | `integer` / `bigint generated by default as identity` |
| `DECIMAL(p,s)` | `numeric(p,s)` |
| `JSON` | `jsonb` |
| `TINYINT(1)` / `BOOLEAN` | `boolean` |
| `DATETIME` / `TIMESTAMP` | `timestamptz` |
| `ENUM(...)` | `varchar + CHECK`（或保留 varchar，应用层校验） |
| `ON UPDATE CURRENT_TIMESTAMP` | 统一 `updated_at` 触发器 |

3. 主键/外键/唯一索引/查询索引按 V2 语义补齐；软删除按需。
4. 金额 `numeric`，禁止 float。
5. 时间统一 UTC。
6. 新增 `owner_user_id bigint null`（预留；首期可全部回填管理员）。
7. 向量扩展：

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

8. 新增识别/导出表（见 §5.3、§6.4）。
9. 表注释 + 数据字典登记。

**验收**：空库一键建表；`go test -tags=integration` 通过；表数、索引、外键与设计一致。

### 4.4 阶段 C：Repository PG 化

改造 `zzz/goodser`：

- `module.go`：`database.DB.Postgres["main"]` 替代 `MySQL["goodser"]`。
- `repository*.go`：`?` → `$n`（优先 `dbutil.Rebind`，复杂 SQL 手工改）。
- UPSERT：`ON DUPLICATE KEY UPDATE` → `ON CONFLICT (...) DO UPDATE SET ... EXCLUDED`。
- `DATABASE()` / `information_schema` / `SHOW` 如有，改 `current_schema()` / `pg_catalog`。
- `NOW()` → `now()`；布尔比较改 `= true/false`。
- 事务与锁保持；`SELECT ... FOR UPDATE` 语义不变。
- 分页统一 `LIMIT $n OFFSET $m`，稳定排序补 `id`。

**测试**：

- 每个 Repository 至少 1 个 PG 集成测试（testutil 临时 schema）。
- 核心链路：入库 → 库存流水 → 出库 → 预留 → 退货 → 统计。
- 迁移重跑幂等测试。

### 4.5 阶段 D：数据迁移

扩展 `cmd/v3migrate`：

1. allowlist 增加 19 张 Goodser 表（V2 `goodser.*` → V3 `zzz_goodser_*`）。
2. 字段转换：
   - `ENUM` 保持字符串；非法值记录并 fail。
   - `JSON` 校验后写 `jsonb`。
   - `DATETIME` 按 UTC 解析；空值保留 `NULL`。
   - `TINYINT(1)` → boolean。
   - `AUTO_INCREMENT` 迁移后重置 identity 序列到 `max(id)`。
3. 归属映射：
   - 方案 A（推荐首期）：`owner_user_id` 统一回填管理员；V3 单 owner。
   - 方案 B：`owner_openid` → V3 `users`/社交账号映射；无法映射的记录进人工清单。
4. 图片：URL 归一化（去签名参数）；不重复上传；保留 key。
5. 增量（如需）：按 `updated_at` 拉取窗口内变更；切换前做最后一次短增量。
6. 校验：
   - 行数一致；主键无重复；FK 无孤儿。
   - JSON 合法；金额 `SUM/MIN/MAX` 一致。
   - 时间抽样一致；图片 URL 可访问率 ≥ 99%。
   - 抽 20 条商品/订单/退货人工对照。

**产出**：迁移报告（前后行数、checksum、耗时、失败清单、回滚步骤）。

### 4.6 阶段 E：模块启用与路由切换

1. 新增独立开关：
   ```yaml
   modules:
     goodser:
       enabled: true
   ```
   或 `.env`：`APP_MODULES_GOODSER_ENABLED=true`。
2. `cmd/server/main.go`：
   ```go
   if cfg.Modules.Goodser.Enabled {
       modules.Register(&goodser.Module{})
   }
   ```
3. 健康检查暴露 `goodser: pg`、`goodser: rustfs`、`goodser: vision`（可选）。
4. 前端/Android/小程序继续用原地址；V2 老机器保留只读回退开关。
5. 观察 7 天：错误率、p95、写失败、库存一致性、图片 404。

### 4.7 客户端兼容策略

| 客户端 | 首期 | 后续 |
|---|---|---|
| Web V3 | 保持 legacy 调用，补 REST 适配层；逐步切 REST | 全量 REST，移除 legacy 依赖 |
| Android | 保持 legacy 调用；新增识别/导出用新 REST | 商品/订单迁移到 REST |
| 小程序 | 保持 legacy + REST 混合 | 统一 REST，保留 legacy 兜底 |
| 第三方 API Key | 保持 `/legacy/*` action | 提供 REST 文档与新 Key scope |

> 原则：**迁移期不改客户端也能工作**；新功能只加新端点。

---

## 5. 新功能一：Android 拍照/扫描识别

### 5.1 用户目标

- 拿起手机拍一件谷子，立即判断「库里是否已有同款/相似款」。
- 有相似款：显示 Top-K 候选、库存、编码，可直接查看/入库。
- 没有相似款：一键新建商品，自动带入照片。
- 第二阶段：一张图多个谷子，自动框选、计数、批量入库。

### 5.2 分阶段范围

| 阶段 | 能力 | 说明 |
|---|---|---|
| R1（MVP） | 单图单商品相似检索 | 拍照/相册 → Top-K → 人工确认 |
| R2 | 多商品检测 + 计数 | 检测框、裁剪、同款聚类、数量 |
| R3 | OCR 强匹配 + VLM 复核 | 编码/名称识别，低置信度复核 |
| R4 | 盘点/出库场景 | 识别结果与库存对比 |

> 首期只做 R1，避免检测模型标注与调参拖慢上线。

### 5.3 数据模型

```sql
-- 商品图片向量
CREATE TABLE zzz_goodser_product_embeddings (
  id            bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  product_id    varchar(36) NOT NULL,
  inventory_id  varchar(36) NOT NULL,
  image_url     varchar(1024) NOT NULL,
  model         varchar(64) NOT NULL,
  dim           integer NOT NULL,
  embedding     vector(512) NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (product_id, image_url, model)
);
CREATE INDEX idx_goodser_embedding_hnsw
  ON zzz_goodser_product_embeddings
  USING hnsw (embedding vector_cosine_ops);

-- 识别任务
CREATE TABLE zzz_goodser_recognition_jobs (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36) NOT NULL,
  image_url     varchar(1024) NOT NULL,
  mode          varchar(20) NOT NULL DEFAULT 'similarity', -- similarity | multi
  status        varchar(20) NOT NULL DEFAULT 'pending',
  result        jsonb,
  error         text,
  created_by    bigint,
  created_at    timestamptz NOT NULL DEFAULT now(),
  finished_at   timestamptz
);

-- 识别项（多商品阶段使用；单商品也写一条便于复核）
CREATE TABLE zzz_goodser_recognition_items (
  id            bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  job_id        varchar(36) NOT NULL,
  box           jsonb,
  crop_url      varchar(1024),
  product_id    varchar(36),
  quantity      integer NOT NULL DEFAULT 1,
  confidence    numeric(5,4) NOT NULL DEFAULT 0,
  matched_by    varchar(20), -- image | text | code | manual
  alternatives  jsonb,
  confirmed     boolean NOT NULL DEFAULT false
);

-- 人工反馈（可选，用于评估/再训练）
CREATE TABLE zzz_goodser_recognition_feedback (
  id            bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  job_id        varchar(36) NOT NULL,
  item_id       bigint,
  predicted_id  varchar(36),
  actual_id     varchar(36),
  rating        smallint,
  note          text,
  created_by    bigint,
  created_at    timestamptz NOT NULL DEFAULT now()
);
```

> `pgvector` 不可用时：向量改存 `bytea`，由 `goodser-vision` 维护 FAISS 索引；接口不变，内部切换。

### 5.4 视觉服务 `goodser-vision`

**技术栈**：Python 3.11 + FastAPI + ONNX Runtime；CPU 默认，GPU 可选。

**首期模型**：

| 能力 | 模型 | 备注 |
|---|---|---|
| 图像向量 | Chinese-CLIP ViT-B/16 | 512 维，中文谷子/包装友好 |
| 备用向量 | SigLIP / DINOv2 | 对比评估 |
| OCR（R3） | PaddleOCR 中文 | 编码/名称 |
| 检测（R2） | YOLO11n/s 或 Grounding DINO + SAM2 | 需少量标注/零样本 |
| VLM 复核（R3） | Qwen2.5-VL-7B 4bit | 16GB GPU，可选 |

**API**：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查 |
| POST | `/v1/embed` | multipart 图片 → `{model, dim, vector}` |
| POST | `/v1/embed/batch` | URL/文件批量 → 向量 |
| POST | `/v1/detect` | 多商品检测（R2） |
| POST | `/v1/ocr` | OCR（R3） |
| GET | `/v1/models` | 模型与版本信息 |

**部署**：

- `deploy/goodser-vision/Dockerfile`：多阶段，模型文件挂载卷。
- `docker-compose.goodser.yml`：仅内网网络；`DEVICE=cpu|cuda`；healthcheck。
- 资源建议：CPU 8C/8G 可跑单图；多商品/检测建议 8–16G GPU。
- 模型缓存目录持久化，避免每次启动下载。
- 不暴露公网；仅 V3 后端可访问。

### 5.5 Go 侧集成

1. 新增 `zzz/goodser/vision_client.go`：
   - 超时、重试、熔断、并发限制。
   - 配置：`goodser.vision.base_url`、`timeout_seconds`、`top_k`、阈值。
2. 商品图片保存/更新后：
   - 写 outbox 或 scheduler 任务 → 调 `/v1/embed` → upsert embedding。
   - 失败可重试；不影响商品保存主流程。
3. 全量重建：
   - `POST /zzz-goodser/vision/index/rebuild`（管理员）
   - 或命令 `goodser embeddings rebuild --inventory <id>`。
4. 扫描接口：
   - `POST /zzz-goodser/inventories/:id/recognitions`（multipart 或 `image_url`）
   - `GET /zzz-goodser/recognitions/:rid`
   - `POST /zzz-goodser/recognitions/:rid/apply`（人工复核后落单）
5. 相似查询 SQL：
   ```sql
   SELECT p.*, 1 - (e.embedding <=> $1) AS score
   FROM zzz_goodser_product_embeddings e
   JOIN zzz_goodser_products p ON p.id = e.product_id
   WHERE e.inventory_id = $2
   ORDER BY e.embedding <=> $1
   LIMIT $3;
   ```

### 5.6 阈值与复核

| 分数 | 建议 |
|---|---|
| ≥ 0.90 | 高置信：默认选中，仍允许修改 |
| 0.75–0.90 | 候选：人工确认 |
| < 0.75 | 视为新商品；可手动搜索 |

- 阈值上线后用真实数据校准。
- 编码 OCR 命中时直接强匹配（R3）。
- 任何识别都不自动改库存。

### 5.7 Android 实现

1. 依赖：
   - CameraX（`camera-core/camera-camera2/camera-lifecycle/camera-view`）
   - `androidx.activity:activity-compose`（已有）
   - 可选 ML Kit（条码/文字）后续
2. 权限：
   - `CAMERA`
   - Android 13+ 相册用 Photo Picker，不再申请存储权限
3. 页面：
   - `ScanScreen`：取景框、闪光灯、相册入口、拍照。
   - `ScanResultScreen`：原图 + Top-3 卡片（商品图、名称、编码、库存、相似度）。
   - 操作：查看商品 / 快速入库 / 新建商品（预填图片）/ 重新拍摄。
4. 上传：
   - 客户端压缩到最长边 1280、JPEG 85%，减少延迟。
   - 上传到 V3 `recognitions` 接口；轮询或同步返回。
5. 离线/失败：
   - 失败提示重试；可选本地队列（后续）。
6. 兼容：
   - 保留现有 legacy API；新功能只加新接口。
   - 服务器地址仍由 `ServerConfigScreen` 配置。

### 5.8 验收标准（R1）

- 50 张真实照片测试集，Top-3 命中率 ≥ 80%。
- 单张识别 p95：CPU ≤ 3s，GPU ≤ 1s（不含网络）。
- 返回 Top-K、相似度、商品信息；人工可改选。
- 低置信度不自动落库。
- 服务不可用时有明确提示，不影响原有入库流程。
- 图片不出内网（默认本地部署）。

---

## 6. 新功能二：平铺图拼接导出

### 6.1 用户目标

- 选中若干谷子 + 各自数量 → 生成一张「商品图 + ×数量」的平铺图。
- 用于发货核对、打包清单、闲鱼/社群分享、盘点留档。
- Web 可下载，Android 可保存/分享。

### 6.2 首期范围

| 能力 | 首期 | 后续 |
|---|---|---|
| 选择商品与数量 | ✅ | 按系列/标签批量选择 |
| 网格排版 | ✅ 3/4/5 列 | 自定义模板/自由拖拽 |
| 商品图 + 数量徽标 | ✅ | 名称/编码/价格可选 |
| 背景 | 透明/白/深色 | 自定义背景图 |
| 输出 | PNG/JPEG | PDF/长图/多页 |
| 历史记录 | ✅ 最近导出 | 收藏模板/批量导出 |
| 异步任务 | >30 件 | 队列/优先级 |

### 6.3 渲染方案

**服务端 Go 渲染**（推荐）：

- `image/draw` + `github.com/fogleman/gg` + `golang.org/x/image/font/opentype`。
- 内嵌 `NotoSansSC` 子集字体（支持中文名称/编码）。
- 图片解码：JPEG/PNG/WebP（`golang.org/x/image/webp`）。
- 圆角/阴影：mask + 简单 alpha 合成。
- 并发下载商品图（worker pool 8），失败用占位图。
- 输出 PNG（支持透明）/ JPEG（quality 90）。

**布局算法**：

1. 目标宽度 2048（可配 1024/2048/3072）。
2. 列数 `cols`（3/4/5），`rows = ceil(n/cols)`。
3. 计算 cell 宽高、间距、页边距、标题/页脚高度。
4. 每个 cell：
   - 商品图 contain/cover，圆角裁切。
   - 右下角数量徽标 `×N`（对比色）。
   - 可选名称/编码（最多 2 行，超出省略）。
5. 标题：库存目录名 + 日期 + 总件数/总种数。
6. 页脚：备注/生成时间（可选）。

**性能**：

- 30 件以内同步返回（目标 ≤ 3s）。
- 30–200 件异步任务（目标 ≤ 10s）。
- 单次最多 200 件；输出最长边 ≤ 4096。
- 图片下载与渲染内存控制；临时文件及时清理。

### 6.4 数据模型

```sql
CREATE TABLE zzz_goodser_export_jobs (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36) NOT NULL,
  status        varchar(20) NOT NULL DEFAULT 'pending',
  request       jsonb NOT NULL,
  result_url    varchar(1024),
  error         text,
  created_by    bigint,
  created_at    timestamptz NOT NULL DEFAULT now(),
  finished_at   timestamptz,
  expires_at    timestamptz
);

CREATE TABLE zzz_goodser_export_templates (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36),
  name          varchar(100) NOT NULL,
  config        jsonb NOT NULL,
  is_default    boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
```

`request` 示例：

```json
{
  "items": [{"product_id": "p1", "quantity": 2}],
  "columns": 4,
  "background": "transparent",
  "format": "png",
  "show_name": true,
  "show_code": false,
  "title": "2026-11-26 发货清单"
}
```

### 6.5 API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/zzz-goodser/inventories/:id/exports` | 创建导出（≤30 同步，>30 返回 202 + job） |
| GET | `/zzz-goodser/exports/:eid` | 查询任务/结果 |
| GET | `/zzz-goodser/inventories/:id/exports` | 导出历史 |
| DELETE | `/zzz-goodser/exports/:eid` | 删除记录（可选删文件） |
| GET | `/zzz-goodser/export-templates` | 模板列表 |
| POST | `/zzz-goodser/export-templates` | 新建模板 |
| PUT | `/zzz-goodser/export-templates/:tid` | 更新 |
| DELETE | `/zzz-goodser/export-templates/:tid` | 删除 |

权限：`goodser.export`（owner read/write）；限流与文件大小限制。

### 6.6 Web UI

- 商品列表增加多选 + 「生成平铺图」按钮。
- 导出面板：
  - 已选商品列表，可改数量/移除。
  - 列数、背景、是否显示名称/编码、格式。
  - 预览缩略图（可选，异步生成）。
  - 生成/下载/复制链接。
- 历史记录抽屉：最近 20 条，重新下载/删除。

### 6.7 Android UI

- 商品列表长按多选 → 底部操作栏「平铺图」。
- 数量调整页 → 生成 → 结果页显示图片 → 保存到相册 / 系统分享。
- 复用服务端渲染，不在客户端重复实现排版。

### 6.8 验收标准

- 30 件导出 ≤ 3s；100 件异步 ≤ 10s。
- 数量与选择一致；商品图缺失有占位。
- 中文名称/编码正常渲染（无方块）。
- PNG 透明背景可用；JPEG 体积合理。
- Web 下载、Android 保存/分享成功。
- 失败任务有明确错误与重试。

---

## 7. 数据模型与接口汇总

### 7.1 新增表

| 表 | 用途 |
|---|---|
| `zzz_goodser_product_embeddings` | 商品图片向量（pgvector） |
| `zzz_goodser_recognition_jobs` | 识别任务 |
| `zzz_goodser_recognition_items` | 识别项/复核结果 |
| `zzz_goodser_recognition_feedback` | 人工反馈（可选） |
| `zzz_goodser_export_jobs` | 导出任务 |
| `zzz_goodser_export_templates` | 导出模板 |

### 7.2 新增配置

```yaml
modules:
  goodser:
    enabled: true

goodser:
  vision:
    base_url: "http://goodser-vision:8000"
    timeout_seconds: 30
    top_k: 5
    auto_match_threshold: 0.90
    review_threshold: 0.75
    max_image_mb: 10
  export:
    max_items: 200
    max_width: 3072
    font_path: "/app/assets/fonts/NotoSansSC-Regular.otf"
    temp_dir: "/tmp/goodser-export"
```

### 7.3 新增权限点

| 权限 | 说明 |
|---|---|
| `goodser.recognition.create` | 创建识别任务 |
| `goodser.recognition.read` | 查询识别结果 |
| `goodser.recognition.apply` | 复核后落单 |
| `goodser.vision.rebuild` | 重建向量索引（管理员） |
| `goodser.export.create` | 创建导出 |
| `goodser.export.read` | 查看/下载导出 |
| `goodser.export.manage` | 删除/模板管理 |

---

## 8. 里程碑与任务拆分

> 任务编号建议沿用 V3 台账风格：`V3.3-GDS-xx`。
> 人日为 AI + 人工评审的净开发估算，不含等待窗口。

### 8.1 里程碑

| 里程碑 | 内容 | 预估 | 依赖 |
|---|---|---:|---|
| M0 规划冻结 | 需求、DB、识别范围、硬件、导出样式确认 | 1d | — |
| M1 基线 | V2 快照、行数、调用量、映射表 | 2d | M0 |
| M2 PG Schema | 19 表 + 6 新表 + pgvector + 字典 | 4d | M1 |
| M3 Repository | PG 方言、UPSERT、事务、集成测试 | 5d | M2 |
| M4 数据迁移 | allowlist、转换、校验、演练、回滚 | 4d | M2 |
| M5 模块启用 | 独立开关、健康检查、路由切换、Web 冒烟 | 2d | M3、M4 |
| M6 视觉服务 | Docker + embed API + 索引/回填 | 5d | M1（可并行） |
| M7 识别 API | 任务、相似查询、复核、权限 | 3d | M5、M6 |
| M8 Android 扫描 | CameraX、上传、结果页、操作 | 4d | M7 |
| M9 平铺导出 | 渲染器、API、模板、Web/Android | 4d | M5 |
| M10 多商品识别 | 检测、裁剪、聚类、计数（可选） | 5d | M8 |
| M11 验收观察 | PG 测试、UAT、文档、7 天观察 | 2d | M5、M8、M9 |

**核心迁移（M0–M5）约 18 人日**（含测试/演练；与 README 的 8–10d 代码迁移相比更保守）。
**新增功能（M6–M9）约 16 人日**；多商品 +5d。
建议排期 5–7 周（并行视觉服务与 Android）。

### 8.2 任务清单

#### 迁移

| 编号 | 任务 | 预估 |
|---|---|---:|
| V3.3-GDS-01 | 冻结 V2 Goodser 写入与快照 | 1d |
| V3.3-GDS-02 | 19 表字段/索引/外键盘点与映射表 | 1d |
| V3.3-GDS-03 | PG 迁移 008：基础 8 表 | 1d |
| V3.3-GDS-04 | PG 迁移 009：P0/维度/组合/捆/池 11 表 | 1d |
| V3.3-GDS-05 | pgvector + 识别/导出 6 表 | 1d |
| V3.3-GDS-06 | Repository PG 化（19 表） | 3d |
| V3.3-GDS-07 | 核心链路 PG 集成测试 | 1d |
| V3.3-GDS-08 | v3migrate allowlist + 类型转换 + 序列重置 | 2d |
| V3.3-GDS-09 | 迁移校验器（行数/FK/JSON/金额/抽样） | 1d |
| V3.3-GDS-10 | 迁移演练 + 回滚 Runbook | 1d |
| V3.3-GDS-11 | 独立模块开关 + 健康检查 | 0.5d |
| V3.3-GDS-12 | 路由切换 + Web/Android/小程序冒烟 | 1.5d |

#### 识别

| 编号 | 任务 | 预估 |
|---|---|---:|
| V3.3-GDS-20 | `goodser-vision` Docker + FastAPI + CLIP embed | 2d |
| V3.3-GDS-21 | 向量表、回填任务、商品图变更触发 | 1.5d |
| V3.3-GDS-22 | 相似查询 API + 阈值 + 权限 | 1d |
| V3.3-GDS-23 | 识别任务表/复核接口 | 1d |
| V3.3-GDS-24 | Android CameraX 拍照/相册/压缩/上传 | 2d |
| V3.3-GDS-25 | Android 结果页 + 查看/入库/新建 | 2d |
| V3.3-GDS-26 | 识别测试集与阈值校准 | 1d |
| V3.3-GDS-27 | 多商品检测/计数（R2，可选） | 5d |

#### 平铺导出

| 编号 | 任务 | 预估 |
|---|---|---:|
| V3.3-GDS-30 | 渲染器：布局、圆角、数量徽标、中文字体 | 2d |
| V3.3-GDS-31 | 图片抓取/占位/并发/临时文件 | 1d |
| V3.3-GDS-32 | 导出任务表 + API + 权限/限流 | 1d |
| V3.3-GDS-33 | Web 多选 + 导出面板 + 历史 | 1.5d |
| V3.3-GDS-34 | Android 多选 + 生成/保存/分享 | 1.5d |
| V3.3-GDS-35 | 模板 CRUD（可选） | 1d |

#### 收尾

| 编号 | 任务 | 预估 |
|---|---|---:|
| V3.3-GDS-40 | 文档四件套 + 数据字典 + Swagger | 1d |
| V3.3-GDS-41 | UAT + 性能基线 + 7 天观察 | 1d |

---

## 9. 验收标准（总）

### 9.1 迁移

- [ ] V3 运行时不连接 MySQL；Goodser 全部走 PG。
- [ ] 19 张表 + 6 张新表迁移/建表成功；空库可一键初始化。
- [ ] V2 → V3 行数、主键、FK、JSON、金额、抽样一致。
- [ ] Web 10 个 Tab、Android 核心流程、小程序核心流程冒烟通过。
- [ ] legacy action 全部 200；错误码与 V2 一致或向后兼容。
- [ ] 路由切回 V2 演练 ≤ 30 分钟。
- [ ] PG 集成测试与方言门禁通过。

### 9.2 识别

- [ ] Android 可拍照/选图并上传。
- [ ] 返回 Top-K 相似商品与相似度；Top-3 命中率 ≥ 80%（50 张测试集）。
- [ ] 人工可改选/改数量；确认后才落单。
- [ ] 识别服务不可用不影响原有入库。
- [ ] 图片默认不出内网。

### 9.3 导出

- [ ] Web/Android 可选择商品与数量并生成图片。
- [ ] 30 件 ≤ 3s；100 件异步 ≤ 10s。
- [ ] 数量、商品图、中文名称/编码正确。
- [ ] PNG 透明/白底/深色背景可用；Android 可保存/分享。
- [ ] 失败任务可重试，错误可读。

---

## 10. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| V3.0 上线冲突 | 迁移窗口被挤占 | 先做 M0/M1 与视觉 POC；生产切换排到 V3.0 稳定后 |
| pgvector 不可用 | 相似检索无法落地 | 退 FAISS（vision 服务持有索引）；接口不变 |
| 无 GPU | 多商品检测慢 | 首期单商品 CLIP（CPU 可跑）；GPU 后再上检测 |
| 商品图缺失/质量差 | 识别率低 | 文本/编码兜底；提示补图；Top-K 人工确认 |
| 同系列外观接近 | 误匹配 | 图像 + OCR + 文本融合；阈值分档 |
| 数据归属差异 | 多用户越权/串数据 | 首期单 owner + `owner_user_id` 预留；后续按 inventory 隔离 |
| 迁移数据脏 | 启动失败/统计错 | 迁移前校验；非法 ENUM/JSON 进失败清单；不静默跳过 |
| 图片 URL 失效 | 导出/识别失败 | 迁移时校验可访问率；统一公开 URL；失败占位 |
| 中文字体缺失 | 导出方块字 | 内嵌 Noto Sans SC 子集；启动自检 |
| 大导出 OOM | 服务不稳定 | 限制 200 件/4096px；worker 池；临时文件清理 |
| Android 权限/机型 | 相机不可用 | CameraX + Photo Picker；API 26/35 真机测试 |
| 双端契约漂移 | 功能不一致 | 统一 OpenAPI/Swagger；契约测试 |

---

## 11. 待确认问题

1. **启动时机**：Goodser 迁移是等 V3.0（12.15）稳定后切，还是现在就用 V2 MySQL 临时启用先跑起来？
   - 推荐：现在做 M0/M1/M6 原型，生产切换排 V3.0 后。
2. **多用户归属**：Goodser 是否只服务一个 owner？还是多个微信用户/Android 用户各自隔离库存？
   - 推荐首期单 owner；schema 预留 `owner_user_id`。
3. **识别首期范围**：先做「单商品相似检索」，还是必须一次做「多商品检测 + 计数」？
   - 推荐先 R1，R2 作为第二阶段。
4. **硬件**：部署视觉服务的机器是否有 NVIDIA GPU？显存多少？
   - CPU 可做单图 CLIP；多商品/检测建议 8–16GB GPU。
5. **导出样式**：平铺图是否需要固定模板/品牌角标/价格？默认透明背景还是白底？
6. **Android 发布**：升级现有 App，还是新建 V3 App？
   - 推荐升级现有包名，保留用户。
7. **小程序**：首期是否必须同步支持拍照识别/平铺导出，还是 Android 先行？
8. **数据迁移窗口**：可接受的停机时长（建议 2–4 小时）与回滚窗口（7 天）。

---

## 12. 建议的第一周动作

1. 确认 §11 的 1–4 项。
2. 导出 V2 表结构与行数，生成 19 表字段映射表。
3. 搭 `goodser-vision` 最小容器：`/healthz` + `/v1/embed`，用 20 张商品图跑通向量。
4. 用 50 张真实照片做一次 Top-K 离线评估，确定模型与阈值。
5. 在 DM 建 V3.3 里程碑与任务，冻结范围。
6. 选 3 个商品做平铺图渲染 POC，确认字体/尺寸/数量徽标样式。

---

## 13. 变更记录

| 日期 | 变更 | 作者 |
|---|---|---|
| 2026-11-26 | 初版：V2→V3 迁移、Android 拍照识别、平铺图导出 | Tom / AI |
