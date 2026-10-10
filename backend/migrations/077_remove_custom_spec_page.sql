-- ============================================================================
-- 077_remove_custom_spec_page.sql
-- 下线「产品管理 → 规格管理 → 自定义规格」页（/product/spec/custom）。
--
-- 为什么删：该页是一份 11 个字段的规格表单，直接把 cpu/memory/disk/bandwidth/os
--   写进 product_spec_templates，但改编自 070 起已经没有意义：
--   - 它写的 spec_family（通用型/计算型/…）是已废弃分组，UI 与 SKU 编码都不再消费；
--   - 它不写 provider_type / option_selections / platform_params / *_template —— 也就是说
--     用这个页面存出来的档位，新建商品时既选不到平台配置项，也生成不出 SKU 与客户选配项；
--   - 「规格模板（配置档）」页 /product/spec/templates 已经完整覆盖同一张表，并且是唯一
--     按适配器目录配置、能真正打通建品链路的入口。
--   保留它等于留了一个「看起来能存档位、实际存出废档位」的坑。
--
-- 改动（只删页面与菜单，不动接口与权限码）：
--   ① 删除菜单行：admin + /product/spec/custom
--   ② 组内排序补空洞：规格管理组只剩「规格模板(1)」「规格映射(3)」「平台配置项(4)」
--      → 规格映射 归 2，平台配置项 归 3
--
-- 保留（刻意的）：
--   - POST /product/spec/templates 与权限码 spec:template:list 继续存在：「规格模板」页的
--     新增/编辑走同一接口与同一权限口径（permission_map.go 里 /product/spec/custom 的映射一并移除）。
--
-- 幂等：DELETE 带 component 约束（只删本次下线的这一行，未来若有同名页面不会被误删），
--   重复执行第二次删 0 行；UPDATE 带旧值判断。
-- 顺序：本迁移先跑，再重启后端（seed 不再声明该行，并会把排序对齐为上面的值）。
-- 回滚：把 db.go 的「自定义规格」行加回 admin:/product/spec（SortOrder=2，规格映射改 3、
--   平台配置项改 4），恢复 permission_map.go 的 /product/spec/custom 映射与 router 组件路由后
--   重启；本迁移的 DELETE 无法回滚（页面代码已删）。
-- ============================================================================

BEGIN;

-- ① 删除已下线的菜单行
DELETE FROM menus
 WHERE platform = 'admin'
   AND path = '/product/spec/custom'
   AND component = 'product/spec/custom/index';

-- ② 组内排序补空洞（规格映射 3 → 2、平台配置项 4 → 3）
UPDATE menus SET sort_order = 2
 WHERE platform = 'admin' AND path = '/product/spec/mappings' AND sort_order <> 2;

UPDATE menus SET sort_order = 3
 WHERE platform = 'admin' AND path = '/product/spec/option-catalog' AND sort_order <> 3;

COMMIT;

-- 验收查询（人工执行）
--   菜单行已删除：
--   SELECT count(*) FROM menus WHERE platform = 'admin' AND path = '/product/spec/custom';
--   期望：0
--   组内排序连续：
--   SELECT m.path, m.name, m.sort_order FROM menus m
--     JOIN menus p ON p.id = m.parent_id
--    WHERE m.platform = 'admin' AND p.path = '/product/spec' ORDER BY m.sort_order;
--   期望：/product/spec/templates(1, 规格模板)、/product/spec/mappings(2, 规格映射)、
--        /product/spec/option-catalog(3, 平台配置项)
--   接口仍在（规格模板页共用）：
--   SELECT count(*) FROM permissions WHERE code = 'spec:template:list';
--   期望：1
