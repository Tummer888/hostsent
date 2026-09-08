<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CheckCircleIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">对账中心</h2>
        </div>
      </div>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">对账条件</h3>
        <t-space size="small">
          <t-button theme="primary" :loading="loading" @click="handleReconcile">
            <template #icon>
              <CheckCircleIcon aria-hidden="true" />
            </template>
            发起对账
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">账期（选填）</span>
          <t-input v-model="period" placeholder="留空表示全量对账，如 2026-08" clearable @enter="handleReconcile" />
        </div>
      </div>
    </section>

    <template v-if="result">
      <div class="recon-grid">
        <div class="stat-card surface-card stat-card--success">
          <span class="stat-card__icon">
            <SwapIcon size="24" aria-hidden="true" />
          </span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(result.income_total) }}</span>
            <span class="stat-card__label">收入合计</span>
          </div>
        </div>
        <div class="stat-card surface-card stat-card--red">
          <span class="stat-card__icon">
            <TimeIcon size="24" aria-hidden="true" />
          </span>
          <div class="stat-card__info">
            <span class="stat-card__value">¥{{ formatPrice(result.expense_total) }}</span>
            <span class="stat-card__label">支出合计</span>
          </div>
        </div>
        <div class="stat-card surface-card stat-card--blue">
          <span class="stat-card__icon">
            <WalletIcon size="24" aria-hidden="true" />
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
          <span class="recon-row__label">账实差异</span>
          <span class="recon-row__value" :class="result.diff === 0 ? 'recon-diff-ok' : 'recon-diff-bad'">
            ¥{{ formatPrice(result.diff) }}
          </span>
        </div>
        <div class="recon-row">
          <span class="recon-row__label">对账状态</span>
          <span class="recon-row__value">
            <t-tag :theme="result.status === 'ok' ? 'success' : 'danger'" variant="light" size="small" shape="round">
              {{ result.status === 'ok' ? '账实相符' : '存在差异' }}
            </t-tag>
          </span>
        </div>
      </section>
    </template>

    <section v-else class="table-card surface-card">
      <t-empty description="点击「发起对账」查看结果" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

import { CheckCircleIcon, SwapIcon, TimeIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { reconcile } from '@/api/finance'
import { formatPrice } from '@/pages/finance/constants'
import type { ReconcileResponse } from '@/types/interface'

defineOptions({ name: 'FinanceReconciliation' })

const period = ref<string>()
const loading = ref(false)
const result = ref<ReconcileResponse | null>(null)

async function handleReconcile() {
  loading.value = true
  try {
    result.value = await reconcile(period.value || undefined)
    if (result.value.status === 'ok') {
      MessagePlugin.success('对账通过，账实相符')
    } else {
      MessagePlugin.warning('对账存在差异，请核查')
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
</style>
