-- 010_goodser_p0_inventory.sql
-- Goodser P0：D1 库存语义、库存流水、退货单
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号
--
-- D1 语义：
--   quantity          = 可用库存
--   reserved_quantity = 已锁定库存（预留中/待出库）
--   total             = quantity + reserved_quantity
--
-- 本迁移把历史的 quantity（总量）转换为可用量，保留 reserved_quantity。
-- 执行前请先备份，并确认没有 reserved_quantity 大于 quantity 的脏数据。

-- ---------- 1. 出库单增加转单/退货时间与索引 ----------
ALTER TABLE zzz_goodser_outbound_orders
  ADD COLUMN converted_at DATETIME DEFAULT NULL,
  ADD COLUMN returned_at DATETIME DEFAULT NULL;

ALTER TABLE zzz_goodser_outbound_orders
  ADD INDEX idx_source_reserve (source_reserve_id);

-- ---------- 2. 单据状态归一 ----------
-- 旧版 reserve/outbound 都可能出现 reserved 状态，统一回到 pending
UPDATE zzz_goodser_outbound_orders
SET status = 'pending'
WHERE status = 'reserved';

-- 旧版前端曾把预留单状态直接写成 reserve，同样归一为 pending
UPDATE zzz_goodser_outbound_orders
SET status = 'pending'
WHERE status = 'reserve';

-- 旧版“预留转出库”会把原预留单标成 confirmed，且有子出库单
-- 新语义下这是 converted 终态，避免报表把预留和出库重复统计
UPDATE zzz_goodser_outbound_orders r
JOIN (
  SELECT DISTINCT source_reserve_id AS rid
  FROM zzz_goodser_outbound_orders
  WHERE source_reserve_id IS NOT NULL AND source_reserve_id <> ''
) c ON c.rid = r.id
SET r.status = 'converted',
    r.converted_at = COALESCE(r.confirmed_at, r.updated_at)
WHERE r.type = 'reserve' AND r.status = 'confirmed';

-- ---------- 3. 库存语义迁移：总量 -> 可用量 ----------
-- 仅对仍有锁定库存的商品执行，避免重复扣减
UPDATE zzz_goodser_products
SET quantity = GREATEST(quantity - reserved_quantity, 0),
    updated_at = CURRENT_TIMESTAMP
WHERE reserved_quantity > 0;

-- ---------- 4. 库存流水 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_stock_movements (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  product_id VARCHAR(36) NOT NULL,
  quantity_delta INT NOT NULL DEFAULT 0,
  reserved_delta INT NOT NULL DEFAULT 0,
  quantity_after INT NOT NULL DEFAULT 0,
  reserved_after INT NOT NULL DEFAULT 0,
  reason VARCHAR(40) NOT NULL,
  ref_type VARCHAR(40) NOT NULL DEFAULT '',
  ref_id VARCHAR(36) NOT NULL DEFAULT '',
  actor VARCHAR(100) NOT NULL DEFAULT '',
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_product_created (product_id, created_at),
  INDEX idx_inventory_created (inventory_id, created_at),
  INDEX idx_ref (ref_type, ref_id),
  INDEX idx_reason_created (reason, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-库存流水';

-- ---------- 5. 退货单 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_returns (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  outbound_order_id VARCHAR(36) NOT NULL,
  order_no VARCHAR(50) NOT NULL DEFAULT '',
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  reason VARCHAR(30) NOT NULL DEFAULT 'return_refund',
  remark TEXT,
  items JSON NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  confirmed_at DATETIME DEFAULT NULL,
  cancelled_at DATETIME DEFAULT NULL,
  INDEX idx_inventory_created (inventory_id, created_at),
  INDEX idx_order_status (outbound_order_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-退货单';
