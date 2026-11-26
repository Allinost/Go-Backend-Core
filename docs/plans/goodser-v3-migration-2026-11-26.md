# Goodser V3 迁移与增强规划

> 版本：v0.2（决策已确认）
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
5. **识别首期直接做多商品检测 + 计数**：Android 拍照 → 检测框 → 逐框相似匹配 → 同款聚类计数 → 人工复核；单图相似检索作为其中一种模式保留。
6. **平铺图导出走服务端渲染**：Web/Android 只提交商品与数量，Go 端合成 PNG/JPEG，保证两端一致。

### 0.2 已确认决策（2026-11-26）

| 事项 | 决策 |
|---|---|
| 启动方式 | **立即并行做完整迁移**，不等 V3.0 稳定；生产切路由仍需独立窗口与回滚点 |
| 数据归属 | **单 owner，首期共享**；schema 预留 `owner_user_id`，后续再按用户隔离 |
| 识别范围 | **多商品检测 + 计数 + 手工确认**；不是只做单商品相似检索 |
| 模型服务 | **独立容器项目 `goodser-vision`**，独立仓库/镜像/API 版本，不塞进 Go 主进程 |
| GPU | **RTX 5060 Ti 16GB**，需与 embedding / TTS 等服务共享显存；视觉服务预算建议 ≤ 8GB |
| 平铺导出 | 服务端 Go 渲染，Web/Android 共用同一套输出 |

### 0.3 关键判断

| 事项 | 结论 |
|---|---|
| V2 是否需要保留 | 保留运行，V3 切换稳定 7 天后再停 V2 Goodser |
| V3 数据库 | PostgreSQL 16 + `pgvector`（不可用时退 FAISS） |
| 路由前缀 | 不改，继续 `/api/v1/zzz-goodser` |
| Web 前端 | 已有 V3 Goodser 页面，切换 API 层即可，不重做 |
| Android | 升级现有 Compose App，保留 legacy 调用兼容 |
| 小程序 | 保留 legacy/REST 兼容，不强制首期改版 |
| 识别服务 | 独立 Docker `goodser-vision`，本地部署，GPU 共享 + CPU 降级 |
| 平铺导出 | 服务端 Go 渲染，异步任务，结果进 RustFS |
| 迁移工期 | 核心迁移约 18 人日；识别 + 导出约 22 人日；建议 5–6 周并行完成 |

### 0.4 与 V3.0 的关系

V3.0 规划原本写的是「业务模块暂不迁移，V3.3 再做 Goodser」；本次已确认 **立即并行启动完整迁移**，因此：

- Goodser 迁移作为独立工作流推进，不等 12.15。
- 与 V3.0 共用的部分（PG 主库、auth/RBAC、RustFS、CI、迁移工具）直接复用，不重复建设。
- 生产切路由仍单独安排窗口，避开公网/反代/认证上线当天；切换前必须有快照、校验报告和 30 分钟回滚方案。
- 若与 V3.0 发布冲突，优先保证 V3.0 核心上线，Goodser 切换顺延，但开发和迁移演练不停。

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
4. Android 增加拍照/相册扫描，支持一张图多个商品检测、相似匹配与计数。
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
- 一张图多个谷子：自动框选、同款聚类、计数、批量复核入库。

### 5.2 分阶段范围

| 阶段 | 能力 | 说明 |
|---|---|---|
| R1（MVP，本次） | **多商品检测 + 相似匹配 + 计数** | 拍照/相册 → 检测框 → 每框 Top-K → 同款聚类 → 人工确认 |
| R2 | OCR 强匹配 + VLM 复核 | 编码/名称识别；低置信度用 VLM 复核（共享显存，可选） |
| R3 | 盘点/出库场景 | 识别结果与当前库存对比，生成差异草稿 |
| R4 | 批量/视频抽帧 | 后续按需 |

> 首期直接做 R1。单商品相似检索保留为 `mode=similarity` 的轻量模式，Android 可切换。
> 一张图多个商品的核心链路：检测 → 裁剪 → 向量召回 → 同款聚类 → 数量汇总 → 人工复核 → 入库草稿。

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

### 5.4 视觉服务 `goodser-vision`（独立容器项目）

**定位**：独立仓库/独立镜像/独立发布节奏，Go 后端只通过 HTTP 调用；不与主进程共享 Python 依赖，也不把模型文件打进 Go 镜像。

**建议仓库结构**：

```text
goodser-vision/
├── app/
│   ├── main.py              # FastAPI 入口
│   ├── api/                 # analyze / embed / detect / ocr / health
│   ├── services/
│   │   ├── detector.py      # YOLO / Grounding DINO
│   │   ├── embedder.py      # Chinese-CLIP
│   │   ├── ocr.py           # PaddleOCR（R2）
│   │   ├── matcher.py       # 同款聚类/计数
│   │   └── vlm.py           # 可选复核，按需加载
│   ├── schemas.py
│   └── settings.py
├── models/                  # 模型下载脚本 + 版本清单
├── tests/
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

**技术栈**：Python 3.11 + FastAPI + ONNX Runtime（优先）/ PyTorch + CUDA；CPU 可降级。

**首期模型与显存预算（RTX 5060 Ti 16GB 共享）**：

| 能力 | 首期模型 | FP16/INT8 显存 | 说明 |
|---|---|---:|---|
| 目标检测 | YOLO11s（或 YOLO11m） | 1.0–1.8GB | 谷子平铺/叠放检测；需 100–300 张标注 |
| 备用检测 | Grounding DINO + SAM2 | 3–5GB | 零样本，显存占用高，按需切换 |
| 图像向量 | Chinese-CLIP ViT-B/16 | 0.6–1.0GB | 512 维，中文包装/角色友好 |
| OCR（R2） | PaddleOCR 中文 | 1.0–1.5GB | 编码/名称强匹配 |
| VLM 复核（R2，可选） | Qwen2.5-VL-3B 4bit | 3–4GB | 7B 4bit 约 6GB，按显存决定 |
| CUDA/运行时开销 | — | 1.5–2GB | 预留 |

**显存策略（关键）**：

- 视觉服务预算 **≤ 8GB**，给 embedding/TTS 等留出另一半显存。
- 默认只加载 **YOLO + CLIP + OCR**（约 5–6GB）。
- VLM **懒加载**：首次复核请求才加载，空闲 5–10 分钟卸载；并发限制 1。
- 单请求并发 1–2，超出排队；拒绝时返回 429，不 OOM。
- 使用 FP16/INT8、TensorRT engine 缓存；启动时记录模型版本与显存占用。
- 通过 `torch.cuda.set_per_process_memory_fraction(0.5)` / `PYTORCH_CUDA_ALLOC_CONF` 限制进程显存。
- 暴露 `/metrics`：GPU 显存、队列长度、推理耗时、失败率。

**API（v1）**：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 存活/模型加载状态 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/v1/models` | 模型名称/版本/维度/设备 |
| POST | `/v1/analyze` | **主接口**：multipart 图片 → 检测框 + 每框 Top-K + 聚类数量 + 耗时 |
| POST | `/v1/embed` | 单图向量 |
| POST | `/v1/embed/batch` | 批量向量（建索引/回填） |
| POST | `/v1/detect` | 仅检测（调试/标注） |
| POST | `/v1/ocr` | OCR（R2） |
| POST | `/v1/verify` | VLM 复核 Top-K（R2，可选） |

`/v1/analyze` 响应示例：

```json
{
  "model": {"detector":"yolo11s-v1","embedder":"chinese-clip-vit-b16-v1","dim":512},
  "elapsed_ms": 820,
  "items": [
    {"box":[120,80,260,240],"crop_url":"...","candidates":[{"product_id":"p1","score":0.93}]}
  ],
  "groups": [
    {"product_id":"p1","quantity":3,"score":0.92,"item_indexes":[0,2,5]}
  ]
}
```

**部署**：

- 独立 compose 项目：`deploy/goodser-vision/docker-compose.yml`，仅内网网络。
- GPU 预留：`deploy.resources.reservations.devices` + `NVIDIA_VISIBLE_DEVICES`；CPU profile 可无 GPU 启动。
- 模型缓存卷持久化；镜像只带代码，模型启动时校验/下载。
- 健康检查 + 自动重启；日志 JSON；指标接 Prometheus。
- 不暴露公网；只允许 V3 后端访问；CORS 关闭。
- 版本：镜像 tag + `/v1/models` + 数据库 `model` 字段三方一致；升级先灰度一台。

**推理链路**：

1. `/v1/analyze` 接收图片 → 预处理（EXIF 纠正、最长边 1920）。
2. YOLO 检测 → NMS 去重 → 裁剪每个框。
3. 每个裁剪图计算 CLIP 向量。
4. Go 端用 pgvector 查 Top-K；服务端也可返回向量由 Go 查询（推荐 Go 查，权限/库存目录过滤在 Go）。
5. 同一 `product_id` 的多个框聚合计数；低置信度保留候选。
6. 返回框、候选、分组数量；Android 展示并允许人工修正。

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
   - `POST /zzz-goodser/inventories/:id/recognitions`（multipart 或 `image_url`，`mode=multi|similarity`）
   - `GET /zzz-goodser/recognitions/:rid`
   - `POST /zzz-goodser/recognitions/:rid/apply`（人工复核后生成入库草稿/入库单）
   - `POST /zzz-goodser/recognitions/:rid/items/:iid/confirm`（逐项确认，可选）
5. 多商品链路：
   - Go 调 `/v1/analyze` 拿检测框与每框向量/候选。
   - 对每个框查 pgvector Top-K，按 `product_id` 聚合 `quantity`。
   - 低于阈值的框标记 `unmatched`，Android 可新建商品或手动搜索。
   - `apply` 只接受已确认项；库存变更走现有入库事务与幂等。
6. 相似查询 SQL（每个框一次，可批量）：
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
   - `ScanScreen`：取景框、闪光灯、相册入口、拍照；提示「尽量平铺、避免遮挡」。
   - `ScanResultScreen`：
     - 原图 + 编号检测框 overlay（可点选/隐藏）。
     - 按商品分组的清单：缩略图、名称/编码、当前库存、识别数量、相似度。
     - 每组可展开 Top-3 候选，支持改选商品、改数量、删除误识别、手动补商品。
     - 未匹配框突出显示，提供「新建商品（预填裁剪图）」。
   - 底部操作：全选/取消、生成入库草稿、重新拍摄。
4. 上传：
   - 检测需要更高分辨率：客户端压缩到最长边 **1600–1920**、JPEG 90%。
   - 上传到 V3 `recognitions` 接口；异步任务轮询（1s 间隔，最长 60s）。
5. 复核与落单：
   - 默认不自动改库存；用户确认后生成入库草稿或直接批量入库（二选一，默认草稿）。
   - 同一商品多框自动合并数量，可手动拆分。
6. 离线/失败：
   - 失败提示重试；可选本地队列（后续）。
7. 兼容：
   - 保留现有 legacy API；新功能只加新接口。
   - 服务器地址仍由 `ServerConfigScreen` 配置。

### 5.8 验收标准（R1）

- 100 张真实照片测试集（平铺/叠放/弱光/遮挡各若干）：
  - 商品检测召回率 ≥ 90%（允许人工补漏）。
  - 匹配 Top-3 命中率 ≥ 80%。
  - ≤ 10 件场景，数量误差 ≤ 1。
- 单张 p95：GPU（共享 16GB）≤ 5s；CPU 降级 ≤ 10s（不含网络）。
- 返回检测框、分组数量、Top-K 候选、相似度；人工可改选/改数量/补商品。
- 低置信度不自动落库；确认后生成入库草稿。
- 视觉服务异常时明确提示，不影响原有入库流程。
- 连续压测 30 分钟显存不增长、不 OOM；与 embedding/TTS 共存时视觉服务占用 ≤ 8GB。
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
| M6 视觉服务 | 独立容器、YOLO 检测、CLIP 向量、OCR 可选、analyze API、显存控制 | 7d | M1（可并行） |
| M7 识别 API | 任务、逐框相似查询、聚类计数、复核、权限 | 4d | M5、M6 |
| M8 Android 扫描 | CameraX、多商品结果页、框选/分组/改数量/入库草稿 | 6d | M7 |
| M9 平铺导出 | 渲染器、API、模板、Web/Android | 5d | M5 |
| M10 识别增强 | OCR 强匹配、VLM 复核、盘点模式（可选） | 5d | M8 |
| M11 验收观察 | PG 测试、UAT、性能基线、7 天观察 | 2d | M5、M8、M9 |

**核心迁移（M0–M5）约 18 人日**（含测试/演练；与 README 的 8–10d 代码迁移相比更保守）。
**新增功能（M6–M9）约 22 人日**；增强 M10 +5d。
**合计约 40 人日**；按 2–3 条并行工作流，建议 **5–6 周**完成 R1 + 导出。

### 8.1.1 立即启动排期（并行）

| 周 | 迁移线 | 识别线（独立容器） | Android/导出线 |
|---|---|---|---|
| W1 | V2 基线/映射表；PG 008–009 | 视觉仓库脚手架；YOLO/CLIP 跑通；拍 100 张样片 | Android CameraX 分支；导出样式 POC |
| W2 | PG 010–012 + Repository 改造 | 检测/裁剪/向量 API；标注首批 100 张 | 导出渲染器；Android 拍照页 |
| W3 | 迁移工具 + 首次演练 | 聚类计数 + analyze API；显存压测 | Android 结果页；Web 导出面板 |
| W4 | 模块开关 + 路由切换演练 | Go 识别 API 联调；阈值校准 | Android 多商品联调；导出联调 |
| W5 | 正式迁移 + 观察 | 识别准确率调优；OCR（可选） | UAT + 修复 |
| W6 | 文档/回滚演练 | 性能/显存稳定性 | 全端验收 |

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
| V3.3-GDS-20 | `goodser-vision` 独立仓库/镜像/FastAPI 骨架 + 健康检查 | 1.5d |
| V3.3-GDS-21 | YOLO 检测 + 标注流程 + 裁剪/NMS | 3d |
| V3.3-GDS-22 | Chinese-CLIP 向量 + `/v1/embed` + 批量回填 | 1.5d |
| V3.3-GDS-23 | `/v1/analyze`：检测 + 向量 + 聚类计数 | 1.5d |
| V3.3-GDS-24 | GPU 显存控制/懒加载/并发/指标（≤8GB） | 1d |
| V3.3-GDS-25 | 向量表、商品图变更触发、重建接口 | 1.5d |
| V3.3-GDS-26 | Go 识别 API：任务、逐框 Top-K、聚合、复核 | 2d |
| V3.3-GDS-27 | Android CameraX 拍照/相册/压缩/上传 | 2d |
| V3.3-GDS-28 | Android 多商品结果页：框 overlay、分组、改数量、入库草稿 | 3d |
| V3.3-GDS-29 | 测试集、阈值校准、准确率报告 | 1.5d |
| V3.3-GDS-29b | OCR 强匹配 + VLM 复核（R2，可选） | 5d |

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
- [ ] 一张图多个商品可检测、分组、计数；检测召回率 ≥ 90%。
- [ ] 每组返回 Top-K 相似商品与相似度；Top-3 命中率 ≥ 80%（100 张测试集）。
- [ ] 人工可改选/改数量/补漏；确认后才生成入库草稿。
- [ ] 单张 p95：GPU ≤ 5s，CPU 降级 ≤ 10s。
- [ ] 视觉服务占用 ≤ 8GB，连续压测不 OOM。
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
| V3.0 上线冲突 | 迁移窗口被挤占 | 迁移开发并行不停；生产切路由避开 V3.0 发布当天，保留回滚点 |
| pgvector 不可用 | 相似检索无法落地 | 退 FAISS（vision 服务持有索引）；接口不变 |
| GPU 共享/显存不足 | 视觉服务 OOM 或拖慢 TTS/embedding | 视觉预算 ≤8GB、并发 1–2、VLM 懒加载/卸载、FP16/INT8、显存压测 |
| 独立容器版本漂移 | Go 与模型服务契约不一致 | `/v1` 版本化 + 契约测试 + 模型版本入库；镜像 tag 固定 |
| 检测标注不足 | 多商品召回低 | 首期 100–300 张标注；允许人工补漏；按系列迭代 |
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

## 11. 待确认问题（已更新）

已确认：立即并行迁移、单 owner、多商品检测 + 计数、独立容器、5060Ti 16GB 共享。

仍需确认：

1. **导出样式**：平铺图是否需要固定模板/品牌角标/价格？默认透明背景还是白底？
2. **Android 发布**：升级现有 App，还是新建 V3 App？
   - 推荐升级现有包名，保留用户。
3. **小程序**：首期是否必须同步支持拍照识别/平铺导出，还是 Android 先行？
4. **迁移窗口**：可接受的停机时长（建议 2–4 小时）与回滚窗口（7 天）。
5. **视觉仓库归属**：`goodser-vision` 放在哪个 Git 仓库/镜像仓库？是否由本仓库 CI 统一构建？
6. **标注数据**：谁来标注首批 100–300 张？能否先接受「检测召回 ≥90% + 人工补漏」？
7. **VLM 复核**：16GB 共享显存下是否首期就启用 3B VLM？还是先 OCR，VLM 后置？
   - 推荐先 OCR；VLM 作为可选，按需懒加载。
8. **GPU 配额**：视觉服务 8GB、embedding/TTS 剩余 8GB 的划分是否固定？是否需要按时间片切换？

---

## 12. 建议的第一周动作

1. 在 DM 建 V3.3 里程碑与任务，冻结 R1 范围（多商品检测 + 计数 + 人工复核）。
2. 导出 V2 表结构与行数，生成 19 表字段映射表；确定 owner 回填规则。
3. 新建 `goodser-vision` 独立仓库/目录：`/healthz` + `/v1/models` + `/v1/embed` 跑通。
4. 收集并标注首批 100 张真实照片（平铺/叠放/弱光/遮挡），跑 YOLO 检测基线。
5. 用 50 张照片做 CLIP Top-K 离线评估，确定阈值与模型。
6. 选 3 个商品做平铺图渲染 POC，确认字体/尺寸/数量徽标样式。
7. 起一个 Android `feature/goodser-scan` 分支，先打通 CameraX 拍照与上传。

---

## 13. 变更记录

| 日期 | 变更 | 作者 |
|---|---|---|
| 2026-11-26 | 初版：V2→V3 迁移、Android 拍照识别、平铺图导出 | Tom / AI |
| 2026-11-26 | v0.2：确认立即并行迁移、单 owner、多商品检测+计数、独立视觉容器、5060Ti 16GB 共享显存；补充显存预算/排期/任务 | Tom / AI |
