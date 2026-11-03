-- 017_goodser_combo_bundle_items.sql
-- Goodser 组合包支持捆组件：组合包可把捆与其他商品/捆一起出售
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号
--
-- 设计说明：
--   item_type=product 时使用 product_id，item_type=bundle 时使用 bundle_id
--   捆仍可单独售卖，组合包只增加引用关系

ALTER TABLE zzz_goodser_combo_items
  ADD COLUMN item_type VARCHAR(20) NOT NULL DEFAULT 'product',
  ADD COLUMN bundle_id VARCHAR(36) DEFAULT NULL,
  ADD INDEX idx_combo_item_bundle (bundle_id);

ALTER TABLE zzz_goodser_combo_items
  MODIFY product_id VARCHAR(36) NULL;
