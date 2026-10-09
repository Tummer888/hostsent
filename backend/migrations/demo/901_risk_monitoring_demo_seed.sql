-- ============================================================================
-- 901_risk_monitoring_demo_seed.sql · 异常行为监控演示造数（doc06 §4.3/§4.4）
-- ----------------------------------------------------------------------------
-- 用途：让「异常行为监控」页一打开就有可核对的真实数据 —— 五条规则、四个等级、
--       三种待办状态、以及五种处置动作（拉黑 / 失效会话 / 调整等级 / 处置 /
--       忽略）全部各有样本，用于人工验收与演示。
--
-- 为什么不做进 db.go 的 seedDemoRiskEvents：
--   那个种子只在 risk_events 为空时执行一次（count>0 直接 return），联调库里
--   已经有事件，改它对本库没有任何效果；演示造数需要可重复执行、可单独回滚。
--
-- 执行方式（手动，不会被 scripts/migrate.sh 自动应用 —— 该脚本只 glob
-- migrations/*.sql，不含 migrations/demo/）：
--   docker exec -i backend-postgres-1 psql -U hostsent -d hostsent -v ON_ERROR_STOP=1 \
--     < backend/migrations/demo/901_risk_monitoring_demo_seed.sql
--
-- 幂等性：事件逐行以业务键 (username, rule_code, ip) 做 NOT EXISTS 守卫；
--         黑名单走 (type, target_value) 唯一索引 ON CONFLICT DO NOTHING；
--         处置流水以 (event_id, action) 守卫；登录日志同样按业务键守卫。
--         重复执行不新增、不覆盖。
--
-- 时间锚点：所有事件都落在 1 小时 ~ 6 天前，**刻意远离**规则引擎的聚合窗口
--           （默认 10 分钟）。否则后续真实登录会聚合进这些演示事件、把
--           occur_count 加上去，演示数据就不再可解释了。
--
-- 账号：沿用 900 号种子造的 demo_user_* 演示客户与会话；撞库/扫描类事件用
--       不存在的账号名（user_id=0）——那正是这类事件的真实形态。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 风险事件（10 条）
--    ip 段刻意避开 live 测试用的 192.0.2.x / 198.51.100.x / 203.0.113.x，
--    否则跑一次 live 用例就会被 cleanupRiskArtifacts 按 IP 删掉。
-- ---------------------------------------------------------------------------

-- ① 暴力破解 · 高 · 待处理 · 未做任何管控（「先看这里」的样本）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status, created_at, updated_at)
SELECT 'brute_force', 'high', u.id, u.username, '61.160.36.7', 'fp-demo-north-02',
       'LOGIN_FAIL_THRESHOLD', '账号在 10 分钟内连续登录失败 9 次',
       '{"fail_count":9,"window_minutes":10,"threshold":5,"failure_reason":"密码错误"}', 9,
       now() - interval '95 minutes', now() - interval '22 minutes', 'pending',
       now() - interval '95 minutes', now() - interval '22 minutes'
FROM users u WHERE u.username = 'user_north_02'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'user_north_02'
                    AND rule_code = 'LOGIN_FAIL_THRESHOLD' AND ip = '61.160.36.7');

-- ② 撞库 · 严重 · 待处理 · 已拉黑来源 IP（「已经封了但还在盯」的样本，
--    这正是状态与动作两个维度分开后才表示得出来的组合）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status, created_at, updated_at)
SELECT 'brute_force', 'critical', 0, 'svc_backup_probe', '91.219.60.14', '',
       'IP_MULTI_ACCOUNT_FAIL', 'IP 在 10 分钟内对 24 个不同账号登录失败，疑似撞库',
       '{"account_count":24,"window_minutes":10,"threshold":8}', 24,
       now() - interval '6 days', now() - interval '40 minutes', 'pending',
       now() - interval '6 days', now() - interval '40 minutes'
WHERE NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'svc_backup_probe'
                    AND rule_code = 'IP_MULTI_ACCOUNT_FAIL' AND ip = '91.219.60.14');

-- ③ 登录频率异常 · 中 · 待处理 · 已调整等级（提级不等于收单）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status, created_at, updated_at)
SELECT 'high_frequency', 'medium', u.id, u.username, '185.199.108.153', 'fp-demo-03',
       'LOGIN_BURST', 'IP 在 5 分钟内发起 42 次登录，频率异常',
       '{"request_count":42,"window_minutes":5,"threshold":30}', 42,
       now() - interval '4 days', now() - interval '3 hours', 'pending',
       now() - interval '4 days', now() - interval '3 hours'
FROM users u WHERE u.username = 'demo_user_03'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'demo_user_03'
                    AND rule_code = 'LOGIN_BURST' AND ip = '185.199.108.153');

-- ④ 设备变更 · 低 · 已处置（收单但未做管控 —— 确认是正常换机）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status,
                         handled_by, handled_at, handle_note, created_at, updated_at)
SELECT 'device_change', 'low', u.id, u.username, '103.28.14.7', 'fp-demo-07-new',
       'DEVICE_FINGERPRINT_CHANGED', '登录设备与历史常用设备不一致',
       '{"new_device":"fp-demo-07-new","known_count":2,"previous_device":"fp-demo-07-old"}', 3,
       now() - interval '5 days', now() - interval '2 days', 'handled',
       a.id, now() - interval '2 days' + interval '10 minutes', '用户自助换机，已确认',
       now() - interval '5 days', now() - interval '2 days'
FROM users u, admins a
WHERE u.username = 'demo_user_07' AND a.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'demo_user_07'
                    AND rule_code = 'DEVICE_FINGERPRINT_CHANGED' AND ip = '103.28.14.7');

-- ⑤ 新 IP 登录 · 中 · 已处置 · 已失效会话（收单 + 管控一次点完）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status,
                         handled_by, handled_at, handle_note, created_at, updated_at)
SELECT 'suspicious_ip', 'medium', u.id, u.username, '45.32.100.21', 'fp-demo-12',
       'NEW_IP_LOGIN', '账号从新 IP 登录', '{"new_ip":"45.32.100.21","known_count":3,"previous_ip":"121.51.9.70"}', 2,
       now() - interval '3 days', now() - interval '26 hours', 'handled',
       a.id, now() - interval '26 hours' + interval '5 minutes', '异地登录已核实并强制下线',
       now() - interval '3 days', now() - interval '26 hours'
FROM users u, admins a
WHERE u.username = 'demo_user_12' AND a.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'demo_user_12'
                    AND rule_code = 'NEW_IP_LOGIN' AND ip = '45.32.100.21');

-- ⑥ 员工主体 · 低 · 已忽略（subject_type='admin'：页面应显示「员工」徽标，
--    且「失效会话」对这条应被明确拒绝而不是假装成功）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status, subject_type,
                         handled_by, handled_at, handle_note, created_at, updated_at)
SELECT 'device_change', 'low', a.id, a.username, '172.20.5.31', 'fp-demo-admin-op',
       'DEVICE_FINGERPRINT_CHANGED', '登录设备与历史常用设备不一致',
       '{"new_device":"fp-demo-admin-op","known_count":4,"previous_device":"fp-admin-01"}', 1,
       now() - interval '38 hours', now() - interval '38 hours', 'ignored', 'admin',
       a.id, now() - interval '37 hours', '运维机重装，已知悉',
       now() - interval '38 hours', now() - interval '37 hours'
FROM admins a WHERE a.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'admin'
                    AND rule_code = 'DEVICE_FINGERPRINT_CHANGED' AND ip = '172.20.5.31');

-- ⑦ 暴力破解 · 高 · 已处置 · 拉黑设备 + 失效会话 + 处置（三个动作齐上）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status,
                         handled_by, handled_at, handle_note, created_at, updated_at)
SELECT 'brute_force', 'high', u.id, u.username, '178.62.33.90', 'fp-demo-risk-01',
       'LOGIN_FAIL_THRESHOLD', '账号在 10 分钟内连续登录失败 12 次',
       '{"fail_count":12,"window_minutes":10,"threshold":5,"failure_reason":"密码错误"}', 12,
       now() - interval '30 hours', now() - interval '20 hours', 'handled',
       a.id, now() - interval '20 hours' + interval '8 minutes', '确认为撞库尝试，已封设备并下线',
       now() - interval '30 hours', now() - interval '20 hours'
FROM users u, admins a
WHERE u.username = 'demo_user_01' AND a.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'demo_user_01'
                    AND rule_code = 'LOGIN_FAIL_THRESHOLD' AND ip = '178.62.33.90');

-- ⑧ 设备变更 · 低 · 已忽略（误报样本）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status,
                         handled_by, handled_at, handle_note, created_at, updated_at)
SELECT 'device_change', 'low', u.id, u.username, '101.35.66.28', 'fp-demo-south-02-new',
       'DEVICE_FINGERPRINT_CHANGED', '登录设备与历史常用设备不一致',
       '{"new_device":"fp-demo-south-02-new","known_count":1,"previous_device":"fp-south-02-old"}', 1,
       now() - interval '18 hours', now() - interval '18 hours', 'ignored',
       a.id, now() - interval '17 hours', '误报：同型号新机，指纹取随机盐',
       now() - interval '18 hours', now() - interval '17 hours'
FROM users u, admins a
WHERE u.username = 'user_south_02' AND a.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'user_south_02'
                    AND rule_code = 'DEVICE_FINGERPRINT_CHANGED' AND ip = '101.35.66.28');

-- ⑨ 新 IP 登录 · 低 · 待处理 · 未做管控（低优先级待办，演示「不是所有待办都要马上动手」）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status, created_at, updated_at)
SELECT 'suspicious_ip', 'low', u.id, u.username, '124.71.18.6', 'fp-demo-sw-01',
       'NEW_IP_LOGIN', '账号从新 IP 登录',
       '{"new_ip":"124.71.18.6","known_count":2,"previous_ip":"113.108.77.20"}', 1,
       now() - interval '9 hours', now() - interval '9 hours', 'pending',
       now() - interval '9 hours', now() - interval '9 hours'
FROM users u WHERE u.username = 'user_sw_01'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'user_sw_01'
                    AND rule_code = 'NEW_IP_LOGIN' AND ip = '124.71.18.6');

-- ⑩ 扫描器 · 高 · 已处置 · 拉黑 + 处置 + 等级由中升到高（完整时间线样本）
INSERT INTO risk_events (risk_type, risk_level, user_id, username, ip, device_fingerprint,
                         rule_code, summary, detail_payload, occur_count,
                         first_occurred_at, last_occurred_at, status,
                         handled_by, handled_at, handle_note, created_at, updated_at)
SELECT 'high_frequency', 'high', 0, 'api_scanner_node3', '45.13.190.44', '',
       'LOGIN_BURST', 'IP 在 5 分钟内发起 61 次登录，频率异常',
       '{"request_count":61,"window_minutes":5,"threshold":30}', 61,
       now() - interval '11 days', now() - interval '6 days', 'handled',
       a.id, now() - interval '6 days' + interval '20 minutes', '自动化扫描器，永久封禁来源 IP',
       now() - interval '11 days', now() - interval '6 days'
FROM admins a WHERE a.username = 'admin'
  AND NOT EXISTS (SELECT 1 FROM risk_events WHERE username = 'api_scanner_node3'
                    AND rule_code = 'LOGIN_BURST' AND ip = '45.13.190.44');

-- ---------------------------------------------------------------------------
-- 2. 处置流水（risk_event_actions）—— 页面「已做动作」列与详情抽屉时间线的数据源
--    与 risk_events.status 是两个维度：拉黑/踢会话不改状态，只追加流水。
-- ---------------------------------------------------------------------------
INSERT INTO risk_event_actions (event_id, action, operator_id, note, detail, created_at)
SELECT e.id, v.action, (SELECT id FROM admins WHERE username = 'admin'), v.note, v.detail,
       now() - v.ago
FROM risk_events e
JOIN (VALUES
  -- ② 撞库：只拉黑、不收单 —— 状态仍是「待处理」
  ('svc_backup_probe', 'IP_MULTI_ACCOUNT_FAIL', '91.219.60.14', 'blacklist',
   '确认是撞库源，先封 IP，继续观察',
   '{"type":"ip","target_value":"91.219.60.14"}', interval '35 minutes'),
  -- ③ 频率异常：只提级，不收单
  ('demo_user_03', 'LOGIN_BURST', '185.199.108.153', 'level',
   '结合工单判断为压测，等级提到高',
   '{"from":"medium","to":"high","closed":false}', interval '90 minutes'),
  -- ④ 换机：只收单，不做管控
  ('demo_user_07', 'DEVICE_FINGERPRINT_CHANGED', '103.28.14.7', 'handle',
   '用户自助换机，已确认', '{"status":"handled","outcomes":[]}', interval '2 days'),
  -- ⑤ 异地登录：收单 + 踢会话
  ('demo_user_12', 'NEW_IP_LOGIN', '45.32.100.21', 'revoke_sessions',
   '异地登录已核实并强制下线', '{"revoked":2,"subject_type":"user"}', interval '26 hours'),
  ('demo_user_12', 'NEW_IP_LOGIN', '45.32.100.21', 'handle',
   '异地登录已核实并强制下线',
   '{"status":"handled","outcomes":["已失效 2 个会话"]}', interval '26 hours'),
  -- ⑥ 员工换机：忽略
  ('admin', 'DEVICE_FINGERPRINT_CHANGED', '172.20.5.31', 'ignore',
   '运维机重装，已知悉', '{"status":"ignored","outcomes":[]}', interval '37 hours'),
  -- ⑦ 撞库：封设备 + 踢会话 + 收单
  ('demo_user_01', 'LOGIN_FAIL_THRESHOLD', '178.62.33.90', 'blacklist',
   '确认为撞库尝试，封设备指纹',
   '{"type":"device","target_value":"fp-demo-risk-01"}', interval '20 hours'),
  ('demo_user_01', 'LOGIN_FAIL_THRESHOLD', '178.62.33.90', 'revoke_sessions',
   '确认为撞库尝试，已封设备并下线', '{"revoked":1,"subject_type":"user"}', interval '20 hours'),
  ('demo_user_01', 'LOGIN_FAIL_THRESHOLD', '178.62.33.90', 'handle',
   '确认为撞库尝试，已封设备并下线',
   '{"status":"handled","outcomes":["已失效 1 个会话"]}', interval '20 hours'),
  -- ⑧ 误报：忽略
  ('user_south_02', 'DEVICE_FINGERPRINT_CHANGED', '101.35.66.28', 'ignore',
   '误报：同型号新机，指纹取随机盐', '{"status":"ignored","outcomes":[]}', interval '17 hours'),
  -- ⑩ 扫描器：先提级 → 再拉黑 → 最后收单（时间线顺序即真实操作顺序）
  ('api_scanner_node3', 'LOGIN_BURST', '45.13.190.44', 'level',
   '来源是自动化扫描器，提级',
   '{"from":"medium","to":"high","closed":false}', interval '6 days 30 minutes'),
  ('api_scanner_node3', 'LOGIN_BURST', '45.13.190.44', 'blacklist',
   '自动化扫描器，永久封禁来源 IP',
   '{"type":"ip","target_value":"45.13.190.44"}', interval '6 days 25 minutes'),
  ('api_scanner_node3', 'LOGIN_BURST', '45.13.190.44', 'handle',
   '自动化扫描器，永久封禁来源 IP',
   '{"status":"handled","outcomes":["已拉黑 IP 45.13.190.44"]}', interval '6 days 20 minutes')
) AS v(username, rule_code, ip, action, note, detail, ago)
  ON e.username = v.username AND e.rule_code = v.rule_code AND e.ip = v.ip
WHERE NOT EXISTS (
  SELECT 1 FROM risk_event_actions a WHERE a.event_id = e.id AND a.action = v.action
);

-- ---------------------------------------------------------------------------
-- 3. 黑名单（对应上面的管控动作；四种 runtime_status 各有一条样本）
--    注意：不要封任何测试/本机会用到的账号与 IP，否则会把自己锁在外面。
-- ---------------------------------------------------------------------------
INSERT INTO blacklists (type, target_value, status, source, reason,
                        effective_at, expired_at, hit_count, created_by, updated_by,
                        created_at, updated_at)
SELECT v.type, v.target_value, v.status, v.source, v.reason,
       now() - v.effective_ago, CASE WHEN v.expire_ago IS NULL THEN NULL ELSE now() - v.expire_ago END,
       v.hit_count, a.id, a.id, now() - v.created_ago, now() - v.effective_ago
FROM admins a
CROSS JOIN (VALUES
  ('ip',     '91.219.60.14',     'active',   'risk_event', '撞库来源 IP（演示：仍在拦截）',      interval '35 minutes', NULL::interval,        12, interval '35 minutes'),
  ('device', 'fp-demo-risk-01',  'active',   'risk_event', '撞库尝试所用设备指纹（演示：仍在拦截）', interval '20 hours',  NULL::interval,         3, interval '20 hours'),
  ('ip',     '45.13.190.44',     'active',   'risk_event', '自动化扫描器来源 IP（演示：仍在拦截）', interval '6 days',     NULL::interval,        61, interval '6 days'),
  ('ip',     '61.160.36.7',      'inactive', 'system',     '暴力破解来源（演示：已停用）',        interval '30 days',    NULL::interval,         9, interval '30 days'),
  ('ip',     '103.28.14.7',      'active',   'risk_event', '设备变更来源（演示：已过期）',        interval '20 days',    interval '12 days',     4, interval '20 days'),
  ('user',   'svc_backup_probe', 'active',   'risk_event', '撞库使用的探测账号名（演示：账号维度）', interval '35 minutes', NULL::interval,         1, interval '35 minutes')
) AS v(type, target_value, status, source, reason, effective_ago, expire_ago, hit_count, created_ago)
WHERE a.username = 'admin'
ON CONFLICT (type, target_value) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 4. 登录日志（黑名单「命中记录」抽屉按 type/target_value 关联登录日志）
--    failure_reason 必须带「命中黑名单」前缀，与登录链路的真实写法一致。
-- ---------------------------------------------------------------------------
INSERT INTO login_logs (user_id, username, login_type, result, failure_reason, ip,
                        user_agent, device_fingerprint, platform, risk_flag, subject_type, created_at)
SELECT 0, 'svc_backup_probe', 'password', 'failed', '命中黑名单(ip)', '91.219.60.14',
       'curl/8.4.0', '', 'web', 'suspicious_ip', 'user', now() - v.ago
FROM (VALUES
  (interval '34 minutes'), (interval '33 minutes'), (interval '32 minutes'),
  (interval '5 days'), (interval '4 days')
) AS v(ago)
WHERE NOT EXISTS (
  SELECT 1 FROM login_logs l
   WHERE l.username = 'svc_backup_probe' AND l.ip = '91.219.60.14'
     AND l.failure_reason = '命中黑名单(ip)'
);

INSERT INTO login_logs (user_id, username, login_type, result, failure_reason, ip,
                        user_agent, device_fingerprint, platform, risk_flag, subject_type, created_at)
SELECT 0, 'api_scanner_node3', 'password', 'failed', '命中黑名单(ip)', '45.13.190.44',
       'python-requests/2.31', '', 'web', 'high_frequency', 'user', now() - v.ago
FROM (VALUES (interval '6 days'), (interval '5 days 20 hours'), (interval '5 days 12 hours')) AS v(ago)
WHERE NOT EXISTS (
  SELECT 1 FROM login_logs l
   WHERE l.username = 'api_scanner_node3' AND l.ip = '45.13.190.44'
     AND l.failure_reason = '命中黑名单(ip)'
);

COMMIT;

-- ============================================================================
-- 复核查询（人工执行）
-- ----------------------------------------------------------------------------
--   -- 状态 × 动作两个维度是否都铺开了
--   SELECT status, COUNT(*) FROM risk_events
--    WHERE username IN ('user_north_02','svc_backup_probe','demo_user_03','demo_user_07',
--                       'demo_user_12','admin','demo_user_01','user_south_02','user_sw_01',
--                       'api_scanner_node3')
--    GROUP BY status;
--   SELECT e.id, e.username, e.status, e.risk_level, array_agg(a.action ORDER BY a.created_at)
--     FROM risk_events e LEFT JOIN risk_event_actions a ON a.event_id = e.id
--    WHERE e.username LIKE 'demo\_%' OR e.username LIKE 'svc\_%' OR e.username LIKE 'api\_%'
--    GROUP BY e.id ORDER BY e.id;
--   SELECT type, target_value, status, effective_at, expired_at FROM blacklists ORDER BY id;
--
-- 回滚段（先删子表再删主表）
-- ----------------------------------------------------------------------------
-- BEGIN;
--   DELETE FROM risk_event_actions WHERE event_id IN (
--     SELECT id FROM risk_events
--      WHERE ip IN ('61.160.36.7','91.219.60.14','185.199.108.153','103.28.14.7','45.32.100.21',
--                   '172.20.5.31','178.62.33.90','101.35.66.28','124.71.18.6','45.13.190.44'));
--   DELETE FROM risk_events
--    WHERE ip IN ('61.160.36.7','91.219.60.14','185.199.108.153','103.28.14.7','45.32.100.21',
--                 '172.20.5.31','178.62.33.90','101.35.66.28','124.71.18.6','45.13.190.44');
--   DELETE FROM blacklists WHERE target_value IN
--     ('91.219.60.14','fp-demo-risk-01','45.13.190.44','61.160.36.7','103.28.14.7','svc_backup_probe');
--   DELETE FROM login_logs WHERE ip IN ('91.219.60.14','45.13.190.44')
--      AND failure_reason = '命中黑名单(ip)';
-- COMMIT;
-- ============================================================================
