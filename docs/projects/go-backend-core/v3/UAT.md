# V3 UAT 检查表

> 目标：12.15 核心上线前，按业务链路逐项验收。每项记录结果、操作人、时间、异常。

## 1. 基础

- [ ] `/api/v1/health/ping` 返回 pong
- [ ] `/api/v1/health/health` 显示 PG/Redis/RustFS 状态
- [ ] `/api/v1/health/modules` 列出 health/auth/settings/migrate/developmentmanager/ops 等
- [ ] PG schema 为 `gobackendcore`，迁移 001–005 已应用
- [ ] `APP_DATABASE_AUTO_MIGRATE=false`

## 2. 认证与权限

- [ ] 管理员可登录
- [ ] 错误密码 5 次后锁定，按策略解锁
- [ ] 自助注册默认 403
- [ ] 管理员 `POST /auth/users` 可创建用户并分配角色
- [ ] 普通用户访问管理员接口 403
- [ ] API Key 创建/列表/调用 integration 成功
- [ ] 配置中心 `auth.allow_registration` 可热更新

## 3. 配置中心

- [ ] `GET /settings/config/schema` 返回配置项
- [ ] 修改 `auth.allow_registration` 立即生效
- [ ] 敏感项回显 `******`
- [ ] 历史记录敏感项脱敏
- [ ] 重置配置恢复默认值

## 4. 项目管控

- [ ] 项目台账可创建/编辑/删除
- [ ] 里程碑可创建/查看/进度
- [ ] 任务可创建/编辑/状态流转
- [ ] 任务依赖可添加，环检测生效
- [ ] 任务日志/工时累计正确
- [ ] 风险/发布/迭代 CRUD
- [ ] 基线快照包含当前任务
- [ ] 评论/检查项/标签/任务资产可用
- [ ] 仪表盘/看板/路线图/周报数据正确
- [ ] 61 任务台账已导入，重复执行不重复建

## 5. 运维

- [ ] 主机台账 CRUD
- [ ] 主机 SSH test/status 可用
- [ ] 凭据不回明文
- [ ] 维护状态 GET 正常
- [ ] `shutdown` 开始/结束
- [ ] `migrate_return` 未确认同步时拒绝开始
- [ ] `migrate_return` 切 L2 后未确认反向同步时拒绝结束
- [ ] `migrate_no_return` 目标节点为 L2
- [ ] 维护操作有审计记录

## 6. 数据与备份

- [ ] `v3migrate plan/dump/verify` 可用
- [ ] `v3migrate rehearsal` 在空 schema 通过
- [ ] `v3migrate report` 生成 Markdown
- [ ] `pg-backup.sh` 生成 dump + sha256
- [ ] `pg-restore.sh` 在临时库恢复成功
- [ ] 恢复后健康检查和冒烟通过

## 7. 公网

- [ ] HTTPS 证书有效
- [ ] `/api/v1/health/ping` 经反代可用
- [ ] WebSocket 升级正常（如使用）
- [ ] 上传大小限制符合配置
- [ ] 真实客户端 IP 正确
- [ ] 安全头/限流生效

## 8. 回归

- [ ] V2 老机器业务未受影响
- [ ] V3 新机器无 MySQL 依赖（V3 模块）
- [ ] 日志无 ERROR/panic
- [ ] 备份任务已配置
- [ ] 维护 Runbook 已交接
