# Goodser 需求迭代（2026-11-03 晚）

## 0.1 需求原文

1. 系列视图 Tab 页应该增加新建系列、编辑、删除的按钮。
2. 组合包、捆也应该补上编辑、删除的按钮。
3. 入库记录需要完善一下标题显示。
4. 推进 P1，暂时可以不推 P2。

## 0.2 原文留档

- 组合包与捆的卡片已有编辑/删除按钮，本轮统一把「新建」入口提升到页签标题栏，并确认卡片操作可见。
- P1 范围（来自 UI 检查清单）：Web 商品图片上传/展示、统计自定义日期区间。P2 暂不推进。

## 1. 方案与决策

| # | 决策 | 说明 |
|---|---|---|
| D1 | 系列 CRUD 复用维度字典接口 | 系列即 `kind=series` 的字典；新建/编辑/删除后刷新系列视图并通知父组件刷新字典，商品表单与捆筛选同步 |
| D2 | 系列删除保留后端引用校验 | 被商品/子字典/捆筛选引用时后端返回错误，前端展示提示，不做前端硬删 |
| D3 | 系列编码可自动生成 | 留空时生成 `series_<timestamp>`，避免用户被必填编码卡住 |
| D4 | 组合包/捆页签统一标题栏 | 标题 + 数量 + 新建按钮；卡片保留编辑/删除，符合“标题栏操作、卡片操作”一致习惯 |
| D5 | 入库记录标题栏 | 增加「入库记录」标题与数量，操作按钮保持在右侧 |
| D6 | 商品图片单图 + 对象存储 | 复用 `uploadImage`（multipart，字段 `image`）；表单预览、列表缩略图、详情大图；保存 `image_url` |
| D6.1 | 图片地址长期有效 | 对象存储桶为公开读，上传返回不带签名查询串的地址；商品读取时归一化历史预签名 URL，修复 7 天后缩略图 403/ORB |
| D7 | 自定义统计区间 | 后端支持 `start_date`/`end_date`（YYYY-MM-DD，含首尾，最长 366 天）；前端提供日期输入，保留 7/30/90/365 预设 |
| D8 | P2 暂缓 | 小程序下钻、最近使用商品、表单 label 等不进入本轮 |

## 2. 接口与数据结构

- 系列：`GET /zzz-goodser/inventories/{id}/series`；`POST/PUT/DELETE /zzz-goodser/settings/dictionaries`
- 图片：`POST /zzz-goodser/legacy/uploadImage`（`image` multipart），返回 `{ url }`
- 统计：`GET /zzz-goodser/inventories/{id}/stats?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`；不传时仍用 `days`
- `StatsOverview` 新增 `start_date` / `end_date`

## 3. 验收标准

- 系列视图：可新建、编辑、删除；删除被引用系列时提示失败；操作后列表即时刷新。
- 组合包/捆：标题栏有新建按钮，卡片有编辑/删除按钮。
- 入库记录：标题栏显示「入库记录」与数量。
- 商品图片：能上传、预览、移除并在列表/详情展示；上传失败有提示。
- 统计：可选自定义起止日期并刷新区间、趋势与收益；CSV 区间文案正确。

## 4. 验收记录（2026-11-03）

| 项 | 结果 |
|---|---|
| `npm run typecheck` | 通过 |
| `npm run build` | 通过 |
| `go test ./zzz/goodser/...` | 通过（含 `TestNormalizeImageURL`） |
| `.local/verify-iteration-2026-11-03.js` | 29 项全部通过：系列新建/编辑/删除、组合包/捆/入库标题栏、图片上传预览、统计自定义区间 31 天 |
| `.local/verify-goodser-updates.js` | 通过（移除已删除的「文创」目录断言） |
| `.local/verify-products-web.js` / `verify-split-web.js` / `verify-settings-web.js` | 全部通过 |
| `.local/audit-web.js` | 67 项检查 0 失败；修复前 130 条图片 requestfailed，修复后无图片错误 |

补充：图片地址原为 7 天预签名 URL，过期后浏览器报 `ERR_BLOCKED_BY_ORB`。现已改为公开读地址，
并在商品/捆物读取时归一化存量 URL，存量数据无需迁移。

