-- ============================================================================
-- 108_instance_product_type.sql
-- 实例产品类型（产品线导航）＋ 下线「云主机实例」重复页
--
-- 背景（用户反馈）：实例管理下同时有「实例运维台」与「云主机实例」两个菜单，
--   它们是同一批数据的两个入口 ——「云主机实例」只读、「实例运维台」带全部运维动作，
--   运营看完一台机器要来回切页。同时产品线只有云主机一条，后期要扩 CDN / 物理机等，
--   需要一个能按产品线快速筛选的入口，避免每加一条产品线就新开一个菜单。
--
-- 改动分两块：
--
-- ① 数据：新增 instances.instance_type（产品类型），并从来源商品回填存量行。
--    判据与 products.product_type 同一命名空间（cloud_host / cdn / ip / storage /
--    bandwidth / physical / other），取值登记见 internal/pkg/model/product_kind.go。
--    回填优先级：sell_product_id → products.product_type；
--                upstream_product_id → 已导入商品的 product_type；
--                兜底 'cloud_host'（历史实例全部是云主机，是当下的真实分布）。
--    为什么单独存列而不联表商品：上游同步发现的实例没有本地商品（sell_product_id 为空），
--    联表会把这些行筛漏；而按产品线筛选必须能兜住它们。
--
-- ② 菜单：删除 admin 的 /instances/inventory（云主机实例）。
--    组内排序补空洞：操作流水 3 → 2（该组只剩「实例运维台(1)」「操作流水(2)」）。
--    产品线区分改由实例运维台内的导航承担：GET /instances/types 给出
--    「全部实例 / 云主机 / CDN / 物理机 / …」+ 各自实例数，点击即按 instance_type 过滤。
--
-- 保留（刻意的）：
--   - 后端只读接口 GET /admin/resource/instances 与 /{id} 继续存在：
--     「用户详情 → 资产」面板（AssetsPanel）复用同一接口，不从 HTTP 层下线。
--   - 权限码 resource:instance 不变（实例域所有页面共用）。
--
-- 幂等：ADD COLUMN IF NOT EXISTS；UPDATE 只处理 instance_type 为空/NULL 的行；
--   DELETE 带 component 约束（只删本次下线的这一行）。
--   Go 侧等价实现：db.go 的 backfillInstanceType（服务启动时执行，与迁移互为冗余）。
-- 顺序：本迁移先跑，再重启后端（seed 不再声明该行，并会把排序对齐为上面的值）。
-- 回滚：把 db.go 的「云主机实例」行加回 admin:/instances（SortOrder=2，操作流水改 3），
--   恢复 permission_map.go 的 /instances/inventory 映射、router 组件路由与页面文件
--   （resource/instances/index.vue，见 git 历史）、permission.ts navMenu 项后重启。
--   本迁移的 DELETE 无法回滚（页面代码已删）；instance_type 列保留无害。
-- ============================================================================

BEGIN;

-- ① 实例产品类型列
ALTER TABLE instances ADD COLUMN IF NOT EXISTS instance_type varchar(32);
CREATE INDEX IF NOT EXISTS idx_instances_instance_type ON instances (instance_type);

COMMENT ON COLUMN instances.instance_type IS '产品类型（cloud_host/cdn/physical/…）：实例导航按产品线分组的判据，见 pkg/model/product_kind.go';

-- ② 存量回填：来源商品 → 商品类型；两者皆无 → cloud_host（历史实例全是云主机）
UPDATE instances AS i
SET instance_type = COALESCE(
    NULLIF(TRIM((
        SELECT p.product_type FROM products AS p
        WHERE p.id = i.sell_product_id AND p.deleted_at IS NULL
    )), ''),
    NULLIF(TRIM((
        SELECT p3.product_type
        FROM resource_products AS rp
        JOIN products AS p3
          ON p3.source_product_id = rp.id
         AND p3.source_mode = 'upstream'
         AND p3.deleted_at IS NULL
        WHERE rp.id = i.upstream_product_id
        LIMIT 1
    )), ''),
    'cloud_host'
)
WHERE i.instance_type IS NULL OR TRIM(i.instance_type) = '';

-- ③ 下线「云主机实例」菜单行
DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/instances/inventory'
   AND component = 'resource/instances/index';

-- ④ 组内排序补空洞（操作流水 3 → 2、生命周期管理 4 → 3）
UPDATE menus SET sort_order = 2
 WHERE platform = 'admin' AND path = '/instances/operations' AND sort_order <> 2;

UPDATE menus SET sort_order = 3
 WHERE platform = 'admin' AND path = '/instances/lifecycle' AND sort_order <> 3;

COMMIT;

-- 验收查询（人工执行）
--   列已建且回填完毕（期望：0 行为空）：
--   SELECT count(*) FROM instances WHERE instance_type IS NULL OR TRIM(instance_type) = '';
--   类型分布（期望：现有实例全部 cloud_host）：
--   SELECT instance_type, count(*) FROM instances GROUP BY 1 ORDER BY 2 DESC;
--   菜单行已删除（期望：0）：
--   SELECT count(*) FROM menus WHERE platform='admin' AND path='/instances/inventory';
--   实例组排序连续（期望：实例运维台(1)、操作流水(2)；生命周期管理/云主机实例等见下方）：
--   SELECT m.path, m.name, m.sort_order FROM menus m JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform='admin' AND p.path='/instances' ORDER BY m.sort_order;
--   接口仍在（用户详情资产面板共用，期望：1）：
--   SELECT count(*) FROM permissions WHERE code='resource:instance';
