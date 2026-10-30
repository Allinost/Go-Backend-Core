-- 003_development_manager_deployment.sql
-- 项目开发管理模块（developmentManager）P2 部署资产：容器 / 资源分配 / 卷映射 / 网络接入点 / 白名单组与成员 / 备份
-- 建表顺序：container → resource_alloc/volume_mapping → whitelist → whitelist_member → access_point → backup（满足外键引用）
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

CREATE TABLE IF NOT EXISTS zzz_development_manager_container (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  container_name VARCHAR(128) NOT NULL COMMENT '容器名（项目内唯一，如 developmentmanager-api）',
  image VARCHAR(255) NOT NULL DEFAULT '' COMMENT '镜像名与标签（如 registry.local/app:1.2.0）',
  compose_ref VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'docker-compose 文件或服务名引用',
  replicas INT NOT NULL DEFAULT 1 COMMENT '副本数',
  network_mode VARCHAR(64) NOT NULL DEFAULT '' COMMENT '网络模式（bridge/host/custom 网络名）',
  env_notes JSON NULL COMMENT '关键环境变量说明（JSON 对象，仅登记不含敏感值）',
  status_note VARCHAR(255) NOT NULL DEFAULT '' COMMENT '运行状态备注（如 常驻 / 按需启动）',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  updated_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '最后修改人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_dm_container_project (project_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='项目关联容器：一个项目可关联多个容器，记录镜像、compose 引用与网络模式';

CREATE TABLE IF NOT EXISTS zzz_development_manager_resource_alloc (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  container_id BIGINT NOT NULL COMMENT '容器 ID（zzz_development_manager_container.id，1:1）',
  cpu_limit VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'CPU 上限（如 2 / 1.5 / 500m）',
  cpu_shares INT NOT NULL DEFAULT 0 COMMENT 'CPU 权重（相对份额，0=未设置）',
  mem_limit VARCHAR(32) NOT NULL DEFAULT '' COMMENT '内存上限（如 2g / 512m）',
  mem_reservation VARCHAR(32) NOT NULL DEFAULT '' COMMENT '内存预留（软限制）',
  gpu_note VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'GPU 分配说明（如 GPU0 显存 8G）',
  storage_quota VARCHAR(64) NOT NULL DEFAULT '' COMMENT '存储配额（如 100G）',
  notes VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '其他资源说明',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  UNIQUE KEY uk_dm_alloc_container (container_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='资源分配：与容器 1:1，独立表记录 CPU/内存/GPU/存储配额';

CREATE TABLE IF NOT EXISTS zzz_development_manager_volume_mapping (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  container_id BIGINT NOT NULL COMMENT '容器 ID（zzz_development_manager_container.id）',
  host_path VARCHAR(512) NOT NULL COMMENT '宿主目录路径',
  container_path VARCHAR(512) NOT NULL COMMENT '容器内挂载路径',
  mode VARCHAR(8) NOT NULL DEFAULT 'rw' COMMENT '挂载模式 rw=读写 / ro=只读',
  purpose VARCHAR(255) NOT NULL DEFAULT '' COMMENT '挂载用途（如 配置 / 数据 / 日志）',
  in_backup TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否纳入备份 0=否 1=是',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_dm_volume_container (container_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='卷映射：记录宿主目录与容器目录的挂载关系及是否纳入备份';

CREATE TABLE IF NOT EXISTS zzz_development_manager_whitelist (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  name VARCHAR(128) NOT NULL COMMENT '白名单组名（项目内唯一，如 ia-market-allow）',
  note VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '组用途说明',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  updated_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '最后修改人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_dm_whitelist_project (project_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='常用白名单组：可复用、可命名，被多条网络接入点引用，允许 0 成员';

CREATE TABLE IF NOT EXISTS zzz_development_manager_whitelist_member (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  whitelist_id BIGINT NOT NULL COMMENT '白名单组 ID（zzz_development_manager_whitelist.id）',
  container_id BIGINT NOT NULL COMMENT '允许访问的容器 ID（zzz_development_manager_container.id）',
  note VARCHAR(255) NOT NULL DEFAULT '' COMMENT '成员说明',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE KEY uk_dm_whitelist_member (whitelist_id, container_id),
  KEY idx_dm_member_container (container_id),
  CONSTRAINT fk_dm_member_whitelist FOREIGN KEY (whitelist_id) REFERENCES zzz_development_manager_whitelist (id),
  CONSTRAINT fk_dm_member_container FOREIGN KEY (container_id) REFERENCES zzz_development_manager_container (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='白名单成员：组到容器的允许关系（行式 + 外键），容器改名自动跟随';

CREATE TABLE IF NOT EXISTS zzz_development_manager_access_point (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  container_id BIGINT NULL COMMENT '宿主容器 ID（可空，宿主直连服务不填）',
  listen_port INT NOT NULL COMMENT '容器内监听端口（1-65535）',
  host_port INT NULL COMMENT '宿主映射端口（NULL=不映射）',
  protocol VARCHAR(8) NOT NULL DEFAULT 'tcp' COMMENT '协议 tcp / udp',
  purpose VARCHAR(255) NOT NULL DEFAULT '' COMMENT '端口用途（如 API / 管理台 / 采集回调）',
  exposure VARCHAR(16) NOT NULL DEFAULT 'intranet' COMMENT '暴露范围 none=不暴露 / intranet=内网 / extranet=公网',
  proxy_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT '反代类型 nginx/caddy/traefik/other，空=无反代直连',
  domain VARCHAR(255) NOT NULL DEFAULT '' COMMENT '反代域名',
  upstream_url VARCHAR(512) NOT NULL DEFAULT '' COMMENT '反代上游地址（如 http://container:8080）',
  tls TINYINT(1) NOT NULL DEFAULT 0 COMMENT '反代是否启用 TLS 0=否 1=是',
  network_zone VARCHAR(16) NOT NULL DEFAULT 'intranet' COMMENT '访问地址所属网络 intranet=内网 / extranet=公网 / proxy_net=反代网络',
  url VARCHAR(1024) NULL COMMENT '完整访问地址（NULL=该地址不可达，前端置灰）',
  reachable TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否可达 0=不可达 1=可达（与 url 是否有值联动）',
  notes VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '备注',
  whitelist_id BIGINT NULL COMMENT '引用的白名单组 ID（NULL=不限制访问）',
  deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '软删除标记 0=正常 1=已删除',
  deleted_at DATETIME NULL COMMENT '软删除时间',
  created_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人用户名',
  updated_by VARCHAR(64) NOT NULL DEFAULT '' COMMENT '最后修改人用户名',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_dm_ap_project (project_id, deleted),
  KEY idx_dm_ap_whitelist (whitelist_id),
  KEY idx_dm_ap_container (container_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='网络接入点：端口 + 反代 + 访问地址 + 白名单引用四合一，行粒度=端口×访问地址';

CREATE TABLE IF NOT EXISTS zzz_development_manager_backup (
  id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '主键',
  project_id BIGINT NOT NULL COMMENT '所属项目 ID（zzz_development_manager_project.id）',
  container_id BIGINT NULL COMMENT '关联容器 ID（可空）',
  volume_id BIGINT NULL COMMENT '关联卷 ID（可空，zzz_development_manager_volume_mapping.id）',
  backup_type VARCHAR(16) NOT NULL COMMENT '备份类型 full=整机 / db=数据库 / volume=卷',
  strategy_cron VARCHAR(64) NOT NULL DEFAULT '' COMMENT '备份策略 cron 表达式（如 0 3 * * *）',
  destination VARCHAR(512) NOT NULL DEFAULT '' COMMENT '备份目标位置（本地路径/对象存储）',
  retention_days INT NOT NULL DEFAULT 0 COMMENT '保留天数（0=不限）',
  last_run_at DATETIME NULL COMMENT '最近执行时间',
  last_status VARCHAR(16) NOT NULL DEFAULT '' COMMENT '最近执行结果 success/failed/running，空=未执行',
  last_error VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '最近失败原因',
  restore_note VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '恢复步骤说明',
  enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用 0=停用 1=启用',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  KEY idx_dm_backup_project (project_id, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='备份情况：记录备份策略、目标、保留期与最近执行结果';

-- 同步项目类型注释（Q12 新增 frontend_base，兼容 002 已应用或未应用的两种情况）
ALTER TABLE zzz_development_manager_project MODIFY COLUMN type VARCHAR(32) NOT NULL COMMENT '项目类型 base=后端基座 / template=前端模板 / backend_module=后端模块 / frontend_module=前端模块 / frontend_base=前端基座';
