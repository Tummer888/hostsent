<template>
  <div class="payment-capability">
    <div class="payment-capability__head">
      <h4 class="payment-capability__title">{{ title }}</h4>
      <t-space size="small">
        <t-tag :theme="modeTheme(descriptor.mode)" variant="light" size="small" shape="round">
          {{ labelOf(MODE_LABELS, descriptor.mode) }}
        </t-tag>
        <t-tag v-if="!descriptor.implemented" theme="warning" variant="light" size="small" shape="round">适配器未接入</t-tag>
        <t-tag v-else theme="success" variant="light" size="small" shape="round">适配器已接入</t-tag>
      </t-space>
    </div>

    <div class="payment-capability__grid">
      <div class="matrix-cell">
        <span class="matrix-cell__label">支持场景</span>
        <div class="matrix-cell__tags">
          <template v-if="scenes.length">
            <t-tag v-for="s in scenes" :key="s" theme="default" variant="light" size="small" shape="round">
              {{ labelOf(SCENE_LABELS, s) }}
            </t-tag>
          </template>
          <span v-else class="matrix-cell__empty">未声明</span>
        </div>
      </div>

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
        <span class="matrix-cell__label">签名 / 证书</span>
        <div class="matrix-cell__tags">
          <t-tag theme="default" variant="light" size="small" shape="round">
            签名：{{ labelOf(SIGNER_LABELS, descriptor.signer_type) }}
          </t-tag>
          <t-tag theme="default" variant="light" size="small" shape="round">
            证书：{{ labelOf(CERT_MODE_LABELS, descriptor.cert_mode) }}
          </t-tag>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">资金能力</span>
        <div class="matrix-cell__tags">
          <t-tag :theme="descriptor.supports_payout ? 'success' : 'default'" variant="light" size="small" shape="round">
            打款：{{ descriptor.supports_payout ? '支持' : '不支持' }}
          </t-tag>
          <t-tag :theme="descriptor.supports_partial_refund ? 'success' : 'default'" variant="light" size="small" shape="round">
            部分退款：{{ descriptor.supports_partial_refund ? '支持' : '不支持' }}
          </t-tag>
          <t-tag :theme="descriptor.supports_query ? 'success' : 'default'" variant="light" size="small" shape="round">
            主动查单：{{ descriptor.supports_query ? '支持' : '不支持' }}
          </t-tag>
        </div>
      </div>

      <div class="matrix-cell">
        <span class="matrix-cell__label">费率 / 结算</span>
        <div class="matrix-cell__tags">
          <t-tag theme="default" variant="light" size="small" shape="round">
            费率：{{ feeText }}
          </t-tag>
          <t-tag theme="default" variant="light" size="small" shape="round">
            结算：{{ descriptor.settle_mode || '—' }}
          </t-tag>
          <t-tag theme="default" variant="light" size="small" shape="round">
            币种：{{ descriptor.currency || 'CNY' }}
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

      <div v-if="descriptor.doc_url" class="matrix-cell matrix-cell--full">
        <span class="matrix-cell__label">官方文档</span>
        <a class="matrix-cell__doc" :href="descriptor.doc_url" target="_blank" rel="noopener noreferrer">{{ descriptor.doc_url }}</a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import { modeTheme } from '@/pages/payment/constants'
import type { PaymentCapabilityDescriptor } from '@/types/interface'

import { CERT_MODE_LABELS, MODE_LABELS, OPERATION_LABELS, SCENE_LABELS, SIGNER_LABELS, labelOf } from './capability-labels'

const props = withDefaults(
  defineProps<{
    descriptor?: PaymentCapabilityDescriptor | null
    title?: string
  }>(),
  {
    descriptor: null,
    title: '能力矩阵',
  },
)

const EMPTY: PaymentCapabilityDescriptor = {
  mode: 'api',
  scenes: null,
  operations: null,
  credential_schema: null,
  endpoint_schema: null,
  cert_mode: '',
  signer_type: 'none',
  supports_payout: false,
  supports_partial_refund: false,
  supports_query: false,
  currency: 'CNY',
  fee_rate: 0,
  settle_mode: '',
  implemented: false,
  doc_url: '',
}

const descriptor = computed<PaymentCapabilityDescriptor>(() => props.descriptor || EMPTY)
const scenes = computed(() => descriptor.value.scenes || [])
const operations = computed(() => descriptor.value.operations || [])
const credentialFields = computed(() => descriptor.value.credential_schema || [])
const feeText = computed(() => (descriptor.value.fee_rate > 0 ? `${(descriptor.value.fee_rate * 100).toFixed(2)}%` : '—'))
</script>

<style scoped lang="css">
.payment-capability {
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-2);
  padding: var(--space-md) var(--space-lg);
}

.payment-capability__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  margin-bottom: var(--space-md);
}

.payment-capability__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.payment-capability__grid {
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

.matrix-cell__doc {
  font-size: 12px;
  color: var(--color-primary);
  word-break: break-all;
}

@media (max-width: 768px) {
  .payment-capability__grid {
    grid-template-columns: 1fr;
  }
}
</style>
