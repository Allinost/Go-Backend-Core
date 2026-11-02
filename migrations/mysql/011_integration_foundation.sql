-- 011_integration_foundation.sql
-- 集成基座 P0：外盒事件、连接器配置、动作调用日志
-- 迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号
--
-- 设计说明：
--   outbox 事件与业务写操作同事务落库，分发器异步投递
--   connectors 保存外部系统的连接配置，敏感字段由应用层加密后写入
--   action_logs 记录外部动作调用，用于审计、排障与限流分析

-- ---------- 1. 外盒事件 ----------
CREATE TABLE IF NOT EXISTS zzz_integration_outbox (
  id VARCHAR(36) PRIMARY KEY,
  topic VARCHAR(100) NOT NULL,
  source VARCHAR(50) NOT NULL DEFAULT '',
  aggregate_type VARCHAR(50) NOT NULL DEFAULT '',
  aggregate_id VARCHAR(64) NOT NULL DEFAULT '',
  payload JSON NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1000) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  delivered_at DATETIME DEFAULT NULL,
  INDEX idx_status_created (status, created_at),
  INDEX idx_topic_created (topic, created_at),
  INDEX idx_aggregate (aggregate_type, aggregate_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='集成基座-外盒事件';

-- ---------- 2. 连接器配置 ----------
CREATE TABLE IF NOT EXISTS zzz_integration_connectors (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(100) NOT NULL DEFAULT '',
  type VARCHAR(30) NOT NULL,
  config JSON NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_type_enabled (type, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='集成基座-连接器配置';

-- ---------- 3. 动作调用日志 ----------
CREATE TABLE IF NOT EXISTS zzz_integration_action_logs (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  action VARCHAR(80) NOT NULL,
  actor VARCHAR(100) NOT NULL DEFAULT '',
  auth_method VARCHAR(20) NOT NULL DEFAULT '',
  api_key_id BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT '',
  params JSON,
  result JSON,
  error VARCHAR(1000) NOT NULL DEFAULT '',
  duration_ms INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_action_created (action, created_at),
  INDEX idx_actor_created (actor, created_at),
  INDEX idx_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='集成基座-动作调用日志';
