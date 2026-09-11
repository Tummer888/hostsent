<template>
  <div class="capability-matrix">
    <div class="capability-matrix__head">
      <h4 class="capability-matrix__title">{{ title }}</h4>
      <t-space size="small">
        <t-tag :theme="descriptor.kind === 'compute' ? 'success' : 'default'" variant="light" size="small" shape="round">
          {{ labelOf(KIND_LABELS, descriptor.kind) }}
        </t-tag>
        <t-tag v-if="!descriptor.implemented" theme="warning" variant="light" size="small" shape="round">适配器未接入</t-tag>
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
            <t-tag v-for="cycle in billingCycles" :key="cycle" theme="default" variant="light" size="small" shape="round">
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
          <t-tag theme="default" variant="light" size="small" shape="round">
            限流：{{ rateLimitText }}
          </t-tag>
          <t-tag v-if="descriptor.supports_paging" theme="default" variant="light" size="small" shape="round">分页读取</t-tag>
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

      <!-- 平台字段字典：D5 预埋位，待接入平台先展示"待核实"的字段名映射 -->
      <div v-if="dictionaryEntries.length" class="matrix-cell matrix-cell--full">
        <span class="matrix-cell__label">
          平台字段字典
          <t-tag v-if="dictionarySource" theme="warning" variant="light" size="small" shape="round">{{ dictionarySource }}</t-tag>
        </span>
        <div class="matrix-cell__tags">
          <t-tooltip v-for="entry in dictionaryEntries" :key="entry.key" :content="entry.value" placement="top">
            <t-tag theme="default" variant="light" size="small" shape="round">
              <code class="matrix-cell__code">{{ entry.key }}</code>
            </t-tag>
          </t-tooltip>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import type { CapabilityDescriptor } from '@/types/interface'

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
  implemented: false,
}

const descriptor = computed<CapabilityDescriptor>(() => props.descriptor || EMPTY)
const operations = computed(() => descriptor.value.operations || [])
const syncScopes = computed(() => descriptor.value.sync_scopes || [])
const billingCycles = computed(() => descriptor.value.billing_cycles || [])
const credentialFields = computed(() => descriptor.value.credential_schema || [])
const rateLimitText = computed(() => {
  const qps = descriptor.value.rate_limit?.qps || 0
  return qps > 0 ? `${qps} QPS` : '不限'
})

// 平台字段字典（D5 预埋）：字符串键值直接展示，数组/对象转为 JSON 摘要作为 tooltip。
const dictionarySource = computed(() => {
  const src = (descriptor.value.field_dictionary || {})['source']
  return typeof src === 'string' ? src : ''
})

const dictionaryEntries = computed(() => {
  const dict = descriptor.value.field_dictionary || {}
  return Object.entries(dict)
    .filter(([key]) => key !== 'source')
    .map(([key, value]) => ({
      key,
      value: typeof value === 'string' ? value : JSON.stringify(value),
    }))
})
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

.matrix-cell__code {
  font-family: var(--hs-font-mono);
}

@media (max-width: 768px) {
  .capability-matrix__grid {
    grid-template-columns: 1fr;
  }
}
</style>
