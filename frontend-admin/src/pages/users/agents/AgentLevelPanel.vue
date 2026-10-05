<template>
  <div class="agent-panel">
    <!-- 折扣矩阵：行 = 目标（全站/分类），列 = 代理等级 -->
    <section class="matrix-card surface-card">
      <p class="panel-desc">
        代理拿货折扣的唯一来源：行是折扣目标（全站兜底 / 商品分类），列是代理等级（按权重降序，最左最优先）。
        <strong>点击任意单元格</strong>即可设置该等级在该目标上的折扣率。
      </p>

      <div class="card-head">
        <h3 class="card-title">折扣矩阵</h3>
        <span class="card-hint">
          单元格为折扣率（0.85 = 八五折，<strong>越小越优惠</strong>）；空 = 未配置（不打折）。
          括号内为毛利率，低于 0 表示亏本，后端会拒绝保存。
        </span>
      </div>

      <t-loading :loading="matrixLoading" size="small">
        <div v-if="matrix.columns.length === 0" class="empty-hint">
          还没有启用中的代理等级，先在下方创建等级后再配折扣。
        </div>
        <div v-else class="matrix-scroll">
          <table class="matrix-table">
            <thead>
              <tr>
                <th class="matrix-table__corner">目标 / 等级</th>
                <th v-for="col in matrix.columns" :key="col.agent_level_id" class="matrix-table__col">
                  <div class="col-head">
                    <span class="col-head__name">{{ col.name }}</span>
                    <span class="col-head__meta">权重 {{ col.weight }}</span>
                  </div>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in matrix.rows" :key="rowKey(row)">
                <td class="matrix-table__row-head">
                  <span class="row-head__name">{{ row.target_name }}</span>
                  <span v-if="row.cost_rate > 0" class="row-head__cost">
                    成本 {{ rateText(row.cost_rate) }}
                  </span>
                </td>
                <td
                  v-for="cell in row.cells"
                  :key="cell.agent_level_id"
                  class="matrix-table__cell"
                  :class="{ 'is-editable': canEditCells }"
                  :title="canEditCells ? '点击设置该等级在此目标的折扣率' : ''"
                  @click="openCellEdit(row, cell)"
                >
                  <div v-if="cell.configured" class="cell-value">
                    <span class="cell-value__rate">{{ rateText(cell.discount_rate) }}</span>
                    <span
                      v-if="row.cost_rate > 0"
                      class="cell-value__margin"
                      :class="{ 'is-loss': cell.discount_rate < row.cost_rate }"
                    >
                      {{ marginText(cell.discount_rate, row.cost_rate) }}
                    </span>
                  </div>
                  <span v-else class="cell-empty" :class="{ 'cell-empty--action': canEditCells }">
                    {{ canEditCells ? '＋ 设置' : '—' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </t-loading>

      <!-- 阶梯填充：锚点 + 步长 → 一键写入某目标下的所有等级 -->
      <div class="ladder">
        <div class="ladder__head">
          <h4 class="ladder__title">阶梯填充</h4>
          <span class="ladder__hint">按「最优等级折扣 + 每级递增」一次配好一个目标的全部等级（全站 / 分类 / 单个商品）；商品阶梯优先于分类和全站</span>
        </div>
        <div class="ladder__form">
          <div class="ladder__field">
            <span class="ladder__label">目标</span>
            <t-select v-model="ladder.target_type" :options="targetTypeOptions" class="ladder__control" />
          </div>
          <div v-if="ladder.target_type === 'category'" class="ladder__field">
            <span class="ladder__label">分类</span>
            <t-select
              v-model="ladder.target_id"
              :options="categoryOptions"
              filterable
              placeholder="选择分类"
              class="ladder__control"
            />
          </div>
          <div v-if="ladder.target_type === 'product'" class="ladder__field">
            <span class="ladder__label">商品</span>
            <t-select
              v-model="ladder.target_id"
              :options="productOptions"
              filterable
              placeholder="选择商品（在售）"
              class="ladder__control ladder__control--wide"
            />
          </div>
          <div class="ladder__field">
            <span class="ladder__label">最优等级折扣率</span>
            <t-input-number
              v-model="ladder.anchor_rate"
              :min="0"
              :max="1"
              :step="0.01"
              :precision="2"
              theme="normal"
              class="ladder__control"
              @change="refreshPreview"
            />
          </div>
          <div class="ladder__field">
            <span class="ladder__label">每级递增</span>
            <t-input-number
              v-model="ladder.step"
              :min="0"
              :max="1"
              :step="0.01"
              :precision="2"
              theme="normal"
              class="ladder__control"
              @change="refreshPreview"
            />
          </div>
          <t-button
            v-permission="'agent_level:update'"
            class="ladder__apply"
            theme="primary"
            variant="outline"
            :loading="applying"
            @click="handleApplyLadder"
          >
            应用到该目标
          </t-button>
        </div>
        <div v-if="ladderPreview.length" class="ladder__preview">
          <span
            v-for="cell in ladderPreview"
            :key="cell.agent_level_id"
            class="ladder__chip"
            :class="{ 'is-loss': !cell.feasible }"
          >
            {{ cell.name }} {{ rateText(cell.discount_rate) }}
          </span>
        </div>
        <ul v-if="ladderWarnings.length" class="ladder__warnings">
          <li v-for="warning in ladderWarnings" :key="warning">{{ warning }}</li>
        </ul>
      </div>
    </section>

    <section class="toolbar surface-card">
      <div class="toolbar__grid">
        <div class="toolbar-field">
          <span class="toolbar-field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="搜索等级名称 / 编码" @enter="handleSearch" />
        </div>
        <div class="toolbar-field">
          <span class="toolbar-field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
      <div class="toolbar__actions">
        <t-space>
          <t-button theme="primary" @click="handleSearch">查询</t-button>
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-panel surface-card">
      <t-table
        row-key="id"
        :data="tableData"
        :columns="columns"
        :loading="loading"
        :pagination="isMobile ? undefined : pagination"
        cell-empty-content="—"
        @page-change="handlePageChange"
      >
        <template #name="{ row }">
          <div class="primary-cell">
            <strong>{{ row.name }}</strong>
            <span class="muted">{{ row.code }}</span>
          </div>
        </template>
        <template #fallback="{ row }">
          <span v-if="fallbackRateOf(row) !== null" class="cell-fallback">{{ rateText(fallbackRateOf(row)!) }}</span>
          <span v-else class="muted">未配置</span>
        </template>
        <template #discount_count="{ row }">
          <span>{{ row.discount_count }} 条</span>
        </template>
        <template #member_count="{ row }">
          <span>{{ row.member_count }} 人</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
        <template #operation="{ row }">
          <MobileAction
            v-if="isMobile"
            :options="buildMobileActionOptions([
              { content: '编辑', value: 'edit', hidden: () => !has('agent_level:update') },
              { content: '删除', value: 'delete', hidden: () => !has('agent_level:delete'), theme: 'error' },
            ])"
            @select="(value) => handleMobileAction(value, row)"
          />
          <t-space v-else size="8px">
            <t-link v-permission="'agent_level:update'" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link v-permission="'agent_level:delete'" theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
          </t-space>
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
      :header="editing ? '编辑代理等级' : '新建代理等级'"
      width="760px"
      :confirm-loading="submitting"
      @confirm="handleSubmit"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="等级名称" name="name">
            <t-input v-model="form.name" placeholder="如：核心代理" />
          </t-form-item>
          <t-form-item label="等级编码" name="code">
            <t-input v-model="form.code" placeholder="如：core_agent" />
          </t-form-item>
          <t-form-item label="权重" name="weight">
            <div class="weight-field">
              <t-input-number v-model="form.weight" :min="0" :step="1" theme="normal" />
              <span class="field-hint">数值越大越优先，折扣最优（如核心代理 20 &gt; 普通代理 10）</span>
            </div>
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
        </div>
        <t-form-item label="说明" name="description">
          <t-input v-model="form.description" />
        </t-form-item>

        <t-form-item label="折扣配置" name="discounts">
          <div class="discount-editor">
            <p class="discount-editor__hint">
              折扣率 0.85 = 八五折（越小越优惠），留空或 0 表示未配置（该目标不打折）。
              命中优先级：商品 &gt; 分类 &gt; 全站兜底。下拉里标「已配置」的是本等级其它行已占用的目标，避免重复添加。
            </p>

            <div class="discount-editor__batch">
              <t-select
                v-model="batchProductIds"
                :options="batchProductOptions"
                multiple
                filterable
                clearable
                placeholder="批量添加商品折扣：可多选（仅列在售且未添加的商品）"
                class="discount-editor__batch-select"
              />
              <t-button
                variant="outline"
                size="small"
                :disabled="batchProductIds.length === 0"
                @click="addBatchProducts"
              >
                添加为折扣项{{ batchProductIds.length ? `（${batchProductIds.length}）` : '' }}
              </t-button>
            </div>

            <div v-for="(item, index) in discountRows" :key="index" class="discount-editor__row">
              <t-select
                v-model="item.target_type"
                :options="discountTargetOptions"
                class="discount-editor__type"
                @change="() => onTargetTypeChange(item)"
              />
              <template v-if="item.target_type === 'category'">
                <t-select
                  v-model="item.target_id"
                  :options="categoryRowOptions(index, item)"
                  filterable
                  placeholder="选择分类"
                  class="discount-editor__target"
                />
              </template>
              <template v-else-if="item.target_type === 'product'">
                <t-select
                  v-model="item.target_id"
                  :options="productRowOptions(index, item)"
                  filterable
                  placeholder="选择商品"
                  class="discount-editor__target"
                />
              </template>
              <span v-else class="discount-editor__target discount-editor__target--static">全站兜底</span>
              <t-input-number
                v-model="item.discount_rate"
                :min="0"
                :max="1"
                :step="0.01"
                :precision="2"
                theme="normal"
                placeholder="折扣率"
                class="discount-editor__value"
              />
              <span class="discount-editor__margin">{{ discountMarginText(item) }}</span>
              <t-link theme="danger" hover="color" @click="removeDiscountRow(index)">删除</t-link>
            </div>
            <t-button variant="outline" size="small" @click="addDiscountRow">添加单条折扣项</t-button>
          </div>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 单格折扣编辑：矩阵上点任意格子直接改这一格 -->
    <t-dialog
      v-model:visible="cellDialog.visible"
      :header="'设置折扣 · ' + cellDialog.level_name"
      width="440px"
      :confirm-btn="{ content: '保存', loading: cellDialog.saving }"
      :cancel-btn="{ content: cellDialog.rate > 0 ? '取消' : '关闭' }"
      @confirm="handleCellSave"
    >
      <t-form label-align="top">
        <t-form-item label="折扣目标">
          <t-input :value="cellDialog.target_name" readonly disabled />
        </t-form-item>
        <t-form-item label="折扣率">
          <t-input-number
            v-model="cellDialog.rate"
            :min="0"
            :max="1"
            :step="0.01"
            :precision="2"
            theme="normal"
            style="width: 100%"
          />
          <span class="field-hint">
            0.85 = 八五折，<strong>越小越优惠</strong>；填 0 表示清除该格（该目标不打折）。{{ cellCostHint }}
          </span>
        </t-form-item>
        <p v-if="cellMarginPreview" class="cell-margin-preview" :class="{ 'is-loss': cellMarginLoss }">
          {{ cellMarginPreview }}
        </p>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  applyAgentLadder,
  createAgentLevel,
  deleteAgentLevel,
  getAgentLevelList,
  getAgentMatrix,
  previewAgentLadder,
  updateAgentCell,
  updateAgentLevel,
  type AgentDiscountItem,
  type AgentDiscountTargetType,
  type AgentLevelInfo,
  type AgentLevelListQuery,
  type AgentLevelRequest,
  type AgentLadderPreviewCell,
  type AgentMatrixResponse,
  type AgentMatrixRow,
} from '@/api/agent-level'
import { getProductCategoryList, getProductList } from '@/api/product'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'AgentLevelPanel' })

const { isMobile } = useIsMobile()
const { has } = usePermission()

const loading = ref(false)
const matrixLoading = ref(false)
const submitting = ref(false)
const applying = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const editingId = ref<number | null>(null)

const tableData = ref<AgentLevelInfo[]>([])
const matrix = ref<AgentMatrixResponse>({ columns: [], rows: [] })
const filters = reactive<AgentLevelListQuery>({ page: 1, page_size: 10, keyword: '', status: '' })
const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50, 100],
})
const mobilePage = reactive({ current: 1, pageSize: 10, total: 0 })

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '禁用', value: 'disabled' },
]

const targetTypeOptions = [
  { label: '全站兜底', value: 'all' },
  { label: '按分类', value: 'category' },
  { label: '单个商品', value: 'product' },
]

const discountTargetOptions = [
  { label: '全站兜底', value: 'all' },
  { label: '分类', value: 'category' },
  { label: '商品（例外）', value: 'product' },
]

const columns = computed<PrimaryTableCol<AgentLevelInfo>[]>(() => [
  { colKey: 'name', title: '等级信息', minWidth: 180 },
  { colKey: 'weight', title: '权重', width: 90 },
  { colKey: 'fallback', title: '全站兜底折扣', width: 140 },
  { colKey: 'discount_count', title: '折扣项', width: 90 },
  { colKey: 'member_count', title: '成员数', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'description', title: '说明', minWidth: 160 },
  { colKey: 'operation', title: '操作', width: isMobile.value ? 70 : 130, fixed: 'right' },
])

const formRef = ref<FormInstanceFunctions>()

// 折扣编辑器：按 (target_type, target_id) 去重展示；全站项的 target_id 固定 0。
const discountRows = ref<AgentDiscountItem[]>([])

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入等级名称', type: 'error' }],
  code: [{ required: true, message: '请输入等级编码', type: 'error' }],
  status: [{ required: true, message: '请选择状态', type: 'error' }],
}

// —— 阶梯填充状态 ——
const ladder = reactive({
  target_type: 'all' as AgentDiscountTargetType,
  target_id: 0,
  anchor_rate: 0.7,
  step: 0.05,
})
const ladderPreview = ref<AgentLadderPreviewCell[]>([])
const ladderWarnings = ref<string[]>([])

// —— 分类 / 商品下拉（真实数据，不造模拟项）——
const categoryOptions = ref<{ label: string; value: number }[]>([])
const productOptions = ref<{ label: string; value: number }[]>([])

interface CategoryNode {
  id: number
  name: string
  status?: number
  children?: unknown[]
}

async function loadCategoryOptions() {
  try {
    const data = await getProductCategoryList()
    const flat: { label: string; value: number }[] = []
    // 只列启用中的分类：给停用分类配折扣不会生效（矩阵不列、算价不命中），
    // 列出来只会让运营配一个「看起来配了、实际没用」的折扣。
    const walk = (items: CategoryNode[], depth: number) => {
      for (const item of items) {
        if (item.status !== undefined && item.status !== 1) continue
        // 子分类用可见的树形前缀，纯空格缩进在下拉里几乎看不出来。
        const prefix = depth === 0 ? '' : `${'　'.repeat(depth - 1)}└ `
        flat.push({ label: `${prefix}${item.name}`, value: item.id })
        if (Array.isArray(item.children) && item.children.length) {
          walk(item.children as CategoryNode[], depth + 1)
        }
      }
    }
    walk((data.items || []) as unknown as CategoryNode[], 0)
    categoryOptions.value = flat
  } catch {
    categoryOptions.value = []
  }
}

async function loadProductOptions() {
  try {
    // 只列在售商品：下架商品配了折扣也不会再有订单命中；
    // 取前 200 条足够覆盖当前规模，下拉可搜索。
    const data = await getProductList({ page: 1, page_size: 200, status: 1 } as never)
    productOptions.value = (data.items || []).map((item: { id: number; name: string }) => ({
      label: item.name,
      value: item.id,
    }))
  } catch {
    productOptions.value = []
  }
}

function rateText(rate: number): string {
  // 0.85 → 「0.85（八五折）」；中文折数的口语表达比裸小数好读。
  const discount = (rate * 10).toFixed(1).replace(/\.0$/, '')
  return `${rate.toFixed(2)}（${discount}折）`
}

function marginText(rate: number, cost: number): string {
  if (cost <= 0) return ''
  const margin = (rate - cost) * 100
  return `毛利 ${margin >= 0 ? '' : '-'}${Math.abs(margin).toFixed(1)}%`
}

function discountMarginText(item: AgentDiscountItem): string {
  const cost = costRateOfTarget(item.target_type, item.target_id)
  if (cost <= 0 || !item.discount_rate) return ''
  return marginText(item.discount_rate, cost)
}

/** 取目标的成本率：分类取矩阵行里的 cost_rate；商品行后端未下发成本，返回 0。 */
function costRateOfTarget(targetType: string, targetId: number): number {
  if (targetType !== 'category') return 0
  const row = matrix.value.rows.find((r) => r.target_type === 'category' && r.target_id === targetId)
  return row?.cost_rate || 0
}

/** 列表里的「全站兜底折扣」：从 matrix 里取该等级 all 行的值。 */
function fallbackRateOf(row: AgentLevelInfo): number | null {
  const allRow = matrix.value.rows.find((r) => r.target_type === 'all')
  const cell = allRow?.cells.find((c) => c.agent_level_id === row.id)
  if (!cell || !cell.configured) return null
  return cell.discount_rate
}

function rowKey(row: { target_type: string; target_id: number }): string {
  return `${row.target_type}:${row.target_id}`
}

async function loadMatrix() {
  matrixLoading.value = true
  try {
    const data = await getAgentMatrix()
    matrix.value = data
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载折扣矩阵失败')
  } finally {
    matrixLoading.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const data = await getAgentLevelList(filters)
    tableData.value = data.items || []
    pagination.current = data.meta.page
    pagination.pageSize = data.meta.page_size
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载代理等级失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  filters.page = 1
  pagination.current = 1
  void loadData()
}

function handleReset() {
  Object.assign(filters, { page: 1, page_size: 10, keyword: '', status: '' })
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  filters.page = pageInfo.current
  filters.page_size = pageInfo.pageSize
  void loadData()
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

function emptyForm(): AgentLevelRequest {
  return { name: '', code: '', weight: 0, status: 'active', description: '' }
}

const form = reactive<AgentLevelRequest>(emptyForm())

function openCreate() {
  editing.value = false
  editingId.value = null
  Object.assign(form, emptyForm())
  discountRows.value = [{ target_type: 'all', target_id: 0, discount_rate: 0 }]
  dialogVisible.value = true
}

function openEdit(row: AgentLevelInfo) {
  editing.value = true
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    code: row.code,
    weight: row.weight,
    status: row.status,
    description: row.description || '',
  })
  discountRows.value = (row.discounts || []).map((item) => ({ ...item }))
  if (discountRows.value.length === 0) {
    discountRows.value = [{ target_type: 'all', target_id: 0, discount_rate: 0 }]
  }
  dialogVisible.value = true
}

function addDiscountRow() {
  discountRows.value.push({ target_type: 'category', target_id: 0, discount_rate: 0 })
}

// —— 批量添加商品 + 已选标记 —
const batchProductIds = ref<number[]>([])

const batchProductOptions = computed(() =>
  productOptions.value.filter((option) => !targetUsedByOther('product', option.value, -1)),
)

/** 目标是否已被其它行占用（index=-1 表示「任意行」）。重复目标后端会静默覆盖，
 *  界面上必须提前拦住，否则运营以为配了两条，实际只有后一条生效。 */
function targetUsedByOther(targetType: string, targetId: number, index: number): boolean {
  return discountRows.value.some(
    (row, i) =>
      i !== index &&
      row.target_type === targetType &&
      row.target_id === targetId &&
      (targetId > 0 || targetType === 'all'),
  )
}

/** 行内分类下拉：已占用的分类标记「已配置」并禁用；行自身当前值不在启用列表时补进去（停用分类回显）。 */
function categoryRowOptions(index: number, item: AgentDiscountItem) {
  const options = categoryOptions.value.map((option) => {
    if (!targetUsedByOther('category', option.value, index)) return option
    return { ...option, disabled: true, label: `${option.label}（已配置）` }
  })
  const current = item.target_id
  if (current > 0 && !options.some((option) => option.value === current)) {
    options.push({
      label: `${item.target_name || `分类 #${current}`}（已停用）`,
      value: current,
      disabled: true,
    })
  }
  return options
}

/** 行内商品下拉：同上，另把已下架但已配置的商品回显出来。 */
function productRowOptions(index: number, item: AgentDiscountItem) {
  const options = productOptions.value.map((option) => {
    if (!targetUsedByOther('product', option.value, index)) return option
    return { ...option, disabled: true, label: `${option.label}（已配置）` }
  })
  const current = item.target_id
  if (current > 0 && !options.some((option) => option.value === current)) {
    options.push({
      label: `${item.target_name || `商品 #${current}`}（已下架）`,
      value: current,
      disabled: true,
    })
  }
  return options
}

/** 批量添加：选中的每个商品生成一行，折扣率沿用最近一条商品行的值（方便一次配同价）。 */
function addBatchProducts() {
  const ids = batchProductIds.value.filter(
    (id) => !targetUsedByOther('product', id, -1),
  )
  if (ids.length === 0) return
  const lastProductRate = [...discountRows.value]
    .reverse()
    .find((row) => row.target_type === 'product')?.discount_rate
  for (const id of ids) {
    const name = productOptions.value.find((option) => option.value === id)?.label || `商品 #${id}`
    discountRows.value.push({
      target_type: 'product',
      target_id: id,
      target_name: name,
      discount_rate: lastProductRate ?? 0,
    })
  }
  batchProductIds.value = []
}

function removeDiscountRow(index: number) {
  discountRows.value.splice(index, 1)
}

function onTargetTypeChange(item: AgentDiscountItem) {
  // 切换目标类型后原 target_id 不再有意义，必须清零，否则会把折扣挂到错误的商品/分类上。
  item.target_id = 0
}

async function handleSubmit() {
  const result = await formRef.value?.validate()
  if (result !== true) return
  // 前端先做一次同后端一致的拦截：省一次往返，并把原因指到具体那一行。
  for (const item of discountRows.value) {
    if (!item.discount_rate) continue
    if (item.discount_rate > 1) {
      MessagePlugin.error('折扣率不能大于 1')
      return
    }
    if (item.target_type === 'category' && !item.target_id) {
      MessagePlugin.error('请选择折扣对应的分类')
      return
    }
    if (item.target_type === 'product' && !item.target_id) {
      MessagePlugin.error('请选择折扣对应的商品')
      return
    }
    const cost = costRateOfTarget(item.target_type, item.target_id)
    if (cost > 0 && item.discount_rate < cost) {
      MessagePlugin.error(`折扣率低于该分类成本率（${rateText(cost)}），会亏本，已阻止保存`)
      return
    }
  }
  submitting.value = true
  try {
    const payload: AgentLevelRequest = {
      ...form,
      discounts: discountRows.value.filter((item) => item.target_type === 'all' || item.target_id > 0),
    }
    if (editing.value && editingId.value != null) {
      await updateAgentLevel(editingId.value, payload)
      MessagePlugin.success('代理等级已更新')
    } else {
      await createAgentLevel(payload)
      MessagePlugin.success('代理等级已创建')
    }
    dialogVisible.value = false
    await Promise.all([loadData(), loadMatrix()])
    await refreshPreview()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

function handleDelete(row: AgentLevelInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除代理等级',
    body: `确认删除代理等级「${row.name}」？该等级下若仍有用户，删除会被拒绝，请先把这些用户改为其他等级或取消代理身份。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteAgentLevel(row.id)
        MessagePlugin.success('已删除')
        dialog.destroy()
        await Promise.all([loadData(), loadMatrix()])
      } catch (error) {
        MessagePlugin.error((error as Error)?.message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

async function refreshPreview() {
  if (matrix.value.columns.length === 0) {
    ladderPreview.value = []
    ladderWarnings.value = []
    return
  }
  try {
    // 成本基准由服务端按目标解析（商品级 = cost_price÷price），预览与保存同一条口径，
    // 避免「预览说没事、保存被拦」。
    const data = await previewAgentLadder({
      anchor_rate: ladder.anchor_rate,
      step: ladder.step,
      target_type: ladder.target_type,
      target_id: ladder.target_type === 'all' ? 0 : ladder.target_id,
    })
    ladderPreview.value = data.cells || []
    ladderWarnings.value = data.warnings || []
  } catch (error) {
    ladderPreview.value = []
    ladderWarnings.value = [(error as Error)?.message || '预览失败']
  }
}

async function handleApplyLadder() {
  if (ladder.target_type === 'category' && !ladder.target_id) {
    MessagePlugin.error('请先选择分类')
    return
  }
  if (ladder.target_type === 'product' && !ladder.target_id) {
    MessagePlugin.error('请先选择商品')
    return
  }
  applying.value = true
  try {
    const data = await applyAgentLadder({
      target_type: ladder.target_type,
      target_id: ladder.target_type === 'all' ? 0 : ladder.target_id,
      anchor_rate: ladder.anchor_rate,
      step: ladder.step,
    })
    matrix.value = data
    MessagePlugin.success('折扣已按阶梯写入')
    await Promise.all([loadData(), refreshPreview()])
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '应用失败')
  } finally {
    applying.value = false
  }
}

// —— 矩阵单格编辑 ——
const canEditCells = computed(() => has('agent_level:update'))

const cellDialog = reactive({
  visible: false,
  saving: false,
  agent_level_id: 0,
  level_name: '',
  target_type: 'all' as AgentDiscountTargetType,
  target_id: 0,
  target_name: '',
  cost_rate: 0,
  rate: 0,
})

const cellCostHint = computed(() =>
  cellDialog.cost_rate > 0 ? `该分类成本率 ${rateText(cellDialog.cost_rate)}，低于它会被拒绝。` : '',
)
const cellMarginPreview = computed(() => {
  if (!cellDialog.rate || cellDialog.cost_rate <= 0) return ''
  return marginText(cellDialog.rate, cellDialog.cost_rate)
})
const cellMarginLoss = computed(
  () => cellDialog.rate > 0 && cellDialog.cost_rate > 0 && cellDialog.rate < cellDialog.cost_rate,
)

function openCellEdit(row: AgentMatrixRow, cell: { agent_level_id: number; discount_rate: number; configured: boolean }) {
  if (!canEditCells.value) return
  const col = matrix.value.columns.find((item) => item.agent_level_id === cell.agent_level_id)
  cellDialog.agent_level_id = cell.agent_level_id
  cellDialog.level_name = col?.name || `等级 #${cell.agent_level_id}`
  cellDialog.target_type = row.target_type
  cellDialog.target_id = row.target_id
  cellDialog.target_name = row.target_type === 'all' ? '全站兜底' : row.target_name || `分类 #${row.target_id}`
  cellDialog.cost_rate = row.cost_rate || 0
  cellDialog.rate = cell.configured ? cell.discount_rate : 0
  cellDialog.visible = true
}

async function handleCellSave() {
  const rate = cellDialog.rate || 0
  if (rate > 1) {
    MessagePlugin.error('折扣率不能大于 1')
    return
  }
  if (rate > 0 && cellDialog.cost_rate > 0 && rate < cellDialog.cost_rate) {
    MessagePlugin.error(`折扣率低于该分类成本率（${rateText(cellDialog.cost_rate)}），会亏本，已阻止保存`)
    return
  }
  cellDialog.saving = true
  try {
    const data = await updateAgentCell({
      agent_level_id: cellDialog.agent_level_id,
      target_type: cellDialog.target_type,
      target_id: cellDialog.target_type === 'all' ? 0 : cellDialog.target_id,
      discount_rate: rate,
    })
    matrix.value = data
    MessagePlugin.success(rate > 0 ? '折扣已保存' : '已清除该格折扣')
    cellDialog.visible = false
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    cellDialog.saving = false
  }
}

function handleMobileAction(value: string | number | Record<string, any>, row: AgentLevelInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

onMounted(async () => {
  await Promise.all([loadData(), loadMatrix(), loadCategoryOptions(), loadProductOptions()])
  await refreshPreview()
})

// 宿主页（用户组管理 → 代理分组）页头上的「新建代理等级」走这里。
defineExpose({ openCreate })
</script>

<style scoped>
.agent-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.surface-card {
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  box-shadow: none;
}

.toolbar,
.table-panel,
.matrix-card {
  padding: 16px 20px;
}

.panel-desc {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.card-head {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.card-title {
  margin: 0;
  font-size: 16px;
}

.card-hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.empty-hint {
  padding: 20px 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.matrix-scroll {
  overflow-x: auto;
}

.matrix-table {
  border-collapse: collapse;
  width: 100%;
  min-width: 520px;
  font-size: 13px;
}

.matrix-table th,
.matrix-table td {
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  padding: 8px 10px;
  text-align: center;
  white-space: nowrap;
}

.matrix-table__corner,
.matrix-table__row-head {
  text-align: left;
  background: var(--td-bg-color-container-hover, #f8f9fa);
}

.col-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.col-head__name {
  font-weight: 600;
}

.col-head__meta {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.row-head__cost {
  display: block;
  font-size: 11px;
  color: var(--td-warning-color, #e37318);
  margin-top: 2px;
}

.cell-value {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cell-value__rate {
  font-weight: 600;
}

.cell-value__margin {
  font-size: 11px;
  color: var(--td-success-color, #2ba471);
}

.cell-value__margin.is-loss {
  color: var(--td-error-color, #d54941);
}

.cell-empty {
  color: var(--color-muted-foreground);
}

.cell-empty--action {
  color: var(--td-brand-color, #0052d9);
}

.matrix-table__cell.is-editable {
  cursor: pointer;
}

.matrix-table__cell.is-editable:hover {
  background: var(--td-brand-color-1, #f2f7ff);
}

.cell-margin-preview {
  margin: 0;
  font-size: 12px;
  color: var(--td-success-color, #2ba471);
}

.cell-margin-preview.is-loss {
  color: var(--td-error-color, #d54941);
}

.cell-fallback {
  font-weight: 600;
}

.ladder {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--td-brand-color-1);
}

.ladder__head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.ladder__title {
  margin: 0;
  font-size: 15px;
}

.ladder__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.ladder__form {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  flex-wrap: wrap;
  margin-top: 10px;
}

.ladder__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ladder__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.ladder__control {
  width: 160px;
}

.ladder__control--wide {
  width: 240px;
}

.ladder__preview {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.ladder__chip {
  padding: 2px 10px;
  border: 1px solid var(--td-brand-color-3, #a6c8ff);
  border-radius: 999px;
  font-size: 12px;
  color: var(--td-brand-color, #0052d9);
}

.ladder__chip.is-loss {
  border-color: var(--td-error-color, #d54941);
  color: var(--td-error-color, #d54941);
}

.ladder__warnings {
  margin: 8px 0 0;
  padding-left: 18px;
  font-size: 12px;
  color: var(--td-error-color, #d54941);
}

.toolbar__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.toolbar-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toolbar-field__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.toolbar__actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--td-brand-color-1);
}

.primary-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.muted {
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.field-hint {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}

.discount-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.discount-editor__hint {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.discount-editor__row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.weight-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 320px;
}

/* 弹窗内按钮别被 flex 容器拉成通栏 */
.discount-editor > .t-button {
  align-self: flex-start;
}

.discount-editor__batch {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px dashed var(--td-brand-color-3, #a6c8ff);
  border-radius: var(--hs-radius-md, 6px);
  background: var(--td-brand-color-1, #f2f7ff);
}

.discount-editor__batch-select {
  flex: 1;
  min-width: 260px;
}

.discount-editor__type {
  width: 130px;
}

.discount-editor__target {
  width: 220px;
}

.discount-editor__target--static {
  display: inline-flex;
  align-items: center;
  height: 32px;
  padding: 0 10px;
  border: 1px dashed var(--td-component-stroke, #dcdcdc);
  border-radius: var(--hs-radius-md, 6px);
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.discount-editor__value {
  width: 130px;
}

.discount-editor__margin {
  font-size: 12px;
  color: var(--td-success-color, #2ba471);
  min-width: 84px;
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }

  .ladder__control {
    width: 100%;
  }
}
</style>
