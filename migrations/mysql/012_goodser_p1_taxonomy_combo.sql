-- 012_goodser_p1_taxonomy_combo.sql
-- Goodser P1：受控维度字典、角色关联、组合包 BOM
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号
--
-- 设计说明：
--   dictionaries 用 kind 区分 series/character/set_type/product_type 四类字典
--   character 通过 parent_id 关联 series，商品可关联多个 character
--   组合包不持有独立库存，可用量由组件商品可用量推导

-- ---------- 1. 通用字典 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_dictionaries (
  id VARCHAR(36) PRIMARY KEY,
  kind VARCHAR(30) NOT NULL,
  code VARCHAR(50) NOT NULL DEFAULT '',
  name VARCHAR(100) NOT NULL,
  parent_id VARCHAR(36) DEFAULT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  extra JSON,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_kind_code (kind, code),
  INDEX idx_kind_parent (kind, parent_id),
  INDEX idx_kind_enabled (kind, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-受控维度字典';

-- ---------- 2. 商品维度字段 ----------
ALTER TABLE zzz_goodser_products
  ADD COLUMN series_id VARCHAR(36) DEFAULT NULL,
  ADD COLUMN set_type_id VARCHAR(36) DEFAULT NULL,
  ADD COLUMN product_type_id VARCHAR(36) DEFAULT NULL,
  ADD INDEX idx_series (series_id),
  ADD INDEX idx_set_type (set_type_id),
  ADD INDEX idx_product_type (product_type_id);

-- ---------- 3. 商品与角色多对多 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_product_characters (
  product_id VARCHAR(36) NOT NULL,
  character_id VARCHAR(36) NOT NULL,
  PRIMARY KEY (product_id, character_id),
  INDEX idx_character (character_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-商品角色关联';

-- ---------- 4. 组合包 ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_combos (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  code VARCHAR(50) NOT NULL DEFAULT '',
  name VARCHAR(100) NOT NULL,
  description TEXT,
  image_url VARCHAR(500) DEFAULT NULL,
  price DECIMAL(10,2) NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_inventory_status (inventory_id, status),
  INDEX idx_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-组合包';

CREATE TABLE IF NOT EXISTS zzz_goodser_combo_items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  combo_id VARCHAR(36) NOT NULL,
  product_id VARCHAR(36) NOT NULL,
  quantity INT NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE KEY uk_combo_product (combo_id, product_id),
  INDEX idx_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-组合包组件';

-- ---------- 5. 出库单记录来源组合 ----------
ALTER TABLE zzz_goodser_outbound_orders
  ADD COLUMN combo_id VARCHAR(36) DEFAULT NULL,
  ADD COLUMN combo_quantity INT NOT NULL DEFAULT 0,
  ADD INDEX idx_combo (combo_id);

-- ---------- 6. 默认字典 ----------
INSERT INTO zzz_goodser_dictionaries (id, kind, code, name, parent_id, sort_order, enabled)
VALUES
  ('dict_set_single', 'set_type', 'single', '单款', NULL, 10, 1),
  ('dict_set_pair', 'set_type', 'pair', '对装', NULL, 20, 1),
  ('dict_set_group', 'set_type', 'group', '套装', NULL, 30, 1),
  ('dict_type_polaroid', 'product_type', 'polaroid', '拍立得', NULL, 10, 1),
  ('dict_type_figure', 'product_type', 'figure', '手办', NULL, 20, 1),
  ('dict_type_badge', 'product_type', 'badge', '徽章', NULL, 30, 1),
  ('dict_type_other', 'product_type', 'other', '其他', NULL, 99, 1)
ON DUPLICATE KEY UPDATE name = VALUES(name), sort_order = VALUES(sort_order), enabled = VALUES(enabled);
