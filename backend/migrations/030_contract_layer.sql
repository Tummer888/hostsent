-- ============================================================================
-- 030_contract_layer.sql
-- 资源管理双链路重构 · P2 契约层（T2.2 / T2.5）
--
-- 目的：落地"三份契约"中的规格契约与能力契约的存储层，为接入第 3、4 家
--   异构上游/平台做好预埋（详见 docs/实施计划/16 §3~§5、17 §5 迁移清单）。
--
-- 内容：
--   1. provider_types —— 渠道类型注册表（能力描述符 JSON），替代
--      provider_service.go 里硬编码的类型展示 map（地雷 L9）。
--   2. spec_atoms     —— 规格原子字典（平台无关的最小维度 + 各平台字段映射）。
--   3. external_specs —— 外部规格快照（上游/平台的原始规格登记，与内部标准规格解耦）。
--   4. spec_bindings  —— 外部规格 ↔ 内部标准规格的双向绑定（由 spec_mappings 演进；
--      spec_mappings 旧表与旧列保留一版只读，P8 再清理）。
--   5. resource_providers 传输契约列：credentials JSON + 超时/重试/限流。
--   6. product_spec_templates 规格契约列：spec_values + source。
--   7. products.spec_atom_overrides —— 商品级原子覆盖。
--
-- 幂等（R4）：CREATE TABLE IF NOT EXISTS；ADD COLUMN IF NOT EXISTS；
--   种子用 INSERT ... ON CONFLICT DO NOTHING（不覆盖人工修改的展示字段）。
-- 时序：先执行本迁移再部署读新表的后端；AutoMigrate 会补表但不会写字典种子，
--   本迁移是字典与注册表种子的权威来源。
-- 回滚：DROP TABLE IF EXISTS spec_bindings, external_specs, spec_atoms, provider_types;
--   ALTER TABLE ... DROP COLUMN IF EXISTS <各新增列>。字典为可重建数据，直接重跑迁移即可。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. provider_types：渠道类型注册表（T2.2）
-- ---------------------------------------------------------------------------
-- descriptor 存 CapabilityDescriptor（能力/同步 scope/凭证字段/签名策略/字段字典）。
-- 后端启动时会按已注册适配器刷新 descriptor，但"假渠道"只需本表一行即可出现在
-- 后台添加表单里（验收要求），故本表是展示层的权威来源。
CREATE TABLE IF NOT EXISTS provider_types (
  id              bigserial PRIMARY KEY,
  type            varchar(50)  NOT NULL,
  name            varchar(100) NOT NULL,
  kind            varchar(16)  NOT NULL DEFAULT 'upstream',  -- upstream | compute
  descriptor      jsonb,                                    -- CapabilityDescriptor
  icon            varchar(64),
  doc_url         varchar(255),
  adapter_version varchar(32),
  sort_order      integer      NOT NULL DEFAULT 0,
  status          integer      NOT NULL DEFAULT 1,
  created_at      timestamptz,
  updated_at      timestamptz
);

-- 唯一性用「命名唯一索引」而非列内联 UNIQUE 约束：GORM 模型对 uniqueIndex 的处理
-- 与内联 UNIQUE 约束不一致（见 provider_types 启动期 DropConstraint 报错），
-- 两者名称必须一致，故此处显式建 uk_provider_types_type。
ALTER TABLE provider_types DROP CONSTRAINT IF EXISTS provider_types_type_key;
CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_types_type ON provider_types (type);
CREATE INDEX IF NOT EXISTS idx_provider_types_kind ON provider_types (kind);
CREATE INDEX IF NOT EXISTS idx_provider_types_status ON provider_types (status);

-- 种子：已接入 2 类（descriptor 由后端启动时按适配器刷新）+ 待接入平台占位。
-- 占位行 status=1、adapter_version='-1'（未注册适配器），后台可展示但连接测试会明确失败。
INSERT INTO provider_types (type, name, kind, icon, adapter_version, sort_order, status, created_at, updated_at) VALUES
  ('mofangyun',     '魔方云',     'compute',  'cloud',    '1.0.0', 10, 1, now(), now()),
  ('mofangfinance', '魔方财务',   'upstream', 'finance',  '1.0.0', 20, 1, now(), now()),
  ('aliyun',        '阿里云 ECS', 'upstream', 'aliyun',   '-1',    30, 1, now(), now()),
  ('tencent',       '腾讯云 CVM', 'upstream', 'tencent',  '-1',    40, 1, now(), now()),
  ('aws',           'AWS EC2',    'upstream', 'aws',      '-1',    50, 1, now(), now()),
  ('openstack',     'OpenStack',  'upstream', 'openstack','-1',    60, 1, now(), now()),
  ('proxmox',       'Proxmox VE', 'upstream', 'proxmox',  '-1',    70, 1, now(), now()),
  ('kvm',           'KVM (libvirt)', 'compute', 'kvm',    '-1',    80, 1, now(), now()),
  ('hyperv',        'Hyper-V',    'compute',  'hyperv',   '-1',    90, 1, now(), now()),
  ('pve',           'Proxmox VE (自营)', 'compute', 'proxmox', '-1', 100, 1, now(), now())
ON CONFLICT (type) DO NOTHING;

-- 待接入平台的能力描述符预埋（§6.2）：为自营虚拟化平台写一份带 field_dictionary
-- 的占位描述符，后台能力矩阵可展示"该平台字段字典（待核实）"；接入适配器后由
-- SyncTypeRegistry 以实测描述符覆盖（COALESCE 保证已存在的非空 descriptor 不被回退）。
INSERT INTO provider_types (type, name, kind, adapter_version, sort_order, status, descriptor, created_at, updated_at) VALUES
  ('kvm', 'KVM (libvirt)', 'compute', '-1', 80, 1,
   '{"kind":"compute","sync_scopes":["pool","region","image","instance"],"spec_atoms":["compute.cpu","compute.memory","storage.system.size","network.bandwidth","placement.region","image.id"],"billing_cycles":["month","year"],"operations":["provision","start","stop","restart","resize","destroy"],"renew_mode":"direct","destroy_mode":"immediate","signer_type":"none","credential_schema":[{"key":"endpoint","label":"libvirt 连接地址","type":"string","required":true},{"key":"ssh_key","label":"SSH 私钥","type":"password","required":false,"secret":true}],"supports_paging":false,"field_dictionary":{"source":"待核（libvirt domain XML，行业常见形态）","cpu":"<vcpu>N</vcpu>","memory":"<memory unit=MiB>N</memory>","disk":"<disk><target dev=vda><source file=...>","disk_type":"<driver name=qemu type=qcow2>","bandwidth":"<bandwidth><inbound average=...>","region_zone":"存储池 + 网络池","image":"基础镜像 qcow2 路径","ip_count":"网卡数 + 静态 IP 分配"}}'::jsonb,
   now(), now()),
  ('hyperv', 'Hyper-V', 'compute', '-1', 90, 1,
   '{"kind":"compute","sync_scopes":["pool","region","image","instance"],"spec_atoms":["compute.cpu","compute.memory","storage.system.size","network.bandwidth","placement.region","image.id"],"billing_cycles":["month","year"],"operations":["provision","start","stop","restart","resize","destroy"],"renew_mode":"direct","destroy_mode":"immediate","signer_type":"none","credential_schema":[{"key":"host","label":"Hyper-V 主机","type":"string","required":true},{"key":"username","label":"账号","type":"string","required":true},{"key":"password","label":"密码","type":"password","required":true,"secret":true}],"supports_paging":false,"field_dictionary":{"source":"待核（PowerShell/WMI，行业常见形态）","cpu":"ProcessorCount","memory":"MemoryStartup（字节，注意单位换算）","disk":"VirtualHardDisk 路径 + 容量","disk_type":"动态/固定 VHD","bandwidth":"QoS 策略 / Set-VMNetworkAdapter","region_zone":"集群 + 节点","image":"VHDX 模板","ip_count":"SwitchName + IP"}}'::jsonb,
   now(), now()),
  ('pve', 'Proxmox VE (自营)', 'compute', '-1', 100, 1,
   '{"kind":"compute","sync_scopes":["pool","region","image","instance"],"spec_atoms":["compute.cpu","compute.memory","storage.system.size","network.bandwidth","placement.region","image.id"],"billing_cycles":["month","year"],"operations":["provision","start","stop","restart","resize","destroy"],"renew_mode":"direct","destroy_mode":"immediate","signer_type":"none","credential_schema":[{"key":"endpoint","label":"API 地址","type":"string","required":true},{"key":"token_id","label":"Token ID","type":"string","required":true},{"key":"token_secret","label":"Token Secret","type":"password","required":true,"secret":true}],"supports_paging":false,"field_dictionary":{"source":"待核（Proxmox VE API，行业常见形态）","cpu":"cores","memory":"memory（MiB）","disk":"scsi0 / virtio0","disk_type":"discard/ssd 选项","bandwidth":"net0 rate 限速","region_zone":"node / pool","image":"ostemplate","ip_count":"ip 参数"}}'::jsonb,
   now(), now())
ON CONFLICT (type) DO UPDATE SET descriptor = COALESCE(provider_types.descriptor, EXCLUDED.descriptor);

-- ---------------------------------------------------------------------------
-- 2. spec_atoms：规格原子字典（T2.5）
-- ---------------------------------------------------------------------------
-- 平台无关的最小规格维度。platform_fields 记录各平台（mofangyun/kvm/hyperv/pve…）
-- 的读写字段名映射；待接入平台的字段名先放"待核"标记（D5：自营规格以平台字段为准，
-- 每个平台先有一份字段字典，后期逐个确切接入，接入时只改 JSON 不改表结构）。
CREATE TABLE IF NOT EXISTS spec_atoms (
  id            bigserial PRIMARY KEY,
  key           varchar(64)  NOT NULL,          -- 命名空间式：compute.cpu / storage.system.size
  name          varchar(64)  NOT NULL,          -- 展示名：vCPU / 内存 / 带宽
  unit          varchar(16),                    -- core / MB / GB / Mbps
  value_type    varchar(16)  NOT NULL DEFAULT 'int',  -- int|decimal|enum|bool|string
  enum_values   jsonb,                          -- value_type=enum 时的候选值
  min_value     numeric(16,4),
  max_value     numeric(16,4),
  step_value    numeric(16,4),
  required      boolean      NOT NULL DEFAULT false,
  configurable  boolean      NOT NULL DEFAULT false,  -- 是否允许下单时选择
  applies_to    varchar(16)  NOT NULL DEFAULT 'both', -- self | upstream | both
  platform_fields jsonb,                        -- {"mofangyun":{"read":"cpu_num","write":"cpu"}, ...}
  description   text,
  sort_order    integer      NOT NULL DEFAULT 0,
  status        integer      NOT NULL DEFAULT 1,
  created_at    timestamptz,
  updated_at    timestamptz
);

-- 同 provider_types：唯一性用命名唯一索引，避免 GORM uniqueIndex 与内联 UNIQUE
-- 约束名不一致导致启动期 DropConstraint 报 42704。
ALTER TABLE spec_atoms DROP CONSTRAINT IF EXISTS spec_atoms_key_key;
CREATE UNIQUE INDEX IF NOT EXISTS uk_spec_atoms_key ON spec_atoms (key);
CREATE INDEX IF NOT EXISTS idx_spec_atoms_applies_to ON spec_atoms (applies_to);
CREATE INDEX IF NOT EXISTS idx_spec_atoms_status ON spec_atoms (status);

-- 字典种子：§6 平台字段预埋表。魔方云字段为实测值；KVM/Hyper-V/PVE 字段名标注
-- "待核"，接入时必须对照真实文档/接口核对后再写死。
INSERT INTO spec_atoms (key, name, unit, value_type, enum_values, min_value, step_value, required, configurable, applies_to, platform_fields, description, sort_order, status, created_at, updated_at) VALUES
  ('compute.cpu', 'CPU', 'core', 'int', NULL, 1, 1, true, true, 'both',
   '{"mofangyun":{"read":"cpu_num","write":"cpu","source":"实测"},"kvm":{"read":"<vcpu>N</vcpu>","write":"<vcpu>N</vcpu>","source":"待核"},"hyperv":{"read":"ProcessorCount","write":"ProcessorCount","source":"待核"},"pve":{"read":"cores","write":"cores","source":"待核"}}'::jsonb,
   '虚拟 CPU 核数', 10, 1, now(), now()),
  ('compute.memory', '内存', 'MB', 'int', NULL, 128, 128, true, true, 'both',
   '{"mofangyun":{"read":"memory_size","write":"memory","source":"实测"},"kvm":{"read":"<memory unit=MiB>","write":"<memory unit=MiB>","source":"待核"},"hyperv":{"read":"MemoryStartup","write":"MemoryStartup","source":"待核（单位字节，需换算）"},"pve":{"read":"memory","write":"memory","source":"待核（MiB）"}}'::jsonb,
   '内存容量（MB）', 20, 1, now(), now()),
  ('storage.system.size', '系统盘', 'GB', 'int', NULL, 20, 10, true, true, 'both',
   '{"mofangyun":{"read":"disk_size","write":"system_disk_size","source":"实测"},"kvm":{"read":"<disk><target dev=vda>","write":"<disk><target dev=vda>","source":"待核"},"hyperv":{"read":"VirtualHardDisk","write":"VirtualHardDisk","source":"待核"},"pve":{"read":"scsi0/virtio0","write":"scsi0/virtio0","source":"待核"}}'::jsonb,
   '系统盘大小（GB）', 30, 1, now(), now()),
  ('storage.system.type', '系统盘类型', NULL, 'enum', '["ssd","premium","hdd","qcow2","vhd"]'::jsonb, NULL, NULL, false, true, 'both',
   '{"mofangyun":{"read":"disk_type","write":"store","source":"实测（写侧用 store 存储标识）"},"kvm":{"read":"<driver name=qemu type=...>","write":"<driver name=qemu type=...>","source":"待核"},"hyperv":{"read":"动态/固定 VHD","write":"动态/固定 VHD","source":"待核"},"pve":{"read":"discard/ssd","write":"discard/ssd","source":"待核"}}'::jsonb,
   '系统盘介质类型', 40, 1, now(), now()),
  ('storage.data.size', '数据盘', 'GB', 'int', NULL, 0, 10, false, true, 'both',
   '{"mofangyun":{"read":"other_data_disk","write":"other_data_disk","source":"实测（数组，按表单数组语法展开）"},"kvm":{"read":"<disk> 附加盘","write":"<disk> 附加盘","source":"待核"},"pve":{"read":"scsi1+","write":"scsi1+","source":"待核"}}'::jsonb,
   '数据盘大小（GB，0 表示无）', 50, 1, now(), now()),
  ('network.bandwidth', '带宽', 'Mbps', 'int', NULL, 0, 1, false, true, 'both',
   '{"mofangyun":{"read":"bandwidth","write":"bw","source":"实测"},"kvm":{"read":"<bandwidth><inbound average=...>","write":"<bandwidth><inbound average=...>","source":"待核"},"hyperv":{"read":"Set-VMNetworkAdapter","write":"Set-VMNetworkAdapter","source":"待核"},"pve":{"read":"net0 rate","write":"net0 rate","source":"待核"}}'::jsonb,
   '公网带宽（Mbps，0 表示不限）', 60, 1, now(), now()),
  ('network.traffic', '流量', 'GB', 'int', NULL, 0, 1, false, true, 'both',
   '{"mofangyun":{"read":"flow_limit","write":"flow_limit","source":"实测"},"kvm":{"read":"-","write":"-","source":"待核"},"pve":{"read":"-","write":"-","source":"待核"}}'::jsonb,
   '月流量额度（GB）', 70, 1, now(), now()),
  ('network.ip_count', '公网 IP 数', '个', 'int', NULL, 0, 1, false, true, 'both',
   '{"mofangyun":{"read":"ip_num","write":"ip_num","source":"实测"},"kvm":{"read":"网卡数 + 静态 IP 分配","write":"网卡数 + 静态 IP 分配","source":"待核"},"hyperv":{"read":"SwitchName + IP","write":"SwitchName + IP","source":"待核"},"pve":{"read":"ip 参数","write":"ip 参数","source":"待核"}}'::jsonb,
   '公网 IP 数量', 80, 1, now(), now()),
  ('placement.region', '区域', NULL, 'string', NULL, NULL, NULL, true, false, 'both',
   '{"mofangyun":{"read":"region","write":"area","source":"实测（area 即资源池）"},"kvm":{"read":"存储池 + 网络池","write":"存储池 + 网络池","source":"待核"},"hyperv":{"read":"集群 + 节点","write":"集群 + 节点","source":"待核"},"pve":{"read":"node/pool","write":"node/pool","source":"待核"}}'::jsonb,
   '区域 / 资源池', 90, 1, now(), now()),
  ('placement.zone', '可用区', NULL, 'string', NULL, NULL, NULL, false, false, 'both',
   '{"mofangyun":{"read":"zone","write":"node","source":"实测（写侧 node 节点）"},"pve":{"read":"node","write":"node","source":"待核"}}'::jsonb,
   '可用区 / 节点', 100, 1, now(), now()),
  ('image.os', '操作系统', NULL, 'string', NULL, NULL, NULL, false, true, 'both',
   '{"mofangyun":{"read":"os_image","write":"os","source":"实测"},"kvm":{"read":"基础镜像 qcow2 路径","write":"基础镜像 qcow2 路径","source":"待核"},"hyperv":{"read":"VHDX 模板","write":"VHDX 模板","source":"待核"},"pve":{"read":"ostemplate","write":"ostemplate","source":"待核"}}'::jsonb,
   '操作系统 / 镜像', 110, 1, now(), now()),
  ('image.id', '镜像 ID', NULL, 'string', NULL, NULL, NULL, false, false, 'upstream',
   '{"mofangyun":{"read":"os_image","write":"os","source":"实测"}}'::jsonb,
   '镜像唯一标识（上游目录镜像）', 120, 1, now(), now()),
  ('billing.cycle', '计费周期', NULL, 'enum', '["hour","day","month","year"]'::jsonb, NULL, NULL, true, true, 'both',
   '{"mofangyun":{"read":"-","write":"-","source":"本地账期"},"mofangfinance":{"read":"-","write":"-","source":"上游账期"}}'::jsonb,
   '计费 / 续费周期', 130, 1, now(), now())
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. external_specs：外部规格快照（T2.5）
-- ---------------------------------------------------------------------------
-- 上游/平台的规格五花八门，先原样快照（raw）再尽力归一（normalized），
-- 与内部标准规格解耦。适配器归一逻辑升级后可据 raw 重算，无需重新拉取。
CREATE TABLE IF NOT EXISTS external_specs (
  id            bigserial PRIMARY KEY,
  provider_id   bigint,
  provider_type varchar(50)  NOT NULL,
  external_id   varchar(128) NOT NULL,       -- ecs.g7.large / S5.MEDIUM4 / plan_code / flavor name
  external_name varchar(200),
  external_kind varchar(24)  NOT NULL DEFAULT 'flavor',  -- flavor|plan|template|pool_spec
  raw           jsonb,                        -- 原始字段快照
  normalized    jsonb,                        -- 归一后的原子取值（允许残缺）
  fingerprint   varchar(64),                  -- 归一特征哈希，用于自动匹配与变更检测
  status        varchar(16)  NOT NULL DEFAULT 'active',  -- active|offline（上游下架，软删除）
  synced_at     timestamptz,
  created_at    timestamptz,
  updated_at    timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_external_specs_type_external
  ON external_specs (provider_type, external_id);
CREATE INDEX IF NOT EXISTS idx_external_specs_provider_id ON external_specs (provider_id);
CREATE INDEX IF NOT EXISTS idx_external_specs_status ON external_specs (status);
CREATE INDEX IF NOT EXISTS idx_external_specs_fingerprint ON external_specs (fingerprint);

-- ---------------------------------------------------------------------------
-- 4. spec_bindings：外部规格 ↔ 内部标准规格双向绑定（T2.5）
-- ---------------------------------------------------------------------------
-- 由 spec_mappings 演进：新增 external_spec_id / direction / platform_params /
-- match_type / confidence / 状态机。spec_mappings 旧表保留一版只读（P8 清理）。
-- 状态机：unmapped → auto_mapped → confirmed → stale（指纹变化或归一升级触发）。
CREATE TABLE IF NOT EXISTS spec_bindings (
  id               bigserial PRIMARY KEY,
  external_spec_id bigint       NOT NULL,
  spec_template_id bigint,                        -- product_spec_templates.id
  direction        varchar(16)  NOT NULL DEFAULT 'outbound', -- inbound|outbound
  platform_params  jsonb,                         -- area_id/node_id/store/ip_group/flavor_ref 等
  match_type       varchar(16)  NOT NULL DEFAULT 'manual',   -- auto_exact|auto_range|manual
  status           varchar(16)  NOT NULL DEFAULT 'unmapped', -- unmapped|auto_mapped|confirmed|stale
  confidence       integer      NOT NULL DEFAULT 0,          -- 0-100
  confirmed_by     bigint,
  confirmed_at     timestamptz,
  remark           varchar(255),
  priority         integer      NOT NULL DEFAULT 0,          -- 预留：一规格多平台绑定的择优顺序
  created_at       timestamptz,
  updated_at       timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_spec_bindings_external_direction
  ON spec_bindings (external_spec_id, direction);
CREATE INDEX IF NOT EXISTS idx_spec_bindings_template ON spec_bindings (spec_template_id);
CREATE INDEX IF NOT EXISTS idx_spec_bindings_status ON spec_bindings (status);

-- ---------------------------------------------------------------------------
-- 5. resource_providers：传输契约列（T2.3 / T2.4）
-- ---------------------------------------------------------------------------
-- credentials：可扩展凭证 JSON（secret 字段值带 enc:v1: 前缀 = AES-256-GCM 密文）。
--   旧 api_key/api_secret 保留一版只读；解密失败必须报错，不再回退明文（地雷 L8）。
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS credentials jsonb;
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS timeout_seconds integer NOT NULL DEFAULT 0;
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS retry_max integer NOT NULL DEFAULT 0;
ALTER TABLE resource_providers ADD COLUMN IF NOT EXISTS rate_limit_qps integer NOT NULL DEFAULT 0;

-- 存量凭证搬迁：把非空的 api_key/api_secret 放进 credentials（值原样，密文仍带原编码；
-- 无 enc:v1: 前缀的历史明文由 credentials 包按"明文兼容"处理）。
-- 仅搬迁一次：credentials 为空的记录才写，避免覆盖人工新录入的凭证。
UPDATE resource_providers
SET credentials = jsonb_strip_nulls(jsonb_build_object('api_key', NULLIF(api_key, ''), 'api_secret', NULLIF(api_secret, '')))
WHERE credentials IS NULL AND (COALESCE(api_key, '') <> '' OR COALESCE(api_secret, '') <> '');

-- ---------------------------------------------------------------------------
-- 6. product_spec_templates：规格契约列（T2.5）
-- ---------------------------------------------------------------------------
-- spec_values：原子 key → 取值（替代散落的自由字段）；source：self 自建 / imported 上游归一。
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS spec_values jsonb;
ALTER TABLE product_spec_templates ADD COLUMN IF NOT EXISTS source varchar(16) DEFAULT 'self';

-- ---------------------------------------------------------------------------
-- 7. products：商品级原子覆盖（T2.5）
-- ---------------------------------------------------------------------------
-- 允许单个商品覆盖规格模板的原子取值，而不必新建模板。
ALTER TABLE products ADD COLUMN IF NOT EXISTS spec_atom_overrides jsonb;

COMMIT;
