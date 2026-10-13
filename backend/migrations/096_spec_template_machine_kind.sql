-- ============================================================================
-- 096_spec_template_machine_kind.sql
-- doc122：魔方云双开通链路的机型化规格模板。
--
-- 背景：
--   魔方云的云主机有两条开通链路，在面板侧不是两个接口，而是同一 POST /clouds
--   的两组参数取值：
--     · 独立 IP（VPS）：ip_num >= 1，nat_acl_limit / nat_web_limit = -1；
--     · NAT 机型：      ip_num = 0，nat_acl_limit 或 nat_web_limit >= 0，
--                       机器没有独立 IP，客户经节点转发地址 nat_host_ip:ext_port 访问。
--   （面板写侧见 DcimCloud.php:2635-2636 的 nat_*_limit 缺省 -1 与 :2698 的
--     ip_num 原样下发；客户端 NAT 入口门禁见 DcimCloud.php:147-153。）
--   我方 product_spec_templates 此前只有 provider_type（平台）这一个归类维度，
--   机型参数只能靠运营手写进 platform_params，既无预置也无互斥校验：把 NAT 参数
--   配进 VPS 档位会一路带到面板才被拒，最坏情况是开出「既无独立 IP 又无转发入口」
--   的机器，客户拿不到访问地址。
--
-- 本迁移只加一个归类/校验维度，取值由服务层按 platform_params 内容裁定：
--   machine_kind = 'vps' / 'nat' / ''（未标注）。
--   默认 '' 而不是 'vps'：存量模板没有这个维度，强行回填会把「NAT 参数配错的老
--   模板」标成 VPS 并触发互斥校验失败，运营反而改不动老数据。'' 表示未标注，
--   校验跳过，只有新模板才强制选择机型。
--
-- 幂等：IF NOT EXISTS，重复执行无副作用。
-- 顺序：本迁移先跑；列由 GORM AutoMigrate 在服务启动时补对齐
--      （SpecTemplate.MachineKind 与 DDL 双写，见 internal/pkg/db/db.go）。
-- 回滚：DROP INDEX idx_spec_templates_machine_kind;
--      ALTER TABLE product_spec_templates DROP COLUMN machine_kind;
--      （列可留空字段，回滚不影响既有数据。）
-- ============================================================================

BEGIN;

ALTER TABLE product_spec_templates
  ADD COLUMN IF NOT EXISTS machine_kind VARCHAR(16) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_spec_templates_machine_kind
  ON product_spec_templates (machine_kind);

COMMIT;
