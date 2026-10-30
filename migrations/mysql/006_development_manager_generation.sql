-- 006_development_manager_generation.sql
-- 项目开发管理模块（developmentManager）P5 生成流程：按需求生成方案/代码/文档的任务记录
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

CREATE TABLE IF NOT EXISTS zzz_development_manager_generation (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  requirement_id BIGINT NOT NULL COMMENT '需求 ID（zzz_development_manager_requirement.id）',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  kind VARCHAR(16) NOT NULL COMMENT '生成类型 plan=方案 / code=代码 / doc=文档',
  engine VARCHAR(16) NOT NULL COMMENT '请求引擎 ai / template / manual',
  engine_used VARCHAR(16) NOT NULL DEFAULT '' COMMENT '实际使用引擎（AI 不可用时自动降级为 template）',
  auto_commit TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否请求自动提交（默认 0 关，仅 kind=code 生效）',
  committed TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已自动提交 0=否 1=是',
  branch VARCHAR(128) NOT NULL DEFAULT '' COMMENT '自动提交所在分支',
  commit_hash VARCHAR(64) NOT NULL DEFAULT '' COMMENT '自动提交的 commit hash',
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT '任务状态 pending/running/success/failed',
  input_snapshot JSON NULL COMMENT '输入快照（需求字段/项目/请求引擎/补充说明）',
  output_ref VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '产物引用（文件路径/目录/分支+commit）',
  files JSON NULL COMMENT '生成文件相对路径数组',
  error VARCHAR(2000) NOT NULL DEFAULT '' COMMENT '失败原因（自动提交失败也记录在此，产物仍保留）',
  operator VARCHAR(64) NOT NULL DEFAULT '' COMMENT '触发人用户名',
  started_at DATETIME NULL COMMENT '开始执行时间',
  finished_at DATETIME NULL COMMENT '执行结束时间',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_dm_generation_requirement (requirement_id, id),
  KEY idx_dm_generation_project (project_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生成任务：记录按需求生成方案/代码/文档的请求、实际引擎、产物与提交结果';
