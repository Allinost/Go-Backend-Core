-- 013_goodser_bundles.sql
-- Goodser 捆：主商品条件约束下搭配副区商品
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号
--
-- 设计说明：
--   捆是销售模板，不持有独立库存，下单时展开为主商品与副区商品明细
--   主商品可通过显式清单或维度条件筛选，两者同时存在时按 AND 校验
--   副区商品由店主显式挑选，下单时按最低金额、最少数量等规则校验
--   价格模式支持 sum 合计、fixed 固定价、main_only 只收主商品

-- ---------- 1. 捆模板 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_bundles (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  code VARCHAR(50) NOT NULL DEFAULT '',
  name VARCHAR(100) NOT NULL,
  description TEXT,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  main_filter JSON,
  min_main_quantity INT NOT NULL DEFAULT 1,
  min_main_amount DECIMAL(10,2) NOT NULL DEFAULT 0,
  min_sub_quantity INT NOT NULL DEFAULT 0,
  max_sub_quantity INT NOT NULL DEFAULT 0,
  min_sub_amount DECIMAL(10,2) NOT NULL DEFAULT 0,
  price_mode VARCHAR(20) NOT NULL DEFAULT 'sum',
  fixed_price DECIMAL(10,2) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_bundle_inventory_status (inventory_id, status),
  INDEX idx_bundle_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-捆模板';

-- ---------- 2. 捆主商品显式清单 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_main_products (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  bundle_id VARCHAR(36) NOT NULL,
  product_id VARCHAR(36) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE KEY uk_bundle_main (bundle_id, product_id),
  INDEX idx_bundle_main_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-捆主商品';

-- ---------- 3. 捆副区商品候选 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_bundle_sub_products (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  bundle_id VARCHAR(36) NOT NULL,
  product_id VARCHAR(36) NOT NULL,
  default_quantity INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE KEY uk_bundle_sub (bundle_id, product_id),
  INDEX idx_bundle_sub_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-捆副区商品';

-- ---------- 4. 出库单记录来源捆 ----------
ALTER TABLE zzz_goodser_outbound_orders
  ADD COLUMN bundle_id VARCHAR(36) DEFAULT NULL,
  ADD COLUMN bundle_main_product_id VARCHAR(36) DEFAULT NULL,
  ADD COLUMN bundle_quantity INT NOT NULL DEFAULT 0,
  ADD COLUMN bundle_snapshot JSON,
  ADD INDEX idx_bundle (bundle_id);
