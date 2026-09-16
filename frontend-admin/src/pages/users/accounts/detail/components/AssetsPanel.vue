<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded"
      class="degraded-tip"
      theme="warning"
      message="实例摘要采集失败，下方为实时查询结果；若同样为空请稍后重试。"
    />

    <div class="table-card__head">
      <h3 class="card-title">云主机资产</h3>
      <span class="table-card__meta">共 {{ total }} 台</span>
    </div>

    <t-table
      row-key="id"
      :data="items"
      :columns="columns"
      :loading="loading"
      size="small"
      hover
      table-layout="fixed"
      cell-empty-content="—"
      :pagination="isMobile ? undefined : pagination"
      @page-change="onPageChange"
    >
      <template #name="{ row }">
        <div>
          <span class="cell-strong">{{ row.name || '—' }}</span>
          <div class="cell-sub">{{ row.instance_id }}</div>
        </div>
      </template>
      <template #specs="{ row }">
        <span class="cell-muted">{{ instanceSpecsText(row.cpu, row.memory, row.disk) }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="instanceStatusTheme(row.status)" variant="light" size="small" shape="round">
          {{ instanceStatusLabel(row.status) }}
        </t-tag>
      </template>
      <template #expire_at="{ row }">
        <span class="time-text">{{ expireText(row.expire_at) }}</span>
      </template>
      <template #op="{ row }">
        <t-link theme="primary" hover="color" @click="goDetail(row.id)">详情</t-link>
      </template>
      <template #empty>
        <t-empty description="该用户名下暂无云主机实例" />
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
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import MobilePagination from '@/components/mobile-pagination/index.vue'
import { getInstanceList } from '@/api/instance'
import {
  expireText,
  instanceSpecsText,
  instanceStatusLabel,
  instanceStatusTheme,
} from '@/pages/users/constants'
import type { InstanceItem, InstanceOpsListQuery } from '@/types/interface'

const props = defineProps<{
  userId: number
  isMobile: boolean
  /** 聚合接口的 instances 段是否降级（采集失败） */
  degraded?: boolean
}>()

const router = useRouter()
const items = ref<InstanceItem[]>([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })
const total = computed(() => pagination.total)

const columns: PrimaryTableCol<InstanceItem>[] = [
  { colKey: 'name', title: '实例', minWidth: 200 },
  { colKey: 'specs', title: '规格', width: 200 },
  { colKey: 'region', title: '地域', width: 140 },
  { colKey: 'public_ip', title: '公网 IP', width: 140 },
  { colKey: 'billing_mode', title: '计费方式', width: 110 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'expire_at', title: '到期时间', width: 130 },
  { colKey: 'op', title: '操作', width: 80, fixed: 'right' },
]

async function load() {
  if (!props.userId) return
  loading.value = true
  try {
    const params: InstanceOpsListQuery = {
      user_id: props.userId,
      page: pagination.current,
      page_size: pagination.pageSize,
    }
    const data = await getInstanceList(params)
    items.value = data.items || []
    pagination.total = data.meta?.total || 0
    mobilePage.total = pagination.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例列表失败')
  } finally {
    loading.value = false
  }
}

function onPageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  mobilePage.current = pageInfo.current
  mobilePage.pageSize = pageInfo.pageSize
  void load()
}

function goMobilePage(target: number) {
  pagination.current = target
  mobilePage.current = target
  void load()
}

function handleMobilePageSizeChange(size: number) {
  pagination.pageSize = size
  pagination.current = 1
  mobilePage.pageSize = size
  mobilePage.current = 1
  void load()
}

function goDetail(id: number) {
  router.push({ path: `/instances/detail/${id}` })
}

// 懒加载：父级在切到本 Tab 时才挂载本组件；userId 变化（切用户）时重查。
watch(() => props.userId, () => {
  pagination.current = 1
  mobilePage.current = 1
  void load()
  }, { immediate: true })
</script>