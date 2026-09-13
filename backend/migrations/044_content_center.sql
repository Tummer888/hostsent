-- ============================================================================
-- 044_content_center.sql
--
-- 背景（docs/实施计划/100 内容中心与站点内容管理设计）：
--   1) 内容域此前只有 announcements 一张表，新闻/帮助中心/用户条款/隐私政策
--      在三端与后端全部为零：门户页脚「新闻资讯 / 法律条文 / 隐私政策」指向
--      /#contact 锚点，用户中心帮助入口指向工单页，注册页两个协议链接是 href="#"。
--   2) 页脚四项（服务承诺 / 社交按钮 / 链接栏目 / 友情链接）全部硬编码在
--      AppFooter.vue 的模块级 const 里，运营改不动。
--   3) 公告正文是纯文本 textarea，公告表缺置顶与富文本格式标记。
--
-- 内容：
--   1. content_articles    —— 新闻 / 帮助 / 条款 / 隐私四类共用一张表（kind 判别）
--   2. content_categories  —— 新闻分类（平铺）与帮助目录树（parent_id 组树）
--   3. friendly_links      —— 友情链接（独立表：需要 logo / 排序 / 上下架）
--   4. announcements 增补  —— pinned / slug / body_format
--
-- 设计要点：
--   - slug 做门户路由（/news/:slug 比 /news/37 对 SEO 与可读性都更好），
--     唯一索引带 kind，不同类型的 slug 允许重名。
--   - body 存「服务端已净化的 HTML」（internal/pkg/sanitize 白名单策略），
--     body_format 显式标注格式，避免存量纯文本被当 HTML 渲染。
--   - 复用公告的 draft/published/offline 状态语义，管理员心智一致。
--
-- 幂等：CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS /
--       CREATE INDEX IF NOT EXISTS / INSERT ... ON CONFLICT DO NOTHING。
--
-- 回滚：
--   DROP TABLE IF EXISTS friendly_links, content_categories, content_articles;
--   ALTER TABLE announcements DROP COLUMN IF EXISTS pinned,
--     DROP COLUMN IF EXISTS slug, DROP COLUMN IF EXISTS body_format;
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 内容文章（新闻 / 帮助 / 条款 / 隐私）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS content_articles (
    id           BIGSERIAL PRIMARY KEY,
    kind         varchar(20)  NOT NULL,                    -- news / help / terms / privacy
    category_id  bigint       NOT NULL DEFAULT 0,          -- content_categories.id，0=未分类
    slug         varchar(120) NOT NULL,                    -- URL 友好标识
    title        varchar(255) NOT NULL,
    summary      varchar(500) NOT NULL DEFAULT '',         -- 列表摘要；空则由正文剥标签生成
    body         text         NOT NULL DEFAULT '',         -- 已净化的 HTML
    body_format  varchar(10)  NOT NULL DEFAULT 'html',     -- html（预留 md 扩展位）
    cover        varchar(255) NOT NULL DEFAULT '',
    tags         varchar(255) NOT NULL DEFAULT '',         -- 逗号分隔（二期启用）
    status       varchar(20)  NOT NULL DEFAULT 'draft',    -- draft / published / offline
    pinned       boolean      NOT NULL DEFAULT false,
    sort_order   int          NOT NULL DEFAULT 0,
    version      varchar(20)  NOT NULL DEFAULT '',         -- 条款/隐私的版本号，如 v1.2
    publish_at   timestamptz,
    offline_at   timestamptz,
    operator_id  bigint       NOT NULL DEFAULT 0,
    created_at   timestamptz,
    updated_at   timestamptz
);

-- 同类型内 slug 唯一；不同 kind 可重名（news/help 各有 install 是合理的）
CREATE UNIQUE INDEX IF NOT EXISTS uk_content_articles_kind_slug
    ON content_articles (kind, slug);
CREATE INDEX IF NOT EXISTS idx_content_articles_kind_status
    ON content_articles (kind, status);
CREATE INDEX IF NOT EXISTS idx_content_articles_publish_at
    ON content_articles (publish_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_articles_category
    ON content_articles (kind, category_id);

-- ---------------------------------------------------------------------------
-- 2. 内容分类（新闻分栏 + 帮助目录树，一张表两种形态）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS content_categories (
    id          BIGSERIAL PRIMARY KEY,
    kind        varchar(20) NOT NULL,                  -- news / help
    parent_id   bigint      NOT NULL DEFAULT 0,        -- 0=顶级；帮助中心靠它成树
    name        varchar(64) NOT NULL,
    slug        varchar(64) NOT NULL,
    description varchar(255) NOT NULL DEFAULT '',
    icon        varchar(64)  NOT NULL DEFAULT '',
    sort_order  int          NOT NULL DEFAULT 0,
    status      varchar(20)  NOT NULL DEFAULT 'active',-- active / disabled
    created_at  timestamptz,
    updated_at  timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_content_categories_kind_slug
    ON content_categories (kind, slug);
CREATE INDEX IF NOT EXISTS idx_content_categories_parent
    ON content_categories (kind, parent_id, sort_order);

-- ---------------------------------------------------------------------------
-- 3. 友情链接
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS friendly_links (
    id          BIGSERIAL PRIMARY KEY,
    name        varchar(64)  NOT NULL,
    url         varchar(255) NOT NULL,
    logo        varchar(255) NOT NULL DEFAULT '',
    description varchar(255) NOT NULL DEFAULT '',
    sort_order  int          NOT NULL DEFAULT 0,
    open_in_new boolean      NOT NULL DEFAULT true,    -- 外链默认新窗口
    status      varchar(20)  NOT NULL DEFAULT 'active',-- active / disabled
    created_at  timestamptz,
    updated_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_friendly_links_status
    ON friendly_links (status, sort_order);

-- ---------------------------------------------------------------------------
-- 4. 公告表增补（doc100 §5.4）
--    content 在富文本方案下语义为「已净化的 HTML」，用 body_format 显式标注；
--    pinned 支撑「重要通知压过常规公告」；slug 支撑门户 /announcements/:slug。
-- ---------------------------------------------------------------------------
ALTER TABLE announcements
  ADD COLUMN IF NOT EXISTS pinned      boolean      NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS slug        varchar(120) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS body_format varchar(10)  NOT NULL DEFAULT 'text';

-- 门户按 slug 取公告详情
CREATE INDEX IF NOT EXISTS idx_announcements_slug
    ON announcements (slug) WHERE slug <> '';

-- 列表排序 pinned 优先
CREATE INDEX IF NOT EXISTS idx_announcements_pinned_publish
    ON announcements (pinned DESC, publish_at DESC);

COMMIT;
