-- 007_v3_table_comments.sql
-- 为 V3 基座/公共表补充表注释与核心字段注释，便于数据库运维和数据字典展示。

COMMENT ON TABLE users IS '用户账号';
COMMENT ON COLUMN users.username IS '登录名，唯一';
COMMENT ON COLUMN users.email IS '邮箱，可为空';
COMMENT ON COLUMN users.status IS '状态：active/disabled';
COMMENT ON COLUMN users.deleted_at IS '软删除时间';

COMMENT ON TABLE roles IS '角色';
COMMENT ON COLUMN roles.name IS '角色标识，权限关联使用';
COMMENT ON TABLE permissions IS '权限点';
COMMENT ON COLUMN permissions.resource IS '资源标识，如 dm.task';
COMMENT ON COLUMN permissions.action IS '动作，如 read/create/update/delete';
COMMENT ON TABLE role_permissions IS '角色-权限关联';
COMMENT ON TABLE user_roles IS '用户-角色关联';

COMMENT ON TABLE api_keys IS 'API Key';
COMMENT ON COLUMN api_keys.key_prefix IS 'Key 前缀，用于展示与检索';
COMMENT ON COLUMN api_keys.key_hash IS 'Key 哈希，不存明文';
COMMENT ON COLUMN api_keys.scopes IS '允许的权限范围，默认 *:*';
COMMENT ON COLUMN api_keys.status IS '状态：active/revoked';

COMMENT ON TABLE scheduler_tasks IS '调度任务定义';
COMMENT ON COLUMN scheduler_tasks.expression IS 'Cron 表达式或间隔表达式';
COMMENT ON COLUMN scheduler_tasks.next_run_at IS '下次执行时间';
COMMENT ON TABLE scheduler_task_logs IS '调度执行日志';

COMMENT ON TABLE file_references IS '文件引用台账，用于孤儿文件扫描';
COMMENT ON COLUMN file_references.storage_name IS '存储名，如 rustfs';
COMMENT ON COLUMN file_references.object_key IS '对象存储 key';
COMMENT ON COLUMN file_references.module_name IS '引用方模块';
COMMENT ON TABLE scan_targets IS '文件引用扫描目标（表/字段）';

COMMENT ON TABLE audit_log IS '操作审计日志';
COMMENT ON COLUMN audit_log.action IS '审计动作';
COMMENT ON COLUMN audit_log.resource IS '审计资源';
COMMENT ON TABLE feature_flags IS '功能开关';

COMMENT ON TABLE zzz_integration_outbox IS '集成事件 Outbox';
COMMENT ON TABLE zzz_integration_connectors IS '集成连接器配置';
COMMENT ON TABLE zzz_integration_action_logs IS '集成 Action 调用日志';

COMMENT ON TABLE zzz_webhook_configs IS 'Webhook 配置';
COMMENT ON TABLE zzz_webhook_delivery_logs IS 'Webhook 投递日志';

COMMENT ON TABLE zzz_servicenav_type IS '服务导航分类';
COMMENT ON TABLE zzz_servicenav_item IS '服务导航条目';

COMMENT ON TABLE zzz_development_manager_project IS '项目开发管理：项目';
COMMENT ON COLUMN zzz_development_manager_project.project_key IS '项目唯一标识';
COMMENT ON COLUMN zzz_development_manager_project.type IS '项目类型：backend_module/frontend_module/container_project 等';
COMMENT ON COLUMN zzz_development_manager_project.status IS '状态：active/archived';
COMMENT ON TABLE zzz_development_manager_project_relation IS '项目依赖/关系';
COMMENT ON TABLE zzz_development_manager_project_asset IS '项目资产（文档、镜像、制品等）';
COMMENT ON TABLE zzz_development_manager_container IS '部署容器定义';
COMMENT ON TABLE zzz_development_manager_resource_alloc IS '容器资源配额';
COMMENT ON TABLE zzz_development_manager_volume_mapping IS '容器卷映射';
COMMENT ON TABLE zzz_development_manager_whitelist IS '访问白名单';
COMMENT ON TABLE zzz_development_manager_whitelist_member IS '白名单成员';
COMMENT ON TABLE zzz_development_manager_access_point IS '访问入口';
COMMENT ON TABLE zzz_development_manager_backup IS '备份记录';
COMMENT ON TABLE zzz_development_manager_db_table IS '数据字典：表';
COMMENT ON TABLE zzz_development_manager_db_column IS '数据字典：字段';
COMMENT ON TABLE zzz_development_manager_requirement IS '需求';
COMMENT ON COLUMN zzz_development_manager_requirement.code IS '需求编号';
COMMENT ON COLUMN zzz_development_manager_requirement.status IS '需求状态';
COMMENT ON TABLE zzz_development_manager_requirement_version IS '需求版本快照';
COMMENT ON TABLE zzz_development_manager_generation IS '代码生成记录';
COMMENT ON COLUMN zzz_development_manager_generation.status IS '生成状态';
