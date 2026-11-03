# 捆物池与组合包含捆（2026-11-03 设计稿）

> 状态：已按推荐方案实现（2026-11-03）。

## 0.1 需求原文

> 捆物池由我规定，每一个捆物可以选择绑定的捆物池，并限定捆物池最少选择商品数量和最低捆物金额

> 我的意思是组合包可以把捆物组合起来出售，捆还是可以单独卖的，如果出现库存不足则不显示出来

> 允许多个主商品，保留最低数量/金额校验

## 0.2 现状

- 捆（`zzz_goodser_bundles`）已有：主商品筛选/显式清单、副区商品清单
  （`zzz_goodser_bundle_sub_products`）、`min_sub_quantity` / `max_sub_quantity` /
  `min_sub_amount`、价格模式与固定价。
- 组合包（`zzz_goodser_combos`）只包含商品组件（`zzz_goodser_combo_items.product_id`），
  库存按组件可用量计算。
- 客户库存查询已过滤 `availability <= 0` 的组合包，捆只按关键词过滤，没有库存过滤。

## 1. 目标模型

### 1.1 捆物池（新实体）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | VARCHAR(36) | 主键 |
| `inventory_id` | VARCHAR(36) | 所属库存目录 |
| `code` | VARCHAR(50) | 编码，可留空自动生成 |
| `name` | VARCHAR(100) | 池名称，例如「盗墓笔记·吧唧池」 |
| `description` | TEXT | 说明 |
| `status` | VARCHAR(20) | `active` / `inactive` |
| `min_item_count` | INT | 最少选择商品**数量（件）**，同一商品可重复选择 |
| `min_amount` | DECIMAL(10,2) | 最低捆物金额 |
| `created_at` / `updated_at` | DATETIME | 时间戳 |

池成员表 `zzz_goodser_bundle_pool_items`：`pool_id`、`product_id`、`default_quantity`、`sort_order`。
同一池内商品唯一；池可在多个捆之间复用。

### 1.2 捆

- 新增 `pool_id`，一个捆绑定一个捆物池。
- 主商品仍支持多个（`main_filter` + `main_product_ids`），保留 `min_main_quantity` / `min_main_amount` 校验。
- 生效规则：
  - 最少捆物数量 = `max(pool.min_item_count, bundle.min_sub_quantity)`（按总件数）
  - 最低捆物金额 = `max(pool.min_amount, bundle.min_sub_amount)`
  - 最多捆物数量 = `bundle.max_sub_quantity`（0 表示不限制）
  - 同一捆物商品允许重复选择，数量累加
- 捆仍可单独售卖，不因被组合包引用而停用。
- 库存不足判定（客户侧）：
  1. 没有任何可用主商品，或
  2. 池内可用商品种数 < 生效最少种数
  满足任一条件时从客户库存查询/组合包展开中隐藏，管理端仍可见。

### 1.3 组合包

- 组件新增类型：`item_type = product | bundle`。
- 商品组件沿用 `product_id`；捆组件使用 `bundle_id`，`quantity` 表示整套捆的数量。
- `zzz_goodser_combo_items` 增加 `item_type`、`bundle_id`，`product_id` 改为可空。
- 组合包可用量 = 所有组件的 `floor(组件可用量 / 组件数量)` 的最小值；
  捆组件可用量按 1.2 的捆可售量计算。
- 组合包价格仍为固定价，不随捆内商品价格浮动。
- 客户查询中，组合包可用量 <= 0 时隐藏（现有逻辑扩展到捆组件）。

## 2. 数据结构迁移

- `015_goodser_bundle_pools.sql`
  - 新建 `zzz_goodser_bundle_pools`、`zzz_goodser_bundle_pool_items`
  - `zzz_goodser_bundles` 增加 `pool_id` 与索引
  - 存量数据迁移：为每个有副区商品的捆自动创建同名池，复制副区商品，
    回填 `pool_id`；无副区商品的捆不建池
- `016_goodser_combo_bundle_items.sql`
  - `zzz_goodser_combo_items` 增加 `item_type`、`bundle_id`
  - `product_id` 改为可空
  - 历史行 `item_type` 默认 `product`

## 3. 接口变更

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/zzz-goodser/bundle-pools?inventory_id=` | 池列表（含成员与规则） |
| POST | `/zzz-goodser/bundle-pools` | 新建池 |
| PUT | `/zzz-goodser/bundle-pools/:pid` | 更新池与成员 |
| DELETE | `/zzz-goodser/bundle-pools/:pid` | 删除池（被捆引用时拒绝） |

- 捆创建/更新请求新增 `pool_id`；捆详情与客户视图返回 `pool` 对象。
- 组合包组件请求新增 `item_type`、`bundle_id`；组合包详情返回组件类型与捆可用量。
- 新增错误码：池不存在、池成员为空、池被引用、组合包捆组件无效等。

## 4. Web / 小程序

- 捆页签工具栏新增「捆物池管理」：池列表、新建/编辑/删除、成员多选与规则设置。
- 捆表单新增「捆物池」下拉，展示池规则；副区商品配置由池成员替代。
- 组合包表单组件类型切换「商品 / 捆」，选择捆时展示捆名称、包含商品与可用量。
- 组合包卡片、客户查询、小程序组合包详情显示组件类型标签（商品/捆）。
- 客户查询与小程序继续隐藏库存不足的组合包和捆。

## 5. 验收标准

- 捆物池可增删改查，成员与规则保存后重新打开一致。
- 一个池可被多个捆绑定；删除被引用池时返回明确错误。
- 捆可设置多个主商品，最低主商品数量/金额仍校验。
- 下单预览按池规则校验：选品种数、最低金额、最大种数。
- 组合包可同时包含商品与捆；捆仍可单独下单。
- 主商品无库存或池内可售种数不足时，客户查询不返回该捆；
  组合包任一组件不足时，组合包不返回。
- 存量捆迁移后行为与迁移前一致；`go test ./zzz/goodser/...`、`make sop-check` 通过。

## 6. 待确认

1. 捆物池按库存目录隔离，允许跨捆复用（推荐）。
2. 捆与池的规则取 `max` 合并，保留捆级最大种数限制（推荐）。
3. 组合包含捆时使用组合包固定价，不按捆内商品重新计价（推荐）。
4. 库存不足只在客户侧隐藏，管理端仍展示并标注原因（推荐）。

## 7. 实施记录（2026-11-03）

- 迁移：`016_goodser_bundle_pools.sql`、`017_goodser_combo_bundle_items.sql`。
- 新接口：`/zzz-goodser/bundle-pools` 增删改查。
- 捆新增 `pool_id` / `pool` / `availability`；组合包组件新增
  `item_type` / `bundle_id` / `bundle_name`。
- 组合包展开捆组件时必须由调用方传 `combo_bundles` 手工选择主商品与捆物，
  服务端不做自动搭配；同一捆物商品可重复选择并按数量累加。
- 库存目录支持 `sort_order`，列表按 `sort_order`、`created_at` 升序，Web 默认打开第一个。
- 客户库存查询隐藏主商品无库存或池可售数量不足的捆；组合包按组件（含捆）可用量过滤。
- Web：捆页签新增「捆物池管理」，捆表单支持选择池并只读展示池成员；组合包表单支持
  「商品 / 捆」组件切换与捆组件添加；订单表单在组合包含捆时展开手工选择区；
  库存目录新建/编辑支持排序。
- 验证：
  - `go test ./zzz/goodser/...` 通过（新增 `TestBundleEffectiveSubRules`、
    `TestBundleAvailability`、`TestBundlePoolSubProducts`）。
  - `.local/verify-bundle-pool-api.js`：17 项全部通过（池/捆/组合包 CRUD、规则校验、
    组合包手工选捆与重复选择、库存目录排序、引用保护、客户查询）。
  - `.local/verify-bundle-pool-web.js`：34 项全部通过（池管理、捆绑定池、组合包含捆、
    订单手工选捆、库存目录排序与默认打开第一个）。

