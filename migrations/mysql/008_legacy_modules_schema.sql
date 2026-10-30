-- 008_legacy_modules_schema.sql
-- 历史模块建表统一迁移：goodser / shopchecker / login3rd / ia / webhook / servicenav
-- 由各模块原 initTables 逻辑收敛而来，CREATE TABLE IF NOT EXISTS 保证存量库安全
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

-- ---------- goodser ----------
CREATE TABLE IF NOT EXISTS zzz_goodser_inventories (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-库存';

CREATE TABLE IF NOT EXISTS zzz_goodser_products (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  code VARCHAR(50) NOT NULL,
  main_zone VARCHAR(10) NOT NULL,
  sub_zone VARCHAR(10) NOT NULL,
  seq_number INT NOT NULL,
  quantity INT NOT NULL DEFAULT 0,
  reserved_quantity INT NOT NULL DEFAULT 0,
  status_code VARCHAR(10) NOT NULL DEFAULT 'A',
  name VARCHAR(500) NOT NULL,
  original_price DECIMAL(12,2) NOT NULL DEFAULT 0,
  market_price DECIMAL(12,2) NOT NULL DEFAULT 0,
  expected_price DECIMAL(12,2) NOT NULL DEFAULT 0,
  remark TEXT,
  storage_location VARCHAR(255) DEFAULT '',
  image_url VARCHAR(1024) DEFAULT '',
  images JSON DEFAULT NULL,
  tags JSON DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_inventory_id (inventory_id),
  INDEX idx_code (inventory_id, code),
  INDEX idx_zone (inventory_id, main_zone, sub_zone),
  INDEX idx_seq_number (inventory_id, seq_number),
  FULLTEXT INDEX ft_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-商品';

CREATE TABLE IF NOT EXISTS zzz_goodser_recycled_seq_numbers (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  main_zone VARCHAR(10) NOT NULL,
  sub_zone VARCHAR(10) NOT NULL,
  seq_number INT NOT NULL,
  recycled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE INDEX idx_zone_seq (inventory_id, main_zone, sub_zone, seq_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-回收序号';

CREATE TABLE IF NOT EXISTS zzz_goodser_seq_counters (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  main_zone VARCHAR(10) NOT NULL,
  sub_zone VARCHAR(10) NOT NULL,
  current_max INT NOT NULL DEFAULT 0,
  UNIQUE INDEX idx_unique_zone (inventory_id, main_zone, sub_zone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-序号计数器';

CREATE TABLE IF NOT EXISTS zzz_goodser_outbound_orders (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  order_no VARCHAR(50) NOT NULL,
  type VARCHAR(20) NOT NULL,
  status VARCHAR(20) NOT NULL,
  order_info TEXT,
  remark TEXT,
  items JSON NOT NULL,
  source_reserve_id VARCHAR(36) DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  confirmed_at DATETIME DEFAULT NULL,
  cancelled_at DATETIME DEFAULT NULL,
  INDEX idx_inventory_id (inventory_id),
  INDEX idx_order_no (order_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-出库单';

CREATE TABLE IF NOT EXISTS zzz_goodser_inbound_logs (
  id VARCHAR(36) PRIMARY KEY,
  inventory_id VARCHAR(36) NOT NULL,
  order_no VARCHAR(50) DEFAULT '',
  type VARCHAR(20) NOT NULL,
  remark TEXT,
  items JSON NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_inventory_id (inventory_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-入库记录';

CREATE TABLE IF NOT EXISTS zzz_goodser_status_codes (
  id VARCHAR(36) PRIMARY KEY,
  code VARCHAR(10) NOT NULL,
  label VARCHAR(100) NOT NULL,
  is_system TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE INDEX idx_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-状态码';

CREATE TABLE IF NOT EXISTS zzz_goodser_tags (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  color VARCHAR(7) NOT NULL DEFAULT '#1890ff',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品服务-标签';

INSERT IGNORE INTO zzz_goodser_status_codes (id, code, label, is_system) VALUES
  ('sys-a', 'A', '正常', 1),
  ('sys-b', 'B', '预留', 1),
  ('sys-c', 'C', '已拆', 1),
  ('sys-d', 'D', '损坏', 1),
  ('sys-e', 'E', '过期', 1),
  ('sys-f', 'F', '停用', 1),
  ('sys-n', 'N', '全新', 1);

-- ---------- shopchecker ----------
CREATE TABLE IF NOT EXISTS zzz_shopchecker_products (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  platform VARCHAR(20) NOT NULL,
  url VARCHAR(2048) NOT NULL,
  image_url VARCHAR(1024) DEFAULT '',
  target_price DECIMAL(12,2) DEFAULT 0,
  price_drop_ratio DECIMAL(5,2) DEFAULT 0,
  last_price DECIMAL(12,2) DEFAULT 0,
  last_stock_status VARCHAR(20) NOT NULL DEFAULT 'unknown',
  last_checked_at DATETIME NULL,
  next_check_at DATETIME NULL,
  check_interval VARCHAR(20) NOT NULL DEFAULT '1h',
  cron_expr VARCHAR(100) DEFAULT '',
  custom_webhook_ids JSON NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  remark TEXT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_enabled_next_check (enabled, next_check_at),
  INDEX idx_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='店铺采集-监控商品';

CREATE TABLE IF NOT EXISTS zzz_shopchecker_price_records (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  product_id VARCHAR(36) NOT NULL,
  price DECIMAL(12,2) NOT NULL,
  original_price DECIMAL(12,2) DEFAULT 0,
  title VARCHAR(255) DEFAULT '',
  stock_status VARCHAR(20) NOT NULL DEFAULT 'unknown',
  is_notified TINYINT(1) NOT NULL DEFAULT 0,
  raw_data JSON NULL,
  error_msg VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_product_id (product_id),
  INDEX idx_created (created_at),
  INDEX idx_is_notified (is_notified)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='店铺采集-价格记录';

CREATE TABLE IF NOT EXISTS zzz_shopchecker_notification_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  product_id VARCHAR(36) NOT NULL,
  price_record_id BIGINT NOT NULL,
  webhook_ids JSON NULL,
  old_price DECIMAL(12,2) DEFAULT 0,
  new_price DECIMAL(12,2) NOT NULL,
  old_stock_status VARCHAR(20) NOT NULL DEFAULT '',
  new_stock_status VARCHAR(20) NOT NULL DEFAULT '',
  notify_type VARCHAR(20) NOT NULL DEFAULT 'price_drop',
  result_summary VARCHAR(20) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_product_id (product_id),
  INDEX idx_price_record_id (price_record_id),
  INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='店铺采集-通知记录';

-- ---------- login3rd ----------
CREATE TABLE IF NOT EXISTS login3rd_platforms (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  category VARCHAR(32) NOT NULL,
  login_type VARCHAR(32) NOT NULL DEFAULT 'browser',
  has_auto BOOLEAN NOT NULL DEFAULT FALSE,
  login_url TEXT,
  home_url TEXT,
  form_config JSON,
  icon_url TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='第三方登录-平台';

CREATE TABLE IF NOT EXISTS login3rd_accounts (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  platform_id VARCHAR(64) NOT NULL,
  label VARCHAR(128) NOT NULL,
  username VARCHAR(255),
  encrypted_password TEXT,
  encrypted_token TEXT,
  webhook_ids JSON,
  session_data JSON,
  login_method VARCHAR(32),
  session_expires_at TIMESTAMP NULL,
  last_verified_at TIMESTAMP NULL,
  verified_status VARCHAR(32) DEFAULT 'unknown',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  last_login_at TIMESTAMP NULL,
  last_error TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  FOREIGN KEY (platform_id) REFERENCES login3rd_platforms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='第三方登录-账号';

-- ---------- ia ----------
CREATE TABLE IF NOT EXISTS zzz_ia_accounts (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  type VARCHAR(20) NOT NULL DEFAULT 'broker',
  broker VARCHAR(100) DEFAULT '',
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  cost_method VARCHAR(10) NOT NULL DEFAULT 'wa',
  status VARCHAR(10) NOT NULL DEFAULT 'active',
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_type_status (type, status),
  INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 投资账户';

CREATE TABLE IF NOT EXISTS zzz_ia_assets (
  id VARCHAR(36) PRIMARY KEY,
  code VARCHAR(32) NOT NULL,
  name VARCHAR(100) NOT NULL,
  asset_type VARCHAR(20) NOT NULL,
  sub_type VARCHAR(20) DEFAULT '',
  market VARCHAR(20) NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  tracking_index VARCHAR(32) DEFAULT '',
  annual_rate DECIMAL(10,6) DEFAULT NULL,
  start_date DATE DEFAULT NULL,
  maturity_date DATE DEFAULT NULL,
  lock_days INT DEFAULT 0,
  risk_level VARCHAR(10) DEFAULT '',
  issuer VARCHAR(100) DEFAULT '',
  status VARCHAR(10) NOT NULL DEFAULT 'active',
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_code_market (code, market),
  INDEX idx_type_market (asset_type, market),
  INDEX idx_maturity (maturity_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 标的主数据';

CREATE TABLE IF NOT EXISTS zzz_ia_cash_flows (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id VARCHAR(36) NOT NULL,
  flow_type VARCHAR(20) NOT NULL,
  amount DECIMAL(18,4) NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  flow_date DATE NOT NULL,
  counter_account_id VARCHAR(36) DEFAULT NULL,
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_acct_date (account_id, flow_date),
  INDEX idx_type (flow_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 资金流水';

CREATE TABLE IF NOT EXISTS zzz_ia_transactions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) NOT NULL,
  trade_type VARCHAR(20) NOT NULL,
  shares DECIMAL(20,6) DEFAULT 0,
  price DECIMAL(18,6) DEFAULT 0,
  amount DECIMAL(18,4) DEFAULT 0,
  fee DECIMAL(18,4) DEFAULT 0,
  fee_detail JSON DEFAULT NULL,
  tax DECIMAL(18,4) DEFAULT 0,
  trade_at DATETIME NOT NULL,
  settle_date DATE DEFAULT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'confirmed',
  method VARCHAR(10) DEFAULT '',
  realized_pnl DECIMAL(18,4) DEFAULT NULL,
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_acct_asset_status (account_id, asset_id, status),
  INDEX idx_trade_at (trade_at),
  INDEX idx_type (trade_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 交易流水';

CREATE TABLE IF NOT EXISTS zzz_ia_fees (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) DEFAULT NULL,
  fee_type VARCHAR(20) NOT NULL,
  amount DECIMAL(18,4) NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  charge_date DATE NOT NULL,
  period VARCHAR(20) DEFAULT '',
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_acct_date (account_id, charge_date),
  INDEX idx_type (fee_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 独立费用';

CREATE TABLE IF NOT EXISTS zzz_ia_positions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) NOT NULL,
  shares DECIMAL(20,6) NOT NULL DEFAULT 0,
  available_shares DECIMAL(20,6) NOT NULL DEFAULT 0,
  locked_shares DECIMAL(20,6) NOT NULL DEFAULT 0,
  avg_cost DECIMAL(18,6) NOT NULL DEFAULT 0,
  cost_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
  last_price DECIMAL(18,6) DEFAULT NULL,
  last_price_at DATETIME DEFAULT NULL,
  last_price_source VARCHAR(20) DEFAULT '',
  target_price DECIMAL(18,6) DEFAULT NULL,
  stop_loss_price DECIMAL(18,6) DEFAULT NULL,
  target_set_at DATETIME DEFAULT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
  status VARCHAR(10) NOT NULL DEFAULT 'holding',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_acct_asset (account_id, asset_id),
  INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 实时持仓';

CREATE TABLE IF NOT EXISTS zzz_ia_price_history (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  asset_id VARCHAR(36) NOT NULL,
  nav_date DATE NOT NULL,
  nav DECIMAL(18,6) NOT NULL,
  source VARCHAR(20) NOT NULL DEFAULT 'official',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_asset_date (asset_id, nav_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 价格净值历史';

CREATE TABLE IF NOT EXISTS zzz_ia_portfolio_snapshots (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  snap_date DATE NOT NULL,
  account_id VARCHAR(36) DEFAULT NULL,
  total_cost DECIMAL(20,4) DEFAULT 0,
  total_value DECIMAL(20,4) DEFAULT 0,
  total_pnl DECIMAL(20,4) DEFAULT 0,
  cash_amount DECIMAL(20,4) DEFAULT 0,
  by_type JSON DEFAULT NULL,
  by_currency JSON DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_date_acct (snap_date, account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 组合快照';

CREATE TABLE IF NOT EXISTS zzz_ia_realized_pnl (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) NOT NULL,
  transaction_id BIGINT NOT NULL,
  lot_transaction_id BIGINT DEFAULT NULL,
  method VARCHAR(10) NOT NULL DEFAULT 'wa',
  shares DECIMAL(20,6) NOT NULL DEFAULT 0,
  sell_price DECIMAL(18,6) NOT NULL DEFAULT 0,
  cost_price DECIMAL(18,6) NOT NULL DEFAULT 0,
  gross_pnl DECIMAL(18,4) NOT NULL DEFAULT 0,
  fee DECIMAL(18,4) NOT NULL DEFAULT 0,
  tax DECIMAL(18,4) NOT NULL DEFAULT 0,
  net_pnl DECIMAL(18,4) NOT NULL DEFAULT 0,
  realized_at DATE NOT NULL,
  INDEX idx_acct_asset (account_id, asset_id),
  INDEX idx_tx (transaction_id),
  INDEX idx_date (realized_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 已实现收益台账';

CREATE TABLE IF NOT EXISTS zzz_ia_dca_plans (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  account_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) NOT NULL,
  amount DECIMAL(18,4) NOT NULL,
  cycle VARCHAR(10) NOT NULL,
  exec_day INT NOT NULL DEFAULT 1,
  start_date DATE NOT NULL,
  end_date DATE DEFAULT NULL,
  next_exec_date DATE NOT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'active',
  auto_buy TINYINT(1) NOT NULL DEFAULT 0,
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_status_next (status, next_exec_date),
  INDEX idx_acct (account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 定投计划';

CREATE TABLE IF NOT EXISTS zzz_ia_fx_rates (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  base_ccy VARCHAR(8) NOT NULL,
  quote_ccy VARCHAR(8) NOT NULL,
  rate DECIMAL(16,8) NOT NULL,
  rate_date DATE NOT NULL,
  source VARCHAR(10) NOT NULL DEFAULT 'manual',
  UNIQUE KEY uk_pair_date (base_ccy, quote_ccy, rate_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 汇率';

CREATE TABLE IF NOT EXISTS zzz_ia_allocations (
  id VARCHAR(36) PRIMARY KEY,
  account_id VARCHAR(36) DEFAULT NULL,
  category VARCHAR(20) NOT NULL,
  target_ratio DECIMAL(7,6) NOT NULL,
  tolerance DECIMAL(7,6) NOT NULL DEFAULT 0.05,
  status VARCHAR(10) NOT NULL DEFAULT 'active',
  remark VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_scope_status (account_id, status),
  UNIQUE KEY uk_scope_category (account_id, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 资产配比目标';

CREATE TABLE IF NOT EXISTS zzz_ia_price_alerts (
  id VARCHAR(36) PRIMARY KEY,
  account_id VARCHAR(36) NOT NULL,
  asset_id VARCHAR(36) NOT NULL,
  alert_type VARCHAR(20) NOT NULL,
  target_price DECIMAL(18,6) NOT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'active',
  triggered_at DATETIME DEFAULT NULL,
  triggered_price DECIMAL(18,6) DEFAULT NULL,
  note VARCHAR(500) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_status_asset (status, asset_id),
  INDEX idx_acct (account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='zzz-IA 价格告警';

-- ---------- webhook ----------
CREATE TABLE IF NOT EXISTS webhook_configs (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  type VARCHAR(20) NOT NULL,
  url VARCHAR(2048) NOT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'active',
  tags JSON DEFAULT NULL,
  description VARCHAR(500) DEFAULT '',
  source VARCHAR(50) NOT NULL DEFAULT 'system',
  app VARCHAR(50) DEFAULT '',
  signing_key VARCHAR(255) DEFAULT '',
  access_token VARCHAR(255) DEFAULT '',
  auth_type VARCHAR(20) DEFAULT '',
  auth_value TEXT DEFAULT NULL,
  custom_headers JSON DEFAULT NULL,
  whitelist_keywords JSON DEFAULT NULL,
  whitelist_mode VARCHAR(10) DEFAULT 'any',
  max_per_minute INT DEFAULT 60,
  max_per_hour INT DEFAULT 1000,
  dedup_window INT DEFAULT 0,
  max_retries INT DEFAULT 3,
  retry_interval INT DEFAULT 30,
  timeout_ms INT DEFAULT 10000,
  last_sent_at DATETIME DEFAULT NULL,
  last_success_at DATETIME DEFAULT NULL,
  last_error VARCHAR(500) DEFAULT '',
  total_sent INT DEFAULT 0,
  total_failed INT DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_status (status),
  INDEX idx_type (type),
  INDEX idx_source_app (source, app)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Webhook 配置';

CREATE TABLE IF NOT EXISTS webhook_delivery_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  webhook_id VARCHAR(36) NOT NULL,
  source VARCHAR(50) NOT NULL,
  title VARCHAR(255) DEFAULT '',
  content_hash VARCHAR(64) NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  request_body TEXT DEFAULT NULL,
  response_code INT DEFAULT 0,
  response_body TEXT DEFAULT NULL,
  error_msg VARCHAR(500) DEFAULT '',
  retry_count INT DEFAULT 0,
  duration_ms INT DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_webhook_status (webhook_id, status),
  INDEX idx_content_hash (content_hash, created_at),
  INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Webhook 投递日志';

-- ---------- servicenav ----------
CREATE TABLE IF NOT EXISTS services_nav_type (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(50) NOT NULL,
  remark VARCHAR(200) DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务导航-类型';

CREATE TABLE IF NOT EXISTS services_nav_item (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  types JSON NOT NULL,
  protocol VARCHAR(20) NOT NULL DEFAULT 'http',
  address VARCHAR(2048) DEFAULT '',
  intranet_port INT NULL,
  easytier_port INT NULL,
  account VARCHAR(255) DEFAULT '',
  password_hint VARCHAR(255) DEFAULT '',
  remark VARCHAR(500) DEFAULT '',
  project_id BIGINT NULL,
  container_id BIGINT NULL,
  access_point_id BIGINT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务导航-条目';
