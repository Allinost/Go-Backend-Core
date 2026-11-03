-- 集成基座：动作调用日志记录幂等键与重放状态
ALTER TABLE zzz_integration_action_logs
  ADD COLUMN request_id VARCHAR(100) NOT NULL DEFAULT '' AFTER params,
  ADD COLUMN replayed TINYINT(1) NOT NULL DEFAULT 0 AFTER request_id,
  ADD INDEX idx_request_id (request_id);
