-- 014_goodser_cost_price.sql
-- Goodser 商品成本价：用于收益统计，留空时回退到原价
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

ALTER TABLE zzz_goodser_products
  ADD COLUMN cost_price DECIMAL(12,2) NOT NULL DEFAULT 0 AFTER expected_price;

CREATE INDEX idx_products_cost_price ON zzz_goodser_products (cost_price);
