-- ============================================================================
-- 062_flash_discounts.sql
-- doc108 §8J：面向**用户**的限时活动折扣（与面向代理的拿货折扣彻底分开）。
--
-- 为什么新建表而不是复用 price_policies（P5-01 的折扣策略）：
--   price_policies 在迁移 059 把折扣来源迁回代理等级后就已全仓无人读取，
--   它的「生效窗口」字段只剩展示作用；而它绑定的主体是「用户组/代理」，
--   与本次要做的「面向普通用户、按时间窗生效的营销活动」不是同一个东西。
--   把两件事塞进一张表会让「这个折扣对谁生效、谁优先」重新变得说不清，
--   所以这里新建独立两表，price_policies 保留为历史遗留（菜单已下线）。
--
-- 算价接入：通过 pricing.Deps.PromotionRule 注入（见 server.go 装配）。
--   命中语义：**代理用户不参与活动折扣**，只走 agent_level_discounts；
--   活动折扣是「用户侧的营销活动」，与代理拿货价是两条互不叠加的线。
--
-- 幂等：CREATE TABLE IF NOT EXISTS + CREATE INDEX IF NOT EXISTS，可重复执行。
-- 回滚：
--   DROP TABLE IF EXISTS flash_discount_items, flash_discounts;
--   （取消菜单下线需手工补回菜单行，见文件末尾注释。）
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- A. 活动主体
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS flash_discounts (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(64)  NOT NULL,
    code           VARCHAR(64)  NOT NULL,
    description    VARCHAR(255),
    -- 折扣类型：rate（折扣率 0.85=八五折）/ amount（直减固定金额）
    discount_type  VARCHAR(16)  NOT NULL DEFAULT 'rate',
    discount_value NUMERIC(10,4) NOT NULL DEFAULT 0,
    -- 无定向条目时的作用范围：all / category / product
    scope          VARCHAR(16)  NOT NULL DEFAULT 'all',
    -- 生效窗口：start_at 为空 = 立即开始；end_at 为空 = 不限结束。
    -- 窗口在算价时实时判断，不写回 status，避免调度漏跑导致状态与事实不一致。
    start_at       TIMESTAMPTZ,
    end_at         TIMESTAMPTZ,
    status         VARCHAR(32)  NOT NULL DEFAULT 'active',
    remark         VARCHAR(255),
    created_at     TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_flash_discounts_code ON flash_discounts (code);
-- 算价每次命中都要按「启用 + 时间窗」筛，这条索引是热路径。
CREATE INDEX IF NOT EXISTS idx_flash_discounts_window ON flash_discounts (status, start_at, end_at);

COMMENT ON TABLE  flash_discounts          IS '限时活动折扣：面向普通用户的营销折扣，按生效窗口实时命中（doc108 §8J）';
COMMENT ON COLUMN flash_discounts.scope    IS '无定向条目时的作用范围：all / category / product';
COMMENT ON COLUMN flash_discounts.start_at IS '生效开始；NULL = 立即开始';
COMMENT ON COLUMN flash_discounts.end_at   IS '生效结束；NULL = 不限结束';

-- ---------------------------------------------------------------------------
-- B. 定向条目（分类 / 单个商品；有条目时按条目命中，商品优先于分类）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS flash_discount_items (
    id          BIGSERIAL PRIMARY KEY,
    discount_id BIGINT      NOT NULL,
    target_type VARCHAR(16) NOT NULL,  -- category / product
    target_id   BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_flash_discount_items
    ON flash_discount_items (discount_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_flash_discount_items_target
    ON flash_discount_items (target_type, target_id);

COMMENT ON TABLE flash_discount_items IS '活动折扣定向条目：分类/商品粒度（商品优先于分类命中）';

-- ---------------------------------------------------------------------------
-- C. 菜单下线：产品管理 → 定价与计费 → 「折扣策略」
--
--    该页对应的 price_policies 自迁移 059 起已不参与算价，页面上却还写着
--    「上线折扣请用折扣策略」，把运营往一个不生效的地方带。促销管理下的
--    「折扣活动」才是面向用户的活动折扣入口，代理折扣在用户组管理页。
--
--    注意：这只删库里的菜单行，同时需删掉 db.go seedMenuDefaults 里的同名
--    种子（否则重启会被种回来）——两处都改过才算真的下线。
-- ---------------------------------------------------------------------------
DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/product/pricing/policies'
   AND component = 'product/pricing/policies/index';

COMMIT;
