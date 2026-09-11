<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WalletIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">用户钱包</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="handleQuery">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">余额查询</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">用户 ID</span>
          <t-input v-model="userId" placeholder="请输入用户 ID" clearable @enter="handleQuery" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" :loading="loading" @click="handleQuery">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section v-if="wallet" class="wallet-stat-grid">
      <div class="stat-card surface-card stat-card--success">
        <span class="stat-card__icon">
          <WalletIcon size="24" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(wallet.balance) }}</span>
          <span class="stat-card__label">可用余额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon">
          <MoneyIcon size="24" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(wallet.frozen) }}</span>
          <span class="stat-card__label">冻结金额</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon">
          <SwapIcon size="24" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(wallet.total_income) }}</span>
          <span class="stat-card__label">累计收入</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--red">
        <span class="stat-card__icon">
          <TimeIcon size="24" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">¥{{ formatPrice(wallet.total_expense) }}</span>
          <span class="stat-card__label">累计支出</span>
        </div>
      </div>
    </section>

    <section v-if="!wallet && searched" class="table-card surface-card">
      <t-empty description="未查询到钱包数据，请确认用户 ID 是否正确" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { MoneyIcon, RefreshIcon, SearchIcon, SwapIcon, TimeIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getWallet } from '@/api/finance'
import { formatPrice } from '@/pages/finance/constants'
import type { WalletInfo } from '@/types/interface'

defineOptions({ name: 'FinanceWallets' })

const userId = ref<string>()
const wallet = ref<WalletInfo | null>(null)
const loading = ref(false)
const searched = ref(false)

async function handleQuery() {
  const id = Number(userId.value)
  if (!id || id <= 0) {
    MessagePlugin.warning('请输入有效的用户 ID')
    return
  }
  loading.value = true
  try {
    wallet.value = await getWallet(id)
    searched.value = true
  } catch (error) {
    wallet.value = null
    MessagePlugin.error((error as Error).message || '查询钱包失败')
  } finally {
    loading.value = false
  }
}

function handleReset() {
  userId.value = undefined
  wallet.value = null
  searched.value = false
}

onMounted(() => {
  // 默认查询用户 ID 为 1，便于演示
  if (!userId.value) userId.value = '1'
  handleQuery()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>
