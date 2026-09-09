# Phase 2 · 数据库规范化（分析 + 分阶段迁移）

> 阶段：Phase 2（P1）｜ 位置：`backend/migrations/`
> 总体原则：**单一真相、不销毁即迁移、备份先行、幂等可回滚**。
> 本目录下的 `0xx_phase2_*.sql` 为**暂存迁移**（staged）：已按规划完成目标表设计与迁移 SQL，
> **未在生产/线上库执行**。按 00 规划 Phase 2 的要求，这些需先在一个功能分支＋测试库验证，
> 再上生产。执行前先 `pg_dump` 备份（本阶段已生成 `backups/pre-phase2-*.sql`）。

---

## 1. 现状诊断（已核对 2026-09-09 实际表结构）

### 1.1 账务域分裂
| 表 | 角色 | 问题 |
|---|---|---|
| `wallet_accounts` | 余额/冻结（9 列） | **权威**（`wallet_service` 依赖） |
| `wallet_transactions` | 资金流水（15 列） | **权威流水**（含 balance_before/after、biz_key 幂等） |
| `users.balance` | 冗余余额（numeric） | 与 `wallet_accounts.balance` 双写，易不一致 |
| `user_transactions` | 旧流水（7 列） | 与 `wallet_transactions` 重叠，仅供「用户明细聚合」读 |
| `bills` / `user_bills` | 账单 | `user_bills` 与 `bills` 语义重叠 |

### 1.2 实例域重叠
| 表 | 列 | 问题 |
|---|---|---|
| `instances` | 22 列（含规格列 + raw_data） | **权威**（`sync_repo` 写，`uc/instance` 读） |
| `user_instances` | 9 列（user_id/name/region/specs/status/expire_at） | 与 `instances` 重叠，仅供「用户明细聚合」读 |

### 1.3 商品/规格并存 + JSON 滥用
- `products.specs`、`products.config_options`、`resource_products.raw_specs` 为 `text` 存 JSON。
- `resource_products` 已把 cpu/memory/disk/disk_type/bandwidth/os/region/zone 上移为列（好）。
- `product_pricing`/`product_specs`/`product_spec_templates`/`spec_mappings` 四张表角色重叠。

### 1.4 标识二义
- `resource_products.id`（本地主键）与 `resource_products.upstream_id`（上游 id）并存。
- `products.source_product_id` 存的是 `resource_products.id`（本地），命名易误认为上游 id。
  （**注意**：该字段的 JSON 契约 `source_product_id` 已被 admin 前端消费，改名属契约变更，
  故以「文档澄清 + 回填校验」而非改名落地，见 4.4。）

---

## 2. 目标设计

### 2.1 账务（收敛到 wallet）
```
wallet_accounts   : 余额唯一权威（balance/frozen/total_income/total_expense）
wallet_transactions: 流水唯一权威
users.balance     : 改为「派生缓存」，由钱包变更后同步（保读性能），禁止直改
user_transactions : 退役 → 明细聚合改读 wallet_transactions（需改写 user_detail_service）
user_bills        : 退役 → 明细聚合改读 bills
bills             : 保留为权威账单
```

### 2.2 实例（收敛到 instances）
```
instances         : 唯一权威（已有规格列 + raw_data + expire_at）
user_instances    : 退役 → 明细聚合改读 instances
补外键：instances.user_id→users.id、instances.provider_id→resource_providers.id
唯一约束：instances.instance_id（已有）
```

### 2.3 商品/规格
```
resource_products.raw_specs/specs : 关键字段已上移为列；保留 text 作为原样快照
products.config_options          : 上游 config_groups → product_config_options / product_config_options_sub 子表
四张规格表角色摊牌后去掉合并重复：
   - product_specs/product_spec_templates 二者留一或明确分工
   - spec_mappings 保留跨上游映射
```
> **数据模型核实（2026-09-09）**：真正的上游 `config_groups` 并不在 `products.config_options`（那是扁平覆盖 map
> `{"cpu":"4"}`），而是在 **`resource_products.raw_specs`** 的 `config_groups` 键，由
> `catalog/buildProvisionRequest` 的 `extractConfigGroups(rp.RawSpecs)` **惰性解析**。
> 因此 `015` 迁移已按此真实来源（`resource_products.raw_specs -> 'config_groups'`，经 `source_product_id` 关联）
> 修正回填。**T2.3 的服务层重构**（让 catalog 读写 product_config_options/sub 替代惰性解析）涉及
> 克隆→开通→上游下单核心链路，且当前无嵌套 config_groups 数据，属于需专项设计与回归的重构，
> 本规划按「服务改造 + 走通核心流程 + 再执行 015」另行推进，暂不在此硬改。

### 2.4 标识
```
source_product_id 语义：明确指向 resource_products.id（本地），文档/字段注释澄清。
回填校验：确保 products.source_product_id 都能在 resource_products.id 找到对应行。
```

---

## 3. 暂存迁移清单（未执行）

| 文件 | 子任务 | 类型 | 可回滚 |
|---|---|---|---|
| `011_phase2_add_instance_fks.sql` | T2.2 实例外键/唯一 | 安全（校验后加） | 是 |
| `012_phase2_validate_source_product.sql` | T2.4 标识回填校验 | 只读校验+报告 | 是 |
| `013_phase2_retire_redundant_tables.sql` | T2.2/T2.1 退役 user_instances/user_transactions/user_bills | 破坏性（代码层已就绪，见进度更新） | 是（快照+备份恢复） |
| `014_phase2_account_ledger.sql` | T2.1 账务收敛（wallet 权威 + users.balance 派生触发） | 安全（加 trigger+回填，已语法/执行校验） | 是（drop trigger） |
| `015_phase2_product_config_tables.sql` | T2.3 商品配置子表 | 结构（建表+幂等回填，已语法/执行校验） | 是（drop 表） |

> **执行门禁**：`013` 需先部署新后端（读权威表）再执行 DROP；`014/015` 为安全/结构性迁移，
> 可在测试库先跑（014 含 trigger + 回填，015 建空表 + 幂等回填）。`011/012` 为只读校验或安全加约束。

> **进度更新**：`user_detail_service.GetAggregate`（明细聚合）与用户列表 `total_consume_amount`
> 已改为直接读权威表 `instances`/`wallet_transactions`/`bills`（见
> `user/account/repository/user_detail_repo.go`、`user_repo.go`）。并且已在 `internal/pkg/db/db.go`
> 把 `usermodel.UserInstance/UserBill/UserTransaction` 移出 `AutoMigrate`、移除 `db.Seed` 填充。
>
> **已执行**：`010`（清占位）、`011`（实例外键）、`014`（账务收敛 trigger + 回填）、`013`
> （退役 `user_instances`/`user_transactions`/`user_bills`，已快照到 `retired_user_*_20260909`）；
> 并已重新构建/部署后端（新代码读权威表）。当前库只剩权威表。
> `015`（商品配置子表）仍为**结构性暂存**：需产品目录服务改为读写子表后再执行。

---

## 4. 分阶段迁移脚本（节选）

### 4.1 `011_phase2_add_instance_fks.sql`（安全）
```sql
BEGIN;
-- 仅当存在不悬挂依赖时才加外键；否则先输出报告
ALTER TABLE instances DROP CONSTRAINT IF EXISTS fk_instances_user;
ALTER TABLE instances DROP CONSTRAINT IF EXISTS fk_instances_provider;
ALTER TABLE instances
  ADD CONSTRAINT fk_instances_user FOREIGN KEY (user_id) REFERENCES users(id);
ALTER TABLE instances
  ADD CONSTRAINT fk_instances_provider FOREIGN KEY (provider_id) REFERENCES resource_providers(id);
COMMIT;
```
> 注：若 `instances` 存在悬挂 `provider_id`（如 `i-mfy-*` 占位行），应先跑 `010_cleanup_orphan_instances.sql`。

### 4.2 `012_phase2_validate_source_product.sql`（只读校验）
```sql
-- 列出引用不到 resource_products.id 的 products（应输出 0 行才是健康数据）
SELECT p.id, p.name, p.source_product_id
FROM products p
LEFT JOIN resource_products rp ON rp.id = p.source_product_id
WHERE p.source_product_id <> 0 AND rp.id IS NULL;
```

### 4.3 退役 `user_instances`/`user_transactions`/`user_bills`（破坏性，需先改服务）
```sql
-- 前提：user_detail_service.GetAggregate 已改为从 instances / wallet_transactions / bills 读取。
BEGIN;
DROP TABLE IF EXISTS user_instances;
DROP TABLE IF EXISTS user_transactions;
DROP TABLE IF EXISTS user_bills;
COMMIT;
-- 回滚：从 backups/pre-phase2-*.sql 恢复。或：
--   CREATE TABLE user_instances AS SELECT ... FROM backups/pre-phase2-*.sql;
```

### 4.4 商品配置子表（T2.3 目标，需先改产品目录服务）
```sql
-- product_config_options（上游 configoption）
CREATE TABLE IF NOT EXISTS product_config_options (
  id BIGSERIAL PRIMARY KEY,
  product_id BIGINT NOT NULL REFERENCES products(id),
  upstream_key BIGINT,            -- 上游 configoption 键
  option_name VARCHAR(64),
  option_type INTEGER,
  sub_options JSONB,              -- 子项列表（或再拆 product_config_options_sub）
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
-- 数据迁移：从 products.config_options(text JSON) 提取 → product_config_options
```

---

## 5. 结论与建议

- 本阶段**未对线上库执行破坏性迁移**；已生成完整备份（`backups/pre-phase2-*.sql`）与暂存迁移。
- 建议按 `011 → 012`（安全）→ 改写 `user_detail_service` / 产品目录服务 → `013/014/015`（破坏性，测试库演练）推进。
- 每次迁移独立事务、可回滚、幂等，与 00 规划 Phase 2「备份 + 事务 + 可重复执行」一致。
