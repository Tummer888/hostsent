<template>
  <div class="agent-panel">
    <section class="matrix-card surface-card">
      <div class="card-head">
        <div class="card-head__main">
          <h3 class="card-title">④ 生效矩阵</h3>
          <p class="card-hint">
            这里显示的是③折扣组「应用」后<strong>此刻真实生效</strong>的折扣率，不是配置草稿。
            行 = 折扣目标，列 = 代理分组（按权重降序，最左最优先）。点任意单元格可直接微调这一格。
          </p>
        </div>
        <div class="card-head__actions">
          <t-input
            v-model="targetKeyword"
            size="small"
            clearable
            placeholder="搜索目标名称"
            class="target-search"
          />
          <t-radio-group v-model="viewMode" variant="default-filled" size="small">
            <t-radio-button value="all">全部</t-radio-button>
            <t-radio-button value="configured">只看已配置</t-radio-button>
          </t-radio-group>
        </div>
      </div>

      <t-loading :loading="matrixLoading" size="small">
        <div v-if="matrix.columns.length === 0" class="empty-hint">
          还没有启用中的代理分组，先到「① 代理分组」分区创建分组后再配折扣。
        </div>
        <div v-else-if="visibleRows.length === 0" class="empty-hint">
          当前筛选下没有可显示的目标。
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
              <template v-for="entry in visibleRows" :key="entry.kind === 'section' ? `section-${entry.key}` : entry.key">
                <tr v-if="entry.kind === 'section'" class="matrix-section-row">
                  <td :colspan="matrix.columns.length + 1" class="matrix-section-cell">
                    <span>{{ entry.label }}</span>
                    <t-link
                      v-if="entry.section === 'products'"
                      theme="primary"
                      hover="color"
                      class="matrix-section-toggle"
                      @click="showProductRows = !showProductRows"
                    >
                      {{ showProductRows ? '收起' : `展开 ${entry.count} 个` }}
                    </t-link>
                  </td>
                </tr>
                <tr v-else>
                  <td class="matrix-table__row-head">
                    <t-tag
                      v-if="entry.row!.target_type === 'product'"
                      theme="warning"
                      variant="light"
                      size="small"
                      shape="round"
                      class="row-head__badge"
                    >
                      商品
                    </t-tag>
                    <span class="row-head__name" :title="entry.row!.target_name">{{ entry.row!.target_name }}</span>
                    <span v-if="entry.row!.cost_rate > 0" class="row-head__cost">
                      成本 {{ rateText(entry.row!.cost_rate) }}
                    </span>
                  </td>
                  <td
                    v-for="cell in entry.row!.cells"
                    :key="cell.agent_level_id"
                    class="matrix-table__cell"
                    :class="{ 'is-editable': canEditCells, 'is-loss': isLossCell(entry.row!, cell) }"
                    :title="cellTitle(entry.row!, cell)"
                    @click="openCellEdit(entry.row!, cell)"
                  >
                    <template v-if="cell.configured">
                      <span class="cell-rate">{{ shortRate(cell.discount_rate) }}</span>
                      <span
                        v-if="entry.row!.cost_rate > 0"
                        class="cell-margin"
                        :class="{ 'is-loss': cell.discount_rate < entry.row!.cost_rate }"
                      >
                        {{ shortMargin(cell.discount_rate, entry.row!.cost_rate) }}
                      </span>
                    </template>
                    <span v-else class="cell-empty">{{ canEditCells ? '＋' : '—' }}</span>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </t-loading>

      <p class="panel-foot">
        折扣率越小越优惠；每格下方小字是毛利率（低于 0 会亏本，后端拒绝保存）。
        <strong>批量配置请用「③ 折扣组」</strong>——那里按商品分组整批写；本页只做单格微调。
      </p>
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
  getAgentMatrix,
  updateAgentCell,
  type AgentDiscountTargetType,
  type AgentMatrixCell,
  type AgentMatrixResponse,
  type AgentMatrixRow,
} from '@/api/agent-level'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'AgentLevelPanel' })

const { has } = usePermission()

const matrixLoading = ref(false)
const matrix = ref<AgentMatrixResponse>({ columns: [], rows: [], product_rows: [] })

// 矩阵行数 = 全站 1 + 启用分类 N + 已配置商品 M。分类/商品一多就眼花，
// 因此给「搜索 + 只看已配置」两个收窄工具，并把商品段做成可折叠。
const targetKeyword = ref('')
const viewMode = ref<'all' | 'configured'>('all')
const showProductRows = ref(false)

type MatrixRenderEntry =
  | { kind: 'section'; key: string; section: 'products'; label: string; count: number }
  | { kind: 'row'; key: string; row: AgentMatrixRow }

const rowOf = (row: AgentMatrixRow): AgentMatrixRow => row

/** 该行是否有任一已配置的格子（用于「只看已配置」）。 */
function hasConfigured(row: AgentMatrixRow): boolean {
  return row.cells.some((cell) => cell.configured)
}

function matchesKeyword(row: AgentMatrixRow): boolean {
  const keyword = targetKeyword.value.trim().toLowerCase()
  if (!keyword) return true
  return (row.target_name || '').toLowerCase().includes(keyword)
}

/** 渲染序列：全站 + 分类 → 商品段分隔行 → 商品行（可折叠）。 */
const visibleRows = computed<MatrixRenderEntry[]>(() => {
  const list: MatrixRenderEntry[] = []
  for (const row of matrix.value.rows) {
    if (viewMode.value === 'configured' && !hasConfigured(row) && row.target_type !== 'all') continue
    if (!matchesKeyword(row)) continue
    list.push({ kind: 'row', key: `${row.target_type}:${row.target_id}`, row: rowOf(row) })
  }
  const products = matrix.value.product_rows || []
  // 商品行本身只有「配过商品级折扣」的（商品可能上千，矩阵不全列），因此
  // 「只看已配置」对商品段不构成收窄，仍要与分类段一样受搜索词约束。
  const matchedProducts = products.filter(matchesKeyword)
  // 分隔行始终显示当入口（否则收起后就没有再展开的地方了），行本身按展开状态追加。
  if (matchedProducts.length) {
    list.push({
      kind: 'section',
      key: 'products',
      section: 'products',
      label: `商品例外阶梯（${matchedProducts.length} 个商品）· 命中优先级高于分类与全站`,
      count: matchedProducts.length,
    })
    if (showProductRows.value) {
      for (const row of matchedProducts) {
        list.push({ kind: 'row', key: `${row.target_type}:${row.target_id}`, row: rowOf(row) })
      }
    }
  }
  return list
})

function shortRate(rate: number): string {
  // 矩阵里只显示纯数值（0.85）——「八五折」这种口语展开留给单格弹窗与列表，
  // 否则每格两行文字（数值 + 折数）在小列宽下会挤成一团。
  return rate.toFixed(2)
}

function shortMargin(rate: number, cost: number): string {
  const margin = (rate - cost) * 100
  return `${margin >= 0 ? '+' : ''}${margin.toFixed(0)}%`
}

function rateText(rate: number): string {
  const discount = (rate * 10).toFixed(1).replace(/\.0$/, '')
  return `${rate.toFixed(2)}（${discount}折）`
}

function marginText(rate: number, cost: number): string {
  if (cost <= 0) return ''
  const margin = (rate - cost) * 100
  return `毛利 ${margin >= 0 ? '' : '-'}${Math.abs(margin).toFixed(1)}%`
}

function isLossCell(row: AgentMatrixRow, cell: AgentMatrixCell): boolean {
  return Boolean(cell.configured) && row.cost_rate > 0 && cell.discount_rate < row.cost_rate
}

function cellTitle(row: AgentMatrixRow, cell: AgentMatrixCell): string {
  if (!canEditCells.value) return ''
  if (!cell.configured) return '点击设置该代理分组在此目标的折扣率'
  const margin = row.cost_rate > 0 ? `，${marginText(cell.discount_rate, row.cost_rate)}` : ''
  return `当前 ${rateText(cell.discount_rate)}${margin}；点击修改`
}

async function loadMatrix() {
  matrixLoading.value = true
  try {
    const data = await getAgentMatrix()
    matrix.value = data
    // 商品例外行很少（只有配过的商品），有就该看得到，默认展开。
    if ((data.product_rows || []).length) showProductRows.value = true
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载折扣矩阵失败')
  } finally {
    matrixLoading.value = false
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
  if (!parts.length) return '该目标其它代理分组暂未配置折扣；可到「③ 折扣组」按商品分组整批配'
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

onMounted(loadMatrix)

// 宿主页（用户组管理 → ④ 生效矩阵）：代理分组或折扣组变化后经 reload 刷新生效矩阵。
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

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.card-head__main {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 260px;
  flex: 1;
}

.card-head__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.target-search {
  width: 200px;
}

.card-title {
  margin: 0;
  font-size: 16px;
}

.card-hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.empty-hint {
  padding: 20px 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.matrix-scroll {
  overflow: auto;
  max-height: 62vh;
  border: 1px solid var(--td-component-stroke, #e7e7e7);
  border-radius: var(--hs-radius-md, 6px);
}

.matrix-table {
  border-collapse: separate;
  border-spacing: 0;
  width: 100%;
  min-width: 520px;
  font-size: 13px;
}

.matrix-table th,
.matrix-table td {
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  border-right: 1px solid var(--td-component-stroke, #e7e7e7);
  padding: 6px 8px;
  text-align: center;
  white-space: nowrap;
}

.matrix-table th:last-child,
.matrix-table td:last-child {
  border-right: none;
}

/* 表头吸顶：行多了滚动时仍知道每一列是哪个代理分组。 */
.matrix-table thead th {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--td-bg-color-container-hover, #f8f9fa);
}

.matrix-table__corner,
.matrix-table__row-head {
  position: sticky;
  left: 0;
  z-index: 1;
  text-align: left;
  background: var(--td-bg-color-container-hover, #f8f9fa);
  max-width: 240px;
}

.matrix-table__corner {
  z-index: 3;
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
  padding: 6px 10px;
  text-align: left;
  font-size: 12px;
  font-weight: 600;
  color: var(--td-brand-color-8, #0052d9);
  background: var(--td-brand-color-1, #f2f7ff);
}

.matrix-section-toggle {
  margin-left: 10px;
  font-weight: 400;
}

.row-head__badge {
  margin-right: 6px;
}

.row-head__name {
  font-weight: 500;
}

.row-head__cost {
  display: block;
  font-size: 11px;
  color: var(--td-warning-color, #e37318);
  margin-top: 1px;
}

.matrix-table__cell {
  cursor: default;
  min-width: 84px;
}

.matrix-table__cell.is-editable {
  cursor: pointer;
}

.matrix-table__cell.is-editable:hover {
  background: var(--td-brand-color-1, #f2f7ff);
}

.matrix-table__cell.is-loss {
  background: rgba(213, 73, 65, 0.06);
}

.cell-rate {
  font-weight: 600;
}

/* 毛利率作为折扣率下方的小字：既保留可判断性，又不占列宽。 */
.cell-margin {
  display: block;
  font-size: 11px;
  color: var(--td-success-color, #2ba471);
}

.cell-margin.is-loss {
  color: var(--td-error-color, #d54941);
}

.cell-empty {
  color: var(--color-muted-foreground);
}

.cell-margin-preview {
  margin: 0;
  font-size: 12px;
  color: var(--td-success-color, #2ba471);
}

.cell-margin-preview.is-loss {
  color: var(--td-error-color, #d54941);
}

.cell-siblings {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.field-hint {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.panel-foot {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

@media (max-width: 768px) {
  .target-search {
    width: 100%;
  }
}
</style>
