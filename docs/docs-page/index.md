---
title: 首页
nav_order: 0
---

# Go Backend Core

## 项目概述

模块化 Go 后端服务（Modular Monolith），提供统一 API 网关、多业务模块与公共服务，支撑 Web / 小程序 / App 等多端接入。

### 核心特性

- **模块化架构** — 基于 `Module` 接口与全局注册表，15 个已注册模块按序初始化、逆序关闭
- **统一三层模型** — Handler → Service → Repository，模块间经 Service Interface 解耦
- **双引擎** — 主引擎 `:29090` 业务 API，调试引擎 `:29092` Swagger UI + 健康检查
- **事件驱动** — 本地 EventBus 解耦配置变更等模块间通信
- **可观测** — Prometheus 指标、zerolog 结构化日志、独立审计日志

### 技术栈

| 层级 | 选型 |
|------|------|
| 语言 | Go 1.26 |
| HTTP 框架 | Gin |
| 数据库 | MySQL / PostgreSQL / Redis / MinIO / RustFS（统一 Manager + 熔断） |
| ORM | GORM + 原生 `database/sql`（业务模块多为手写 SQL） |
| 任务调度 | robfig/cron + asynq + 模块内 ticker |
| 日志 | zerolog |
| 文档 | swaggo Swagger |
| 监控 | Prometheus + Grafana（独立 Compose） |

### 文档导航

| 章节 | 内容 |
|------|------|
| [架构设计](architecture/) | 系统架构、技术选型、模块化设计 |
| [数据模型](data-model) | 数据库策略、表设计、命名规范 |
| [功能设计](features/) | 登录接入方案 |
| [开发指南](development/) | 开发规划、测试规范 |
| [运维部署](operations/) | 部署方案、环境管理 |

> **提示**：本站点为导航页。文档正文的唯一事实源是仓库根目录 [`docs/`](https://github.com/Allinost/go-backend-core/tree/main/docs)，各页面已标注对应源文档链接。
