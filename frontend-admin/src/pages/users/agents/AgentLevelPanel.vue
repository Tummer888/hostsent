<template>
  <div class="agent-panel">
    <!-- 折扣矩阵：行 = 目标（全站/分类/商品例外），列 = 代理分组 -->
    <section class="matrix-card surface-card">
      <p class="panel-desc">
        <strong>④ 生效矩阵</strong>：代理拿货折扣此刻<strong>真实生效</strong>的值。行是折扣目标（全站兜底 / 商品分类 / 单个商品），
        列是代理分组（按权重降序，最左最优先）。<strong>点击任意单元格</strong>可直接微调该格。
        这里显示的是③折扣组「应用」后的结果，不是配置草稿。
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
          还没有启用中的代理分组，先到「代理分组」分区创建分组后再配折扣。
        </div>
        <div v-else class="matrix-scroll">
          <table class="matrix-table">
            <thead>
              <tr>
                <th class="matrix-table__corner">目标 / 代理分组</th>
                <th v-for="col in matrix.columns" :key="col.agent_level_id" class="matrix-table__col">
                  <div class="col-head">
                    <span class="col-head__name">{{ col.name }}</span>
                    <span class="col-head__meta">权重 {{ col.weight }}</span>
                  </div>
                </th>
              </tr>
            </thead>
            <tbody>
              <template v-for="(entry, idx) in matrixRenderList" :key="entry.kind === 'section' ? `section-${idx}` : rowKey(entry.row!)">
                <tr v-if="entry.kind === 'section'" class="matrix-section-row">
                  <td :colspan="matrix.columns.length + 1" class="matrix-section-cell">
                    商品例外阶梯（{{ entry.count }} 个商品）· 命中优先级高于分类与全站
                  </td>
                </tr>
                <tr v-else>
                <td class="matrix-table__row-head">
                  <t-tag v-if="entry.row!.target_type === 'product'" theme="warning" variant="light" size="small" shape="round" class="row-head__badge">
                    商品
                  </t-tag>
                  <span class="row-head__name">{{ entry.row!.target_name }}</span>
                  <span v-if="entry.row!.cost_rate > 0" class="row-head__cost">
                    成本 {{ rateText(entry.row!.cost_rate) }}
                  </span>
                </td>
                <td
                  v-for="cell in entry.row!.cells"
                  :key="cell.agent_level_id"
                  class="matrix-table__cell"
                  :class="{ 'is-editable': canEditCells }"
                  :title="canEditCells ? '点击设置该分组在此目标的折扣率' : ''"
                  @click="openCellEdit(entry.row!, cell)"
                >
                  <div v-if="cell.configured" class="cell-value">
                    <span class="cell-value__rate">{{ rateText(cell.discount_rate) }}</span>
                    <span
                      v-if="entry.row!.cost_rate > 0"
                      class="cell-value__margin"
                      :class="{ 'is-loss': cell.discount_rate < entry.row!.cost_rate }"
                    >
                      {{ marginText(cell.discount_rate, entry.row!.cost_rate) }}
                    </span>
                  </div>
                  <span v-else class="cell-empty" :class="{ 'cell-empty--action': canEditCells }">
                    {{ canEditCells ? '＋ 设置' : '—' }}
                  </span>
                </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </t-loading>

      <!-- 阶梯填充：锚点 + 步长 → 一键写入某目标下的所有代理分组 -->
      <div class="ladder">
        <div class="ladder__head">
          <h4 class="ladder__title">阶梯填充</h4>
          <span class="ladder__hint">按「最优分组折扣 + 每级递增」一次配好一个目标的全部代理分组（全站 / 分类 / 单个商品）；商品阶梯优先于分类和全站</span>
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
            <span class="ladder__label">最优分组折扣率</span>
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
        <p v-if="cellSiblingsText" class="cell-siblings">{{ cellSiblingsText }}</p>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'

import {
  applyAgentLadder,
  getAgentMatrix,
  previewAgentLadder,
  updateAgentCell,
  type AgentDiscountTargetType,
  type AgentLadderPreviewCell,
  type AgentMatrixResponse,
  type AgentMatrixRow,
} from '@/api/agent-level'
import { getProductCategoryList, getProductList } from '@/api/product'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'AgentLevelPanel' })

const { has } = usePermission()

const matrixLoading = ref(false)
const applying = ref(false)
const matrix = ref<AgentMatrixResponse>({ columns: [], rows: [], product_rows: [] })

const targetTypeOptions = [
  { label: '全站兜底', value: 'all' },
  { label: '按分类', value: 'category' },
  { label: '单个商品', value: 'product' },
]

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
    // 全量商品（含未上架）：对接期/未上架的商品也要能预配折扣，只做标注不做过滤；
    // 在售排前、未上架排后并带「（已下架）」后缀。取前 500 条足够覆盖当前规模。
    const data = await getProductList({ page: 1, page_size: 500 } as never)
    const items = (data.items || []) as Array<{ id: number; name: string; status?: number }>
    const onSale: { label: string; value: number }[] = []
    const offShelf: { label: string; value: number }[] = []
    for (const item of items) {
      const entry = { label: item.status === 1 ? item.name : `${item.name}（已下架）`, value: item.id }
      ;(item.status === 1 ? onSale : offShelf).push(entry)
    }
    productOptions.value = [...onSale, ...offShelf]
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

type MatrixRenderEntry = { kind: 'section'; count: number } | { kind: 'row'; row: AgentMatrixRow }

/** 矩阵渲染序列：分类/全站行 → 「商品例外阶梯」分隔行 → 商品行（同一张表，列对齐不跳）。 */
const matrixRenderList = computed<MatrixRenderEntry[]>(() => {
  const list: MatrixRenderEntry[] = matrix.value.rows.map((row) => ({ kind: 'row', row }))
  const products = matrix.value.product_rows || []
  if (products.length) {
    list.push({ kind: 'section', count: products.length })
    for (const row of products) list.push({ kind: 'row', row })
  }
  return list
})

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
    await refreshPreview()
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
  cellDialog.cost_rate > 0 ? `该目标成本率 ${rateText(cellDialog.cost_rate)}，低于它会被拒绝。` : '',
)
const cellMarginPreview = computed(() => {
  if (!cellDialog.rate || cellDialog.cost_rate <= 0) return ''
  return marginText(cellDialog.rate, cellDialog.cost_rate)
})
const cellMarginLoss = computed(
  () => cellDialog.rate > 0 && cellDialog.cost_rate > 0 && cellDialog.rate < cellDialog.cost_rate,
)

// 单格弹窗的跨分组上下文：同一目标下其它代理分组的折扣一览。
const cellSiblingsText = computed(() => {
  if (!cellDialog.visible) return ''
  const source = cellDialog.target_type === 'product' ? matrix.value.product_rows : matrix.value.rows
  const row = (source || []).find(
    (r) => r.target_type === cellDialog.target_type && r.target_id === cellDialog.target_id,
  )
  if (!row) return ''
  const parts: string[] = []
  for (const cell of row.cells) {
    if (cell.agent_level_id === cellDialog.agent_level_id) continue
    if (!cell.configured) continue
    const col = matrix.value.columns.find((c) => c.agent_level_id === cell.agent_level_id)
    parts.push(`${col?.name || `分组#${cell.agent_level_id}`} ${rateText(cell.discount_rate)}`)
  }
  if (!parts.length) return '该目标其它代理分组暂未配置折扣；可用「阶梯填充」一次配齐'
  return `其它分组：${parts.join(' · ')}`
})

function openCellEdit(row: AgentMatrixRow, cell: { agent_level_id: number; discount_rate: number; configured: boolean }) {
  if (!canEditCells.value) return
  const col = matrix.value.columns.find((item) => item.agent_level_id === cell.agent_level_id)
  cellDialog.agent_level_id = cell.agent_level_id
  cellDialog.level_name = col?.name || `代理分组 #${cell.agent_level_id}`
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
    MessagePlugin.error(`折扣率低于该目标成本率（${rateText(cellDialog.cost_rate)}），会亏本，已阻止保存`)
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

onMounted(async () => {
  await Promise.all([loadMatrix(), loadCategoryOptions(), loadProductOptions()])
  await refreshPreview()
})

// 宿主页（用户组管理 → 折扣设置）：代理分组或折扣组变化后经 reload 刷新生效矩阵。
defineExpose({ reload: loadMatrix })
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

.matrix-section-row td {
  padding: 8px 10px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--td-brand-color-8, #0052d9);
  background: var(--td-brand-color-1, #f2f7ff);
}

.cell-siblings {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.row-head__badge {
  margin-right: 6px;
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

.field-hint {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

@media (max-width: 768px) {
  .ladder__control {
    width: 100%;
  }
}
</style>
