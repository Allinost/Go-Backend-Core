-- 005_development_manager_requirement.sql
-- 项目开发管理模块（developmentManager）P4 需求管理：需求登记 + 版本快照
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

CREATE TABLE IF NOT EXISTS zzz_development_manager_requirement (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  code VARCHAR(32) NOT NULL COMMENT '需求编号（项目内唯一，格式 RQM-0001）',
  title VARCHAR(200) NOT NULL COMMENT '需求标题',
  content MEDIUMTEXT NULL COMMENT '需求正文（markdown）',
  type VARCHAR(16) NOT NULL DEFAULT 'feature' COMMENT '需求类型 feature=新功能 / change=变更 / fix=修复 / ops=运维',
  priority VARCHAR(8) NOT NULL DEFAULT 'P2' COMMENT '优先级 P0=最高 / P1 / P2 / P3=最低',
  status VARCHAR(16) NOT NULL DEFAULT 'draft' COMMENT '状态 draft=草稿 / reviewing=评审中 / approved=已通过 / planned=已排期 / developing=开发中 / done=已完成 / closed=已关闭',
  source VARCHAR(16) NOT NULL DEFAULT 'manual' COMMENT '需求来源 manual=人工 / ai=AI / customer=客户',
  owner VARCHAR(64) NOT NULL DEFAULT '' COMMENT '负责人用户名',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  updated_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '最后修改人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY uk_dm_requirement_code (project_id, code),
  KEY idx_dm_requirement_project (project_id, deleted, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='需求：项目级需求登记与状态流转，增改删均产生版本快照';

CREATE TABLE IF NOT EXISTS zzz_development_manager_requirement_version (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  requirement_id BIGINT NOT NULL COMMENT '需求 ID（zzz_development_manager_requirement.id）',
  version_no INT NOT NULL COMMENT '版本号（从 1 递增）',
  title VARCHAR(200) NOT NULL COMMENT '该版本的标题快照',
  content MEDIUMTEXT NULL COMMENT '该版本的正文快照',
  status VARCHAR(16) NOT NULL COMMENT '该版本的状态快照（含 deleted=需求已删除）',
  change_note VARCHAR(500) NOT NULL DEFAULT '' COMMENT '本次变更说明',
  operator VARCHAR(64) NOT NULL DEFAULT '' COMMENT '操作人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '生成时间',
  UNIQUE KEY uk_dm_requirement_version (requirement_id, version_no),
  KEY idx_dm_requirement_version_req (requirement_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='需求版本：每次创建/修改/状态流转/删除都追加一条快照，保证可回溯';
