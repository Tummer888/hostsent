# swag 接口文档

## 目标
- 使用 `swaggo/swag` 生成后端接口文档
- 统一输出 Swagger 页面
- 后续新增接口时同步补充注释并重新生成

## 约定
- Swagger 注释放在 handler 层
- 文档入口放在 `cmd/server/main.go` 或独立的 swagger 初始化文件中
- 生成产物放在 `docs/swagger` 或 `backend/docs/swagger` 下

## 生成命令
```bash
swag init -g cmd/server/main.go -o docs/swagger
```

## 访问方式
- `/swagger/index.html`
