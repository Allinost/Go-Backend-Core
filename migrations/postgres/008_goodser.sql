-- 008_goodser.sql
-- Goodser V3.3：19 张业务表迁入 PostgreSQL
-- 约定：主键保留 V2 varchar(36)；金额 numeric；JSONB；时间 timestamptz；布尔 boolean
-- 迁移器按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

-- ============================================================
-- 1. 库存目录 / 商品 / 序号
-- ============================================================
CREATE TABLE IF NOT EXISTS zzz_goodser_inventories (
  id             varchar(36) PRIMARY KEY,
  name           varchar(255) NOT NULL,
  sort_order     integer NOT NULL DEFAULT 0,
  owner_user_id  bigint,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_inventories IS '商品服务-库存目录';
CREATE INDEX IF NOT EXISTS idx_goodser_inventories_owner ON zzz_goodser_inventories (owner_user_id);

CREATE TABLE IF NOT EXISTS zzz_goodser_products (
  id                 varchar(36) PRIMARY KEY,
  inventory_id       varchar(36) NOT NULL,
  code               varchar(50) NOT NULL,
  main_zone          varchar(10) NOT NULL,
  sub_zone           varchar(10) NOT NULL,
  seq_number         integer NOT NULL,
  quantity           integer NOT NULL DEFAULT 0,
  reserved_quantity  integer NOT NULL DEFAULT 0,
  status_code        varchar(10) NOT NULL DEFAULT 'A',
  name               varchar(500) NOT NULL,
  series_id          varchar(36),
  set_type_id        varchar(36),
  product_type_id    varchar(36),
  original_price     numeric(12,2) NOT NULL DEFAULT 0,
  market_price       numeric(12,2) NOT NULL DEFAULT 0,
  expected_price     numeric(12,2) NOT NULL DEFAULT 0,
  cost_price         numeric(12,2) NOT NULL DEFAULT 0,
  remark             text,
  storage_location   varchar(255) NOT NULL DEFAULT '',
  image_url          varchar(1024) NOT NULL DEFAULT '',
  images             jsonb,
  tags               jsonb,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_products IS '商品服务-商品';
CREATE INDEX IF NOT EXISTS idx_goodser_products_inventory ON zzz_goodser_products (inventory_id);
CREATE INDEX IF NOT EXISTS idx_goodser_products_code ON zzz_goodser_products (inventory_id, code);
CREATE INDEX IF NOT EXISTS idx_goodser_products_zone ON zzz_goodser_products (inventory_id, main_zone, sub_zone);
CREATE INDEX IF NOT EXISTS idx_goodser_products_seq ON zzz_goodser_products (inventory_id, seq_number);
CREATE INDEX IF NOT EXISTS idx_goodser_products_series ON zzz_goodser_products (series_id);
CREATE INDEX IF NOT EXISTS idx_goodser_products_name ON zzz_goodser_products (name);

CREATE TABLE IF NOT EXISTS zzz_goodser_recycled_seq_numbers (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36) NOT NULL,
  main_zone     varchar(10) NOT NULL,
  sub_zone      varchar(10) NOT NULL,
  seq_number    integer NOT NULL,
  recycled_at   timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_recycled_seq_numbers IS '商品服务-回收序号';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_recycled_zone_seq
  ON zzz_goodser_recycled_seq_numbers (inventory_id, main_zone, sub_zone, seq_number);

CREATE TABLE IF NOT EXISTS zzz_goodser_seq_counters (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36) NOT NULL,
  main_zone     varchar(10) NOT NULL,
  sub_zone      varchar(10) NOT NULL,
  current_max   integer NOT NULL DEFAULT 0
);
COMMENT ON TABLE zzz_goodser_seq_counters IS '商品服务-序号计数器';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_seq_zone
  ON zzz_goodser_seq_counters (inventory_id, main_zone, sub_zone);

-- ============================================================
-- 2. 出库/预留/入库/退货/流水
-- ============================================================
CREATE TABLE IF NOT EXISTS zzz_goodser_outbound_orders (
  id                       varchar(36) PRIMARY KEY,
  inventory_id             varchar(36) NOT NULL,
  order_no                 varchar(50) NOT NULL,
  type                     varchar(20) NOT NULL,
  status                   varchar(20) NOT NULL,
  order_info               text,
  remark                   text,
  items                    jsonb NOT NULL DEFAULT '[]'::jsonb,
  source_reserve_id        varchar(36),
  combo_id                 varchar(36),
  combo_quantity           integer NOT NULL DEFAULT 0,
  bundle_id                varchar(36),
  bundle_main_product_id   varchar(36),
  bundle_quantity          integer NOT NULL DEFAULT 0,
  bundle_snapshot          jsonb,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  confirmed_at             timestamptz,
  cancelled_at             timestamptz,
  converted_at             timestamptz,
  returned_at              timestamptz
);
COMMENT ON TABLE zzz_goodser_outbound_orders IS '商品服务-出库单/预留单';
CREATE INDEX IF NOT EXISTS idx_goodser_orders_inventory ON zzz_goodser_outbound_orders (inventory_id);
CREATE INDEX IF NOT EXISTS idx_goodser_orders_no ON zzz_goodser_outbound_orders (order_no);
CREATE INDEX IF NOT EXISTS idx_goodser_orders_source_reserve ON zzz_goodser_outbound_orders (source_reserve_id);
CREATE INDEX IF NOT EXISTS idx_goodser_orders_combo ON zzz_goodser_outbound_orders (combo_id);
CREATE INDEX IF NOT EXISTS idx_goodser_orders_bundle ON zzz_goodser_outbound_orders (bundle_id);

CREATE TABLE IF NOT EXISTS zzz_goodser_inbound_logs (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36) NOT NULL,
  order_no      varchar(50) NOT NULL DEFAULT '',
  type          varchar(20) NOT NULL,
  remark        text,
  items         jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_inbound_logs IS '商品服务-入库记录';
CREATE INDEX IF NOT EXISTS idx_goodser_inbound_inventory ON zzz_goodser_inbound_logs (inventory_id);
CREATE INDEX IF NOT EXISTS idx_goodser_inbound_created ON zzz_goodser_inbound_logs (created_at);

CREATE TABLE IF NOT EXISTS zzz_goodser_returns (
  id                 varchar(36) PRIMARY KEY,
  inventory_id       varchar(36) NOT NULL,
  outbound_order_id  varchar(36) NOT NULL,
  order_no           varchar(50) NOT NULL DEFAULT '',
  status             varchar(20) NOT NULL DEFAULT 'pending',
  reason             varchar(30) NOT NULL DEFAULT 'return_refund',
  remark             text,
  items              jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  confirmed_at       timestamptz,
  cancelled_at       timestamptz
);
COMMENT ON TABLE zzz_goodser_returns IS '商品服务-退货单';
CREATE INDEX IF NOT EXISTS idx_goodser_returns_inventory ON zzz_goodser_returns (inventory_id, created_at);
CREATE INDEX IF NOT EXISTS idx_goodser_returns_order_status ON zzz_goodser_returns (outbound_order_id, status);

CREATE TABLE IF NOT EXISTS zzz_goodser_stock_movements (
  id              bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  inventory_id    varchar(36) NOT NULL,
  product_id      varchar(36) NOT NULL,
  quantity_delta  integer NOT NULL DEFAULT 0,
  reserved_delta  integer NOT NULL DEFAULT 0,
  quantity_after  integer NOT NULL DEFAULT 0,
  reserved_after  integer NOT NULL DEFAULT 0,
  reason          varchar(40) NOT NULL,
  ref_type        varchar(40) NOT NULL DEFAULT '',
  ref_id          varchar(36) NOT NULL DEFAULT '',
  actor           varchar(100) NOT NULL DEFAULT '',
  remark          varchar(500) NOT NULL DEFAULT '',
  created_at      timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_stock_movements IS '商品服务-库存流水';
CREATE INDEX IF NOT EXISTS idx_goodser_movements_product ON zzz_goodser_stock_movements (product_id, created_at);
CREATE INDEX IF NOT EXISTS idx_goodser_movements_inventory ON zzz_goodser_stock_movements (inventory_id, created_at);
CREATE INDEX IF NOT EXISTS idx_goodser_movements_ref ON zzz_goodser_stock_movements (ref_type, ref_id);
CREATE INDEX IF NOT EXISTS idx_goodser_movements_reason ON zzz_goodser_stock_movements (reason, created_at);

-- ============================================================
-- 3. 标签 / 状态码 / 字典 / 角色
-- ============================================================
CREATE TABLE IF NOT EXISTS zzz_goodser_status_codes (
  id          varchar(36) PRIMARY KEY,
  code        varchar(10) NOT NULL,
  label       varchar(100) NOT NULL,
  is_system   boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_status_codes IS '商品服务-状态码';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_status_code ON zzz_goodser_status_codes (code);

CREATE TABLE IF NOT EXISTS zzz_goodser_tags (
  id          varchar(36) PRIMARY KEY,
  name        varchar(100) NOT NULL,
  color       varchar(7) NOT NULL DEFAULT '#1890ff',
  created_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_tags IS '商品服务-标签';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_tag_name ON zzz_goodser_tags (name);

CREATE TABLE IF NOT EXISTS zzz_goodser_dictionaries (
  id          varchar(36) PRIMARY KEY,
  kind        varchar(30) NOT NULL,
  code        varchar(50) NOT NULL DEFAULT '',
  name        varchar(100) NOT NULL,
  parent_id   varchar(36),
  sort_order  integer NOT NULL DEFAULT 0,
  enabled     boolean NOT NULL DEFAULT true,
  extra       jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_dictionaries IS '商品服务-受控维度字典';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_dict_kind_code ON zzz_goodser_dictionaries (kind, code);
CREATE INDEX IF NOT EXISTS idx_goodser_dict_kind_parent ON zzz_goodser_dictionaries (kind, parent_id);
CREATE INDEX IF NOT EXISTS idx_goodser_dict_kind_enabled ON zzz_goodser_dictionaries (kind, enabled);

CREATE TABLE IF NOT EXISTS zzz_goodser_product_characters (
  product_id    varchar(36) NOT NULL,
  character_id  varchar(36) NOT NULL,
  PRIMARY KEY (product_id, character_id)
);
COMMENT ON TABLE zzz_goodser_product_characters IS '商品服务-商品角色关联';
CREATE INDEX IF NOT EXISTS idx_goodser_product_characters_char ON zzz_goodser_product_characters (character_id);

-- ============================================================
-- 4. 组合包 / 捆 / 捆物池
-- ============================================================
CREATE TABLE IF NOT EXISTS zzz_goodser_combos (
  id            varchar(36) PRIMARY KEY,
  inventory_id  varchar(36) NOT NULL,
  code          varchar(50) NOT NULL DEFAULT '',
  name          varchar(100) NOT NULL,
  description   text,
  image_url     varchar(500),
  price         numeric(10,2) NOT NULL DEFAULT 0,
  status        varchar(20) NOT NULL DEFAULT 'active',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_combos IS '商品服务-组合包';
CREATE INDEX IF NOT EXISTS idx_goodser_combos_inventory_status ON zzz_goodser_combos (inventory_id, status);
CREATE INDEX IF NOT EXISTS idx_goodser_combos_code ON zzz_goodser_combos (code);

CREATE TABLE IF NOT EXISTS zzz_goodser_combo_items (
  id          bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  combo_id    varchar(36) NOT NULL,
  item_type   varchar(20) NOT NULL DEFAULT 'product',
  product_id  varchar(36),
  bundle_id   varchar(36),
  quantity    integer NOT NULL DEFAULT 1,
  sort_order  integer NOT NULL DEFAULT 0
);
COMMENT ON TABLE zzz_goodser_combo_items IS '商品服务-组合包组件';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_combo_product
  ON zzz_goodser_combo_items (combo_id, product_id) WHERE item_type = 'product';
CREATE INDEX IF NOT EXISTS idx_goodser_combo_items_bundle ON zzz_goodser_combo_items (bundle_id);

CREATE TABLE IF NOT EXISTS zzz_goodser_bundles (
  id                varchar(36) PRIMARY KEY,
  inventory_id      varchar(36) NOT NULL,
  code              varchar(50) NOT NULL DEFAULT '',
  name              varchar(100) NOT NULL,
  description       text,
  status            varchar(20) NOT NULL DEFAULT 'active',
  main_filter       jsonb,
  min_main_quantity integer NOT NULL DEFAULT 1,
  min_main_amount   numeric(10,2) NOT NULL DEFAULT 0,
  min_sub_quantity  integer NOT NULL DEFAULT 0,
  max_sub_quantity  integer NOT NULL DEFAULT 0,
  min_sub_amount    numeric(10,2) NOT NULL DEFAULT 0,
  price_mode        varchar(20) NOT NULL DEFAULT 'sum',
  fixed_price       numeric(10,2) NOT NULL DEFAULT 0,
  pool_id           varchar(36),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_bundles IS '商品服务-捆模板';
CREATE INDEX IF NOT EXISTS idx_goodser_bundles_inventory_status ON zzz_goodser_bundles (inventory_id, status);
CREATE INDEX IF NOT EXISTS idx_goodser_bundles_code ON zzz_goodser_bundles (code);
CREATE INDEX IF NOT EXISTS idx_goodser_bundles_pool ON zzz_goodser_bundles (pool_id);

CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_main_products (
  id          bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  bundle_id   varchar(36) NOT NULL,
  product_id  varchar(36) NOT NULL,
  sort_order  integer NOT NULL DEFAULT 0
);
COMMENT ON TABLE zzz_goodser_bundle_main_products IS '商品服务-捆主商品';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_bundle_main ON zzz_goodser_bundle_main_products (bundle_id, product_id);
CREATE INDEX IF NOT EXISTS idx_goodser_bundle_main_product ON zzz_goodser_bundle_main_products (product_id);

CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_sub_products (
  id                bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  bundle_id         varchar(36) NOT NULL,
  product_id        varchar(36) NOT NULL,
  default_quantity  integer NOT NULL DEFAULT 1,
  sort_order        integer NOT NULL DEFAULT 0
);
COMMENT ON TABLE zzz_goodser_bundle_sub_products IS '商品服务-捆副区商品';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_bundle_sub ON zzz_goodser_bundle_sub_products (bundle_id, product_id);
CREATE INDEX IF NOT EXISTS idx_goodser_bundle_sub_product ON zzz_goodser_bundle_sub_products (product_id);

CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_pools (
  id              varchar(36) PRIMARY KEY,
  inventory_id    varchar(36) NOT NULL,
  code            varchar(50) NOT NULL DEFAULT '',
  name            varchar(100) NOT NULL,
  description     text,
  status          varchar(20) NOT NULL DEFAULT 'active',
  min_item_count  integer NOT NULL DEFAULT 1,
  min_amount      numeric(10,2) NOT NULL DEFAULT 0,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE zzz_goodser_bundle_pools IS '商品服务-捆物池';
CREATE INDEX IF NOT EXISTS idx_goodser_bundle_pools_inventory ON zzz_goodser_bundle_pools (inventory_id, status);
CREATE INDEX IF NOT EXISTS idx_goodser_bundle_pools_code ON zzz_goodser_bundle_pools (code);

CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_pool_items (
  id                bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  pool_id           varchar(36) NOT NULL,
  product_id        varchar(36) NOT NULL,
  default_quantity  integer NOT NULL DEFAULT 1,
  sort_order        integer NOT NULL DEFAULT 0
);
COMMENT ON TABLE zzz_goodser_bundle_pool_items IS '商品服务-捆物池成员';
CREATE UNIQUE INDEX IF NOT EXISTS uk_goodser_bundle_pool_item ON zzz_goodser_bundle_pool_items (pool_id, product_id);
CREATE INDEX IF NOT EXISTS idx_goodser_bundle_pool_item_product ON zzz_goodser_bundle_pool_items (product_id);
