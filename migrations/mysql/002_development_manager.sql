-- 002_development_manager.sql
-- 项目开发管理模块（developmentManager）P1 数据表：项目台账 / 项目关联 / 归类资产索引
-- 表前缀规范见 docs/data-model.md：基座表无前缀，模块表统一 zzz_<模块名>_ 前缀
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

CREATE TABLE IF NOT EXISTS zzz_development_manager_project (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_key VARCHAR(64) NOT NULL COMMENT '项目唯一键（目录与接口引用用，如 go-backend-core）',
  name VARCHAR(128) NOT NULL COMMENT '项目名称',
  type VARCHAR(32) NOT NULL COMMENT '项目类型 base=后端基座 / template=前端模板 / backend_module=后端模块 / frontend_module=前端模块 / frontend_base=前端基座',
  description TEXT NULL COMMENT '项目简介',
  repo_url VARCHAR(255) NOT NULL DEFAULT '' COMMENT '代码仓库地址（可空）',
  repo_path VARCHAR(255) NOT NULL DEFAULT '' COMMENT '仓库本地路径（可空，相对仓库根目录）',
  default_branch VARCHAR(64) NOT NULL DEFAULT 'main' COMMENT '默认分支名',
  tech_stack JSON NULL COMMENT '技术栈摘要数组（语言/框架/数据库），仅作展示',
  status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT '状态 active=启用 / archived=归档',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  updated_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '最后修改人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY uk_pdm_project_key (project_key),
  KEY idx_pdm_type_status (type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目台账：登记受管项目（基座/模板/模块）的基本信息与仓库位置，删除时软删除并级联标记关联';

CREATE TABLE IF NOT EXISTS zzz_development_manager_project_relation (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  from_project_id BIGINT NOT NULL COMMENT '关联发起方项目 ID（zzz_development_manager_project.id）',
  to_project_id BIGINT NOT NULL COMMENT '关联接收方项目 ID（zzz_development_manager_project.id）',
  relation_type VARCHAR(32) NOT NULL COMMENT '关联类型 uses_base=绑定基座 / uses_template=绑定模板 / binds_backend_module=绑定后端模块 / related=其他横向关联',
  note VARCHAR(500) NOT NULL DEFAULT '' COMMENT '关联说明',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记，删除任一端项目时同步置 1',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY uk_pdm_relation (from_project_id, to_project_id, relation_type),
  KEY idx_pdm_rel_to (to_project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目关联：记录项目间基座/模板/模块绑定关系，多对多，删除项目时级联软删除关联';

CREATE TABLE IF NOT EXISTS zzz_development_manager_project_asset (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  category VARCHAR(16) NOT NULL COMMENT '资产类别 requirement=需求 / code=代码 / doc=文档 / asset=附件资产',
  title VARCHAR(255) NOT NULL COMMENT '资产标题',
  uri VARCHAR(1024) NOT NULL COMMENT '资产定位（仓库相对路径 或 fileref ID）',
  format VARCHAR(32) NOT NULL DEFAULT '' COMMENT '文件格式（md/png/zip 等，可空）',
  version VARCHAR(32) NOT NULL DEFAULT '' COMMENT '资产版本号（可空）',
  size BIGINT NOT NULL DEFAULT 0 COMMENT '文件大小字节数（未知为 0）',
  checksum VARCHAR(128) NOT NULL DEFAULT '' COMMENT '校验和（可空）',
  note VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '备注',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_pdm_asset_project (project_id, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='归类资产索引：需求/代码/文档/资产按项目归档的路径索引，文件本体不入库';
