-- 009_rename_legacy_tables.sql
-- P8 表前缀规范化：把历史表统一为 zzz_<模块>_ 前缀
-- 008 负责按历史名建表，本迁移负责存量/新建库统一改名（迁移按版本只执行一次）
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

RENAME TABLE
  login3rd_platforms TO zzz_login3rd_platforms,
  login3rd_accounts TO zzz_login3rd_accounts,
  webhook_configs TO zzz_webhook_configs,
  webhook_delivery_logs TO zzz_webhook_delivery_logs,
  services_nav_item TO zzz_servicenav_item,
  services_nav_type TO zzz_servicenav_type;
