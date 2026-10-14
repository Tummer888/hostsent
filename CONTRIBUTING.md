# 开发规范（CONTRIBUTING）

> 本规范依据 2026-10 规范性审计整改方案（`docs/实施计划/115-后端工程规范整改方案.md`）制定，
> 是后端工程的强制约定；新增代码不符合本规范的，CI 门禁会拦截或在 review 中退回。

## 1. 目录层级规则

后端模块位于 `backend/internal/modules/{admin,uc,site}`，按业务域组织。

**规则**：域内含 ≥2 个子模块时采用分组式 `域/子模块/五件套`；单模块域采用平铺式五件套。

- 分组式：`admin/finance/{account/recharge, bill, …}`、`admin/product/{catalog,…}`、`admin/resource/{product,provider,sync}`、`admin/system/{config,notification,payment,sales,content}`、`admin/user/{account,security,verification,…}`
- 平铺式：`admin/{instance,logcenter,manager,menu,order,ticket}`
- 例外形态（域级共享/子包，均须在包注释声明，新增沿用同形态）：
  - `admin/instance` = 域级共享模型（`model/`）+ 子模块（`lifecycle/`）
  - `admin/user/account/level` = account 子模块下的嵌套子模块（独立五件套）
  - `admin/resource/common/{dto,providerkind}` = 域内共享的纯函数/结构工具包（无 model、无表）
  - `admin/logcenter/catalog` = 日志中心域内共享的日志源注册表（纯常量，无表）
  - `uc/captcha/adapter` = captcha 自治模块 service 端口的外部实现（渠道路由/目标解析），装配层注入
- 新建模块必须先判断归属域；域不存在才新建域目录

### 模块统一模板（五件套）

```text
modules/<端>/<域>/<模块>/
├── dto/          # 传输对象（请求/响应结构，含 binding 校验标签）
├── model/        # GORM 模型（一表一文件，含 TableName()）
├── repository/   # 数据访问（只依赖 model 与 gorm，禁写业务判断）
├── service/      # 业务编排（依赖 repository 与其它域的 service 接口）
└── handler/      # HTTP 层（参数绑定 → 调 service → 统一响应，禁业务逻辑）
```

## 2. uc 端三种合法模块形态

| 形态 | 结构 | 约束 |
| --- | --- | --- |
| 自治模块 | 完整五件套 | 拥有独立表与业务的域（auth/instance/member…） |
| 薄适配 | handler + dto + service | service 委托 admin 域 service，只做请求/响应转换 |
| 纯透传 | 仅 handler | handler 内**禁止业务编排**：只许参数绑定 → 调 admin service → 统一响应；超过 3 步编排必须升级为薄适配 |

现状：纯透传 = `uc/{finance,point,referral}`；薄适配 = `uc/{order,product,payment,menu}`；其余为自治模块。

薄适配与自治模块的服务层**不得 import admin 域 model**；请求/响应结构一律以本模块
`dto` 声明，与 admin 域结构的字段转换由装配层（`internal/server`）的适配器完成
（参考 `uc_order_adapters.go`）。若域内确需共享结构，admin 侧应在 `dto` 包提供
可复用类型，uc 侧经 dto 引用而非 model。

## 3. 依赖方向（单向，违例即退回）

```text
uc/service   → admin/service（经接口注入，禁止 import admin 域 model）
跨域         → 对方 reader/service 接口（参考 instance_reader 模式）
pkg          → 禁止 import modules（唯一豁免：pkg/db 迁移中心；pkg 面向模块的
               数据一律用纯 DTO 声明，如 middleware.AdminAuditEntry，模型转换
               由装配层适配器完成）
handler      → 永不直接使用 repository / gorm
```

## 4. 错误处理

- `fmt.Errorf` 包装错误**一律用 `%w`**，保持 `errors.Is/As` 可追根因；禁止 `%v`/`%s` 丢链
- 业务错误统一走各域 `service/errors.go` 定义哨兵错误；跨域透传错误必须保留包装链
- 仓储层「查无此行」应翻译为域内哨兵（如 `ErrNotFound`），service 层再出口供
  handler 判断；**handler 不得依赖 `gorm.ErrRecordNotFound`**——语义落点（404/409）
  由 service 层哨兵决定
- `service/errors.go` 只放哨兵错误（`var ErrXxx = errors.New(...)` 与同义再出口），
  其它代码一律不放该文件

## 4b. 响应统一

- handler **禁止手写 `c.JSON(gin.H{...})`**：成功走 `response.Success/SuccessMessage`，
  失败走 `response.Error/ErrorWithStatus`（`ErrorWithStatus` 保留 HTTP 状态码语义，
  与 `Error` 的业务码进 body 双轨并存，二者契约等价）
- 响应结构统一为 `response.Body{code, message, data, timestamp}`；
  新增接口不得引入第三套响应体

## 5. 并发约定

- 每个 `go func()` 必须：入口 `defer recover()`（记日志+错误上报）+ 使用可取消的 `context`
- 禁止在 handler 内开 goroutine（异步动作下沉到 service / 任务队列）

## 6. 数据库迁移

见 [backend/migrations/README.md](backend/migrations/README.md)：编号取「最大+1」、只增不改、
一编号一文件、空洞不复用；历史文件永不修改；破坏性变更遵守 expand-contract（先兼容后收紧）。

## 7. Live 测试约定

- 构建 tag：文件头 `//go:build live`，命名 `*_live_test.go`
- 总开关：`LIVE_<域>=1`（如 `LIVE_ZJMF=1`、`LIVE_ALIPAY=1`），未设置时 `t.Skip`
- 凭证**只走环境变量**（如 `ZJMF_USER`/`ZJMF_PASS`），禁止硬编码任何账号/密钥
- 数据库走 `LIVE_DB_DSN`（libpq 格式，默认本机 `hostsent` 库）；应用装配与其
  同源——不设置时 `liveApp` 强制改写配置指向 127.0.0.1，避免容器环境串库
- 全新库引导：`LIVE_BOOTSTRAP=1 go test -tags live ./internal/server/ -run TestLiveBootstrapSchema`
  按**生产同款路径**（AutoMigrate + Seed，不跑历史 SQL 迁移）把空库建到可用。
  模型变更（新表/新列/新索引）必须在全新库上过一遍这套引导——老库上测不出的
  「缺表缺列缺索引」问题全部由它兜住（CI 已内置，见 §10）
- 同表多模型的 AutoMigrate 注册顺序：**后注册者覆盖先注册者**（gorm
  ReorderModels 按表名去重），字段超集模型必须排在最后，否则列与索引在
  全新库上静默丢失
- 历史迁移重放（`TestLivePhaseMigrations`）只按序演进过的开发库能跑，需
  `LIVE_MIGRATION_REPLAY=1` 显式开启；全新库上它必然失败（中间态列已被后续
  迁移替代），CI 不带此开关

## 8. 日志与配置

- 业务代码禁止直接 `import "log"`，统一用 `internal/pkg/logger`
- 禁止业务代码直接 `os.Getenv`，配置一律经 `internal/pkg/config` 加载
- 日志输出涉及手机号/邮箱/密钥时使用 `internal/pkg/mask` 脱敏

## 9. 命名澄清（易混域）

| 包 | 含义 | 表 |
| --- | --- | --- |
| `admin/product/catalog` | 终端销售商品（定价/规格/SKU） | `products` |
| `admin/resource/product` | 上游资源商品（供应商镜像，含成本价/同步状态） | `resource_products` |

两者经 `catalog.Product.SourceProductID/SourceProviderID` 关联，禁止互相内嵌表结构。

## 10. 提交与验证清单

每次提交前必须全绿：

```bash
gofmt -l .            # 无输出
go build ./...
go vet ./...
go test ./...         # 涉及域的单测至少全绿
golangci-lint run --timeout=5m --build-tags live   # 本地与 CI 同参数，v2.6.1
../scripts/check_coverage.sh                       # 核心链路覆盖率门禁
# 前端另跑 vue-tsc --noEmit
```

CI 额外执行（见 `.github/workflows/ci.yml`）：

- **golangci-lint**：已转 error 门禁（errcheck/staticcheck/unused/revive，
  普通 + `-tags live` 双构建，配置 `backend/.golangci.yml`）。
  revive 显式关闭了四类存量风格规则（exported / package-comments /
  unused-parameter / unexported-return），新代码不要依赖新的豁免
- **`-race` 抽样**：finance / instance / order 核心包
- **live 测试（全新库）**：postgres service 空库上先 `LIVE_BOOTSTRAP=1` 引导
  （生产同款 AutoMigrate + Seed 路径），再跑 `go test -tags live`
  `./internal/server/...` 与 `./internal/pkg/db/...`（批次四 D2）。
  `TestLivePhaseMigrations` 因缺中间态列在全新库自动跳过（需
  `LIVE_MIGRATION_REPLAY=1` 且库按迁移顺序演进过才会执行）
- **结构检查**：`scripts/check_structure.sh`——目录层级、uc 形态、依赖方向、
  响应统一、错误链；存量违例在脚本豁免清单中登记（棘轮模式），新增违例直接 fail。
  CI 先跑 `--selfcheck` fixture 自测再跑正式检查
- **模块形态登记**：新建模块目录必须先在 `scripts/module_shapes.tsv` 登记形态
  （group/sub/flat/mixed/auton/thin/passthru/shared/self），未登记即 CI fail
- **装配层行数棘轮**：`assembly_*/routes_*` 超过 800 行告警、超过 1200 行 fail，
  拆分标准=每域一个装配文件
- **openapi 生成器自检**：`python3 scripts/gen_openapi.py` 退出码为门禁——
  新增路由必须登记 summary/参数规则，`backend/docs/openapi.yaml` 是生成产物勿手改
- **覆盖率**：`go test -cover ./internal/modules/...` 全量报告（只展示）；
  门禁只盯核心链路两包（`scripts/check_coverage.sh`，阈值 70%，棘轮式上调）：
  admin/order/service（下单/退款/开通）与 uc/order/service（用户下单全链路）。
  这两包是资金与履约主链路，改动作必须带测试；其余包覆盖率由开发者自行保证
