-- 016_goodser_bundle_pools.sql
-- Goodser 捆物池：每个捆绑定一个可复用的捆物池
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号
--
-- 设计说明：
--   捆物池按库存目录隔离，可在多个捆之间复用
--   池成员决定下单时可选的捆物商品，规则限制最少选择商品数量和最低捆物金额
--   存量捆的副区商品自动迁移为同名池，保证迁移后行为不变

-- ---------- 1. 捆物池 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_pools (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  code VARCHAR(50) NOT NULL DEFAULT '',
  name VARCHAR(100) NOT NULL,
  description TEXT,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  min_item_count INT NOT NULL DEFAULT 1,
  min_amount DECIMAL(10,2) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_bundle_pool_inventory_status (inventory_id, status),
  INDEX idx_bundle_pool_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-捆物池';

-- ---------- 2. 捆物池成员 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_pool_items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  pool_id VARCHAR(36) NOT NULL,
  product_id VARCHAR(36) NOT NULL,
  default_quantity INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE KEY uk_bundle_pool_item (pool_id, product_id),
  INDEX idx_bundle_pool_item_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-捆物池成员';

-- ---------- 3. 捆绑定捆物池 ----------
ALTER TABLE zzz_goodser_bundles
  ADD COLUMN pool_id VARCHAR(36) DEFAULT NULL,
  ADD INDEX idx_bundle_pool (pool_id);

-- ---------- 4. 存量副区商品迁移为捆物池 ----------
INSERT INTO zzz_goodser_bundle_pools
  (id, inventory_id, code, name, description, status, min_item_count, min_amount)
SELECT CONCAT('bp-', SUBSTRING(MD5(b.id), 1, 30)), b.inventory_id, CONCAT(b.code, '-POOL'), CONCAT(b.name, ' 捆物池'),
       NULL, 'active', GREATEST(b.min_sub_quantity, 1), b.min_sub_amount
FROM zzz_goodser_bundles b
WHERE b.pool_id IS NULL
  AND EXISTS (SELECT 1 FROM zzz_goodser_bundle_sub_products sp WHERE sp.bundle_id = b.id);

UPDATE zzz_goodser_bundles b
JOIN zzz_goodser_bundle_pools p ON p.id = CONCAT('bp-', SUBSTRING(MD5(b.id), 1, 30))
SET b.pool_id = p.id
WHERE b.pool_id IS NULL;

INSERT INTO zzz_goodser_bundle_pool_items (pool_id, product_id, default_quantity, sort_order)
SELECT b.pool_id, sp.product_id, sp.default_quantity, sp.sort_order
FROM zzz_goodser_bundles b
JOIN zzz_goodser_bundle_sub_products sp ON sp.bundle_id = b.id
WHERE b.pool_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM zzz_goodser_bundle_pool_items pi
    WHERE pi.pool_id = b.pool_id AND pi.product_id = sp.product_id
  );
