<template>
  <div class="page-body system-page system-config-module">
    <header class="page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">系统配置</h2>
        </div>
      </div>
    </header>

    <section class="form-card surface-card">
      <t-tabs v-model="activeGroup" @change="onTabChange">
        <t-tab-panel
          v-for="group in groups"
          :key="group.value"
          :value="group.value"
          :label="group.label"
        >
          <div class="tab-panel">
            <t-form label-align="top" :data="formData" @submit.prevent="saveCurrent">
              <div class="form-grid">
                <div
                  v-for="field in group.fields"
                  :key="field.key"
                  class="form-cell"
                  :class="{ 'form-cell--full': field.span === 'full' }"
                >
                  <t-form-item :label="field.label">
                    <!-- 输入框 -->
                    <t-input
                      v-if="field.type === 'input'"
                      v-model="(formData as Record<string, any>)[field.key]"
                      :placeholder="field.placeholder || `请输入${field.label}`"
                      clearable
                    />
                    <!-- 多行文本 -->
                    <t-textarea
                      v-else-if="field.type === 'textarea'"
                      v-model="(formData as Record<string, any>)[field.key]"
                      :autosize="{ minRows: 3, maxRows: 8 }"
                      :placeholder="field.placeholder || `请输入${field.label}`"
                    />
                    <!-- 数字 -->
                    <t-input-number
                      v-else-if="field.type === 'number'"
                      v-model="(formData as Record<string, any>)[field.key]"
                      :min="field.min ?? 0"
                      :max="field.max"
                      :step="field.step ?? 1"
                      theme="column"
                    />
                    <!-- 开关 -->
                    <t-switch
                      v-else-if="field.type === 'switch'"
                      v-model="(formData as Record<string, any>)[field.key]"
                    >
                      <template #label="{ value }">
                        {{ value ? '开启' : '关闭' }}
                      </template>
                    </t-switch>
                    <!-- 下拉选择 -->
                    <t-select
                      v-else-if="field.type === 'select'"
                      v-model="(formData as Record<string, any>)[field.key]"
                      :options="field.options"
                      :placeholder="field.placeholder || `请选择${field.label}`"
                      clearable
                    />
                  </t-form-item>
                  <p v-if="field.hint" class="field-hint">{{ field.hint }}</p>
                </div>
              </div>

              <div class="form-footer">
                <t-button variant="outline" @click="loadCurrent">重置</t-button>
                <t-button theme="primary" type="submit" :loading="savingGroup === group.value">
                  保存配置
                </t-button>
              </div>
            </t-form>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { batchSaveConfigs, getConfigListByGroup } from '@/api/system'
import type { SystemConfigInfo } from '@/api/system'

defineOptions({ name: 'SystemConfig' })

/** 字段类型 */
type FieldType = 'input' | 'textarea' | 'number' | 'switch' | 'select'

interface SelectOption {
  label: string
  value: string
}

/** 配置项字段元信息 */
interface ConfigField {
  key: string // 配置键
  label: string // 表单标签
  type: FieldType // 渲染控件
  valueType: SystemConfigInfo['value_type'] // 存储语义类型
  default?: string | number | boolean // 缺省值
  placeholder?: string
  hint?: string // 表单下方提示
  options?: SelectOption[] // select 专用
  min?: number // number 专用
  max?: number // number 专用
  step?: number // number 专用
  span?: 'full' // 占满整行
}

/** 配置分组定义 */
interface ConfigGroup {
  value: string // config_group 值
  label: string
  fields: ConfigField[]
}

const groups: ConfigGroup[] = [
  {
    value: 'base',
    label: '基础配置',
    fields: [
      { key: 'site_name', label: '站点名称', type: 'input', valueType: 'string', default: '' },
      { key: 'site_logo', label: '站点 Logo 地址', type: 'input', valueType: 'string', default: '', placeholder: 'https://…' },
      { key: 'site_icp', label: 'ICP 备案号', type: 'input', valueType: 'string', default: '' },
      { key: 'site_copyright', label: '版权信息', type: 'input', valueType: 'string', default: '' },
      { key: 'contact_phone', label: '客服电话', type: 'input', valueType: 'string', default: '' },
      { key: 'contact_email', label: '客服邮箱', type: 'input', valueType: 'string', default: '' },
      { key: 'contact_address', label: '联系地址', type: 'textarea', valueType: 'string', default: '', span: 'full' },
      { key: 'system_timezone', label: '系统时区', type: 'select', valueType: 'string', default: 'Asia/Shanghai', options: [
        { label: 'Asia/Shanghai (UTC+8)', value: 'Asia/Shanghai' },
        { label: 'UTC', value: 'UTC' },
        { label: 'Asia/Tokyo (UTC+9)', value: 'Asia/Tokyo' },
        { label: 'America/New_York (UTC-5)', value: 'America/New_York' },
      ] },
      { key: 'date_format', label: '日期格式', type: 'select', valueType: 'string', default: 'YYYY-MM-DD', options: [
        { label: 'YYYY-MM-DD', value: 'YYYY-MM-DD' },
        { label: 'YYYY/MM/DD', value: 'YYYY/MM/DD' },
        { label: 'DD/MM/YYYY', value: 'DD/MM/YYYY' },
        { label: 'MM/DD/YYYY', value: 'MM/DD/YYYY' },
      ] },
      { key: 'currency_unit', label: '货币单位', type: 'input', valueType: 'string', default: 'CNY' },
    ],
  },
  {
    value: 'security',
    label: '安全配置',
    fields: [
      { key: 'password_min_length', label: '密码最小长度', type: 'number', valueType: 'int', default: 8, min: 4, max: 64, hint: '新密码至少包含的字符数量' },
      { key: 'password_require_number', label: '密码必须包含数字', type: 'switch', valueType: 'bool', default: true },
      { key: 'password_require_upper', label: '密码必须包含大写字母', type: 'switch', valueType: 'bool', default: false },
      { key: 'password_require_lower', label: '密码必须包含小写字母', type: 'switch', valueType: 'bool', default: true },
      { key: 'password_require_special', label: '密码必须包含特殊字符', type: 'switch', valueType: 'bool', default: false },
      { key: 'password_expire_days', label: '密码有效期（天）', type: 'number', valueType: 'int', default: 90, min: 0, hint: '0 表示永不过期' },
      { key: 'password_history_keep', label: '历史密码防重用（次）', type: 'number', valueType: 'int', default: 5, min: 0 },
      { key: 'login_fail_lock', label: '登录失败锁定', type: 'switch', valueType: 'bool', default: true },
      { key: 'login_fail_threshold', label: '失败次数阈值', type: 'number', valueType: 'int', default: 5, min: 1 },
      { key: 'login_lock_minutes', label: '锁定时间（分钟）', type: 'number', valueType: 'int', default: 15, min: 1 },
      { key: 'session_timeout_minutes', label: '会话超时（分钟）', type: 'number', valueType: 'int', default: 120, min: 1 },
      { key: 'mfa_required', label: '强制启用双因素认证（MFA）', type: 'switch', valueType: 'bool', default: false },
      { key: 'jwt_expire_minutes', label: 'JWT 有效期（分钟）', type: 'number', valueType: 'int', default: 720, min: 1 },
      { key: 'api_rate_limit', label: '每 IP 请求限流（次/分钟）', type: 'number', valueType: 'int', default: 120, min: 1 },
    ],
  },
  {
    value: 'register',
    label: '注册配置',
    fields: [
      { key: 'register_enabled', label: '开放注册', type: 'switch', valueType: 'bool', default: true },
      { key: 'register_need_audit', label: '注册需人工审核', type: 'switch', valueType: 'bool', default: false },
      { key: 'register_default_role', label: '默认角色', type: 'input', valueType: 'string', default: 'user', placeholder: '角色标识，如 user' },
      { key: 'register_default_tier', label: '默认用户等级', type: 'input', valueType: 'string', default: '', placeholder: '如 base' },
      { key: 'invite_code_required', label: '注册需要邀请码', type: 'switch', valueType: 'bool', default: false },
      { key: 'invite_code_length', label: '邀请码长度', type: 'number', valueType: 'int', default: 8, min: 4, max: 32 },
    ],
  },
  {
    value: 'referral',
    label: '推广返现',
    fields: [
      { key: 'referral.enabled', label: '启用推广邀请返现', type: 'switch', valueType: 'bool', default: true, hint: '关闭后停止计提返现，且用户无法提现或转入余额' },
      { key: 'referral.first_order_rate', label: '首单返现比率', type: 'number', valueType: 'decimal', default: 0.1, min: 0, max: 1, step: 0.01, hint: '0-1 之间的小数，如 0.1 表示返 10%；基数为被邀请人订单实付金额' },
      { key: 'referral.subsequent_rate', label: '后续订单返现比率', type: 'number', valueType: 'decimal', default: 0.05, min: 0, max: 1, step: 0.01, hint: '被邀请人首单之后的每次成功订单适用' },
      { key: 'referral.renewal_rate', label: '续费返现比率', type: 'number', valueType: 'decimal', default: 0.03, min: 0, max: 1, step: 0.01, hint: '被邀请人续费订单适用' },
      { key: 'referral.min_withdraw_amount', label: '最低提现金额（元）', type: 'number', valueType: 'decimal', default: 50, min: 0, step: 1, hint: '单笔返现提现申请的最低金额' },
    ],
  },
  {
    value: 'notify',
    label: '消息模板',
    fields: [
      { key: 'mail_register_verify', label: '注册验证邮件模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{code}、{site_name}、{expire_minutes}' },
      { key: 'mail_password_reset', label: '密码重置邮件模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{code}、{link}、{site_name}、{expire_minutes}' },
      { key: 'mail_order_notify', label: '订单通知邮件模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{order_no}、{amount}、{site_name}' },
      { key: 'sms_verify_code', label: '验证码短信模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{code}、{site_name}、{expire_minutes}' },
      { key: 'sms_alert', label: '告警短信模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{title}、{content}、{site_name}' },
      { key: 'inapp_system_notify', label: '系统通知站内信模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{content}、{site_name}' },
      { key: 'inapp_alert_notify', label: '告警通知站内信模板', type: 'textarea', valueType: 'string', default: '', span: 'full', hint: '可用变量：{title}、{content}、{site_name}' },
    ],
  },
]

const activeGroup = ref<string>(groups[0].value)
const savingGroup = ref<string>('')
// 表单数据：key 与字段类型对应，switch 为 boolean、number 为 number、其余为 string
const formData = reactive<Record<string, string | number | boolean>>({})

function defaultForType(field: ConfigField): string | number | boolean {
  if (field.default !== undefined) return field.default
  if (field.type === 'switch') return false
  if (field.type === 'number') return 0
  return ''
}

/** 加载当前分组配置，将已有值应用到表单；未设置项回落默认值 */
async function loadCurrent() {
  const group = groups.find((g) => g.value === activeGroup.value)
  if (!group) return
  try {
    const configs = await getConfigListByGroup(group.value)
    const configMap: Record<string, SystemConfigInfo> = {}
    for (const cfg of configs) configMap[cfg.config_key] = cfg

    for (const field of group.fields) {
      const cfg = configMap[field.key]
      if (!cfg) {
        formData[field.key] = defaultForType(field)
        continue
      }
      if (field.valueType === 'bool') {
        formData[field.key] = cfg.config_value === 'true'
      } else if (field.valueType === 'int' || field.valueType === 'decimal') {
        formData[field.key] = Number(cfg.config_value) || 0
      } else {
        formData[field.key] = cfg.config_value
      }
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || `加载${group.label}失败`)
    // 加载失败时仍回落默认值，保证表单可编辑
    for (const field of group.fields) formData[field.key] = defaultForType(field)
  }
}

/** 序列化当前分组字段为批量保存请求体 */
function buildItems(group: ConfigGroup) {
  return group.fields.map((field) => {
    const value = formData[field.key]
    let configValue = String(value ?? '')
    if (field.valueType === 'bool') configValue = value ? 'true' : 'false'
    if (field.valueType === 'int' || field.valueType === 'decimal') configValue = String(value ?? '')
    return {
      config_key: field.key,
      config_value: configValue,
      value_type: field.valueType,
      config_group: group.value,
      description: field.label,
      sort_order: group.fields.indexOf(field),
      status: 'active' as const,
    }
  })
}

async function saveCurrent() {
  const group = groups.find((g) => g.value === activeGroup.value)
  if (!group) return
  savingGroup.value = group.value
  try {
    await batchSaveConfigs(group.value, buildItems(group))
    MessagePlugin.success(`${group.label}已保存`)
    await loadCurrent()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || `保存${group.label}失败`)
  } finally {
    savingGroup.value = ''
  }
}

function onTabChange() {
  activeGroup.value = activeGroup.value
  void loadCurrent()
}

onMounted(() => {
  void loadCurrent()
})
</script>

<style scoped>
@import '../shared.css';

.system-config-module {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
  color: var(--td-text-color-primary);
}

.page-header {
  display: flex;
  align-items: center;
  min-height: 32px;
}

.page-header__title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 1.4;
}

.form-card {
  padding: 20px 24px 24px;
  border: 1px solid var(--system-card-border);
  border-radius: var(--td-radius-medium);
  background: var(--td-bg-color-container);
}

.tab-panel {
  padding-top: 20px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 24px;
}

.form-cell--full {
  grid-column: 1 / -1;
}

.field-hint {
  margin: -6px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--system-card-border);
}

@media (max-width: 768px) {
  .form-card {
    padding: 16px;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
