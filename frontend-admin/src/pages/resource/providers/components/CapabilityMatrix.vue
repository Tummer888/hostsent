<template>
  <div class="capability-matrix">
    <div class="capability-matrix__head">
      <h4 class="capability-matrix__title">{{ title }}</h4>
      <t-space size="small">
        <t-tag
          :theme="descriptor.kind === 'compute' ? 'success' : 'default'"
          variant="light"
          size="small"
          shape="round"
        >
          {{ labelOf(KIND_LABELS, descriptor.kind) }}
        </t-tag>
        <t-tag v-if="!descriptor.implemented" theme="warning" variant="light" size="small" shape="round"
          >适配器未接入</t-tag
        >
        <t-tag v-else theme="success" variant="light" size="small" shape="round">适配器已接入</t-tag>
      </t-space>
    </div>

    <div class="capability-matrix__grid">
      <div class="matrix-cell">
        <span class="matrix-cell__label">能力操作</span>
        <div class="matrix-cell__tags">
          <template v-if="operations.length">
            <t-tag v-for="op in operations" :key="op" theme="primary" variant="light" size="small" shape="round">
              {{ labelOf(OPERATION_LABELS, op) }}
            </t-tag>
          </template>
          <span v-else class="matrix-cell__empty">未声明</span>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">支持同步</span>
        <div class="matrix-cell__tags">
          <template v-if="syncScopes.length">
            <t-tag v-for="scope in syncScopes" :key="scope" theme="default" variant="light" size="small" shape="round">
              {{ labelOf(SYNC_SCOPE_LABELS, scope) }}
            </t-tag>
          </template>
          <span v-else class="matrix-cell__empty">未声明</span>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">计费周期</span>
        <div class="matrix-cell__tags">
          <template v-if="billingCycles.length">
            <t-tag
              v-for="cycle in billingCycles"
              :key="cycle"
              theme="default"
              variant="light"
              size="small"
              shape="round"
            >
              {{ labelOf(BILLING_CYCLE_LABELS, cycle) }}
            </t-tag>
          </template>
          <span v-else class="matrix-cell__empty">未声明</span>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">续费 / 销毁</span>
        <div class="matrix-cell__tags">
          <t-tag theme="default" variant="light" size="small" shape="round">
            续费：{{ labelOf(RENEW_MODE_LABELS, descriptor.renew_mode) }}
          </t-tag>
          <t-tag theme="default" variant="light" size="small" shape="round">
            销毁：{{ labelOf(DESTROY_MODE_LABELS, descriptor.destroy_mode) }}
          </t-tag>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">签名 / 限流</span>
        <div class="matrix-cell__tags">
          <t-tag theme="default" variant="light" size="small" shape="round">
            签名：{{ labelOf(SIGNER_LABELS, descriptor.signer_type) }}
          </t-tag>
          <t-tag theme="default" variant="light" size="small" shape="round"> 限流：{{ rateLimitText }} </t-tag>
          <t-tag v-if="descriptor.supports_paging" theme="default" variant="light" size="small" shape="round"
            >分页读取</t-tag
          >
          <t-tag v-if="descriptor.reports_power_state" theme="default" variant="light" size="small" shape="round"
            >可读取电源状态</t-tag
          >
        </div>
      </div>

      <!-- 电源状态语义：能否用上游状态覆盖本地开关机状态。财务型上游只返回
           服务/账期状态（Active/Pending），没有机器电源字段；不标明的话运营会以为
           "回源能纠正状态"，实际是关了机会被同步改回运行中。 -->
      <div class="matrix-cell matrix-cell--full">
        <span class="matrix-cell__label">状态回源口径</span>
        <div class="matrix-cell__tags">
          <t-tag
            :theme="descriptor.reports_power_state ? 'success' : 'warning'"
            variant="light"
            size="small"
            shape="round"
          >
            {{
              descriptor.reports_power_state
                ? '上游可报机器电源状态：回源会按上游纠正开关机状态'
                : '上游不报机器电源状态：回源不覆盖本地开关机状态（只刷新 IP / 规格 / 到期）'
            }}
          </t-tag>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">凭证字段</span>
        <div class="matrix-cell__tags">
          <template v-if="credentialFields.length">
            <t-tag
              v-for="field in credentialFields"
              :key="field.key"
              :theme="field.secret ? 'warning' : 'default'"
              variant="light"
              size="small"
              shape="round"
            >
              {{ field.label }}<span v-if="field.secret"> · 加密</span>
            </t-tag>
          </template>
          <span v-else class="matrix-cell__empty">未声明</span>
        </div>
      </div>

      <!-- 接入参数（接口地址/端口/HTTPS 等）：编辑渠道时按它渲染，这里同源展示，
           避免"能力矩阵看着支持、实际建渠道时表单没这项"的错配。 -->
      <div class="matrix-cell">
        <span class="matrix-cell__label">接入参数</span>
        <div class="matrix-cell__tags">
          <template v-if="endpointFields.length">
            <t-tag v-for="field in endpointFields" :key="field.key" theme="default" variant="light" size="small" shape="round">
              {{ field.label }}<span v-if="field.required"> · 必填</span>
            </t-tag>
          </template>
          <span v-else class="matrix-cell__empty">未声明</span>
        </div>
      </div>

      <!-- 平台可配置项目录：该平台下单时能下发的规格参数（CPU/内存/磁盘/带宽…）。
           这是"平台上能配什么"的权威清单，配置档从这里取值。 -->
      <div v-if="optionCatalog.length" class="matrix-cell matrix-cell--full">
        <span class="matrix-cell__label">
          平台可配置项
          <span class="matrix-cell__count">{{ optionCatalog.length }} 项</span>
        </span>
        <div class="matrix-cell__tags">
          <t-tooltip
            v-for="opt in optionCatalog"
            :key="opt.key"
            :content="optionTooltip(opt)"
            placement="top"
            :disabled="!optionTooltip(opt)"
          >
            <t-tag theme="default" variant="light" size="small" shape="round">
              {{ opt.label }}<span v-if="opt.required" class="matrix-cell__req"> · 必填</span>
            </t-tag>
          </t-tooltip>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import type { CapabilityDescriptor, ConfigOptionSpec } from '@/types/interface'

import {
  BILLING_CYCLE_LABELS,
  DESTROY_MODE_LABELS,
  KIND_LABELS,
  OPERATION_LABELS,
  RENEW_MODE_LABELS,
  SIGNER_LABELS,
  SYNC_SCOPE_LABELS,
  labelOf,
} from './capability-labels'

const props = withDefaults(
  defineProps<{
    descriptor?: CapabilityDescriptor | null
    title?: string
  }>(),
  {
    descriptor: null,
    title: '能力矩阵',
  },
)

const EMPTY: CapabilityDescriptor = {
  kind: 'upstream',
  sync_scopes: null,
  spec_atoms: null,
  billing_cycles: null,
  operations: null,
  renew_mode: 'none',
  destroy_mode: 'unsupported',
  credential_schema: null,
  endpoint_schema: null,
  signer_type: 'none',
  rate_limit: { qps: 0, burst: 0 },
  supports_paging: false,
  reports_power_state: false,
  implemented: false,
}

const descriptor = computed<CapabilityDescriptor>(() => props.descriptor || EMPTY)
const operations = computed(() => descriptor.value.operations || [])
const syncScopes = computed(() => descriptor.value.sync_scopes || [])
const billingCycles = computed(() => descriptor.value.billing_cycles || [])
const credentialFields = computed(() => descriptor.value.credential_schema || [])
const endpointFields = computed(() => descriptor.value.endpoint_schema || [])
const optionCatalog = computed(() => descriptor.value.option_catalog || [])
const rateLimitText = computed(() => {
  const qps = descriptor.value.rate_limit?.qps || 0
  return qps > 0 ? `${qps} QPS` : '不限'
})

/**
 * 配置项 tooltip：只拼生产上真正有用的信息（分组 / 取值来源 / 默认值 / 数量区间 / 单位 / 取值说明）。
 *
 * 不带后端声明里的 Help 原文之外的开发期注释 —— OptionCatalog 的 Help/OptionsHelp 本身就是
 * 给运营看的字段说明（取哪个值、什么含义），不是代码注释，故保留。
 */
function optionTooltip(opt: ConfigOptionSpec): string {
  const parts: string[] = []
  if (opt.group) parts.push(`分组：${opt.group}`)
  if (opt.unit) parts.push(`单位：${opt.unit}`)
  if (opt.default) parts.push(`默认：${opt.default}`)
  if (opt.min_value != null || opt.max_value != null) {
    parts.push(`范围：${opt.min_value ?? 0} ~ ${opt.max_value ?? '不限'}`)
  }
  if (opt.options_help) parts.push(opt.options_help)
  else if (opt.options?.length) parts.push(`可选：${opt.options.map((o) => o.label).join(' / ')}`)
  if (opt.help) parts.push(opt.help)
  return parts.join('；')
}
</script>

<style scoped lang="css">
.capability-matrix {
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-2);
  padding: var(--space-md) var(--space-lg);
}

.capability-matrix__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
}

.capability-matrix__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.capability-matrix__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md) var(--space-lg);
}

.matrix-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.matrix-cell__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.matrix-cell__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.matrix-cell__empty {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.matrix-cell--full {
  grid-column: 1 / -1;
}

/* 计数与"必填"标记：跟标题/标签同行的弱强调，不抢标签本身的注意力。 */
.matrix-cell__count,
.matrix-cell__req {
  margin-left: 4px;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-muted-foreground);
}

.matrix-cell__code {
  font-family: var(--hs-font-mono);
}

@media (max-width: 768px) {
  .capability-matrix__grid {
    grid-template-columns: 1fr;
  }
}
</style>
