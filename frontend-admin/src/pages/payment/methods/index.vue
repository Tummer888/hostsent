<template>
  <div class="page-body payment-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <WalletIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">支付方式</h2>
          <p class="page-header__desc">按支付场景查看收银台可用渠道与路由顺序；用户在上层设置默认方式与优先级后，此处即时反映生效结果。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadOptions">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">场景路由</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">支付场景</span>
          <t-select v-model="scene" :options="sceneOptions" @change="loadOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">可用支付方式</h3>
        <span class="table-card__meta">默认方式：{{ defaultLabel }}</span>
      </div>
      <t-table
        row-key="id"
        :data="options.channels"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #channel_code="{ row }">
          <div class="price-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="price-sub">{{ row.channel_code }}</span>
          </div>
        </template>

        <template #type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ row.type_name || row.type }}</t-tag>
        </template>

        <template #mode="{ row }">
          <t-tag :theme="modeTheme(row.mode)" variant="light" size="small" shape="round">{{ modeLabel(row.mode) }}</t-tag>
        </template>

        <template #limits="{ row }">
          <span class="cell-muted">{{ limitText(row) }}</span>
        </template>

        <template #fee_rate="{ row }">
          <span class="cell-muted">{{ row.fee_rate > 0 ? `${(row.fee_rate * 100).toFixed(2)}%` : '—' }}</span>
        </template>

        <template #priority="{ row }">
          <span class="cell-muted">{{ row.priority }}</span>
        </template>

        <template #is_default="{ row }">
          <t-tag v-if="row.channel_code === options.default" theme="warning" variant="light" size="small" shape="round">默认</t-tag>
          <span v-else class="price-sub">—</span>
        </template>

        <template #empty>
          <t-empty :description="`场景「${sceneLabel(scene)}」暂无可用渠道，请先在支付渠道中启用并勾选该场景`" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { RefreshIcon, WalletIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getMethodOptions } from '@/api/payment'
import {
  formatFenYuan,
  modeLabel,
  modeTheme,
  sceneLabel,
  sceneOptions,
} from '@/pages/payment/constants'
import type { ChannelInfo, MethodOptionsResponse } from '@/types/interface'

defineOptions({ name: 'PaymentMethods' })

const scene = ref('native')
const loading = ref(false)
const options = ref<MethodOptionsResponse>({ scene: 'native', channels: [], default: '' })

const columns: PrimaryTableCol<ChannelInfo>[] = [
  { colKey: 'channel_code', title: '支付方式', minWidth: 200 },
  { colKey: 'type', title: '渠道类型', width: 130 },
  { colKey: 'mode', title: '模式', width: 110 },
  { colKey: 'limits', title: '单笔限额', minWidth: 170 },
  { colKey: 'fee_rate', title: '费率', width: 90 },
  { colKey: 'priority', title: '优先级', width: 90, align: 'center' as const },
  { colKey: 'is_default', title: '默认', width: 80, align: 'center' as const },
]

const defaultLabel = computed(() => {
  const found = options.value.channels.find((c) => c.channel_code === options.value.default)
  return found ? found.name : '—'
})

function limitText(row: ChannelInfo): string {
  const min = row.min_amount_fen > 0 ? `¥${formatFenYuan(row.min_amount_fen)}` : '不限'
  const max = row.max_amount_fen > 0 ? `¥${formatFenYuan(row.max_amount_fen)}` : '不限'
  return `${min} ~ ${max}`
}

async function loadOptions() {
  loading.value = true
  try {
    options.value = await getMethodOptions(scene.value)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载支付方式失败')
    options.value = { scene: scene.value, channels: [], default: '' }
  } finally {
    loading.value = false
  }
}

onMounted(loadOptions)
</script>

<style lang="css">
@import '../shared.css';
</style>
