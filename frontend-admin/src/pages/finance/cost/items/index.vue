<template>
  <div class="page-body finance-module cost-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MoneyIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">成本项配置</h2>
          <p class="page-header__desc">
            母机月费、员工工资、机房带宽、域名证书等固定成本在这里维护，按月计入当月成本；
            一次性支出（如设备采购）选「一次性」并填发生日期
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新增成本项
        </t-button>
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="名称 / 成本对象 / 备注" clearable @enter="handleSearch" />
      </div>
      <div class="field">
        <span class="field__label">分类</span>
        <t-select v-model="filters.category" clearable placeholder="全部分类" :options="categoryOptions" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon><SearchIcon aria-hidden="true" /></template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="summary-bar surface-card">
      <div class="summary-item">
        <span class="summary-item__label">筛选口径·按月计入合计</span>
        <span class="summary-item__value amount-expense">¥{{ formatPrice(monthlyTotal) }}</span>
      </div>
      <div class="summary-item">
        <span class="summary-item__label">成本项条数</span>
        <span class="summary-item__value">{{ total }}</span>
      </div>
      <div class="summary-item summary-item--hint">
        <span class="summary-item__label">说明</span>
        <span class="summary-item__value cell-muted">
          合计为全量口径（不受分页影响）；停用项不计入；一次性项只在发生月计入
        </span>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">成本项列表</h3>
        <span class="table-card__meta">成本总览的「成本项配置」一栏即本表按月合计</span>
      </div>
      <t-table
        row-key="id"
        :data="items"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        cell-empty-content="—"
        :pagination="isMobile ? undefined : pagination"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="cost-name-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span v-if="row.subject" class="cell-muted">{{ row.subject }}</span>
          </div>
        </template>
        <template #category="{ row }">
          <t-tag :theme="categoryTheme(row.category)" variant="light" size="small" shape="round">
            {{ row.category_label || categoryLabel(row.category) }}
          </t-tag>
        </template>
        <template #amount="{ row }">
          <span class="cell-strong">¥{{ formatPrice(row.amount) }}</span>
        </template>
        <template #cycle="{ row }">
          <div class="price-cell">
            <span class="price-main">{{ cycleLabel(row.cycle) }}</span>
            <span class="price-sub">
              {{ row.cycle === 'once' ? `发生 ${row.occurred_on || '—'}` : rangeText(row) }}
            </span>
          </div>
        </template>
        <template #monthly_amount="{ row }">
          <span v-if="row.monthly_amount" class="amount-expense">¥{{ formatPrice(row.monthly_amount) }}</span>
          <span v-else class="cell-muted">—</span>
        </template>
        <template #status="{ row }">
          <t-switch
            :value="row.status === 'active'"
            :loading="statusPending === row.id"
            @change="(value: unknown) => handleToggleStatus(row, Boolean(value))"
          />
        </template>
        <template #remark="{ row }">
          <span class="cell-muted remark-cell">{{ row.remark || '—' }}</span>
        </template>
        <template #op="{ row }">
          <div class="action-cell">
            <t-link theme="primary" @click="openEdit(row)">编辑</t-link>
            <t-link theme="danger" @click="handleDelete(row)">删除</t-link>
          </div>
        </template>
        <template #empty>
          <t-empty description="还没有成本项：加上母机月费、人力与机房带宽，成本口径才算完整">
            <template #action>
              <t-button theme="primary" size="small" @click="openCreate">新增成本项</t-button>
            </template>
          </t-empty>
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
      :header="editingId ? '编辑成本项' : '新增成本项'"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSubmit"
      @close="dialogVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="名称" help="如「华南母机-01 月费」「客服小王 工资」">
          <t-input v-model="form.name" placeholder="成本项名称" clearable />
        </t-form-item>
        <div class="form-row">
          <t-form-item label="分类">
            <t-select v-model="form.category" :options="categoryOptions" placeholder="选择分类" />
          </t-form-item>
          <t-form-item label="金额（元）">
            <t-input-number v-model="form.amount" :min="0" :precision="2" theme="column" style="width: 100%" />
          </t-form-item>
        </div>
        <t-form-item label="计费周期">
          <t-radio-group v-model="form.cycle" variant="default-filled">
            <t-radio-button value="monthly">按月</t-radio-button>
            <t-radio-button value="once">一次性</t-radio-button>
          </t-radio-group>
        </t-form-item>
        <t-form-item v-if="form.cycle === 'once'" label="发生日期" help="一次性支出只计入该日期所在自然月">
          <t-date-picker v-model="form.occurredOn" format="YYYY-MM-DD" placeholder="选择发生日期" style="width: 100%" />
        </t-form-item>
        <t-form-item v-else label="生效区间" help="留空结束日期表示长期有效；区间与自然月相交即整月计入">
          <t-date-range-picker
            v-model="form.effectiveRange"
            separator="~"
            placeholder="开始日期 ~ 结束日期（可只填开始）"
            style="width: 100%"
          />
        </t-form-item>
        <t-form-item label="成本对象" help="母机名 / 员工 / 线路，用于区分同名成本项">
          <t-input v-model="form.subject" placeholder="可选" clearable />
        </t-form-item>
        <t-form-item label="备注">
          <t-textarea v-model="form.remark" placeholder="可选，如合同号、付款方式" :autosize="{ minRows: 2, maxRows: 3 }" />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, MoneyIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import FilterCard from '@/components/filter-card/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { createCostItem, deleteCostItem, getCostItems, updateCostItem } from '@/api/cost'
import { useIsMobile } from '@/composables/useIsMobile'
import { formatPrice, toDateString } from '@/pages/finance/constants'
import type { CostItemInfo, CostItemRequest } from '@/types/interface'

defineOptions({ name: 'FinanceCostItems' })

/** 分类与后端 model.CategoryLabels 同名同值（后端也回 category_label，这里只作兜底）。 */
const categoryOptions = [
  { label: '自营宿主机', value: 'self_hosted' },
  { label: '上游运营', value: 'upstream_ops' },
  { label: '人力成本', value: 'labor' },
  { label: '基础设施', value: 'infra' },
  { label: '其他', value: 'other' },
]

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

const { isMobile } = useIsMobile()

const loading = ref(false)
const saving = ref(false)
const items = ref<CostItemInfo[]>([])
const total = ref(0)
const monthlyTotal = ref(0)
const statusPending = ref(0)

const filters = reactive<{ keyword: string; category?: string; status?: string }>({
  keyword: '',
  category: undefined,
  status: undefined,
})

const page = reactive({ current: 1, pageSize: 20 })
const mobilePage = computed(() => ({ current: page.current, pageSize: page.pageSize, total: total.value }))
const pagination = computed(() => ({
  current: page.current,
  pageSize: page.pageSize,
  total: total.value,
  showJumper: true,
  pageSizeOptions: [10, 20, 50],
}))

const columns: PrimaryTableCol<CostItemInfo>[] = [
  { colKey: 'name', title: '成本项', minWidth: 180 },
  { colKey: 'category', title: '分类', width: 120 },
  { colKey: 'amount', title: '金额', width: 120 },
  { colKey: 'cycle', title: '周期 / 生效', width: 190 },
  { colKey: 'monthly_amount', title: '当月计入', width: 110 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'remark', title: '备注', minWidth: 140 },
  { colKey: 'op', title: '操作', width: 120, fixed: 'right' },
]

const dialogVisible = ref(false)
const editingId = ref(0)
const form = reactive<{
  name: string
  category: string
  amount: number
  cycle: string
  occurredOn: string
  effectiveRange: string[]
  subject: string
  remark: string
}>({
  name: '',
  category: 'self_hosted',
  amount: 0,
  cycle: 'monthly',
  occurredOn: '',
  effectiveRange: [],
  subject: '',
  remark: '',
})

function cycleLabel(cycle: string): string {
  return cycle === 'once' ? '一次性' : '按月'
}

function categoryLabel(category: string): string {
  return categoryOptions.find((item) => item.value === category)?.label || category || '—'
}

function categoryTheme(category: string): string {
  switch (category) {
    case 'self_hosted':
      return 'primary'
    case 'upstream_ops':
      return 'warning'
    case 'labor':
      return 'success'
    case 'infra':
      return 'default'
    default:
      return 'default'
  }
}

function rangeText(row: CostItemInfo): string {
  if (!row.effective_from) return '—'
  return row.effective_to ? `${row.effective_from} ~ ${row.effective_to}` : `${row.effective_from} 起长期`
}

async function load() {
  loading.value = true
  try {
    const data = await getCostItems({
      keyword: filters.keyword || undefined,
      category: filters.category,
      status: filters.status,
      page: page.current,
      page_size: page.pageSize,
    })
    items.value = data.items
    total.value = data.meta.total
    monthlyTotal.value = data.monthly_total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载成本项失败')
    items.value = []
    total.value = 0
    monthlyTotal.value = 0
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.current = 1
  void load()
}

function handleResetFilters() {
  filters.keyword = ''
  filters.category = undefined
  filters.status = undefined
  handleSearch()
}

function handlePageChange(pageInfo: PageInfo) {
  page.current = pageInfo.current
  page.pageSize = pageInfo.pageSize
  void load()
}

function goMobilePage(current: number) {
  page.current = current
  void load()
}

function handleMobilePageSizeChange(pageSize: number) {
  page.pageSize = pageSize
  page.current = 1
  void load()
}

function openCreate() {
  editingId.value = 0
  form.name = ''
  form.category = 'self_hosted'
  form.amount = 0
  form.cycle = 'monthly'
  form.occurredOn = ''
  form.effectiveRange = []
  form.subject = ''
  form.remark = ''
  dialogVisible.value = true
}

function openEdit(row: CostItemInfo) {
  editingId.value = row.id
  form.name = row.name
  form.category = row.category
  form.amount = row.amount
  form.cycle = row.cycle || 'monthly'
  form.occurredOn = row.occurred_on || ''
  form.effectiveRange = row.effective_from ? [row.effective_from, row.effective_to || ''] : []
  form.subject = row.subject
  form.remark = row.remark
  dialogVisible.value = true
}

/** 把表单与既有行组装成后端要求的全量请求体（PUT 为整条替换）。 */
function buildPayload(status?: string): CostItemRequest {
  const payload: CostItemRequest = {
    name: form.name.trim(),
    category: form.category,
    amount: Number(form.amount || 0),
    cycle: form.cycle,
    subject: form.subject.trim(),
    remark: form.remark.trim(),
  }
  if (status) payload.status = status
  if (form.cycle === 'once') {
    payload.occurred_on = toDateString(form.occurredOn)
  } else {
    const [from, to] = form.effectiveRange || []
    payload.effective_from = toDateString(from)
    payload.effective_to = toDateString(to)
  }
  return payload
}

async function handleSubmit() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请填写成本项名称')
    return
  }
  if (form.cycle === 'once' && !toDateString(form.occurredOn)) {
    MessagePlugin.warning('一次性成本需要选择发生日期')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateCostItem(editingId.value, buildPayload())
      MessagePlugin.success('成本项已更新')
    } else {
      await createCostItem(buildPayload())
      MessagePlugin.success('成本项已添加，从生效月起计入成本')
    }
    dialogVisible.value = false
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleToggleStatus(row: CostItemInfo, active: boolean) {
  statusPending.value = row.id
  const nextStatus = active ? 'active' : 'disabled'
  try {
    await updateCostItem(row.id, {
      name: row.name,
      category: row.category,
      amount: row.amount,
      cycle: row.cycle,
      occurred_on: row.occurred_on,
      effective_from: row.effective_from,
      effective_to: row.effective_to,
      subject: row.subject,
      remark: row.remark,
      status: nextStatus,
    })
    row.status = nextStatus
    MessagePlugin.success(active ? '已启用，本月起计入成本' : '已停用，不再计入成本')
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '状态更新失败')
  } finally {
    statusPending.value = 0
  }
}

function handleDelete(row: CostItemInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除成本项',
    body: `确认删除「${row.name}」？删除后该成本项不再计入任何月份的成本。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteCostItem(row.id)
        MessagePlugin.success('成本项已删除')
        dialog.hide()
        await load()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

onMounted(load)
</script>

<style lang="css">
@import '../../shared.css';

.finance-module.cost-module .cost-name-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.finance-module.cost-module .remark-cell {
  display: inline-block;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}

.finance-module.cost-module .summary-item--hint {
  flex: 1;
  min-width: 220px;
  text-align: right;
}

.finance-module.cost-module .form-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}
</style>
