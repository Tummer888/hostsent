-- ============================================================================
-- 029_dual_chain_base.sql
-- 资源管理双链路重构 · P0 基础列（T0.1）
--
-- 目的：把「链路 A｜上游转售」与「链路 B｜自营」的判据显式落库，并给同步调度
--   补上健康/熔断所需的列，供后续 T0.2~T0.5 使用。
--
-- 背景（详见 docs/实施计划/15、17 号文档）：
--   · resource_providers 现用 provider_type 混指「上游转售商」与「虚拟化平台」，
--     新增 kind 区分：upstream=上游转售（目录/定价/生命周期在上游），
--     compute=算力平台（我方定义规格售价，平台仅执行）。默认 upstream 是刻意的，
--     未接入的渠道不应被当成算力平台。
--   · 同步调度依赖 resource_providers.last_sync_at，但 MarkSynced 全仓零调用点，
--     导致 FindDueProviders 每 60s 把所有启用渠道判为到期、疯狂产任务。
--     本次补 sync_paused / consecutive_failures / last_sync_error / last_success_at，
--     配合 T0.2（touch last_sync_at）、T0.3（熔断）止血。
--   · products.provision_mode（self/clone）语义含糊，新增 source_mode
--     作为唯一判据（self/upstream），provision_mode 保留只读一版，P8 再删。
--   · instances 原仅存 product_id，无法区分「售出商品」与「上游商品」，
--     补 sell_product_id / upstream_product_id / provider_instance_id /
--     upstream_order_id / lifecycle_stage，P1 起读写切到新列。
--
-- 幂等（R4）：全部 ADD COLUMN IF NOT EXISTS；回填均带 WHERE ... IS NULL，
--   可重复执行；先加列再回填，顺序不可颠倒（代码随后即读新列）。
-- 时序：先执行本迁移，再部署读取新列的后端；AutoMigrate 不会补索引/回填，
--   本迁移是权威来源。
-- 回滚：ALTER TABLE ... DROP COLUMN IF EXISTS <各新增列>；回填数据不可逆，
--   回滚前请先备份。kind 无法从 provider_type 反向还原，需人工判断。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. resource_providers：链路类型 + 同步健康
-- ---------------------------------------------------------------------------
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS kind varchar(16) NOT NULL DEFAULT 'upstream';

-- 魔方云是虚拟化平台（自营链路的执行器），其余渠道保持 upstream
UPDATE resource_providers SET kind = 'compute' WHERE provider_type = 'mofangyun';

ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS sync_paused boolean NOT NULL DEFAULT false;
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS consecutive_failures integer NOT NULL DEFAULT 0;
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS last_sync_error text;
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS last_success_at timestamptz;

-- ---------------------------------------------------------------------------
-- 2. products：链路判据 + 上游加价
-- ---------------------------------------------------------------------------
ALTER TABLE products ADD COLUMN IF NOT EXISTS source_mode varchar(16);

-- 存量回填：clone=从上游克隆转售 → upstream；其余（含 self）=自营
UPDATE products
SET source_mode = CASE WHEN provision_mode = 'clone' THEN 'upstream' ELSE 'self' END
WHERE source_mode IS NULL;

ALTER TABLE products ADD COLUMN IF NOT EXISTS upstream_markup_type varchar(16);
ALTER TABLE products ADD COLUMN IF NOT EXISTS upstream_markup_value numeric(12,4) DEFAULT 0;
ALTER TABLE products ADD COLUMN IF NOT EXISTS upstream_price_synced_at timestamptz;

-- ---------------------------------------------------------------------------
-- 3. instances：链路语义列
-- ---------------------------------------------------------------------------
ALTER TABLE instances ADD COLUMN IF NOT EXISTS source_mode varchar(16);
ALTER TABLE instances ADD COLUMN IF NOT EXISTS sell_product_id bigint;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS upstream_product_id bigint;
ALTER TABLE instances ADD COLUMN IF NOT EXISTS provider_instance_id varchar(128);
ALTER TABLE instances ADD COLUMN IF NOT EXISTS upstream_order_id varchar(128);
ALTER TABLE instances ADD COLUMN IF NOT EXISTS lifecycle_stage varchar(24);

-- 存量回填：
--   ① 有订单号 = 走我方下单流程开通 → self；
--   ② 订单号为 NULL/0 的存量实例再按所属渠道 kind 判定——compute 平台承载自营
--      实例，upstream 渠道承载同步实例（D6：链路判据单一，不复用 product_id 猜）。
-- 注意 order_id 可空，不能用 `order_id = 0` 匹配（NULL 会让两分支都落空），
-- 实测存量实例 order_id 全为 NULL，故显式用 IS NULL 兜底。
UPDATE instances i
SET source_mode = CASE
    WHEN i.order_id IS NOT NULL AND i.order_id <> 0 THEN 'self'
    WHEN p.kind = 'compute' THEN 'self'
    ELSE 'upstream'
  END
FROM resource_providers p
WHERE p.id = i.provider_id AND i.source_mode IS NULL;

UPDATE instances SET sell_product_id = product_id
WHERE source_mode = 'self' AND sell_product_id IS NULL;

UPDATE instances SET upstream_product_id = product_id
WHERE source_mode = 'upstream' AND upstream_product_id IS NULL;

UPDATE instances SET provider_instance_id = instance_id
WHERE provider_instance_id IS NULL;

-- ---------------------------------------------------------------------------
-- 4. T0.4 未接入适配器的渠道先行停调
-- ---------------------------------------------------------------------------
-- aws/aliyun/openstack/proxmox 尚无适配器实现，FindDueProviders 每轮都会给它们
-- 产生注定失败的任务。这里显式置 sync_paused 并写明原因；后端 T0.3 的前置校验
-- 也会对「工厂未注册」的渠道自动熔断，二者互为兜底。适配器落地后解除暂停即可。
UPDATE resource_providers
SET sync_paused = true, last_sync_error = '适配器未实现'
WHERE provider_type IN ('aws', 'aliyun', 'openstack', 'proxmox')
  AND sync_paused = false;

-- ---------------------------------------------------------------------------
-- 5. T0.5 同步数据治理：统计/清理所需索引
-- ---------------------------------------------------------------------------
-- (provider_id, status, created_at)：按渠道+状态+时间的列表统计
CREATE INDEX IF NOT EXISTS idx_sync_tasks_provider_status_created
  ON sync_tasks (provider_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_logs_provider_status_created
  ON sync_logs (provider_id, status, created_at DESC);
-- created_at：保留策略按时间分批删除（复合索引首列非时间，无法服务该查询）
CREATE INDEX IF NOT EXISTS idx_sync_tasks_created ON sync_tasks (created_at);
CREATE INDEX IF NOT EXISTS idx_sync_logs_created ON sync_logs (created_at);

-- ---------------------------------------------------------------------------
-- 6. T1.1 修正被误标的自营商品（地雷 L5）
-- ---------------------------------------------------------------------------
-- 旧 backfillProductProvisionMode 把空值无脑归一为 self，导致「从上游克隆」的商品
-- 被标成自营。凡绑定了上游渠道（source_provider_id）或上游商品（source_product_id）
-- 的记录一律纠正为 clone/upstream，避免双链路判据错分。
UPDATE products
SET provision_mode = 'clone', source_mode = 'upstream'
WHERE (COALESCE(source_provider_id, 0) <> 0 OR COALESCE(source_product_id, 0) <> 0)
  AND (provision_mode IS DISTINCT FROM 'clone' OR source_mode IS DISTINCT FROM 'upstream');

COMMIT;
