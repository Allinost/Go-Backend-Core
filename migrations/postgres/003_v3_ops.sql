-- Migration 3: Ops host inventory and manual maintenance
-- Created at 2026-11-19T00:00:00Z
-- 主机台账 + L1/L2 手动维护状态；维护切换由运维显式触发，不做自动 failover。

CREATE TABLE IF NOT EXISTS zzz_ops_host (
    id                    BIGSERIAL PRIMARY KEY,
    name                  VARCHAR(64) NOT NULL,
    role                  VARCHAR(16) NOT NULL DEFAULT 'host',
    environment           VARCHAR(16) NOT NULL DEFAULT 'prod',
    address               VARCHAR(255) NOT NULL DEFAULT '',
    ssh_port              INTEGER NOT NULL DEFAULT 22,
    username              VARCHAR(64) NOT NULL DEFAULT '',
    auth_type             VARCHAR(16) NOT NULL DEFAULT 'key',
    credential_ciphertext TEXT NOT NULL DEFAULT '',
    credential_hint       VARCHAR(32) NOT NULL DEFAULT '',
    os                    VARCHAR(128) NOT NULL DEFAULT '',
    tags                  JSONB NOT NULL DEFAULT '[]'::jsonb,
    notes                 TEXT NOT NULL DEFAULT '',
    status                VARCHAR(16) NOT NULL DEFAULT 'unknown',
    last_checked_at       TIMESTAMPTZ,
    last_error            TEXT NOT NULL DEFAULT '',
    deleted               BOOLEAN NOT NULL DEFAULT false,
    created_by            TEXT NOT NULL DEFAULT '',
    updated_by            TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE zzz_ops_host IS 'V3 主机台账：cloud/host/backup；凭据仅存密文';
COMMENT ON COLUMN zzz_ops_host.role IS 'cloud 云服务器 / host 主机 / backup 备份机';
COMMENT ON COLUMN zzz_ops_host.credential_ciphertext IS 'SSH 密码/私钥密文（AES-GCM），接口只回 hint';
COMMENT ON COLUMN zzz_ops_host.status IS 'unknown / online / offline / error';

CREATE UNIQUE INDEX IF NOT EXISTS uq_zzz_ops_host_name
    ON zzz_ops_host (name) WHERE deleted = false;
CREATE INDEX IF NOT EXISTS idx_zzz_ops_host_role
    ON zzz_ops_host (role) WHERE deleted = false;

CREATE TABLE IF NOT EXISTS zzz_ops_maintenance (
    id                  BIGSERIAL PRIMARY KEY,
    mode                VARCHAR(32) NOT NULL,
    status              VARCHAR(32) NOT NULL DEFAULT 'in_progress',
    source_node         VARCHAR(16) NOT NULL DEFAULT 'L1',
    target_node         VARCHAR(16) NOT NULL DEFAULT 'L2',
    switched_to_l2      BOOLEAN NOT NULL DEFAULT false,
    sync_confirmed      BOOLEAN NOT NULL DEFAULT false,
    sync_back_confirmed BOOLEAN NOT NULL DEFAULT false,
    note                TEXT NOT NULL DEFAULT '',
    started_by          TEXT NOT NULL DEFAULT '',
    started_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_by            TEXT NOT NULL DEFAULT '',
    ended_at            TIMESTAMPTZ,
    end_note            TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE zzz_ops_maintenance IS 'L1/L2 手动维护状态机；同时只允许一条 in_progress';
COMMENT ON COLUMN zzz_ops_maintenance.mode IS 'shutdown / migrate_return / migrate_no_return';
COMMENT ON COLUMN zzz_ops_maintenance.status IS 'in_progress / completed / aborted';
COMMENT ON COLUMN zzz_ops_maintenance.target_node IS '维护期间的运行节点；shutdown 时仍为 L1';
COMMENT ON COLUMN zzz_ops_maintenance.sync_confirmed IS '开始维护前已人工完成 L1→L2 同步';
COMMENT ON COLUMN zzz_ops_maintenance.sync_back_confirmed IS '结束维护前已人工完成 L2→L1 反向同步';

-- 同一时间只允许一个进行中的维护任务。
CREATE UNIQUE INDEX IF NOT EXISTS uq_zzz_ops_maintenance_active
    ON zzz_ops_maintenance (status) WHERE status = 'in_progress';
CREATE INDEX IF NOT EXISTS idx_zzz_ops_maintenance_started_at
    ON zzz_ops_maintenance (started_at DESC);
