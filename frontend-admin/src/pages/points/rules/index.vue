<template>
  <div class="page-body point-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">积分规则</h2>
          <p class="page-header__desc">
            规则可配：按实付金额比例或固定积分计发，支持起算门槛、单笔上限与有效期。停用的规则不参与发放。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadRules">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建规则
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">发放场景</span>
          <t-select v-model="filters.scene" clearable placeholder="全部场景" :options="pointSceneOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="pointRuleStatusOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">规则列表</h3>
        <span class="table-card__meta">共 {{ total }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="ruleList"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
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

        <template #status="{ row }">
          <t-tag :theme="pointRuleStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ pointRuleStatusLabel(row.status) }}
          </t-tag>
        </template>

        <template #remark="{ row }">
          <span class="price-sub">{{ row.remark || '—' }}</span>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <template v-if="isMobile">
              <MobileAction
                :options="buildMobileActionOptions([{ content: '编辑', value: 'edit' }])"
                @select="() => openEdit(row)"
              />
            </template>
            <t-link v-else theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无积分规则，点击右上角「新建规则」开始配置" />
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
      v-model:visible="dialogVisible"
      :header="editingId ? `编辑规则：${form.name}` : '新建积分规则'"
      width="560px"
      :confirm-btn="{ content: editingId ? '保存' : '创建', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSubmit"
      @close="dialogVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-alert
          theme="info"
          message="积分仅用于活动与权益兑换，不能抵扣订单金额、不能提现；同一业务单号只会发放一次。"
          style="margin-bottom: 12px"
        />
        <div class="filter-card__grid">
          <div class="field">
            <span class="field__label">规则名称</span>
            <t-input v-model="form.name" placeholder="如 订单消费赠送" clearable />
          </div>
          <div class="field">
            <span class="field__label">发放场景</span>
            <t-select v-model="form.scene" placeholder="请选择场景" :options="pointSceneOptions" />
          </div>
          <div class="field">
            <span class="field__label">计发方式</span>
            <t-select v-model="form.earn_mode" placeholder="请选择计发方式" :options="pointEarnModeOptions" />
          </div>
          <div class="field" v-if="form.earn_mode === 'rate'">
            <span class="field__label">每 1 元发放积分</span>
            <t-input-number v-model="form.points_per_yuan" :min="0" :precision="4" theme="column" />
          </div>
          <div class="field" v-else>
            <span class="field__label">每笔固定积分</span>
            <t-input-number v-model="form.fixed_points" :min="0" :precision="0" theme="column" />
          </div>
          <div class="field">
            <span class="field__label">起算门槛（元，0=不限）</span>
            <t-input-number v-model="form.min_amount" :min="0" :precision="2" theme="column" />
          </div>
          <div class="field">
            <span class="field__label">单笔上限（分，0=不限）</span>
            <t-input-number v-model="form.max_points_per_order" :min="0" :precision="0" theme="column" />
          </div>
          <div class="field">
            <span class="field__label">有效期（天，0=永久）</span>
            <t-input-number v-model="form.valid_days" :min="0" :precision="0" theme="column" />
          </div>
          <div class="field">
            <span class="field__label">状态</span>
            <t-select v-model="form.status" :options="pointRuleStatusOptions" />
          </div>
          <div class="field">
            <span class="field__label">排序（越大越靠前）</span>
            <t-input-number v-model="form.sort_order" :min="0" :precision="0" theme="column" />
          </div>
        </div>
        <div class="field" style="margin-top: 12px">
          <span class="field__label">备注</span>
          <t-input v-model="form.remark" placeholder="选填，规则说明" clearable />
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon, SearchIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { createPointRule, getPointRules, updatePointRule } from '@/api/point'
import {
  pointEarnModeOptions,
  pointRuleStatusLabel,
  pointRuleStatusOptions,
  pointRuleStatusTheme,
  pointRuleSummary,
  pointSceneLabel,
  pointSceneOptions,
} from '@/pages/points/constants'
import type { PointRuleInfo, PointRuleSaveRequest } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'PointsRules' })

const ruleList = ref<PointRuleInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const total = ref(0)

const filters = reactive<{ scene: string | undefined; status: number | undefined }>({
  scene: undefined,
  status: undefined,
})

const pagination = reactive({ current: 1, pageSize: 20, total: 0, showJumper: true })
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<PointRuleSaveRequest>({
  name: '',
  scene: 'order_purchase',
  earn_mode: 'rate',
  points_per_yuan: 1,
  fixed_points: 0,
  min_amount: 0,
  max_points_per_order: 0,
  valid_days: 0,
  status: 1,
  sort_order: 0,
  remark: '',
})

const columns: PrimaryTableCol<PointRuleInfo>[] = [
  { colKey: 'name', title: '规则', minWidth: 200 },
  { colKey: 'scene', title: '场景', width: 120 },
  { colKey: 'summary', title: '计发口径', minWidth: 280 },
  { colKey: 'status', title: '状态', width: 90, align: 'center' as const },
  { colKey: 'remark', title: '备注', minWidth: 160 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 90, fixed: 'right' as const, align: 'center' as const },
]

async function loadRules() {
  loading.value = true
  try {
    const data = await getPointRules({
      scene: filters.scene,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    ruleList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载积分规则失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadRules()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

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
  loadRules()
}

function handleResetFilters() {
  filters.scene = undefined
  filters.status = undefined
  pagination.current = 1
  loadRules()
}

function resetForm() {
  form.name = ''
  form.scene = 'order_purchase'
  form.earn_mode = 'rate'
  form.points_per_yuan = 1
  form.fixed_points = 0
  form.min_amount = 0
  form.max_points_per_order = 0
  form.valid_days = 0
  form.status = 1
  form.sort_order = 0
  form.remark = ''
}

function openCreate() {
  editingId.value = null
  resetForm()
  dialogVisible.value = true
}

function openEdit(row: PointRuleInfo) {
  editingId.value = row.id
  form.name = row.name
  form.scene = row.scene
  form.earn_mode = row.earn_mode
  form.points_per_yuan = row.points_per_yuan
  form.fixed_points = row.fixed_points
  form.min_amount = row.min_amount
  form.max_points_per_order = row.max_points_per_order
  form.valid_days = row.valid_days
  form.status = row.status
  form.sort_order = row.sort_order
  form.remark = row.remark
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入规则名称')
    return
  }
  if (form.earn_mode === 'rate' && (!form.points_per_yuan || form.points_per_yuan <= 0)) {
    MessagePlugin.warning('按比例计发时需填写大于 0 的每元积分')
    return
  }
  if (form.earn_mode === 'fixed' && (!form.fixed_points || form.fixed_points <= 0)) {
    MessagePlugin.warning('固定计发时需填写大于 0 的积分')
    return
  }
  loading.value = true
  try {
    const payload: PointRuleSaveRequest = { ...form }
    if (editingId.value) {
      await updatePointRule(editingId.value, payload)
      MessagePlugin.success('规则已更新')
    } else {
      await createPointRule(payload)
      MessagePlugin.success('规则已创建')
    }
    dialogVisible.value = false
    loadRules()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadRules)
</script>

<style lang="css">
@import '../shared.css';
</style>
