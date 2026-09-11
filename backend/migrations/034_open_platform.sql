-- ============================================================================
-- 034_open_platform.sql
-- 资源管理双链路重构 · P6 对外开放（T6.1）
--
-- 目的：开放平台基础设施 —— 下游应用（open_apps）以签名方式调用 /open/v1/*，
--   支持拉目录/询价/代客下单/实例查询与暂停（不含销毁，D2）/事件回调/对账。
--
-- 内容：
--   1. open_apps             —— 接入应用（secret 加密存，owner_user_id 挂定价与订单归属，D3）
--   2. open_app_scopes       —— 能力位（不含 instance:destroy，D2）
--   3. open_app_ip_rules     —— IP 白名单（可选，无规则=放行）
--   4. open_requests         —— 写接口幂等（(app_id, client_request_id) 唯一）
--   5. open_api_logs         —— 请求/响应摘要 + 耗时 + 错误码
--   6. open_notify_deliveries —— 事件回调死信（状态/次数/最后错误/重投）
--   7. orders 渠道三列       —— channel='open' 区分代客下单渠道
--
-- 幂等（R4）：CREATE TABLE/INDEX IF NOT EXISTS；ADD COLUMN IF NOT EXISTS。
-- 回滚：DROP TABLE IF EXISTS open_notify_deliveries, open_api_logs, open_requests,
--         open_app_ip_rules, open_app_scopes, open_apps;
--   ALTER TABLE orders DROP COLUMN IF EXISTS channel, DROP COLUMN IF EXISTS open_app_id,
--         DROP COLUMN IF EXISTS channel_customer_ref;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. open_apps：接入应用
-- ---------------------------------------------------------------------------
-- app_secret / notify_secret 为 internal/pkg/crypto 加密密文（AES-256-GCM，密钥 app.encrypt_key）；
-- owner_user_id 是该下游在平台的归属账号：定价走其用户组折扣策略（D3），代客下单余额也扣它。
CREATE TABLE IF NOT EXISTS open_apps (
  id            bigserial PRIMARY KEY,
  app_id        varchar(64)  NOT NULL,
  app_secret    text         NOT NULL,
  name          varchar(128) NOT NULL DEFAULT '',
  status        integer      NOT NULL DEFAULT 1,
  rate_limit    integer      NOT NULL DEFAULT 0,
  notify_url    varchar(512) NOT NULL DEFAULT '',
  notify_secret text,
  api_version   varchar(16)  NOT NULL DEFAULT 'v1',
  owner_user_id bigint       NOT NULL,
  created_at    timestamptz  NOT NULL DEFAULT now(),
  updated_at    timestamptz  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_open_apps_app_id
  ON open_apps (app_id);
CREATE INDEX IF NOT EXISTS idx_open_apps_owner
  ON open_apps (owner_user_id);

-- ---------------------------------------------------------------------------
-- 2. open_app_scopes：能力位（D2：不含 instance:destroy，销毁仅我方后台）
-- ---------------------------------------------------------------------------
-- scope ∈ catalog:read / order:create / instance:read / instance:renew /
--         instance:power / instance:suspend / audit:read；
-- object 预留范围限定（如限定可见分类 ID），空串表示不限。
CREATE TABLE IF NOT EXISTS open_app_scopes (
  id         bigserial PRIMARY KEY,
  app_id     bigint      NOT NULL,
  scope      varchar(64) NOT NULL,
  object     varchar(64) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_open_app_scopes
  ON open_app_scopes (app_id, scope, object);

-- ---------------------------------------------------------------------------
-- 3. open_app_ip_rules：IP 白名单（CIDR；应用无规则时放行全部来源）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS open_app_ip_rules (
  id         bigserial PRIMARY KEY,
  app_id     bigint      NOT NULL,
  cidr       varchar(64) NOT NULL,
  note       varchar(255) NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_open_app_ip_rules_app
  ON open_app_ip_rules (app_id);

-- ---------------------------------------------------------------------------
-- 4. open_requests：写接口幂等（doc16 §8.5）
-- ---------------------------------------------------------------------------
-- status ∈ processing / completed；completed 后 response_body 存首次响应快照，
-- 重放同 (app_id, client_request_id) 直接返回快照；processing 停滞超时的行可安全重执行。
CREATE TABLE IF NOT EXISTS open_requests (
  id                bigserial PRIMARY KEY,
  app_id            bigint       NOT NULL,
  client_request_id varchar(128) NOT NULL,
  method            varchar(8)   NOT NULL DEFAULT '',
  path              varchar(255) NOT NULL DEFAULT '',
  status            varchar(16)  NOT NULL DEFAULT 'processing',
  response_status   integer,
  response_body     text,
  created_at        timestamptz  NOT NULL DEFAULT now(),
  updated_at        timestamptz  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_open_requests_app_client
  ON open_requests (app_id, client_request_id);
CREATE INDEX IF NOT EXISTS idx_open_requests_app_created
  ON open_requests (app_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- 5. open_api_logs：请求/响应摘要（排障与对账）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS open_api_logs (
  id               bigserial PRIMARY KEY,
  app_id           bigint,
  method           varchar(8)   NOT NULL DEFAULT '',
  path             varchar(255) NOT NULL DEFAULT '',
  query            text,
  client_request_id varchar(128),
  status_code      integer,
  error_code       integer,
  duration_ms      integer,
  request_digest   text,
  response_digest  text,
  ip               varchar(64),
  created_at       timestamptz  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_open_api_logs_app_created
  ON open_api_logs (app_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- 6. open_notify_deliveries：事件回调投递与死信（T6.5）
-- ---------------------------------------------------------------------------
-- status ∈ pending / success / failed / dead；
-- 工作池按 (status, next_retry_at) 领取重试，attempts 耗尽转 dead，可人工重投。
CREATE TABLE IF NOT EXISTS open_notify_deliveries (
  id            bigserial PRIMARY KEY,
  app_id        bigint      NOT NULL,
  event         varchar(64) NOT NULL,
  payload       jsonb       NOT NULL,
  status        varchar(16) NOT NULL DEFAULT 'pending',
  attempts      integer     NOT NULL DEFAULT 0,
  max_attempts  integer     NOT NULL DEFAULT 8,
  last_error    text,
  next_retry_at timestamptz,
  delivered_at  timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_open_notify_deliveries_claim
  ON open_notify_deliveries (status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_open_notify_deliveries_app_created
  ON open_notify_deliveries (app_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- 7. orders：开放渠道标识（T6.3 代客下单）
-- ---------------------------------------------------------------------------
-- channel：'' 为平台自有渠道（存量语义不变），'open' 为下游应用代客下单；
-- open_app_id 指向 open_apps.id；channel_customer_ref 存下游自己的终端客户标识（对账用）。
ALTER TABLE orders ADD COLUMN IF NOT EXISTS channel varchar(16) NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS open_app_id bigint;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS channel_customer_ref varchar(128);

CREATE INDEX IF NOT EXISTS idx_orders_open_app ON orders (open_app_id);
CREATE INDEX IF NOT EXISTS idx_orders_channel ON orders (channel);

COMMIT;
