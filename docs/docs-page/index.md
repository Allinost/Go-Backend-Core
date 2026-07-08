---
title: 首页
nav_order: 0
---

# Go Backend Core

## 项目概述

模块化 Go 后端服务，提供 NAS/云连接、多端客户端支持（微信小程序 / Android / iOS / HarmonyOS / Web / Desktop）、自建业务系统。

### 核心特性

- **模块化架构** — 3 个基础设施模块 + 32 个公共服务模块 + 3 个业务模块 + 3 个公共层
- **统一三层模型** — Handler → Service → Repository
- **事件驱动** — Redis Stream 总线解耦模块间调用
- **6 平台 OAuth** — 微信 / 飞书 / QQ / Apple ID / 华为 / 荣耀
- **多客户端** — 同一后端服务所有客户端共用

### 技术栈

| 层级 | 选型 |
|------|------|
| 语言 | Go 1.22+ |
| HTTP 框架 | Gin |
| 数据库 | MySQL 8 / PostgreSQL 16 / Redis 7 / RustFS |
| ORM | GORM + golang-migrate |
| 依赖注入 | Wire |
| 日志 | zerolog |
| 任务队列 | asynq |

### 文档导航

| 章节 | 内容 |
|------|------|
| [架构设计](architecture/) | 系统架构、技术选型、模块化设计 |
| [数据模型](data-model) | 数据库策略、表设计、命名规范 |
| [功能设计](features/) | 社交登录接入方案 |
| [开发指南](development/) | 开发规划、测试规范 |
| [运维部署](operations/) | 部署方案、环境管理 |

### 开发阶段

| Phase | 内容 | 时间 |
|-------|------|------|
| 0 | 项目脚手架 | 1 周 |
| 1 | 核心基础设施（16 模块） | 3 周 |
| 2 | 社交登录与多端认证 | 2 周 |
| 3 | 个人物品管理 MVP | 4 周 |
| 4~7 | 知识管理 / 智能家居 / 自动化 / 音乐 | 各 2 周 |
| 8 | 多端联调与运维 | 持续 |
