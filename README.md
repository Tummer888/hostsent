# HostSent

HostSent 是一个云主机管理系统，面向云资源的售卖、管理、监控与售后场景，提供从用户认证、订单处理到实例管理的完整基础能力。

## 项目结构

- `backend`：后端服务，当前基于 Go、Gin、GORM 和 PostgreSQL
- `docs`：项目架构文档、实施计划和进度文档

## 当前状态

- 后端已接入真实用户认证与 JWT 登录流
- 开发环境支持 Docker Compose 启动 PostgreSQL 与后端服务
- 接口文档统一使用 OpenAPI 3 维护

## 开发说明

后端开发环境可直接通过 Docker Compose 启动。接口文档为 `backend/docs/openapi.yaml`，它由路由注册生成，**不要手工编辑**；改完路由后在 `backend/` 下执行：

```bash
python3 scripts/gen_openapi.py
```

生成器扫描 `internal/server/router.go` 与 `internal/server/assembly_oauth.go`，推导路径、tag、摘要、鉴权方式与权限码；需要补充描述 / 参数 / 请求体时改 `backend/docs/openapi.template.yaml` 或 `backend/scripts/openapi_overrides.json`。有路由漏登记时生成会失败并列出路径，而不是让它静默从文档里消失。
