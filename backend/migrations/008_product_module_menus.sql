-- 008_product_module_menus.sql
-- 产品管理模块菜单树落库（对照 docs/实施计划/21-产品管理模块架构优化文档.md）。
-- 结构：采用「顶级分组 → 二级大类(directory) → 三级子菜单(menu)」三层树，
-- 与用户管理(resource/user)子树结构保持一致；非叶子目录(component=NULL) 仅作导航分组。
-- 幂等策略：先 UPDATE 后 INSERT（WHERE NOT EXISTS），不使用 TRUNCATE。

-- 1) 清理历史遗留：003 中旧的两级产品菜单（仅清理管理员平台，避免残留扁平叶子干扰三级树）
DELETE FROM menus
WHERE platform = 'admin'
  AND (
    path = '/products' OR path LIKE '/products/%'
    OR (path = '/product/products' AND parent_id NOT IN (SELECT id FROM menus WHERE path = '/product/mgmt'))
  );

-- 2) 顶级目录：产品管理（更新为 directory）
UPDATE menus
SET name = '产品管理', type = 'directory', component = NULL,
    icon = 'product', sort_order = 4, status = 'active'
WHERE platform = 'admin' AND path = '/product';

INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT 0, 'admin', '产品管理', 'directory', '/product', NULL, 'product', 4, 'active'
WHERE NOT EXISTS (SELECT 1 FROM menus WHERE platform = 'admin' AND path = '/product');

-- 3) 二级大类：directory 节点补插（不存在才插，保持幂等）
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT p.id, 'admin', v.name, 'directory', v.path, NULL, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    -- 1. 商品管理
    ('商品管理',       '/product/mgmt',           'product',         1),
    -- 2. 规格管理
    ('规格管理',       '/product/spec',           'layers',          2),
    -- 3. 定价与计费
    ('定价与计费',     '/product/pricing-center', 'money',           3),
    -- 4. 促销管理
    ('促销管理',       '/product/promotion',      'tag',             4),
    -- 5. 商品分类
    ('商品分类',       '/product/category',       'folder',          5),
    -- 6. 上游商品同步
    ('上游商品同步',   '/product/sync-center',    'cloud-download',  6)
) AS v(name, path, icon, sort_order)
JOIN menus AS p ON p.platform = 'admin' AND p.path = '/product'
WHERE NOT EXISTS (SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = v.path);

-- 4) 三级子菜单：已有节点原地更新（重挂 parent_id + 改名/排序/图标）
UPDATE menus AS m
SET parent_id  = d.id,
    name       = v.name,
    type       = 'menu',
    component  = v.component,
    icon       = v.icon,
    sort_order = v.sort_order,
    status     = 'active'
FROM (
    VALUES
    -- 商品管理
    ('商品列表',   '/product/products',             'product/products/index',            'product',        1, '/product/mgmt'),
    -- 规格管理
    ('规格模板',   '/product/spec/templates',       'product/spec/templates/index',      'catalog',        1, '/product/spec'),
    ('自定义规格', '/product/spec/custom',          'product/spec/custom/index',         'add',            2, '/product/spec'),
    ('规格映射',   '/product/spec/mappings',        'product/spec/mappings/index',       'link',           3, '/product/spec'),
    -- 定价与计费
    ('价格策略',   '/product/pricing',              'product/pricing/index',             'money',          1, '/product/pricing-center'),
    ('价格计算器', '/product/pricing/calculator',   'product/pricing/calculator/index',  'chart-bar',      2, '/product/pricing-center'),
    ('价格历史',   '/product/pricing/history',      'product/pricing/history/index',     'history',        3, '/product/pricing-center'),
    -- 促销管理
    ('优惠券管理', '/product/promotion/coupons',    'product/promotion/coupons/index',   'ticket',         1, '/product/promotion'),
    ('折扣活动',   '/product/promotion/activities', 'product/promotion/activities/index','chart-bar',      2, '/product/promotion'),
    ('套餐组合',   '/product/promotion/bundles',    'product/promotion/bundles/index',   'app',            3, '/product/promotion'),
    ('推荐位管理', '/product/promotion/recommends', 'product/promotion/recommends/index','star',           4, '/product/promotion'),
    -- 商品分类
    ('分类管理',   '/product/categories',           'product/categories/index',          'tag',            1, '/product/category'),
    -- 上游商品同步
    ('同步任务',   '/product/sync/tasks',           'product/sync/tasks/index',          'refresh',        1, '/product/sync-center'),
    ('同步日志',   '/product/sync/logs',            'product/sync/logs/index',           'history',        2, '/product/sync-center'),
    ('差异对比',   '/product/sync/diff',            'product/sync/diff/index',           'data-checked',   3, '/product/sync-center')
) AS v(name, path, component, icon, sort_order, group_path)
JOIN menus AS d ON d.platform = 'admin' AND d.path = v.group_path
WHERE m.platform = 'admin' AND m.path = v.path;

-- 5) 三级子菜单：缺失节点补插
INSERT INTO menus (parent_id, platform, name, type, path, component, icon, sort_order, status)
SELECT d.id, 'admin', v.name, 'menu', v.path, v.component, v.icon, v.sort_order, 'active'
FROM (
    VALUES
    ('商品列表',   '/product/products',             'product/products/index',            'product',        1, '/product/mgmt'),
    -- 规格管理
    ('规格模板',   '/product/spec/templates',       'product/spec/templates/index',      'catalog',        1, '/product/spec'),
    ('自定义规格', '/product/spec/custom',          'product/spec/custom/index',         'add',            2, '/product/spec'),
    ('规格映射',   '/product/spec/mappings',        'product/spec/mappings/index',       'link',           3, '/product/spec'),
    ('价格策略',   '/product/pricing',              'product/pricing/index',             'money',          1, '/product/pricing-center'),
    ('价格计算器', '/product/pricing/calculator',   'product/pricing/calculator/index',  'chart-bar',      2, '/product/pricing-center'),
    ('价格历史',   '/product/pricing/history',      'product/pricing/history/index',     'history',        3, '/product/pricing-center'),
    ('优惠券管理', '/product/promotion/coupons',    'product/promotion/coupons/index',   'ticket',         1, '/product/promotion'),
    ('折扣活动',   '/product/promotion/activities', 'product/promotion/activities/index','chart-bar',      2, '/product/promotion'),
    ('套餐组合',   '/product/promotion/bundles',    'product/promotion/bundles/index',   'app',            3, '/product/promotion'),
    ('推荐位管理', '/product/promotion/recommends', 'product/promotion/recommends/index','star',           4, '/product/promotion'),
    ('分类管理',   '/product/categories',           'product/categories/index',          'tag',            1, '/product/category'),
    ('同步任务',   '/product/sync/tasks',           'product/sync/tasks/index',          'refresh',        1, '/product/sync-center'),
    ('同步日志',   '/product/sync/logs',            'product/sync/logs/index',           'history',        2, '/product/sync-center'),
    ('差异对比',   '/product/sync/diff',            'product/sync/diff/index',           'data-checked',   3, '/product/sync-center')
) AS v(name, path, component, icon, sort_order, group_path)
JOIN menus AS d ON d.platform = 'admin' AND d.path = v.group_path
WHERE NOT EXISTS (
    SELECT 1 FROM menus e WHERE e.platform = 'admin' AND e.path = v.path
);
