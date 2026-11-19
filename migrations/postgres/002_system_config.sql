-- Migration 2: System config center
-- Created at 2026-11-19T00:00:00Z
-- 运行时可配置项统一存 PG；敏感项（如业务账号）由应用加密后写入 value。

CREATE TABLE IF NOT EXISTS system_config (
    key         TEXT PRIMARY KEY,
    value       JSONB NOT NULL DEFAULT 'null'::jsonb,
    version     BIGINT NOT NULL DEFAULT 0,
    checksum    CHAR(64) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    updated_by  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE system_config IS '配置中心当前值；替代 env 中的业务参数/账号';
COMMENT ON COLUMN system_config.key IS '配置项 key，与代码内 Spec 对应';
COMMENT ON COLUMN system_config.value IS 'JSONB 值；敏感项为密文';
COMMENT ON COLUMN system_config.version IS '乐观版本号，每次写入 +1';
COMMENT ON COLUMN system_config.checksum IS 'value 的 SHA-256，用于审计比对';
COMMENT ON COLUMN system_config.updated_by IS '最后修改人';

CREATE INDEX IF NOT EXISTS idx_system_config_updated_at
    ON system_config (updated_at DESC);

CREATE TABLE IF NOT EXISTS system_config_history (
    id         BIGSERIAL PRIMARY KEY,
    key        TEXT NOT NULL,
    old_value  JSONB,
    new_value  JSONB,
    version    BIGINT NOT NULL DEFAULT 0,
    operator   TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE system_config_history IS '配置变更历史，按 key 追加，不可覆盖';
COMMENT ON COLUMN system_config_history.operator IS '操作人 user id；系统任务可为空';

CREATE INDEX IF NOT EXISTS idx_system_config_history_key_id
    ON system_config_history (key, id DESC);
CREATE INDEX IF NOT EXISTS idx_system_config_history_created_at
    ON system_config_history (created_at DESC);
