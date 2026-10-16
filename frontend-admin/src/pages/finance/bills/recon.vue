<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CheckCircleIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">对账中心</h2>
        </div>
      </div>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">账期（选填）</span>
        <t-input v-model="period" placeholder="留空表示全量对账，如 2026-08" clearable @enter="handleReconcile" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" :loading="loading" @click="handleReconcile">
            <template #icon>
              <CheckCircleIcon aria-hidden="true" />
            </template>
            发起对账
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <template v-if="result">
      <div class="recon-grid">
        <div class="stat-card stat-card--success">
          <span class="stat-card__icon">
            <SwapIcon size="20" aria-hidden="true" />
          </span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(result.income_total) }}</span>
            <span class="stat-card__label">收入合计</span>
          </div>
        </div>
        <div class="stat-card stat-card--red">
          <span class="stat-card__icon">
            <TimeIcon size="20" aria-hidden="true" />
          </span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(result.expense_total) }}</span>
            <span class="stat-card__label">支出合计</span>
          </div>
        </div>
        <div class="stat-card stat-card--blue">
          <span class="stat-card__icon">
            <WalletIcon size="20" aria-hidden="true" />
          </span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(result.wallet_balance) }}</span>
            <span class="stat-card__label">钱包余额合计</span>
          </div>
        </div>
      </div>

      <section class="surface-card recon-result">
        <h3 class="card-title">对账结果</h3>
        <div class="recon-row">
          <span class="recon-row__label">对账账期</span>
          <span class="recon-row__value">{{ result.period || '全量' }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">流水笔数</span>
          <span class="recon-row__value">{{ result.tx_count }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">净变动（收入-支出）</span>
          <span class="recon-row__value">¥{{ formatPrice(result.income_total - result.expense_total) }}</span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">账户资产（可用+冻结）</span>
          <span class="recon-row__value">
            ¥{{ formatPrice(result.wallet_balance + (result.wallet_frozen || 0)) }}
          </span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">账实差异</span>
          <span class="recon-row__value" :class="result.diff === 0 ? 'recon-diff-ok' : 'recon-diff-bad'">
            ¥{{ formatPrice(result.diff) }}
          </span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">对账状态</span>
          <span class="recon-row__value">
            <t-tag :theme="result.status === 'ok' ? 'success' : 'danger'" variant="light" size="small" shape="round">
              {{ result.status === 'ok' ? '账实相符' : `${result.issue_count} 项存在差异` }}
            </t-tag>
          </span>
        </div>
      </section>

      <!-- 差异明细：总量差额说不出是谁的问题，逐户/逐单才能直接定位 -->
      <section v-if="result.issues?.length" class="table-card surface-card">
        <h3 class="card-title">差异明细</h3>
        <t-table
          row-key="rowKey"
          :data="issueRows"
          :columns="issueColumns"
          size="small"
          hover
          table-layout="fixed"
        >
          <template #check="{ row }">
            <t-tag :theme="issueTheme(row.check)" variant="light" size="small" shape="round">
              {{ row.label }}
            </t-tag>
          </template>
          <template #user_id="{ row }">
            <span class="recon-user">{{ row.user_id }}</span>
            <span v-if="row.username" class="recon-user__name">{{ row.username }}</span>
          </template>
          <template #account_value="{ row }">¥{{ formatPrice(row.account_value) }}</template>
          <template #ledger_value="{ row }">¥{{ formatPrice(row.ledger_value) }}</template>
          <template #diff="{ row }">
            <span :class="row.diff === 0 ? 'recon-diff-ok' : 'recon-diff-bad'">
              {{ row.diff > 0 ? '+' : '' }}{{ formatPrice(row.diff) }}
            </span>
          </template>
        </t-table>
      </section>
    </template>

    <section v-else class="table-card surface-card">
      <t-empty description="点击「发起对账」查看结果" />
    </section>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, ref } from 'vue'

import { CheckCircleIcon, SwapIcon, TimeIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { reconcile } from '@/api/finance'
import { formatPrice } from '@/pages/finance/constants'
import type { ReconcileResponse } from '@/types/interface'

defineOptions({ name: 'FinanceReconciliation' })

const period = ref<string>()
const loading = ref(false)
const result = ref<ReconcileResponse | null>(null)

// 差异明细列。表头文案由后端 label 给出，前端只补数值格式化。
const issueColumns = [
  { colKey: 'check', title: '检查项', width: 130 },
  { colKey: 'user_id', title: '用户', width: 180 },
  { colKey: 'account_value', title: '账面值', width: 130, align: 'right' as const },
  { colKey: 'ledger_value', title: '台账值', width: 130, align: 'right' as const },
  { colKey: 'diff', title: '差异', width: 120, align: 'right' as const },
  { colKey: 'remark', title: '说明', minWidth: 280 },
]

// 表格需要稳定 row-key；同一用户同一检查项唯一，拼起来即可。
const issueRows = computed(() =>
  (result.value?.issues || []).map((item, index) => ({
    ...item,
    rowKey: `${item.user_id}-${item.check}-${index}`,
  })),
)

// 检查项配色：金额类差异偏红（真金白银对不上），单据类偏黄（多为流程缺口）。
function issueTheme(check: string): 'danger' | 'warning' | 'default' {
  if (['balance', 'frozen'].includes(check)) return 'danger'
  if (['income', 'expense', 'recharge', 'refund', 'orphan_ledger'].includes(check)) return 'warning'
  return 'default'
}

async function handleReconcile() {
  loading.value = true
  try {
    result.value = await reconcile(period.value || undefined)
    if (result.value.status === 'ok') {
      MessagePlugin.success('对账通过，账实相符')
    } else {
      MessagePlugin.warning(`对账存在 ${result.value.issue_count} 项差异，请核查明细`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '对账失败')
  } finally {
    loading.value = false
  }
}

function handleReset() {
  period.value = undefined
  result.value = null
}
</script>

<style lang="css">
@import '../shared.css';

/* 用户列：ID 与用户名同格展示，ID 为主（对账是按账户核的）。 */
.recon-user {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.recon-user__name {
  margin-left: 6px;
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.recon-diff-ok {
  color: var(--hs-status-ok-text);
}

.recon-diff-bad {
  color: var(--hs-status-danger-text);
  font-weight: 600;
}
</style>
