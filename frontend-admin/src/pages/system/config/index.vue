<template>
  <div class="page-body system-page system-config-module">
    <header class="page-header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">系统配置</h2>
          <p class="page-header__desc">
            这里只放后端真正读取的开关。邮件/站内信/短信正文不在此维护 ——
            请到<t-link theme="primary" hover="color" @click="goNotificationTemplates">消息中心 → 通知模板</t-link
            >配置，短信正文另见<t-link theme="primary" hover="color" @click="goSmsTemplates">短信模板</t-link>。
          </p>
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
            <p v-if="group.hint" class="tab-hint">{{ group.hint }}</p>
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
import { useRouter } from 'vue-router'

import { SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { batchSaveConfigs, getConfigListByGroup } from '@/api/system'
import type { SystemConfigInfo } from '@/api/system'

defineOptions({ name: 'SystemConfig' })

const router = useRouter()

/** 模板正文不在本页维护：跳消息中心的两个真实承载页（doc91 §13 第 1 条）。 */
const goNotificationTemplates = () => router.push('/notification/templates')
const goSmsTemplates = () => router.push('/notification/sms-templates')

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
  /** tab 标识（仅前端用，同名于 config_group 时可直接省略 group） */
  value: string
  label: string
  /**
   * 落库的 config_group。首页文案与站点品牌同属 `site` 分组，但要拆成两个 tab，
   * 所以 tab 标识与分组名解耦 —— 否则两个 tab 共享 value 时 activeGroup 会指错。
   */
  group?: string
  /** tab 下的说明（如 JSON 键的格式示例） */
  hint?: string
  fields: ConfigField[]
}

/**
 * 配置表单字段。
 *
 * 硬规则：这里的 `key` 必须与后端 seed（`internal/pkg/db/*.go`）读取的键**逐字一致**。
 * 曾出现过两类静默失效，改键名时务必先 `grep` 后端：
 *   - `password_require_number` / `password_require_special` 后端从不读（真键是
 *     `password_require_digit` / `password_require_symbol`）—— 后台开了也没用；
 *   - `system_timezone` / `date_format` / `currency_unit` / `password_expire_days` /
 *     `session_timeout_minutes` / `jwt_expire_minutes` / 7 个消息模板键在整仓命中数为 0，
 *     是整屏不会生效的表单，已全部删除。
 * 消息模板请到「消息中心 → 通知模板」维护（那里才是后端真正消费的 `notification_templates`）。
 */
const groups: ConfigGroup[] = [
  {
    value: 'site',
    label: '站点品牌',
    fields: [
      { key: 'site.name', label: '官网名称', type: 'input', valueType: 'string', default: '', hint: '官网、控制台、邮件外壳共用同一份品牌名' },
      { key: 'site.slogan', label: '品牌标语', type: 'input', valueType: 'string', default: '' },
      { key: 'site.logo', label: 'Logo 地址', type: 'input', valueType: 'string', default: '', placeholder: '/branding/logo.svg 或 https://…' },
      { key: 'site.favicon', label: 'Favicon 地址', type: 'input', valueType: 'string', default: '', placeholder: '/branding/favicon.svg 或 https://…' },
      { key: 'site.icp', label: 'ICP 备案号', type: 'input', valueType: 'string', default: '' },
      { key: 'site.copyright', label: '版权信息', type: 'input', valueType: 'string', default: '' },
      { key: 'site.license_no', label: '增值电信业务经营许可证号', type: 'input', valueType: 'string', default: '', hint: '留空则页脚不渲染该行，不显示示例证号' },
      { key: 'site.license_org', label: '代理域名注册服务机构', type: 'input', valueType: 'string', default: '' },
      { key: 'site.public_security', label: '公网安备号', type: 'input', valueType: 'string', default: '' },
      { key: 'site.contact_phone', label: '客服电话', type: 'input', valueType: 'string', default: '', hint: '留空则官网页脚不显示热线；请填真实可接通的号码' },
      { key: 'site.contact_email', label: '客服邮箱', type: 'input', valueType: 'string', default: '', hint: '留空则官网页脚不显示邮箱' },
      { key: 'site.contact_address', label: '联系地址', type: 'textarea', valueType: 'string', default: '', span: 'full' },
      { key: 'theme.primary_color', label: '官网主题色', type: 'input', valueType: 'string', default: '#2b5cff', placeholder: '#RRGGBB', hint: '官网按钮与强调色；控制台不跟随此色' },
      { key: 'theme.radius', label: '官网圆角', type: 'input', valueType: 'string', default: '10px', placeholder: '10px' },
    ],
  },
  {
    value: 'home',
    label: '官网首页',
    group: 'site',
    hint: '留空表示使用官网内置默认文案；填了以这里为准。',
    fields: [
      { key: 'home.hero_title', label: '主标题', type: 'input', valueType: 'string', default: '' },
      { key: 'home.hero_subtitle', label: '副标题', type: 'textarea', valueType: 'string', default: '', span: 'full' },
      { key: 'home.hero_image', label: '主视觉图地址', type: 'input', valueType: 'string', default: '', placeholder: '留空则用内置插画' },
      { key: 'home.hero_primary_cta', label: '主按钮文案', type: 'input', valueType: 'string', default: '' },
      { key: 'home.hero_primary_link', label: '主按钮链接', type: 'input', valueType: 'string', default: '', placeholder: '/products' },
      { key: 'home.hero_secondary_cta', label: '次按钮文案', type: 'input', valueType: 'string', default: '' },
      { key: 'home.hero_secondary_link', label: '次按钮链接', type: 'input', valueType: 'string', default: '' },
      { key: 'home.features_title', label: '产品优势区块标题', type: 'input', valueType: 'string', default: '' },
      { key: 'home.cta_title', label: '底部行动区标题', type: 'input', valueType: 'string', default: '' },
      { key: 'home.cta_desc', label: '底部行动区描述', type: 'textarea', valueType: 'string', default: '', span: 'full' },
      { key: 'home.featured_title', label: '推荐商品区块标题', type: 'input', valueType: 'string', default: '' },
      { key: 'home.featured_limit', label: '推荐商品展示数量', type: 'number', valueType: 'int', default: 6, min: 1, max: 24 },
      { key: 'home.announce_title', label: '公告区块标题', type: 'input', valueType: 'string', default: '' },
      { key: 'home.announce_limit', label: '公告区块展示数量', type: 'number', valueType: 'int', default: 5, min: 1, max: 20 },
      { key: 'home.features', label: '产品优势卡片（JSON）', type: 'textarea', valueType: 'json', default: '', span: 'full', hint: '数组，每项 {"icon","title","desc"}；icon 取 server/shield/bolt/support。留空用默认四张卡。只写平台真实提供的能力（如多上游交付、自动开通、周期计费、工单支持），不要写 DDoS 防护/快照备份/按量付费这类平台并未提供的承诺' },
    ],
  },
  {
    value: 'footer',
    label: '官网页脚',
    group: 'site',
    hint: '四个方面均留空则使用官网内置的默认页脚（内置值只描述平台真实提供的能力）；填坏 JSON 时该项回落默认值，不影响其它项。',
    fields: [
      { key: 'site.footer_promises', label: '服务保障条（JSON）', type: 'textarea', valueType: 'json', default: '', span: 'full', hint: '数组，每项 {"icon","title","desc"}；icon 取 time/secured/service/rollback/edit。只写平台真实提供的能力（工单、余额支付、操作留痕、到期提醒、自动续费），不要写免费备案/无忧退订这类没有对应功能的承诺' },
      { key: 'site.footer_socials', label: '社交按钮（JSON）', type: 'textarea', valueType: 'json', default: '', span: 'full', hint: '数组，每项 {"label","icon","url"}；url 必填且须为 http(s) 地址 —— 没有地址的项官网不会渲染（避免死按钮）。icon 取 wechat/qq/github/video/mobile' },
      { key: 'site.footer_columns', label: '链接栏目（JSON）', type: 'textarea', valueType: 'json', default: '', span: 'full', hint: '数组，每项 {"title","links":[{"label","to"}]}；数组为空时官网用品牌名动态拼装默认栏目。栏目里的 to 必须指向真实存在的页面或首页锚点（/#pricing、/#contact 等）' },
      { key: 'site.footer_legal_line', label: '法律行补充文案', type: 'input', valueType: 'string', default: '', span: 'full', hint: '留空则不渲染该行' },
      { key: 'site.wechat', label: '微信公众号名称', type: 'input', valueType: 'string', default: '', hint: '填写后官网页脚以文字展示公众号名；与社交按钮都为空时整块「关注」区域不渲染' },
    ],
  },
  {
    value: 'security',
    label: '安全配置',
    fields: [
      {
        key: 'password_min_length',
        label: '密码最小长度',
        type: 'number',
        valueType: 'int',
        default: 6,
        min: 4,
        max: 64,
        hint: '注册 / 改密 / 重置三处共用的密码策略',
      },
      { key: 'password_max_length', label: '密码最大长度', type: 'number', valueType: 'int', default: 64, min: 8, max: 128 },
      { key: 'password_require_upper', label: '密码必须包含大写字母', type: 'switch', valueType: 'bool', default: false },
      { key: 'password_require_lower', label: '密码必须包含小写字母', type: 'switch', valueType: 'bool', default: false },
      { key: 'password_require_digit', label: '密码必须包含数字', type: 'switch', valueType: 'bool', default: false },
      { key: 'password_require_symbol', label: '密码必须包含特殊字符', type: 'switch', valueType: 'bool', default: false },
      { key: 'login_fail_lock', label: '登录失败锁定', type: 'switch', valueType: 'bool', default: false },
      { key: 'login_fail_threshold', label: '失败次数阈值', type: 'number', valueType: 'int', default: 5, min: 1 },
      { key: 'login_lock_minutes', label: '锁定时间（分钟）', type: 'number', valueType: 'int', default: 15, min: 1 },
      { key: 'mfa_required', label: '强制所有场景二次验证', type: 'switch', valueType: 'bool', default: false, hint: '全局提升：打开后所有关键操作场景都要求验证码，用户侧无法关闭' },
      { key: 'api_rate_limit', label: '每 IP 请求限流（次/分钟）', type: 'number', valueType: 'int', default: 0, min: 0, hint: '0 表示不限；作用于管理端与用户中心接口' },
    ],
  },
  {
    value: 'register',
    label: '注册配置',
    group: 'feature',
    fields: [
      { key: 'register_enabled', label: '开放注册', type: 'switch', valueType: 'bool', default: true, hint: '关闭后注册接口返回「注册已关闭」，存量账号不受影响' },
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

/** 分组落库时用的 config_group：tab 标识与分组解耦时以 group 为准。 */
function groupKey(group: ConfigGroup): string {
  return group.group || group.value
}

/** 加载当前分组配置，将已有值应用到表单；未设置项回落默认值 */
async function loadCurrent() {
  const group = groups.find((g) => g.value === activeGroup.value)
  if (!group) return
  try {
    const configs = await getConfigListByGroup(groupKey(group))
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
  // config_group 由服务端按请求体顶层字段统一覆盖，不需要逐项携带。
  return group.fields.map((field) => {
    const value = formData[field.key]
    let configValue = String(value ?? '')
    if (field.valueType === 'bool') configValue = value ? 'true' : 'false'
    if (field.valueType === 'int' || field.valueType === 'decimal') configValue = String(value ?? '')
    return {
      config_key: field.key,
      config_value: configValue,
      value_type: field.valueType,
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
    await batchSaveConfigs(groupKey(group), buildItems(group))
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

.tab-hint {
  margin: 0 0 16px;
  padding: 10px 14px;
  border-radius: var(--td-radius-medium);
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  font-size: 12.5px;
  line-height: 1.6;
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
