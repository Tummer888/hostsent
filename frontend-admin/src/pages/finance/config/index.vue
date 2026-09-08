<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><SettingIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">财务配置</h2>
          <p class="page-header__desc">财务相关全局配置项，保存后即时生效。</p>
        </div>
      </div>
    </header>

    <section class="form-card surface-card">
      <h3 class="card-title">财务参数</h3>
      <t-form label-align="top" :data="form" @submit.prevent="save">
        <div class="form-grid">
          <t-form-item label="默认计费周期" name="billing">
            <t-select v-model="form.billing" :options="[{label:'按小时',value:'hourly'},{label:'按月',value:'monthly'},{label:'按年',value:'yearly'}]" />
          </t-form-item>
          <t-form-item label="税率（%）" name="tax">
            <t-input-number v-model="form.tax" :min="0" :max="100" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="对账差异阈值（元）" name="reconThreshold">
            <t-input-number v-model="form.reconThreshold" :min="0" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="余额预警阈值（元）" name="balanceWarning">
            <t-input-number v-model="form.balanceWarning" :min="0" :precision="2" theme="column" />
          </t-form-item>
        </div>
        <t-form-item label="允许人工调账" name="manualAdjust">
          <t-switch v-model="form.manualAdjust" />
        </t-form-item>
        <div class="form-footer">
          <t-button variant="outline" @click="load">重置</t-button>
          <t-button theme="primary" type="submit">保存配置</t-button>
        </div>
      </t-form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive } from 'vue'
import { SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { createConfig, getConfigList, updateConfig } from '@/api/system'
import type { SystemConfigInfo } from '@/api/system'

defineOptions({ name: 'FinanceConfig' })

const form = reactive({
  billing: 'monthly',
  tax: 6,
  reconThreshold: 0.01,
  balanceWarning: 50,
  manualAdjust: true,
})

let configMap: Record<string, SystemConfigInfo> = {}

const entries: { key: string; value: () => string; type: 'string' | 'json' | 'bool' | 'int' }[] = [
  { key: 'finance_billing_cycle', value: () => form.billing, type: 'string' },
  { key: 'finance_tax_rate', value: () => String(form.tax), type: 'int' },
  { key: 'finance_recon_threshold', value: () => String(form.reconThreshold), type: 'int' },
  { key: 'finance_balance_warning', value: () => String(form.balanceWarning), type: 'int' },
  { key: 'finance_manual_adjust_enabled', value: () => String(form.manualAdjust), type: 'bool' },
]

async function load() {
  try {
    const data = await getConfigList({ page: 1 })
    configMap = {}
    for (const it of data.items) configMap[it.config_key] = it
    if (configMap['finance_billing_cycle']) form.billing = configMap['finance_billing_cycle'].config_value
    if (configMap['finance_tax_rate']) form.tax = Number(configMap['finance_tax_rate'].config_value)
    if (configMap['finance_recon_threshold']) form.reconThreshold = Number(configMap['finance_recon_threshold'].config_value)
    if (configMap['finance_balance_warning']) form.balanceWarning = Number(configMap['finance_balance_warning'].config_value)
    if (configMap['finance_manual_adjust_enabled']) form.manualAdjust = configMap['finance_manual_adjust_enabled'].config_value === 'true'
  } catch {
    /* 未初始化时保持默认值 */
  }
}

async function save() {
  try {
    for (const e of entries) {
      const cfg = configMap[e.key]
      if (cfg) {
        await updateConfig(cfg.id, {
          config_value: e.value(),
          value_type: cfg.value_type,
          config_group: cfg.config_group,
          description: cfg.description,
          status: cfg.status,
        })
      } else {
        await createConfig({
          config_key: e.key,
          config_value: e.value(),
          value_type: e.type,
          config_group: 'finance',
          status: 'active',
        })
      }
    }
    MessagePlugin.success('财务配置已保存')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  }
}

onMounted(load)
</script>

<style lang="css">
@import '../shared.css';
</style>