<template>
  <div class="scheme-panel">
    <section class="scheme-card surface-card">
      <div class="scheme-head">
        <div>
          <h3 class="scheme-title">③ 折扣组（谁拿几折）</h3>
          <p class="scheme-desc">
            一行 = 一个折扣组，可绑定<strong>一个或多个商品分组</strong>（②），它们共用这一行的费率阶梯；
            列是<strong>代理分组</strong>（①），每格填该代理分组在这些商品上的折扣率。
            最高权重分组填锚点，其余可用「阶梯填充」一键铺开；点「应用」后才写入折扣矩阵生效。
            <strong>同一商品只能归一个折扣组</strong>，重叠时应用会被拒绝（避免两组互相覆盖）。
          </p>
        </div>
        <t-button v-permission="'agent_level:create'" theme="primary" @click="openCreate">新建折扣组</t-button>
      </div>

      <t-loading :loading="loading" size="small">
        <div v-if="rows.length === 0" class="empty-hint">
          还没有折扣组，点右上角「新建折扣组」开始（建议按档位命名，如 6折组、5折组）。
        </div>
        <div v-else class="scheme-scroll">
          <table class="scheme-table">
            <thead>
              <tr>
                <th class="scheme-table__name">折扣组</th>
                <th class="scheme-table__group">绑定商品分组</th>
                <th v-for="level in activeLevels" :key="level.id" class="scheme-table__level">
                  <div class="level-head">
                    <span>{{ level.name }}</span>
                    <span class="level-head__meta">权重 {{ level.weight }}</span>
                  </div>
                </th>
                <th class="scheme-table__op">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="row.id">
                <td class="scheme-table__name">
                  <t-input v-model="row.draft.name" size="small" placeholder="组名称" class="scheme-name-input" />
                  <span v-if="row.draft.group_ids.length" class="scheme-row-meta">
                    {{ row.draft.group_ids.length }} 个分组 · {{ row.targetCount }} 个目标
                  </span>
                </td>
                <td class="scheme-table__group">
                  <t-select
                    v-model="row.draft.group_ids"
                    :options="groupOptionsFor(row)"
                    size="small"
                    multiple
                    clearable
                    filterable
                    :min-collapsed-num="2"
                    placeholder="可多选"
                    class="scheme-group-select"
                  />
                </td>
                <td v-for="level in activeLevels" :key="level.id" class="scheme-table__level">
                  <t-input-number
                    v-model="row.draft.rates[level.id]"
                    size="small"
                    :min="0"
                    :max="1"
                    :step="0.01"
                    :precision="2"
                    theme="normal"
                    class="scheme-rate"
                  />
                </td>
                <td class="scheme-table__op">
                  <t-space size="6px">
                    <t-link
                      v-permission="'agent_level:update'"
                      theme="default"
                      hover="color"
                      @click="openLadder(row)"
                    >
                      阶梯填充
                    </t-link>
                    <t-link
                      v-permission="'agent_level:update'"
                      theme="primary"
                      hover="color"
                      :disabled="row.saving"
                      @click="saveRow(row)"
                    >
                      保存
                    </t-link>
                    <t-popconfirm
                      content="把该组费率写入所绑商品分组下的全部分类/商品？（分组间目标会去重；与其它折扣组重叠时会被拒绝）"
                      @confirm="applyRow(row)"
                    >
                      <t-link v-permission="'agent_level:update'" theme="primary" hover="color" :disabled="row.applying">
                        应用
                      </t-link>
                    </t-popconfirm>
                    <t-popconfirm content="确认删除该折扣组？（不影响已写入的折扣矩阵）" @confirm="deleteRow(row)">
                      <t-link v-permission="'agent_level:delete'" theme="danger" hover="color">删除</t-link>
                    </t-popconfirm>
                  </t-space>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </t-loading>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      header="新建折扣组"
      width="480px"
      :confirm-btn="{ content: '创建', loading: submitting }"
      @confirm="handleCreate"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <t-form-item label="折扣组名称" name="name">
          <t-input v-model="form.name" placeholder="如：6折组（建议按档位命名）" />
        </t-form-item>
        <t-form-item label="编码" name="code">
          <t-input v-model="form.code" placeholder="如：scheme_60" />
        </t-form-item>
        <t-form-item label="绑定商品分组" name="product_group_ids">
          <t-select
            v-model="form.group_ids"
            :options="groupOptions"
            multiple
            clearable
            filterable
            :min-collapsed-num="3"
            placeholder="可多选：多个分组共用本组费率（同一分组只能归一个折扣组）"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
    <!-- 阶梯填充：填好最高权重的锚点，一次把本行其余代理分组按步长铺满 -->
    <t-dialog
      v-model:visible="ladderVisible"
      :header="`阶梯填充 · ${ladderRow?.draft.name || ''}`"
      width="520px"
      :confirm-btn="{ content: '填入本行', theme: 'primary' }"
      @confirm="applyLadder"
    >
      <t-form label-align="top">
        <p class="ladder-note">
          按代理分组权重从高到低依次展开：权重最高的分组取「锚点折扣」，每降一级增加一个「步长」。
          例如锚点 0.50、步长 0.05 → 五折 0.50、六折 0.55、七折 0.60。
        </p>
        <div class="ladder-form">
          <t-form-item label="锚点折扣（最高权重分组）">
            <t-input-number
              v-model="ladder.anchor"
              :min="0"
              :max="1"
              :step="0.01"
              :precision="2"
              theme="normal"
              style="width: 100%"
            />
          </t-form-item>
          <t-form-item label="每级递增">
            <t-input-number
              v-model="ladder.step"
              :min="0"
              :max="1"
              :step="0.01"
              :precision="2"
              theme="normal"
              style="width: 100%"
            />
          </t-form-item>
        </div>
        <div v-if="ladderPreview.length" class="ladder-preview">
          <span v-for="cell in ladderPreview" :key="cell.id" class="ladder-chip">
            {{ cell.name }} {{ cell.rate.toFixed(2) }}
          </span>
        </div>
        <p class="ladder-note ladder-note--warn">
          这里只填入本行草稿，仍需点该行的「保存」才写入折扣组，再点「应用」才生效到商品。
          成本线与权重单调性由后端在保存 / 应用时校验。
        </p>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import {
  applyScheme,
  createScheme,
  deleteScheme,
  getAgentLevelList,
  getProductGroupList,
  getSchemeList,
  updateScheme,
  type ProductGroupInfo,
  type SchemeInfo,
} from '@/api/agent-level'

defineOptions({ name: 'SchemePanel' })

const emit = defineEmits<{ changed: [] }>()

interface SchemeRow {
  id: number
  code: string
  description: string
  status: string
  saving: boolean
  applying: boolean
  /** 绑定分组展开后的去重目标数（后端算好下发，保存后会刷新）。 */
  targetCount: number
  draft: {
    name: string
    group_ids: number[]
    rates: Record<number, number>
  }
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const rows = ref<SchemeRow[]>([])
const groups = ref<ProductGroupInfo[]>([])
const activeLevels = ref<Array<{ id: number; name: string; weight: number }>>([])

const groupOptions = computed(() =>
  groups.value.map((group) => ({
    label: `${group.name}（${group.item_count} 项）`,
    value: group.id,
  })),
)

/**
 * 行内分组下拉：把「已被别的折扣组占用的分组」标出来并禁用。
 *
 * 一个商品分组只归一个折扣组是硬约束（后端唯一索引兜底），在下拉里提前拦住
 * 比让运营选完、保存时才吃到 409 更省事；本行已选的仍可取消。
 */
function groupOptionsFor(row: SchemeRow) {
  return groupOptions.value.map((option) => {
    const owner = groupOwners.value[option.value]
    if (owner && owner !== row.id) {
      return { ...option, disabled: true, label: `${option.label}（已被「${ownerName(owner)}」绑定）` }
    }
    return option
  })
}

function ownerName(schemeId: number): string {
  return rows.value.find((item) => item.id === schemeId)?.draft.name || `折扣组 #${schemeId}`
}

const formRef = ref<FormInstanceFunctions>()
const form = reactive<{ name: string; code: string; group_ids: number[] }>({
  name: '',
  code: '',
  group_ids: [],
})

// 分组 → 占用它的折扣组 ID（由已加载的折扣组反推，不做额外接口）。
const groupOwners = computed<Record<number, number>>(() => {
  const out: Record<number, number> = {}
  for (const row of rows.value) {
    for (const id of row.draft.group_ids) out[id] = row.id
  }
  return out
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入折扣组名称', type: 'error' }],
  code: [{ required: true, message: '请输入编码', type: 'error' }],
}

async function loadData() {
  loading.value = true
  try {
    const [schemeData, groupData, levelData] = await Promise.all([
      getSchemeList(),
      getProductGroupList(),
      getAgentLevelList({ page: 1, page_size: 100, status: 'active' }),
    ])
    groups.value = groupData.items || []
    activeLevels.value = (levelData.items || []).map((item) => ({
      id: item.id,
      name: item.name,
      weight: item.weight,
    }))
    // 行内可编辑草稿：费率列以「当前启用等级」为准；方案里已停用等级的费率不展示但保留在库。
    rows.value = (schemeData.items || []).map((scheme) => toRow(scheme))
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载折扣组失败')
  } finally {
    loading.value = false
  }
}

function toRow(scheme: SchemeInfo): SchemeRow {
  const rates: Record<number, number> = {}
  for (const item of scheme.items || []) rates[item.agent_level_id] = item.discount_rate
  return {
    id: scheme.id,
    code: scheme.code,
    description: scheme.description || '',
    status: scheme.status,
    saving: false,
    applying: false,
    targetCount: scheme.target_count || 0,
    draft: {
      name: scheme.name,
      group_ids: [...(scheme.product_group_ids || [])],
      rates,
    },
  }
}

function payloadOf(row: SchemeRow) {
  const items = activeLevels.value
    .map((level) => ({ agent_level_id: level.id, discount_rate: Number(row.draft.rates[level.id] || 0) }))
  return {
    name: row.draft.name,
    code: row.code,
    description: row.description,
    status: row.status,
    // 数组 = 整体覆盖绑定关系（空数组即解绑全部分组）。
    product_group_ids: row.draft.group_ids,
    // items 非空数组 = 整体覆盖（含停用等级的历史费率，避免保存把库里的清掉）
    items,
  }
}

async function saveRow(row: SchemeRow) {
  if (!row.draft.name.trim()) {
    MessagePlugin.error('请填写折扣组名称')
    return
  }
  row.saving = true
  try {
    await updateScheme(row.id, payloadOf(row))
    MessagePlugin.success('折扣组已保存（尚未生效，点「应用」写入折扣矩阵）')
    await loadData()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    row.saving = false
  }
}

async function applyRow(row: SchemeRow) {
  // 应用前先保存草稿，避免"改了没存就应用"
  if (!row.draft.name.trim()) {
    MessagePlugin.error('请填写折扣组名称')
    return
  }
  row.applying = true
  try {
    await updateScheme(row.id, payloadOf(row))
    const matrix = await applyScheme(row.id)
    // 应用会把折扣写进矩阵；这里的行内目标数同步一下，运营能立刻看到「写了几格」。
    MessagePlugin.success(
      `已应用：${row.draft.group_ids.length} 个商品分组、${matrix.rows.length + (matrix.product_rows?.length || 0)} 行目标已写入折扣矩阵`,
    )
    await loadData()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '应用失败')
  } finally {
    row.applying = false
  }
}

async function deleteRow(row: SchemeRow) {
  try {
    await deleteScheme(row.id)
    MessagePlugin.success('已删除')
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除失败')
  }
}

function openCreate() {
  Object.assign(form, { name: '', code: '', group_ids: [] })
  dialogVisible.value = true
}

async function handleCreate() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  submitting.value = true
  try {
    await createScheme({
      name: form.name,
      code: form.code,
      product_group_ids: form.group_ids,
      items: activeLevels.value.map((level) => ({ agent_level_id: level.id, discount_rate: 0 })),
    })
    MessagePlugin.success('折扣组已创建，请在表格中填写各代理分组的折扣率')
    dialogVisible.value = false
    await loadData()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '创建失败')
  } finally {
    submitting.value = false
  }
}

onMounted(loadData)

// —— 行内阶梯填充 ——
//
// 运营按档位建组时（五折组 / 六折组…），逐个分组填折扣率很啰嗦，而「五折用户 5 折、
// 六折用户 6 折、七折用户 7 折」本质就是一条等差阶梯。这里只填本行草稿，不动库：
// 保存与应用仍走既有入口，成本线与单调性校验因此不用重复实现一遍。
const ladderVisible = ref(false)
const ladderRow = ref<SchemeRow | null>(null)
const ladder = reactive({ anchor: 0.5, step: 0.05 })

// 阶梯按权重降序展开；activeLevels 本身就是后端按 weight desc 下发的。
const ladderPreview = computed(() =>
  activeLevels.value.map((level, index) => ({
    id: level.id,
    name: level.name,
    rate: Math.min(1, Number((ladder.anchor + index * ladder.step).toFixed(4))),
  })),
)

function openLadder(row: SchemeRow) {
  ladderRow.value = row
  // 以该行已填的最高权重分组值作为锚点起点，减少重复输入。
  const first = activeLevels.value[0]
  const current = first ? Number(row.draft.rates[first.id] || 0) : 0
  ladder.anchor = current > 0 ? current : 0.5
  ladder.step = 0.05
  ladderVisible.value = true
}

function applyLadder() {
  const row = ladderRow.value
  if (!row) return
  if (activeLevels.value.length === 0) {
    MessagePlugin.warning('还没有启用中的代理分组')
    return
  }
  for (const cell of ladderPreview.value) {
    row.draft.rates[cell.id] = cell.rate
  }
  MessagePlugin.success('已填入本行草稿，记得点「保存」')
  ladderVisible.value = false
}

defineExpose({ openCreate, reload: loadData })
</script>

<style scoped>
.scheme-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.surface-card {
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  box-shadow: none;
}

.scheme-card {
  padding: 16px 20px;
}

.scheme-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.scheme-title {
  margin: 0;
  font-size: 16px;
}

.scheme-desc {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.empty-hint {
  padding: 20px 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.scheme-scroll {
  overflow-x: auto;
}

.scheme-table {
  border-collapse: collapse;
  width: 100%;
  min-width: 720px;
  font-size: 13px;
}

.scheme-table th,
.scheme-table td {
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  padding: 8px 10px;
  text-align: center;
  white-space: nowrap;
}

.scheme-table__name {
  min-width: 160px;
  text-align: left;
}

.scheme-table__group {
  min-width: 200px;
}

.scheme-name-input {
  width: 150px;
}

.scheme-row-meta {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.scheme-group-select {
  width: 190px;
}

.scheme-rate {
  width: 96px;
}

.level-head {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ladder-note {
  margin: 0 0 12px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
}

.ladder-note--warn {
  margin: 12px 0 0;
  color: var(--td-warning-color, #e37318);
}

.ladder-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.ladder-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 4px;
}

.ladder-chip {
  padding: 2px 10px;
  border: 1px solid var(--td-brand-color-3, #a6c8ff);
  border-radius: 999px;
  font-size: 12px;
  color: var(--td-brand-color, #0052d9);
}

@media (max-width: 640px) {
  .ladder-form {
    grid-template-columns: 1fr;
  }
}

.level-head__meta {
  font-size: 11px;
  color: var(--color-muted-foreground);
}
</style>
