-- 004_development_manager_dictionary.sql
-- 项目开发管理模块（developmentManager）P3 数据表字典：数据表登记 + 字段字典
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

CREATE TABLE IF NOT EXISTS zzz_development_manager_db_table (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  db_type VARCHAR(16) NOT NULL DEFAULT 'mysql' COMMENT '数据库类型 mysql/postgres（本期仅 mysql 支持扫描导入）',
  table_schema VARCHAR(64) NOT NULL DEFAULT '' COMMENT '库名（空=创建时取当前连接默认库）',
  table_name VARCHAR(128) NOT NULL COMMENT '表名',
  purpose VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '表用途说明',
  status VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT '状态 active=使用中 / deprecated=已废弃 / dropped=已删除',
  module_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '归属模块（如 developmentmanager）',
  created_note VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '建表说明/备注',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  updated_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '最后修改人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY uk_dm_db_table (project_id, table_schema, table_name),
  KEY idx_dm_db_table_project (project_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据表登记：记录项目数据表、用途、状态与归属模块，供数据字典展示';

CREATE TABLE IF NOT EXISTS zzz_development_manager_db_column (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  table_id BIGINT NOT NULL COMMENT '数据表 ID（zzz_development_manager_db_table.id）',
  column_name VARCHAR(128) NOT NULL COMMENT '字段名',
  data_type VARCHAR(128) NOT NULL DEFAULT '' COMMENT '字段类型（如 bigint / varchar(64)）',
  nullable TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否可空 0=否 1=是',
  default_value VARCHAR(255) NULL COMMENT '默认值（NULL=无默认值）',
  meaning VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '字段含义（扫描导入时取列注释作为初稿，人工可改）',
  example VARCHAR(255) NOT NULL DEFAULT '' COMMENT '示例值',
  is_pk TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否主键 0=否 1=是',
  is_index TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否索引列（主键/唯一/普通索引） 0=否 1=是',
  note VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '备注',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY uk_dm_db_column (table_id, column_name),
  KEY idx_dm_db_column_table (table_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='字段字典：记录表内字段的类型、可空、默认值与业务含义，可按 information_schema 扫描导入初稿';
