<template>
  <div class="point-page">
    <PointNav />

    <div class="point-header">
      <h2 class="point-title">我的积分</h2>
      <t-button variant="outline" size="small" :loading="loading" @click="load">
        <template #icon><RefreshIcon /></template>
        刷新
      </t-button>
    </div>

    <t-alert theme="info" class="point-alert">
      积分与账户余额相互独立：积分通过消费、续费与活动获得，仅可用于平台活动与权益兑换，<strong>不能抵扣订单或账单，也不能提现或转入余额</strong>。
    </t-alert>

    <section class="stat-grid">
      <div class="stat-card stat-card--primary">
        <span class="stat-card__label">可用积分</span>
        <span class="stat-card__value">{{ formatPoints(overview?.balance) }}</span>
        <span class="stat-card__hint">
          冻结 {{ formatPoints(overview?.frozen) }}
          <template v-if="(overview?.frozen || 0) > 0"> · 兑换处理中</template>
        </span>
      </div>
      <div class="stat-card">
        <span class="stat-card__label">累计获得</span>
        <span class="stat-card__value">{{ formatPoints(overview?.total_earned) }}</span>
        <span class="stat-card__hint">消费、续费与活动发放合计</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__label">累计消耗</span>
        <span class="stat-card__value">{{ formatPoints(overview?.total_spent) }}</span>
        <span class="stat-card__hint">兑换与人工扣减合计</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__label">生效规则</span>
        <span class="stat-card__value">{{ overview?.rules?.length ?? 0 }}</span>
        <span class="stat-card__hint">积分怎么来，见下方规则说明</span>
      </div>
    </section>

    <section class="point-panel">
      <div class="panel-head">
        <span class="section-title">积分获取规则</span>
      </div>
      <t-table
        row-key="id"
        :data="overview?.rules ?? []"
        :columns="ruleColumns"
        size="small"
        :bordered="false"
        cell-empty-content="—"
      >
        <template #scene="{ row }">
          <t-tag variant="light" size="small" shape="round">{{ pointSceneLabel(row.scene) }}</t-tag>
        </template>
        <template #summary="{ row }">
          <span class="rule-summary">{{ pointRuleSummary(row) }}</span>
        </template>
        <template #empty>
          <t-empty description="当前暂无生效的积分规则" />
        </template>
      </t-table>
    </section>

    <section class="point-panel">
      <div class="panel-head">
        <span class="section-title">最近积分流水</span>
        <t-button variant="text" theme="primary" size="small" @click="router.push('/points/transactions')">
          查看全部
        </t-button>
      </div>
      <t-table
        row-key="id"
        :data="overview?.recent ?? []"
        :columns="txColumns"
        size="small"
        :bordered="false"
        cell-empty-content="—"
      >
        <template #type="{ row }">
          <t-tag :theme="pointTxTypeTheme(row.type)" variant="light" size="small" shape="round">
            {{ pointTxTypeLabel(row.type) }}
          </t-tag>
        </template>
        <template #points="{ row }">
          <span :class="row.direction > 0 ? 'point-earn' : 'point-spend'">
            {{ formatPoints(row.direction > 0 ? row.points : -row.points, true) }}
          </span>
        </template>
        <template #balance_after="{ row }">
          <span class="time-text">{{ formatPoints(row.balance_after) }}</span>
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

import { RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getMyPoints, type PointOverview, type PointRuleInfo, type PointTransactionInfo } from '@/api/point'
import {
  formatPoints,
  formatTime,
  pointRuleSummary,
  pointSceneLabel,
  pointTxTypeLabel,
  pointTxTypeTheme,
} from '@/pages/points/constants'
import PointNav from '@/components/point-nav/index.vue'

defineOptions({ name: 'MyPoints' })

const router = useRouter()
const overview = ref<PointOverview | null>(null)
const loading = ref(false)

const ruleColumns: PrimaryTableCol<PointRuleInfo>[] = [
  { colKey: 'name', title: '规则名称', minWidth: 160 },
  { colKey: 'scene', title: '场景', width: 120 },
  { colKey: 'summary', title: '计发方式', minWidth: 260 },
]

const txColumns: PrimaryTableCol<PointTransactionInfo>[] = [
  { colKey: 'created_at', title: '时间', width: 170 },
  { colKey: 'type', title: '类型', width: 110 },
  { colKey: 'points', title: '积分变动', width: 120, align: 'right' },
  { colKey: 'balance_after', title: '变动后余额', width: 120, align: 'right' },
  { colKey: 'ref_no', title: '关联单号', minWidth: 180, ellipsis: true },
  { colKey: 'remark', title: '备注', minWidth: 160, ellipsis: true },
]

async function load() {
  loading.value = true
  try {
    const { data } = await getMyPoints()
    overview.value = data
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载积分信息失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped src="./shared.css"></style>
<style scoped>
.point-earn {
  color: #16a34a;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.point-spend {
  color: #d97706;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.rule-summary {
  color: #475569;
  font-size: 13px;
}
</style>
