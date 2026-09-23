-- ============================================================================
-- 057_user_level_membership_ladder.sql
-- 背景（用户反馈）：用户等级此前是「标准用户 / 企业用户 / 高级企业」三段式，
--   名字像客户类型而不像会员权益层级；注册既不挂客户角色也不挂等级，
--   存量 42 个用户里 28 个 user_level_id 为空。
--
-- 本次把用户等级重新规划为**会员成长体系**：六个层级按累计消费升级（只升不降），
--   白银 → 黄金 → 铂金 → 钻石 → 星耀 → 王者
--   门槛 0 / 1000 / 5000 / 20000 / 50000 / 200000；子账号上限 1/2/5/10/20/50。
--   门槛为运营示例值，可在「用户等级」页调整。
--
-- 三个历史等级**原地改名复用，不新建不删除**：user_levels.id 被 users.user_level_id
--   与 user_level_change_logs.from/to_level_id 引用（14 个用户 + 15 条变更日志指向
--   id=1），新建行会让这些引用变成悬空，删除行则直接丢历史。因此：
--     id=1 标准用户 standard   → 白银会员 silver   （门槛 0，权重不变，起点）
--     id=2 企业用户 business   → 黄金会员 gold
--     id=3 高级企业 enterprise → 钻石会员 diamond
--   其余三级（铂金/星耀/王者）为新增行，由 code 幂等插入。
--
-- 回填：user_level_id IS NULL 的用户一律落到最低等级（白银会员），
--   与「注册即获得最低等级」的新口径一致。
--
-- 幂等：全部 UPDATE 带 WHERE、INSERT 走 ON CONFLICT (code) DO NOTHING，
--   可重复执行；改名步骤先探测目标 code 是否已被占用，占用即跳过（绝不撞唯一索引）。
-- 顺序：本迁移先跑，再重启后端（AutoMigrate 与 seedUserLevels 会把模型/默认值对齐到代码）。
-- 回滚：UPDATE user_levels SET code='standard'   WHERE code='silver';
--       UPDATE user_levels SET code='business'   WHERE code='gold';
--       UPDATE user_levels SET code='enterprise' WHERE code='diamond';
--       DELETE FROM user_levels WHERE code IN ('platinum','star','king');
--       UPDATE users SET user_level_id = NULL WHERE user_level_id = <silver id>;
--       DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE ...);  -- 角色兜底无法精确回滚
--       （回滚不回退 name/门槛/权益文案——那是本次的业务变更本身。）
-- ============================================================================
BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 历史三级原地改名（id 保持不变，引用不悬空）
--    先探测目标 code：若运营已手工建过同名 code，则跳过改名，交给下面的
--    INSERT ... ON CONFLICT 与统一 UPDATE 收敛，不冒撞 idx_user_levels_code 的风险。
-- ---------------------------------------------------------------------------
UPDATE user_levels SET code = 'silver'
 WHERE code = 'standard'
   AND NOT EXISTS (SELECT 1 FROM user_levels t WHERE t.code = 'silver');

UPDATE user_levels SET code = 'gold'
 WHERE code = 'business'
   AND NOT EXISTS (SELECT 1 FROM user_levels t WHERE t.code = 'gold');

UPDATE user_levels SET code = 'diamond'
 WHERE code = 'enterprise'
   AND NOT EXISTS (SELECT 1 FROM user_levels t WHERE t.code = 'diamond');

-- ---------------------------------------------------------------------------
-- 2. 六个等级统一收敛到会员阶梯的取值
--    INSERT 保证缺级（新库 / 被删过的等级）补齐，UPDATE 保证已有行拿到新语义。
-- ---------------------------------------------------------------------------
INSERT INTO user_levels (name, code, weight, status, feature_flags, upgrade_condition,
                         benefits, upgrade_threshold, max_sub_accounts, description,
                         created_by, updated_by, created_at, updated_at)
VALUES
  ('白银会员', 'silver',   10, 'active', 'snapshot,backup',                              '注册即获得',           '{"benefits":["基础工单支持","每周自动备份"]}'::jsonb,                             0,      1,  '会员体系起点等级', 0, 0, NOW(), NOW()),
  ('黄金会员', 'gold',     20, 'active', 'snapshot,backup,daily-backup',                '累计消费满 1000 元',   '{"benefits":["优先工单支持","每日自动备份"]}'::jsonb,                             1000,   2,  '成长型会员等级',   0, 0, NOW(), NOW()),
  ('铂金会员', 'platinum', 30, 'active', 'snapshot,backup,daily-backup,custom-image',   '累计消费满 5000 元',   '{"benefits":["高优先级工单","每日自动备份","自定义镜像"]}'::jsonb,                 5000,   5,  '进阶会员等级',     0, 0, NOW(), NOW()),
  ('钻石会员', 'diamond',  40, 'active', 'snapshot,backup,daily-backup,custom-image',   '累计消费满 20000 元',  '{"benefits":["专属客服通道","每日自动备份","自定义镜像","快照保留 30 天"]}'::jsonb, 20000, 10, '高价值会员等级',   0, 0, NOW(), NOW()),
  ('星耀会员', 'star',     50, 'active', 'snapshot,backup,daily-backup,custom-image,sla','累计消费满 50000 元', '{"benefits":["专属客户经理","SLA 保障","自定义镜像"]}'::jsonb,                     50000, 20, '重点客户会员等级', 0, 0, NOW(), NOW()),
  ('王者会员', 'king',     60, 'active', 'snapshot,backup,daily-backup,custom-image,sla,dedicated-support', '累计消费满 200000 元', '{"benefits":["一对一专属服务","最高 SLA 保障","全部高级权益"]}'::jsonb, 200000, 50, '最高会员等级', 0, 0, NOW(), NOW())
-- 不带列名：name 与 code 上各有一条唯一索引，运营若已手工建过同名行，
-- 两条约束任一条命中都应跳过而不是中止整个迁移。
ON CONFLICT DO NOTHING;

UPDATE user_levels SET
  name              = '白银会员',
  weight            = 10,
  status            = 'active',
  feature_flags     = 'snapshot,backup',
  upgrade_condition = '注册即获得',
  benefits          = '{"benefits":["基础工单支持","每周自动备份"]}'::jsonb,
  upgrade_threshold = 0,
  max_sub_accounts  = 1,
  description       = '会员体系起点等级',
  updated_at        = NOW()
WHERE code = 'silver';

UPDATE user_levels SET
  name              = '黄金会员',
  weight            = 20,
  status            = 'active',
  feature_flags     = 'snapshot,backup,daily-backup',
  upgrade_condition = '累计消费满 1000 元',
  benefits          = '{"benefits":["优先工单支持","每日自动备份"]}'::jsonb,
  upgrade_threshold = 1000,
  max_sub_accounts  = 2,
  description       = '成长型会员等级',
  updated_at        = NOW()
WHERE code = 'gold';

UPDATE user_levels SET
  name              = '铂金会员',
  weight            = 30,
  status            = 'active',
  feature_flags     = 'snapshot,backup,daily-backup,custom-image',
  upgrade_condition = '累计消费满 5000 元',
  benefits          = '{"benefits":["高优先级工单","每日自动备份","自定义镜像"]}'::jsonb,
  upgrade_threshold = 5000,
  max_sub_accounts  = 5,
  description       = '进阶会员等级',
  updated_at        = NOW()
WHERE code = 'platinum';

UPDATE user_levels SET
  name              = '钻石会员',
  weight            = 40,
  status            = 'active',
  feature_flags     = 'snapshot,backup,daily-backup,custom-image',
  upgrade_condition = '累计消费满 20000 元',
  benefits          = '{"benefits":["专属客服通道","每日自动备份","自定义镜像","快照保留 30 天"]}'::jsonb,
  upgrade_threshold = 20000,
  max_sub_accounts  = 10,
  description       = '高价值会员等级',
  updated_at        = NOW()
WHERE code = 'diamond';

UPDATE user_levels SET
  name              = '星耀会员',
  weight            = 50,
  status            = 'active',
  feature_flags     = 'snapshot,backup,daily-backup,custom-image,sla',
  upgrade_condition = '累计消费满 50000 元',
  benefits          = '{"benefits":["专属客户经理","SLA 保障","自定义镜像"]}'::jsonb,
  upgrade_threshold = 50000,
  max_sub_accounts  = 20,
  description       = '重点客户会员等级',
  updated_at        = NOW()
WHERE code = 'star';

UPDATE user_levels SET
  name              = '王者会员',
  weight            = 60,
  status            = 'active',
  feature_flags     = 'snapshot,backup,daily-backup,custom-image,sla,dedicated-support',
  upgrade_condition = '累计消费满 200000 元',
  benefits          = '{"benefits":["一对一专属服务","最高 SLA 保障","全部高级权益"]}'::jsonb,
  upgrade_threshold = 200000,
  max_sub_accounts  = 50,
  description       = '最高会员等级',
  updated_at        = NOW()
WHERE code = 'king';

-- ---------------------------------------------------------------------------
-- 3. 兜底：改名未生效时（目标 code 已被运营手工占用）的旧 code 行收敛
--    先把这些旧等级上的用户迁到对应新等级，再把旧行停用（不删除：
--    user_level_change_logs 里可能引用了它的 id/code，那是历史事实）。
--    停用后它不再参与升级判定（FindBestByThreshold 只看 active），
--    也不会出现在等级下拉里。
-- ---------------------------------------------------------------------------
UPDATE users SET user_level_id = (SELECT id FROM user_levels WHERE code = 'silver'), updated_at = NOW()
 WHERE user_level_id IN (SELECT id FROM user_levels WHERE code = 'standard');

UPDATE users SET user_level_id = (SELECT id FROM user_levels WHERE code = 'gold'), updated_at = NOW()
 WHERE user_level_id IN (SELECT id FROM user_levels WHERE code = 'business');

UPDATE users SET user_level_id = (SELECT id FROM user_levels WHERE code = 'diamond'), updated_at = NOW()
 WHERE user_level_id IN (SELECT id FROM user_levels WHERE code = 'enterprise');

UPDATE user_levels SET status = 'disabled', updated_at = NOW()
 WHERE code IN ('standard', 'business', 'enterprise');

-- ---------------------------------------------------------------------------
-- 4. 无等级用户回填到最低等级（白银会员）
--    口径与注册一致：新账号一律从最低等级起步，等级只由消费升级推进。
-- ---------------------------------------------------------------------------
UPDATE users
   SET user_level_id = (SELECT id FROM user_levels WHERE code = 'silver'),
       updated_at    = NOW()
 WHERE user_level_id IS NULL;

-- ---------------------------------------------------------------------------
-- 5. 客户角色兜底：一条角色都没有的客户账号补上 roles.code='user'
--    与「注册即挂普通用户角色」同一口径。只动「一条角色都没有」的账号：
--    已有角色的用户可能是运营显式分配的，补默认角色会改变其权限语义。
-- ---------------------------------------------------------------------------
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
  FROM users u
  CROSS JOIN (SELECT id FROM roles WHERE code = 'user' LIMIT 1) r
 WHERE NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id)
ON CONFLICT (user_id, role_id) DO NOTHING;

COMMIT;
