<template>
  <div class="page-body resource-module">
    <header v-if="!isEmbedded" class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <LayersIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">容量与位置监测</h2>
          <p class="page-header__desc">只看资源池的容量与位置：CPU / 内存 / 磁盘用量对比配额，池所属地域与可用区</p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadPools">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section v-if="!isEmbedded" class="stat-grid stat-grid--pools">
      <article v-for="card in statCards" :key="card.key" class="stat-card" :class="`stat-card--${card.theme}`">
        <div class="stat-card__icon">
          <component :is="card.icon" size="20" aria-hidden="true" />
        </div>
        <div class="stat-card__text">
          <!--
            计数跟在标题后面（用户要求）：原先计数在卡片底部的小字里，
            与标题隔一行，读数要来回对照。现在「CPU 用量 1 / 8 核」一行读完。

            计数与标题并列而非嵌在标题文本里：共享的 .stat-card__label 带
            overflow:hidden + ellipsis，嵌进去在窄卡上会被截断。
          -->
          <span class="stat-card__title">
            <span class="stat-card__label">{{ card.label }}</span>
            <em v-if="card.count" class="stat-card__count">{{ card.count }}</em>
          </span>
          <span v-if="card.percent == null" class="stat-card__value">{{ card.value }}</span>
          <span v-if="card.hint" class="stat-card__hint">{{ card.hint }}</span>
        </div>
        <!--
          容量类卡片用环形百分比替代原来的大号百分数（用户要求）：
          一眼看出"用了多少"，且六个卡片的视觉重量一致（计数卡是大号数字，
          容量卡是同一枚环），不再出现"两个大字"抢视线。
        -->
        <span
          v-if="card.percent != null"
          class="stat-gauge"
          :style="gaugeStyle(card.percent)"
          :title="`${card.label} ${card.percent}%`"
        >
          <svg class="stat-gauge__ring" viewBox="0 0 36 36" aria-hidden="true">
            <circle class="stat-gauge__track" cx="18" cy="18" r="15.5" />
            <circle class="stat-gauge__fill" cx="18" cy="18" r="15.5" />
          </svg>
          <em class="stat-gauge__num">{{ card.percent }}%</em>
        </span>
      </article>
    </section>

    <FilterCard v-if="!isEmbedded" :columns="4">
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" clearable placeholder="池名 / 上游 ID" @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">所属提供商</span>
        <t-select v-model="filters.provider_id" clearable placeholder="全部提供商" :options="providerOptions" />
      </div>
      <div class="field">
        <span class="field__label">位置（地域）</span>
        <t-select v-model="filters.region" clearable placeholder="全部地域" :options="regionOptions" />
      </div>
      <!-- 层级视图下按「区域 → 宿主机/存储」成树，类别与告警筛选意义不同（区域无容量），故只保留平铺视图使用。 -->
      <div v-if="viewMode === 'flat'" class="field">
        <span class="field__label">池类别</span>
        <t-select v-model="filters.category" :options="categoryOptions" @change="handleSearch" />
      </div>
      <div v-if="viewMode === 'flat'" class="field">
        <span class="field__label">容量告警</span>
        <t-select v-model="filters.alert" clearable placeholder="全部资源池" :options="alertOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-radio-group v-model="viewMode" variant="default-filled" size="small" @change="handleViewChange">
            <t-radio-button value="group">层级视图</t-radio-button>
            <t-radio-button value="flat">平铺视图</t-radio-button>
          </t-radio-group>
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">资源池列表</h3>
        <span class="table-card__meta">
          {{ viewMode === 'group' ? `共 ${total} 个区域（含下属宿主机 / 存储）` : `共 ${total} 个资源池` }}
        </span>
      </div>
      <t-table
        row-key="id"
        :data="poolList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :tree="viewMode === 'group' ? { childrenKey: 'children', defaultExpandAll: true, indent: 24 } : undefined"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="pool-cell">
            <span class="pool-name">{{ row.name }}</span>
            <span v-if="viewMode === 'group' && row.child_count" class="pool-provider">
              {{ row.child_count }} 个{{ row.category === 'location' ? '宿主机 / 存储' : '子池' }}
            </span>
            <!-- 区域行没有子节点时，容量三列只能是空值。这里明说原因，
                 否则读者会把「区域 + 内存/磁盘为空」读成数据缺失或渲染错误。 -->
            <t-tooltip
              v-else-if="viewMode === 'group' && row.category === 'location'"
              content="该区域下暂无宿主机 / 存储节点（或已被当前筛选条件排除），因此没有容量可汇总"
              placement="top"
            >
              <span class="pool-hint">无子节点</span>
            </t-tooltip>
            <span v-else-if="!isEmbedded" class="pool-provider">{{
              row.provider_name || `渠道 #${row.provider_id}`
            }}</span>
          </div>
        </template>

        <template #location="{ row }">
          <div class="loc-cell">
            <span v-if="row.region" class="loc-region">{{ row.region }}</span>
            <t-tag v-else theme="warning" variant="light" size="small" shape="round">未标注位置</t-tag>
            <span v-if="row.zone" class="loc-zone">{{ row.zone }}</span>
          </div>
        </template>

        <template #cpu="{ row }">
          <span v-if="row.category === 'location' && !row.aggregated" class="cell-na">不适用</span>
          <div v-else class="spec-cell">
            <span class="spec-text">{{ row.used_cpu }} / {{ row.total_cpu }}</span>
            <t-progress
              :percentage="row.cpu_usage_percent"
              :color="usageColor(row.cpu_usage_percent)"
              :stroke-width="6"
            />
            <span v-if="row.aggregated" class="spec-hint">区域合计</span>
          </div>
        </template>

        <template #memory="{ row }">
          <span v-if="row.category === 'location' && !row.aggregated" class="cell-na">不适用</span>
          <div v-else class="spec-cell">
            <span class="spec-text"
              >{{ formatPoolMemory(row.used_memory) }} / {{ formatPoolMemory(row.total_memory) }}</span
            >
            <t-progress
              :percentage="row.memory_usage_percent"
              :color="usageColor(row.memory_usage_percent)"
              :stroke-width="6"
            />
          </div>
        </template>

        <template #disk="{ row }">
          <span v-if="row.category === 'location' && !row.aggregated" class="cell-na">不适用</span>
          <div v-else class="spec-cell">
            <span class="spec-text">{{ formatDisk(row.used_disk) }} / {{ formatDisk(row.total_disk) }}</span>
            <t-progress
              :percentage="row.disk_usage_percent"
              :color="usageColor(row.disk_usage_percent)"
              :stroke-width="6"
            />
          </div>
        </template>

        <template #pool_type="{ row }">
          <t-tag :theme="poolTypeTheme(row.pool_type)" variant="light" size="small" shape="round">{{
            poolTypeLabel(row.pool_type)
          }}</t-tag>
        </template>

        <template #probe="{ row }">
          <t-tag v-if="!row.probe_status" theme="default" variant="light" size="small" shape="round">未接入</t-tag>
          <t-tooltip v-else :content="row.probe_message || probeLabel(row.probe_status)" placement="top">
            <t-tag :theme="probeTheme(row.probe_status)" variant="light" size="small" shape="round">
              {{ probeLabel(row.probe_status) }}
            </t-tag>
          </t-tooltip>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 1 ? '启用中' : '已停用' }}
          </t-tag>
        </template>

        <template #ops_url="{ row }">
          <t-link
            v-if="row.provider_ops_url"
            theme="primary"
            hover="color"
            :href="row.provider_ops_url"
            target="_blank"
          >
            运维平台
          </t-link>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="
                buildMobileActionOptions([
                  { content: '详情', value: 'detail', theme: 'default' },
                  { content: '运维平台', value: 'ops', hidden: () => !row.provider_ops_url, theme: 'default' },
                  { content: '删除', value: 'delete', theme: 'error' },
                ])
              "
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openDetailDialog(row)">详情</t-link>
              <span v-if="row.provider_ops_url" class="action-gap" />
              <t-link
                v-if="row.provider_ops_url"
                theme="primary"
                hover="color"
                :href="row.provider_ops_url"
                target="_blank"
              >
                运维平台
              </t-link>
              <span class="action-gap" />
              <t-link theme="danger" hover="color" @click="removePool(row)">删除</t-link>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无资源池数据" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="mobilePage.current"
        :page-size="mobilePage.pageSize"
        :total="mobilePage.total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <t-dialog
      v-model:visible="detailVisible"
      :header="detailPool ? `资源池详情 · ${detailPool.name}` : '资源池详情'"
      width="560px"
      :footer="false"
      @close="detailVisible = false"
    >
      <t-descriptions v-if="detailPool" :column="2" bordered size="small">
        <t-descriptions-item label="池名称">{{ detailPool.name }}</t-descriptions-item>
        <t-descriptions-item label="上游 ID">{{ detailPool.upstream_id || '—' }}</t-descriptions-item>
        <t-descriptions-item label="所属渠道">{{
          detailPool.provider_name || `渠道 #${detailPool.provider_id}`
        }}</t-descriptions-item>
        <t-descriptions-item label="类型">{{ poolTypeLabel(detailPool.pool_type) }}</t-descriptions-item>
        <t-descriptions-item label="位置">
          <span v-if="detailPool.region">
            {{ detailPool.region }}<template v-if="detailPool.zone"> / {{ detailPool.zone }}</template>
          </span>
          <t-tag v-else theme="warning" variant="light" size="small" shape="round">未标注位置</t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="状态">
          <t-tag :theme="detailPool.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ detailPool.status === 1 ? '启用中' : '已停用' }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="类别">
          <t-tag
            :theme="detailPool.category === 'location' ? 'default' : 'primary'"
            variant="light"
            size="small"
            shape="round"
          >
            {{ detailPool.category === 'location' ? '位置型' : '容量型' }}
          </t-tag>
        </t-descriptions-item>
        <!-- 位置型池没有容量配额，不渲染三行 0/0；改为一句说明，避免读成"数据缺失"。 -->
        <t-descriptions-item v-if="detailPool.category === 'location'" label="容量" :span="2">
          <span class="cell-muted"
            >位置型资源项（区域 / 可用区）不持有 CPU / 内存 / 磁盘配额；容量请查所属渠道下的宿主机节点池。</span
          >
        </t-descriptions-item>
        <template v-else>
          <t-descriptions-item label="CPU 配额">
            {{ detailPool.used_cpu }} / {{ detailPool.total_cpu }}（{{ detailPool.cpu_usage_percent }}%）
          </t-descriptions-item>
          <t-descriptions-item label="内存配额">
            {{ formatPoolMemory(detailPool.used_memory) }} / {{ formatPoolMemory(detailPool.total_memory) }}（{{
              detailPool.memory_usage_percent
            }}%）
          </t-descriptions-item>
          <t-descriptions-item label="磁盘配额">
            {{ formatDisk(detailPool.used_disk) }} / {{ formatDisk(detailPool.total_disk) }}（{{
              detailPool.disk_usage_percent
            }}%）
          </t-descriptions-item>
        </template>
        <t-descriptions-item label="最后同步">{{
          detailPool.last_sync_at ? formatTime(detailPool.last_sync_at) : '从未同步'
        }}</t-descriptions-item>
        <t-descriptions-item label="探针状态">
          <t-tag v-if="!detailPool.probe_status" theme="default" variant="light" size="small" shape="round"
            >未接入</t-tag
          >
          <template v-else>
            <t-tag :theme="probeTheme(detailPool.probe_status)" variant="light" size="small" shape="round">
              {{ probeLabel(detailPool.probe_status) }}
            </t-tag>
            <span class="probe-detail-time">{{ formatTime(detailPool.probe_at || '') }}</span>
          </template>
        </t-descriptions-item>
        <t-descriptions-item v-if="detailPool.probe_message" label="探针采样" :span="2">
          {{ detailPool.probe_message }}
        </t-descriptions-item>
        <t-descriptions-item label="运维平台">
          <t-link
            v-if="detailPool.provider_ops_url"
            theme="primary"
            hover="color"
            :href="detailPool.provider_ops_url"
            target="_blank"
          >
            打开运维平台
          </t-link>
          <span v-else class="cell-muted">未配置</span>
        </t-descriptions-item>
      </t-descriptions>
      <t-alert
        v-if="detailPool && detailPool.category === 'location'"
        theme="info"
        message="位置型资源项来自上游区域接口（如魔方云 /areas），只提供位置与节点/存储计数，没有容量与实时状态。容量与探针看该渠道下类型为「宿主机」的池。"
        class="probe-hint"
      />
      <t-alert
        v-else-if="detailPool && !detailPool.probe_status"
        theme="info"
        message="该资源池尚未接入探针流：当前用量来自上游同步周期快照，接入探针后可展示实时状态 / 内存 / 硬盘用量。"
        class="probe-hint"
      />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'

import {
  CloudDownloadIcon,
  ErrorCircleIcon,
  LayersIcon,
  RefreshIcon,
  SearchIcon,
  ServerIcon,
} from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { deletePool, getPoolList, getProviderList } from '@/api/admin'
import type { PoolCapacitySummary, PoolInfo, ProviderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
// 池容量用 formatPoolMemory（入参 GB）；formatMemory 是商品/实例链路的 MB 口径，
// 两者混用会把 15GB 的宿主机显示成「15 MB」（实测故障）。
import { formatDisk, formatPoolMemory } from '@/utils/spec-format'

defineOptions({ name: 'ResourcePools' })

// 作为详情页内嵌面板时传入 provider_id；独立使用时为 undefined（全部）
const props = defineProps<{
  providerId?: number
}>()

const poolList = ref<PoolInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)
const providerOptions = ref<{ label: string; value: number }[]>([])
const regionOptions = ref<{ label: string; value: string }[]>([])

const emptySummary: PoolCapacitySummary = {
  total_pools: 0,
  online_pools: 0,
  total_cpu: 0,
  used_cpu: 0,
  total_memory: 0,
  used_memory: 0,
  total_disk: 0,
  used_disk: 0,
  warning_pools: 0,
  danger_pools: 0,
  unlocated_pools: 0,
  capacity_pools: 0,
  location_pools: 0,
}
const summary = ref<PoolCapacitySummary>({ ...emptySummary })

const alertOptions = [
  { label: '告警池（用量≥80%）', value: 'danger' },
  { label: '预警池（用量≥60%）', value: 'warn' },
]

// 池类别：容量型（宿主机/存储，有 CPU/内存/磁盘）与位置型（区域/可用区，只有位置）是
// 两类完全不同的实体。默认只看容量型——区域混在容量列表里会显示成 0/0 + 空进度条，
// 看着像数据缺失，其实是"没有容量这个概念"。要看区域切到「位置型」。
const categoryOptions = [
  { label: '容量型（宿主机 / 存储）', value: 'capacity' },
  { label: '位置型（区域 / 可用区）', value: 'location' },
  { label: '全部', value: 'all' },
]

// 视图模式：
//   group=层级视图：按「区域 → 宿主机/存储」成树，一屏看清某区域下有几台宿主机、状态如何；
//   flat =平铺视图：单张表列全部池，按类别/告警筛选，适合排查单台池。
// 默认层级视图——这才是资源池真实的组织方式；embed 到渠道详情页时也用层级视图。
const viewMode = ref<'group' | 'flat'>('group')

const filters = reactive<{
  keyword: string
  provider_id: number | undefined
  region: string
  alert: string
  category: string
}>({
  keyword: '',
  provider_id: undefined,
  region: '',
  alert: '',
  category: 'capacity',
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})

const effectiveProviderId = computed(() => {
  if (props.providerId !== undefined) return props.providerId
  return filters.provider_id
})

const isEmbedded = computed(() => props.providerId !== undefined)

// 位置型池（区域/可用区）没有容量语义，其列只渲染位置型字段，不套 CPU/内存/磁盘，
// 否则区域行会永远显示 0 / 0 加空进度条——用户读起来像"采不到数据"，其实是"没有这个概念"。
// 定义在 statCards / columns 之前：两处都按它决定是否渲染容量区块。
//
// 层级视图恒为 false：层级视图里父行是区域的容量汇总（rollup 出的真实数字），
// 必须出容量列；若沿用平铺视图残留的 category=location，汇总值会被整列藏掉。
const isLocationView = computed(() => viewMode.value === 'flat' && filters.category === 'location')
// 混合视图（全部）下不按行区分列，统一展示容量三列；位置型行由单元格内容标「不适用」区分。

// 容量汇总卡片：容量用量取「容量型池」的累加（位置型池没有配额，不计入）。
// 位置型视图下不渲染 CPU/内存/磁盘三张卡——那里它们的值恒为 0，只会误导。
//
// 卡片结构（用户要求）：计数跟在标题后面（count），容量类用环形百分比（percent）。
// 计数卡（容量池/告警池/未标注）没有 percent —— 它们的"用量"不是配额占比，
// 硬算一个百分比只会误导；它们保留大号数值。
type StatCard = {
  key: string
  label: string
  /** 跟在标题后的计数（如「1 / 8 核」「在线 2」）。空串则不渲染。 */
  count: string
  /** 无 percent 时展示的大号数值。 */
  value: string
  hint: string
  icon: unknown
  theme: 'info' | 'success' | 'danger' | 'warning' | 'cyan' | 'indigo' | 'teal' | 'slate'
  /** 容量类卡片的使用率（0-100）；计数类为 null。 */
  percent: number | null
}

const statCards = computed<StatCard[]>(() => {
  const s = summary.value
  const cards: StatCard[] = [
    {
      key: 'pools',
      label: '容量池 / 位置项',
      count: `在线 ${s.online_pools}`,
      value: `${s.capacity_pools} / ${s.location_pools}`,
      hint: `容量池 ${s.capacity_pools} · 位置项 ${s.location_pools}`,
      icon: LayersIcon,
      theme: 'info',
      percent: null,
    },
  ]
  if (!isLocationView.value) {
    const cpuPercent = percent(s.used_cpu, s.total_cpu)
    const memPercent = percent(s.used_memory, s.total_memory)
    const diskPercent = percent(s.used_disk, s.total_disk)
    cards.push(
      {
        key: 'cpu',
        label: 'CPU 用量',
        count: `${s.used_cpu} / ${s.total_cpu} 核`,
        value: `${cpuPercent}%`,
        hint: '',
        icon: ServerIcon,
        theme: 'success' as const,
        percent: cpuPercent,
      },
      {
        key: 'memory',
        label: '内存用量',
        count: `${formatPoolMemory(s.used_memory)} / ${formatPoolMemory(s.total_memory)}`,
        value: `${memPercent}%`,
        hint: '',
        icon: ServerIcon,
        theme: 'cyan' as const,
        percent: memPercent,
      },
      {
        key: 'disk',
        label: '磁盘用量',
        count: `${formatDisk(s.used_disk)} / ${formatDisk(s.total_disk)}`,
        value: `${diskPercent}%`,
        hint: '',
        icon: CloudDownloadIcon,
        theme: 'indigo' as const,
        percent: diskPercent,
      },
      {
        key: 'alert',
        label: '容量告警池',
        count: `预警 ${s.warning_pools}`,
        value: `${s.danger_pools}`,
        hint: `危险 ${s.danger_pools} · 预警 ${s.warning_pools}`,
        icon: ErrorCircleIcon,
        theme: s.danger_pools > 0 ? ('danger' as const) : ('teal' as const),
        percent: null,
      },
    )
  }
  cards.push({
    key: 'location',
    label: '未标注位置',
    count: '',
    value: `${s.unlocated_pools}`,
    hint: '需在上游或渠道补充地域',
    icon: LayersIcon,
    theme: s.unlocated_pools > 0 ? ('warning' as const) : ('slate' as const),
    percent: null,
  })
  return cards
})

/**
 * 环形百分比的样式：用 CSS 变量的 stroke-dasharray 画进度。
 * 圆周长 = 2πr = 2π × 15.5 ≈ 97.39（与模板里的 r=15.5 对应）。
 * 用 dasharray 而不是旋转两个半圆：一圈只需两个 circle，深浅色与动画都好处理。
 */
function gaugeStyle(pct: number): Record<string, string> {
  const circumference = 2 * Math.PI * 15.5
  const clamped = Math.min(100, Math.max(0, pct))
  const filled = (clamped / 100) * circumference
  return {
    '--gauge-circumference': `${circumference.toFixed(2)}`,
    '--gauge-filled': `${filled.toFixed(2)}`,
    '--gauge-color': usageColor(clamped),
    // 0% 时用平头，避免圆头在起点画出孤点（读成"用了一点点"）
    '--gauge-linecap': clamped === 0 ? 'butt' : 'round',
  }
}

function percent(used: number, total: number): number {
  if (!total) return 0
  return Math.min(100, Math.round((used / total) * 100))
}

function usageColor(pct: number): string {
  if (pct >= 80) return '#dc2626'
  if (pct >= 60) return '#d97706'
  return '#16a34a'
}

function probeLabel(status: string): string {
  switch (status) {
    case 'healthy':
      return '探针正常'
    case 'warning':
      return '探针异常'
    case 'down':
      return '探针离线'
    default:
      return status
  }
}

function probeTheme(status: string) {
  if (status === 'healthy') return 'success'
  if (status === 'warning') return 'warning'
  if (status === 'down') return 'danger'
  return 'default'
}

function formatTime(value: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// 位置型视图下不出容量列；其余（容量型 / 全部）出容量列，由单元格按行判类别渲染。
const columns = computed<PrimaryTableCol<PoolInfo>[]>(() => {
  const base: PrimaryTableCol<PoolInfo>[] = [
    { colKey: 'name', title: '池名称', minWidth: 170 },
    { colKey: 'location', title: '位置（地域 / 可用区）', width: 170 },
    { colKey: 'pool_type', title: '类型', width: 100 },
  ]
  if (!isLocationView.value) {
    base.push(
      { colKey: 'cpu', title: 'CPU（已用/总）', minWidth: 150 },
      { colKey: 'memory', title: '内存（已用/总）', minWidth: 150 },
      { colKey: 'disk', title: '磁盘（已用/总）', minWidth: 150 },
    )
  }
  if (!isEmbedded.value && !isLocationView.value) {
    base.push({ colKey: 'probe', title: '探针', width: 100 })
  }
  base.push(
    { colKey: 'status', title: '状态', width: 90 },
    {
      colKey: 'action',
      title: '操作',
      width: isMobile.value ? 70 : isEmbedded.value ? 90 : 160,
      fixed: 'right' as const,
      align: 'center' as const,
    },
  )
  return base
})

async function loadPools() {
  loading.value = true
  try {
    const data = await getPoolList({
      provider_id: effectiveProviderId.value,
      keyword: filters.keyword || undefined,
      region: filters.region || undefined,
      // 层级视图下类别/告警筛选无意义（父行是区域、要带全子树），后端会忽略这两个参数。
      alert: viewMode.value === 'flat' ? filters.alert || undefined : undefined,
      category: viewMode.value === 'flat' && filters.category !== 'all' ? filters.category : undefined,
      group: viewMode.value === 'group' ? 'region' : undefined,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    poolList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    summary.value = data.summary || { ...emptySummary }
    regionOptions.value = (data.regions || []).map((region) => ({ label: region, value: region }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载资源池列表失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  if (props.providerId !== undefined) return
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providerOptions.value = data.items.map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 提供商下拉加载失败不阻塞列表 */
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadPools()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  pagination.current = current
  pagination.pageSize = pageSize
  mobilePage.current = current
  mobilePage.pageSize = pageSize
  await handlePageChange({ current, pageSize } as never)
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}

function handleSearch() {
  pagination.current = 1
  loadPools()
}

function handleResetFilters() {
  filters.keyword = ''
  filters.provider_id = undefined
  filters.region = ''
  filters.alert = ''
  // 重置回默认的「容量型」视图，而不是「全部」——默认视图才是资源池页的主口径。
  filters.category = 'capacity'
  pagination.current = 1
  loadPools()
}

const detailVisible = ref(false)
const detailPool = ref<PoolInfo | null>(null)

function openDetailDialog(row: PoolInfo) {
  detailPool.value = row
  detailVisible.value = true
}

function handleViewChange() {
  pagination.current = 1
  loadPools()
}

watch(effectiveProviderId, () => {
  pagination.current = 1
  loadPools()
})

onMounted(() => {
  loadProviders()
  loadPools()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, unknown>, row: PoolInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetailDialog(row)
      break
    case 'ops':
      if (row.provider_ops_url) window.open(row.provider_ops_url, '_blank', 'noopener')
      break
    case 'delete':
      removePool(row)
      break
  }
}

/**
 * removePool 删除资源池。
 *
 * 弹窗里必须说清一个容易误解的点：池由平台同步 upsert，**在线渠道的池下次同步会重新出现**。
 * 不说的话运营删完发现又回来了，会以为删除没生效。这个操作真正解决的是
 * 演示/历史脏数据（人造 upstream_id 的假池，永远同步不到、也永远刷不掉）。
 */
function removePool(row: PoolInfo) {
  const isSynced = !!row.last_sync_at
  const dialog = DialogPlugin.confirm({
    header: '删除资源池',
    body: isSynced
      ? `确认删除「${row.name}」？该池来自平台同步，若所属渠道仍在线，下次同步会重新出现（删除主要用于清理脏数据）。`
      : `确认删除「${row.name}」？该池不是同步来的（无同步时间），删除后不会再出现。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deletePool(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        loadPools()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}
// 资源池类型标签按语义区分颜色
function poolTypeTheme(type?: string): 'primary' | 'warning' | 'success' | 'danger' | 'default' {
  const t = String(type || '').toLowerCase()
  if (t.includes('node') || t.includes('host')) return 'primary'
  if (t.includes('storage') || t.includes('存储')) return 'success'
  if (t.includes('memory') || t.includes('内存')) return 'warning'
  if (t.includes('compute') || t.includes('计算')) return 'primary'
  if (t.includes('network') || t.includes('网络')) return 'danger'
  return 'default'
}

// 上游类型 → 中文展示。
//
// 两位来源不同的类型码并存，都要有中文名（否则列表里直接冒出英文原文）：
//   - 真实同步来的：魔方云 node（宿主机）、region（区域）、storage（存储）；
//   - 历史/演示种子里的：compute（通用计算池）、memory（高内存池）——这些是
//     seedUpstreamData 造的示例数据，不是上游真实类型，但不该在页面上显示成英文。
function poolTypeLabel(type?: string): string {
  const t = String(type || '').toLowerCase()
  if (t === 'node' || t === 'host') return '宿主机'
  if (t === 'region' || t === 'zone') return '区域'
  if (t === 'storage') return '存储'
  if (t === 'compute') return '计算池'
  if (t === 'memory') return '内存池'
  if (t === 'cluster') return '集群'
  return type || '—'
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
/* 6 张容量/位置卡片：3 列两行比默认 4 列更均衡 */
.resource-module .stat-grid--pools {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.resource-module .stat-card__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* 标题 + 计数同一行：两者基线与间距对齐，窄卡下允许标题省略、计数保留 */
.resource-module .stat-card__title {
  display: flex;
  align-items: baseline;
  gap: 6px;
  max-width: 100%;
  min-width: 0;
}

.resource-module .stat-card__title .stat-card__label {
  min-width: 0;
}

/* 计数跟在标题后面：同字号但加深、等宽数字，形成"标题 + 读数"的一行层级 */
.resource-module .stat-card__count {
  flex-shrink: 0;
  font-style: normal;
  font-weight: 600;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
  font-size: 12px;
  white-space: nowrap;
}

/*
  环形百分比（用户要求）：容量类卡片右侧一枚小环，替代原来的大号百分数。
  六个卡片视觉重量因此一致 —— 计数卡是大号数字，容量卡是同一枚环，不再"两个大字"抢视线。
*/
.resource-module .stat-gauge {
  position: relative;
  z-index: 1;
  margin-left: auto;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.resource-module .stat-gauge__ring {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  /* 从 12 点方向起画，符合仪表盘直觉 */
  transform: rotate(-90deg);
}

.resource-module .stat-gauge__track {
  fill: none;
  stroke: var(--hs-inset-border, #e5e7eb);
  stroke-width: 3.2;
}

.resource-module .stat-gauge__fill {
  fill: none;
  stroke: var(--gauge-color, #16a34a);
  stroke-width: 3.2;
  /* 0% 时用平头：圆头会在起点画出一个孤点，看着像"用了一点点" */
  stroke-linecap: var(--gauge-linecap, round);
  /* 画多长由 gaugeStyle 按用量算好，写进这两个变量 */
  stroke-dasharray: var(--gauge-filled) var(--gauge-circumference);
  transition: stroke-dasharray 360ms var(--hs-ease-out, ease-out);
}

.resource-module .stat-gauge__num {
  font-style: normal;
  font-size: 12px;
  font-weight: 700;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

@media (max-width: 768px) {
  .resource-module .stat-grid--pools {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.pool-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.pool-name {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.pool-provider {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.loc-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.loc-region {
  font-size: 12px;
  color: var(--color-foreground);
}

.loc-zone {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.spec-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.spec-text {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

/* 「区域合计」标记：提示该行的容量是子池汇总而非区域自身配额。 */
.spec-hint {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

/* 「不适用」而非「—」：区域本身没有容量这个概念，与容量池的 0 值不是一回事。
   用「—」会读成"这里应该有个数但没采到"，用「不适用」才说明是字段本身不存在。 */
.cell-na {
  font-size: 12px;
  color: var(--color-muted-foreground);
  font-style: italic;
}

/* 「无子节点」：区域行有位置但子树为空时的解释性标记。 */
.pool-hint {
  font-size: 11px;
  color: var(--color-muted-foreground);
  border-bottom: 1px dashed var(--color-border);
  cursor: help;
}

.action-gap {
  display: inline-block;
  width: 8px;
}

.probe-hint {
  margin-top: var(--space-md);
}

.probe-detail-time {
  margin-left: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
