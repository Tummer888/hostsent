-- ============================================================================
-- 047_retire_legacy_notify_keys.sql
--
-- 背景（docs/实施计划/91 §9.1 末行）：
--   system_configs 里曾有 7 个「消息模板」自由文本键 ——
--     mail_register_verify / mail_password_reset / mail_order_notify /
--     sms_verify_code / sms_alert / inapp_system_notify / inapp_alert_notify
--   它们只被管理端「系统配置 → 消息模板」表单写入，**后端从头到尾没有任何读取点**：
--   发出去的验证码/通知正文来自 captcha 的 renderOTPBody 与
--   notification_templates，与这 7 个键无关。运营在这个界面里填了模板，
--   以为改了文案，实际对外行为一点没变 —— 这是最伤人的那类静默失效。
--
--   本轮把消息中心落地（doc90）后，模板有了真正的承载表，因此按 §9.1 收口：
--   非空的历史值导入 sms_templates（code=legacy_<key>）留档，然后删除这些配置行。
--
-- 内容：
--   1. 逐键导入：仅当 config_value 非空时才插入，避免造出一堆空模板；
--      已存在同名 code 时 DO NOTHING（幂等，且不覆盖运营在短信模板页改过的正文）；
--   2. 删除这 7 个配置行（不论空值与否）—— 表单已在前端移除，
--      残留行只会让后来的人以为它还在生效。
--
-- 关于 scene 映射：sms_templates.scene 取值 otp/notify/alert/marketing（041）。
--   验证码类（注册/改密/短信验证码）→ otp；订单/系统站内信 → notify；告警 → alert。
--   历史值若真被填过，其语义最接近的落点就是这几类。
--
-- 幂等：INSERT ... ON CONFLICT (code) DO NOTHING；DELETE ... WHERE config_key IN (...)。
--       重复执行无副作用。
--
-- 回滚：配置行已删除，回滚需要从备份恢复 system_configs 的这 7 行；
--       导入的 legacy_* 模板可用
--       DELETE FROM sms_templates WHERE code LIKE 'legacy\_%';
--       清掉（这些行只为留档，不参与任何发送链路）。
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. 非空历史值导入短信模板表（留档，不参与发送链路）
-- ---------------------------------------------------------------------------
INSERT INTO sms_templates (code, name, scene, content, status, remark, created_at, updated_at)
SELECT 'legacy_' || c.config_key,
       CASE c.config_key
         WHEN 'mail_register_verify' THEN '注册验证邮件模板（历史配置导入）'
         WHEN 'mail_password_reset'  THEN '密码重置邮件模板（历史配置导入）'
         WHEN 'mail_order_notify'    THEN '订单通知邮件模板（历史配置导入）'
         WHEN 'sms_verify_code'      THEN '验证码短信模板（历史配置导入）'
         WHEN 'sms_alert'            THEN '告警短信模板（历史配置导入）'
         WHEN 'inapp_system_notify'  THEN '系统通知站内信模板（历史配置导入）'
         WHEN 'inapp_alert_notify'   THEN '告警通知站内信模板（历史配置导入）'
       END,
       CASE c.config_key
         WHEN 'mail_register_verify' THEN 'otp'
         WHEN 'mail_password_reset'  THEN 'otp'
         WHEN 'sms_verify_code'      THEN 'otp'
         WHEN 'sms_alert'            THEN 'alert'
         WHEN 'inapp_alert_notify'   THEN 'alert'
         ELSE 'notify'
       END,
       left(c.config_value, 1000),
       'active',
       '由 migrations/047 自 system_configs.' || c.config_key || ' 导入（doc91 §9.1）',
       now(), now()
  FROM system_configs c
 WHERE c.config_key IN (
        'mail_register_verify', 'mail_password_reset', 'mail_order_notify',
        'sms_verify_code', 'sms_alert', 'inapp_system_notify', 'inapp_alert_notify'
       )
   AND btrim(coalesce(c.config_value, '')) <> ''
ON CONFLICT (code) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. 删除这些配置行（后端无读取点，值已在上一步留档）
-- ---------------------------------------------------------------------------
DELETE FROM system_configs
 WHERE config_key IN (
        'mail_register_verify', 'mail_password_reset', 'mail_order_notify',
        'sms_verify_code', 'sms_alert', 'inapp_system_notify', 'inapp_alert_notify'
       );

COMMIT;
