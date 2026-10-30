-- 007_project_type_container.sql
-- 项目类型扩展：新增 container_project（容器项目，管理 docker compose 部署栈）
-- 注意：迁移按英文分号拆分执行，本文件注释与字符串内不得出现英文分号

ALTER TABLE zzz_development_manager_project MODIFY COLUMN type VARCHAR(32) NOT NULL COMMENT '项目类型 base=后端基座 / template=前端模板 / backend_module=后端模块 / frontend_module=前端模块 / frontend_base=前端基座 / container_project=容器项目（docker compose）';
