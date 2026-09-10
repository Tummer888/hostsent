-- 016_product_cover_image.sql
-- 商品封面图：官网门户（frontend-site）产品卡与详情页展示用。
-- 背景：products 表此前无任何图片字段，官网产品卡只能退化到纯文字卡片；
--       见 docs/实施计划/80-官网门户与品牌配置架构设计.md §9.3。
-- 幂等策略：ADD COLUMN IF NOT EXISTS；存量行留空，由前端用占位图兜底。

ALTER TABLE products ADD COLUMN IF NOT EXISTS cover_image VARCHAR(255);

COMMENT ON COLUMN products.cover_image IS '封面图 URL（官网产品卡/详情页展示，空则由前端占位图兜底）';
