<template>
  <div class="page-body point-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <GiftIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">积分概览</h2>
          <p class="page-header__desc">
            积分账本独立于资金账本：通过订单、续费与活动获得，仅用于活动与权益兑换，不可抵扣订单金额、不可提现、不可转入余额。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadOverview">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="wallet-stat-grid">
      <div class="stat-card stat-card--green">
        <span class="stat-card__icon stat-card__icon--green">
          <GiftIcon size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatPoints(overview.total_balance) }}</span>
          <span class="stat-card__label">全平台可用积分</span>
        </div>
      </div>
      <div class="stat-card stat-card--blue">
        <span class="stat-card__icon stat-card__icon--blue">
          <TrendingUpIcon size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatPoints(overview.total_earned) }}</span>
          <span class="stat-card__label">累计发放</span>
        </div>
      </div>
      <div class="stat-card stat-card--orange">
        <span class="stat-card__icon stat-card__icon--orange">
          <ArrowDownCircleIcon size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatPoints(overview.total_spent) }}</span>
          <span class="stat-card__label">累计消耗</span>
        </div>
      </div>
      <div class="stat-card stat-card--cyan">
        <span class="stat-card__icon stat-card__icon--cyan">
          <UserIcon size="22" aria-hidden="true" />
        </span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatPoints(overview.account_count) }}</span>
          <span class="stat-card__label">积分账户数</span>
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">生效中的积分规则</h3>
        <span class="table-card__meta">共 {{ overview.rules.length }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="overview.rules"
        :columns="ruleColumns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #name="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="price-sub">{{ row.code }}</span>
          </div>
        </template>
        <template #scene="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ pointSceneLabel(row.scene) }}</t-tag>
        </template>
        <template #summary="{ row }">
          <span class="cell-muted">{{ pointRuleSummary(row) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂未配置生效的积分规则，请前往「积分规则」新建" />
        </template>
      </t-table>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">最近积分流水</h3>
        <t-link theme="primary" hover="color" @click="goTransactions">查看全部</t-link>
      </div>
      <t-table
        row-key="id"
        :data="overview.recent"
        :columns="txColumns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #user="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.username || '—' }}</span>
            <span class="price-sub">ID {{ row.user_id }}</span>
          </div>
        </template>
        <template #type="{ row }">
          <t-tag :theme="pointTxTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ pointTxTypeLabel(row.type) }}
          </t-tag>
        </template>
        <template #points="{ row }">
          <span :class="row.direction >= 0 ? 'amount-income' : 'amount-expense'">
            {{ row.direction >= 0 ? '+' : '-' }}{{ formatPoints(row.points) }}
          </span>
        </template>
        <template #balance_after="{ row }">
          <span class="price-main">{{ formatPoints(row.balance_after) }}</span>
        </template>
        <template #ref_no="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.ref_no || '—' }}</span>
            <span class="price-sub">{{ row.remark || row.tx_no }}</span>
          </div>
        </template>
        <template #created_at="{ row }">
          <span class="time-text">{{ formatTime(row.created_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="暂无积分流水" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ArrowDownCircleIcon, GiftIcon, RefreshIcon, TrendingUpIcon, UserIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getPointOverview } from '@/api/point'
import {
  formatPoints,
  formatTime,
  pointRuleSummary,
  pointSceneLabel,
  pointTxTypeLabel,
  pointTxTypeTheme,
} from '@/pages/points/constants'
import type { PointOverviewResponse, PointRuleInfo, PointTransactionInfo } from '@/types/interface'

defineOptions({ name: 'PointsOverview' })

const router = useRouter()
const loading = ref(false)
const overview = ref<PointOverviewResponse>({
  total_balance: 0,
  total_earned: 0,
  total_spent: 0,
  account_count: 0,
  rules: [],
  recent: [],
})

const ruleColumns: PrimaryTableCol<PointRuleInfo>[] = [
  { colKey: 'name', title: '规则', minWidth: 200 },
  { colKey: 'scene', title: '场景', width: 120 },
  { colKey: 'summary', title: '计发口径', minWidth: 300 },
]

const txColumns: PrimaryTableCol<PointTransactionInfo>[] = [
  { colKey: 'user', title: '用户', minWidth: 150 },
  { colKey: 'type', title: '类型', width: 110 },
  { colKey: 'points', title: '积分变动', width: 120 },
  { colKey: 'balance_after', title: '变动后', width: 110 },
  { colKey: 'ref_no', title: '来源单号', minWidth: 200 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

async function loadOverview() {
  loading.value = true
  try {
    overview.value = await getPointOverview()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载积分概览失败')
  } finally {
    loading.value = false
  }
}

function goTransactions() {
  void router.push('/points/transactions')
}

onMounted(loadOverview)
</script>

<style lang="css">
@import '../shared.css';
</style>
