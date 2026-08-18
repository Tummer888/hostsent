# HostSent

HostSent 是一个云主机管理系统，面向云资源的售卖、管理、监控与售后场景，提供从用户认证、订单处理到实例管理的完整基础能力。

## 项目结构

- `backend`：后端服务，当前基于 Go、Gin、GORM 和 PostgreSQL
- `docs`：项目架构文档、实施计划和进度文档

## 当前状态

- 后端已接入真实用户认证与 JWT 登录流
- 开发环境支持 Docker Compose 启动 PostgreSQL 与后端服务
- 接口文档使用 `swaggo/swag` 生成

## 开发说明

后端开发环境可直接通过 Docker Compose 启动，接口文档可通过 Swagger 页面查看。
