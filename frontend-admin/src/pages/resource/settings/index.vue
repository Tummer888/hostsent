<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">资源模块系统配置</h2>
          <p class="page-header__desc">维护上游同步调度、定价加价策略与异常事件通知的全局默认配置。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" @click="handleResetAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          重置为默认
        </t-button>
        <t-button theme="primary" @click="handleSaveAll">保存全部</t-button>
      </t-space>
    </header>

    <section class="setting-card surface-card">
      <div class="card-head">
        <span class="card-head__chip card-head__chip--blue">
          <DashboardIcon size="16" aria-hidden="true" />
        </span>
        <div class="card-head__text">
          <h3 class="card-title">同步调度配置</h3>
          <p class="card-subtitle">控制后台自动同步上游商品与库存的执行节奏。</p>
        </div>
      </div>
      <t-form :data="scheduleForm" label-align="right" label-width="110px">
        <div class="form-grid">
          <t-form-item label="同步间隔" name="interval">
            <t-select v-model="scheduleForm.interval" :options="intervalOptions" placeholder="请选择同步间隔" />
          </t-form-item>
          <t-form-item label="失败重试次数" name="retry_count">
            <t-input v-model="scheduleForm.retry_count" maxlength="2" placeholder="0 - 10 之间的整数" />
          </t-form-item>
          <t-form-item label="并发数" name="concurrency">
            <t-input v-model="scheduleForm.concurrency" maxlength="2" placeholder="同时同步的提供商数量上限" />
          </t-form-item>
          <t-form-item label="自动同步" name="auto_sync">
            <t-switch v-model="scheduleForm.auto_sync" />
          </t-form-item>
        </div>
      </t-form>
      <div class="form-footer">
        <t-button variant="outline" @click="handleResetSection('同步调度配置')">恢复本区默认</t-button>
        <t-button theme="success" @click="handleSaveSection('同步调度配置')">保存</t-button>
      </div>
    </section>

    <section class="setting-card surface-card">
      <div class="card-head">
        <span class="card-head__chip card-head__chip--green">
          <MoneyIcon size="16" aria-hidden="true" />
        </span>
        <div class="card-head__text">
          <h3 class="card-title">定价策略</h3>
          <p class="card-subtitle">按提供商成本价生成前台售价时的全局加价规则。</p>
        </div>
      </div>
      <t-form :data="pricingForm" label-align="right" label-width="110px">
        <div class="form-grid">
          <t-form-item label="加价方式" name="mode">
            <t-radio-group v-model="pricingForm.mode" variant="default-filled">
              <t-radio-button value="fixed">固定金额</t-radio-button>
              <t-radio-button value="percent">百分比</t-radio-button>
            </t-radio-group>
          </t-form-item>
          <t-form-item :label="pricingForm.mode === 'percent' ? '默认比例（%）' : '默认金额'" name="ratio">
            <t-input v-model="pricingForm.ratio" placeholder="未单独设置的商品使用该默认值" />
          </t-form-item>
          <t-form-item label="最低利润" name="min_profit">
            <t-input v-model="pricingForm.min_profit" placeholder="售价低于成本的保底利润额" />
          </t-form-item>
        </div>
      </t-form>
      <div class="form-footer">
        <t-button variant="outline" @click="handleResetSection('定价策略')">恢复本区默认</t-button>
        <t-button theme="success" @click="handleSaveSection('定价策略')">保存</t-button>
      </div>
    </section>

    <section class="setting-card surface-card">
      <div class="card-head">
        <span class="card-head__chip card-head__chip--amber">
          <InfoCircleIcon size="16" aria-hidden="true" />
        </span>
        <div class="card-head__text">
          <h3 class="card-title">通知配置</h3>
          <p class="card-subtitle">订阅异常事件并指定接收人，第一时间触达运维人员。</p>
        </div>
      </div>
      <t-form :data="{ receivers: notifyForm.receivers }" label-align="right" label-width="110px">
        <t-form-item label="接收人" name="receivers">
          <t-input
            v-model="notifyForm.receivers"
            placeholder="多个接收人使用英文逗号分隔，例如：admin@hostsent.cn, ops@hostsent.cn"
          />
        </t-form-item>
        <t-form-item label="通知事件" name="events">
          <t-checkbox-group v-model="notifyForm.events" :options="eventOptions" />
        </t-form-item>
      </t-form>
      <div class="form-footer">
        <t-button variant="outline" @click="handleResetSection('通知配置')">恢复本区默认</t-button>
        <t-button theme="success" @click="handleSaveSection('通知配置')">保存</t-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'

import {
  DashboardIcon,
  InfoCircleIcon,
  MoneyIcon,
  RefreshIcon,
  SettingIcon,
} from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

defineOptions({ name: 'ResourceSettings' })

const SAVE_SUCCESS_TIP = '配置已保存（当前为前端暂存，后端接口待接入）'

const intervalOptions = [
  { label: '每 5 分钟', value: '300' },
  { label: '每 10 分钟', value: '600' },
  { label: '每 30 分钟', value: '1800' },
  { label: '每小时', value: '3600' },
  { label: '每天一次', value: '86400' },
]

const eventOptions = [
  { label: '同步失败', value: 'sync_failed' },
  { label: '连接异常', value: 'connect_error' },
  { label: '库存不足', value: 'stock_low' },
]

const scheduleDefaults = { interval: '1800', retry_count: '3', concurrency: '5', auto_sync: true }
const pricingDefaults = { mode: 'percent', ratio: '15', min_profit: '5.00' }
const notifyDefaults = {
  receivers: 'admin@hostsent.cn, ops@hostsent.cn',
  events: ['sync_failed', 'stock_low'] as string[],
}

const scheduleForm = reactive({ ...scheduleDefaults })
const pricingForm = reactive({ ...pricingDefaults })
const notifyForm = reactive<{ receivers: string; events: string[] }>({
  receivers: notifyDefaults.receivers,
  events: [...notifyDefaults.events],
})

function handleResetAll() {
  MessagePlugin.info('该配置接口待接入，重置仅作用于当前表单')
}

function handleResetSection(sectionName: string) {
  void sectionName
  MessagePlugin.info('该配置接口待接入，重置仅作用于当前表单')
}

function handleSaveSection(sectionName: string) {
  void sectionName
  MessagePlugin.success(SAVE_SUCCESS_TIP)
}

function handleSaveAll() {
  MessagePlugin.success(SAVE_SUCCESS_TIP)
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #16a34a, #15803d);
  --chip-shadow: 0 4px 10px rgba(22, 163, 74, 0.25);
}

.setting-card {
  padding: var(--space-lg) 20px;
}

.card-head {
  display: flex;
  align-items: flex-start;
  gap: var(--space-md);
  margin-bottom: var(--space-lg);
}

.card-head__chip {
  width: 32px;
  height: 32px;
  border-radius: var(--hs-radius-md);
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: var(--hs-shadow-xs);
}

.card-head__chip--blue {
  background: #2563eb;
}

.card-head__chip--green {
  background: #16a34a;
}

.card-head__chip--amber {
  background: #d97706;
}

.card-head__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.card-subtitle {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-xl);
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  margin-top: var(--space-md);
  padding-top: var(--space-md);
  border-top: 1px solid var(--color-border);
}

@media (max-width: 900px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
