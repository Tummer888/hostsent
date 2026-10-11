# 迁移目录说明

本目录的 `*.sql` 文件是**数据库演进的记录与对齐依据**，不由独立迁移 runner 执行：

- 表结构由 GORM `AutoMigrate` 在服务启动时自动补列（`internal/pkg/db/db.go`）
- 数据回填通过 `db.go` 中的 Go seed 幂等完成
- `internal/pkg/db/migrations_live_verify_test.go` 等测试会读取本目录文件做对齐校验（build tag `live`）

## 编号规则

1. **编号只增不改**：新建迁移必须取「当前最大编号 + 1」，禁止插入/复用已有编号。
2. **一个编号一个文件**：同一次提交含多个迁移时，按顺序依次取号，不得共用。
3. **历史文件永不修改内容**；确需修正演进路径时，新增一个迁移来覆盖。
4. 编号空洞（如 011→013、024→026）是历史修订被移除的记录，**不回填、不复用**。
5. 文件头注释必须写清背景与影响范围；跨域菜单/数据治理类迁移需在注释中列出新旧路径对照。

## 历史遗留处理记录

- 2026-10-12：清理三对重号——`051_merge_verification_lists`→085、
  `068_merge_sales_and_notification_into_system`→086、
  `070_spec_template_platform_params`→087（详见各文件头部编号说明）。
