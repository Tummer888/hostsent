<template>
  <div class="ag-panel">
    <section class="ag-card surface-card">
      <div class="ag-head">
        <div>
          <h3 class="ag-title">代理分组</h3>
          <p class="ag-desc">
            一行 = 一个代理分组（也就是拿货折扣的档位）。<strong>归属</strong>决定某个代理属于哪个分组：
            在「折扣设置」里，行是折扣组（绑定商品分组）、列就是这里的代理分组，格子里填折扣率。
          </p>
        </div>
        <div class="ag-head__actions">
          <t-button v-permission="'agent_level:update'" variant="outline" @click="openAssignPanel">
            未归属账号（{{ unassignedTotal }}）
          </t-button>
          <t-button v-permission="'agent_level:create'" theme="primary" @click="openCreate">新建代理分组</t-button>
        </div>
      </div>

      <t-table
        row-key="id"
        :data="groups"
        :columns="columns"
        :loading="loading"
        cell-empty-content="—"
        size="small"
        hover
      >
        <template #name="{ row }">
          <div class="ag-cell">
            <strong>{{ row.name }}</strong>
            <span class="muted">{{ row.code }}</span>
          </div>
        </template>
        <template #weight="{ row }">
          <span class="muted">权重 {{ row.weight }}</span>
        </template>
        <template #member_count="{ row }">
          <t-link theme="primary" hover="color" @click="openMembers(row)">{{ row.member_count }} 人</t-link>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '停用' }}
          </t-tag>
        </template>
        <template #operation="{ row }">
          <t-space size="8px">
            <t-link v-permission="'agent_level:update'" theme="primary" hover="color" @click="openMembers(row)">管理成员</t-link>
            <t-link v-permission="'agent_level:update'" theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-popconfirm
              content="确认删除该代理分组？（组内仍有代理时会被拒绝）"
              @confirm="handleDelete(row)"
            >
              <t-link v-permission="'agent_level:delete'" theme="danger" hover="color">删除</t-link>
            </t-popconfirm>
          </t-space>
        </template>
      </t-table>
    </section>

    <!-- 分组本身：名称/编码/权重/状态（权重决定折扣矩阵里的列顺序与阶梯优先级） -->
    <t-dialog
      v-model:visible="formVisible"
      :header="editing ? '编辑代理分组' : '新建代理分组'"
      width="520px"
      :confirm-btn="{ content: '保存', loading: submitting }"
      @confirm="handleSubmit"
    >
      <t-form ref="formRef" :data="form" :rules="rules" label-align="top">
        <div class="form-grid">
          <t-form-item label="分组名称" name="name">
            <t-input v-model="form.name" placeholder="如：钻石代理" />
          </t-form-item>
          <t-form-item label="分组编码" name="code">
            <t-input v-model="form.code" placeholder="如：diamond_agent" />
          </t-form-item>
          <t-form-item label="权重" name="weight">
            <t-input-number v-model="form.weight" :min="0" :step="1" theme="normal" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
        </div>
        <p class="field-hint">
          权重越大越优先、折扣越优：矩阵里列头从左到右按权重降序，阶梯填充也以最高权重的分组为锚点。
          停用后该分组不再参与算价（组内代理按无折扣处理）。
        </p>
        <t-form-item label="说明" name="description">
          <t-input v-model="form.description" placeholder="选填" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 归属管理：把账号纳入本分组 / 把本组账号移出（取消代理身份） -->
    <t-dialog
      v-model:visible="memberVisible"
      :header="`管理成员 · ${currentGroup?.name || ''}`"
      width="880px"
      :footer="false"
    >
      <t-tabs v-model="memberTab">
        <t-tab-panel value="members" :label="`本组成员（${memberTotal}）`">
          <div class="member-toolbar">
            <t-input
              v-model="memberKeyword"
              clearable
              placeholder="搜索用户名 / 邮箱 / 手机 / 姓名"
              @enter="searchMembers"
            />
            <t-button theme="primary" variant="outline" @click="searchMembers">查询</t-button>
            <t-button
              v-permission="'agent_level:update'"
              theme="danger"
              variant="outline"
              :disabled="selectedMemberIds.length === 0"
              :loading="submitting"
              @click="removeSelected"
            >
              移出本组{{ selectedMemberIds.length ? `（${selectedMemberIds.length}）` : '' }}
            </t-button>
          </div>
          <t-table
            row-key="id"
            :data="members"
            :columns="memberColumns"
            :loading="memberLoading"
            :pagination="memberPagination"
            :selected-row-keys="selectedMemberIds"
            cell-empty-content="—"
            size="small"
            @select-change="onMemberSelect"
            @page-change="onMemberPageChange"
          />
          <p class="field-hint">移出 = 取消代理身份（该账号回到普通客户，不再享受任何代理折扣）。</p>
        </t-tab-panel>

        <t-tab-panel value="unassigned" :label="`未归属账号（${unassignedTotal}）`">
          <div class="member-toolbar">
            <t-input
              v-model="unassignedKeyword"
              clearable
              placeholder="搜索未归属任何代理分组的账号"
              @enter="searchUnassigned"
            />
            <t-button theme="primary" variant="outline" @click="searchUnassigned">查询</t-button>
            <t-button
              v-permission="'agent_level:update'"
              theme="primary"
              :disabled="selectedUnassignedIds.length === 0"
              :loading="submitting"
              @click="assignSelected"
            >
              纳入本组{{ selectedUnassignedIds.length ? `（${selectedUnassignedIds.length}）` : '' }}
            </t-button>
          </div>
          <t-table
            row-key="id"
            :data="unassigned"
            :columns="memberColumns"
            :loading="memberLoading"
            :pagination="unassignedPagination"
            :selected-row-keys="selectedUnassignedIds"
            cell-empty-content="—"
            size="small"
            @select-change="onUnassignedSelect"
            @page-change="onUnassignedPageChange"
          />
          <p class="field-hint">纳入本组会覆盖原有归属：若该账号已在别的分组，会从原分组转到本组。</p>
        </t-tab-panel>
      </t-tabs>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule, PageInfo, PrimaryTableCol } from 'tdesign-vue-next'

import {
  assignAgentGroupMembers,
  createAgentLevel,
  deleteAgentLevel,
  getAgentGroupMembers,
  getAgentLevelList,
  updateAgentLevel,
  type AgentLevelInfo,
  type AgentLevelRequest,
  type AgentMemberInfo,
} from '@/api/agent-level'

defineOptions({ name: 'AgentGroupPanel' })

const emit = defineEmits<{ changed: [] }>()

const loading = ref(false)
const submitting = ref(false)
const groups = ref<AgentLevelInfo[]>([])
const unassignedTotal = ref(0)

const statusOptions = [
  { label: '启用', value: 'active' },
  { label: '停用', value: 'disabled' },
]

const columns = computed<PrimaryTableCol<AgentLevelInfo>[]>(() => [
  { colKey: 'name', title: '代理分组', minWidth: 200 },
  { colKey: 'weight', title: '权重', width: 110 },
  { colKey: 'member_count', title: '代理数', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'description', title: '说明', minWidth: 180 },
  { colKey: 'operation', title: '操作', width: 210 },
])

// —— 分组增删改 ——
const formRef = ref<FormInstanceFunctions>()
const formVisible = ref(false)
const editing = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<AgentLevelRequest>({ name: '', code: '', weight: 0, status: 'active', description: '' })

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入分组名称', type: 'error' }],
  code: [{ required: true, message: '请输入分组编码', type: 'error' }],
  status: [{ required: true, message: '请选择状态', type: 'error' }],
}

function openCreate() {
  editing.value = false
  editingId.value = null
  Object.assign(form, { name: '', code: '', weight: 0, status: 'active', description: '' })
  formVisible.value = true
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
  formVisible.value = true
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  submitting.value = true
  try {
    // { name, code, weight, status, description }
    const payload: AgentLevelRequest = {
      name: form.name,
      code: form.code,
      weight: Number(form.weight || 0),
      status: form.status,
      description: form.description || '',
    }
    if (editing.value && editingId.value != null) {
      await updateAgentLevel(editingId.value, payload)
      MessagePlugin.success('代理分组已更新')
    } else {
      await createAgentLevel(payload)
      MessagePlugin.success('代理分组已创建，可在「折扣设置」里配折扣')
    }
    formVisible.value = false
    await loadGroups()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: AgentLevelInfo) {
  try {
    await deleteAgentLevel(row.id)
    MessagePlugin.success('已删除')
    await loadGroups()
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '删除失败')
  }
}

// —— 归属管理 ——
const memberVisible = ref(false)
const memberTab = ref<'members' | 'unassigned'>('members')
const currentGroup = ref<AgentLevelInfo | null>(null)

const memberColumns: PrimaryTableCol<AgentMemberInfo>[] = [
  { colKey: 'row-select', type: 'multiple', width: 50 },
  { colKey: 'username', title: '账号', minWidth: 180 },
  { colKey: 'real_name', title: '姓名', width: 120 },
  { colKey: 'email', title: '邮箱', minWidth: 180 },
  { colKey: 'phone', title: '手机', width: 130 },
  { colKey: 'status', title: '状态', width: 90 },
]

const members = ref<AgentMemberInfo[]>([])
const unassigned = ref<AgentMemberInfo[]>([])
const memberLoading = ref(false)
const memberKeyword = ref('')
const unassignedKeyword = ref('')
const memberTotal = ref(0)
const selectedMemberIds = ref<Array<string | number>>([])
const selectedUnassignedIds = ref<Array<string | number>>([])

const memberPagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true })
const unassignedPagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true })

function openMembers(row: AgentLevelInfo) {
  currentGroup.value = row
  memberTab.value = 'members'
  memberKeyword.value = ''
  unassignedKeyword.value = ''
  selectedMemberIds.value = []
  selectedUnassignedIds.value = []
  memberPagination.current = 1
  unassignedPagination.current = 1
  memberVisible.value = true
  void Promise.all([loadMembers(), loadUnassigned()])
}

// 页头「未归属账号」：直接打开归属面板，落在未归属页签上（没有分组也能先看候选池）。
function openAssignPanel() {
  currentGroup.value = groups.value[0] || null
  memberTab.value = 'unassigned'
  memberKeyword.value = ''
  unassignedKeyword.value = ''
  selectedMemberIds.value = []
  selectedUnassignedIds.value = []
  memberPagination.current = 1
  unassignedPagination.current = 1
  memberVisible.value = true
  void Promise.all([loadUnassigned(), currentGroup.value ? loadMembers() : Promise.resolve()])
}

async function loadMembers() {
  if (!currentGroup.value) {
    members.value = []
    memberPagination.total = 0
    memberTotal.value = 0
    return
  }
  memberLoading.value = true
  try {
    const data = await getAgentGroupMembers(currentGroup.value.id, {
      page: memberPagination.current,
      page_size: memberPagination.pageSize,
      keyword: memberKeyword.value || undefined,
    })
    members.value = data.items || []
    memberPagination.total = data.meta.total
    memberTotal.value = data.meta.total
    unassignedTotal.value = data.unassigned_total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载成员失败')
  } finally {
    memberLoading.value = false
  }
}

async function loadUnassigned() {
  // 未归属池与分组无关：即便一个分组都还没建，运营也应该能看到候选账号。
  // ID 传 0 走「未归属」分支（服务端在该分支下不校验分组存在性）。
  memberLoading.value = true
  try {
    const data = await getAgentGroupMembers(currentGroup.value?.id ?? 0, {
      page: unassignedPagination.current,
      page_size: unassignedPagination.pageSize,
      keyword: unassignedKeyword.value || undefined,
      unassigned: true,
    })
    unassigned.value = data.items || []
    unassignedPagination.total = data.meta.total
    unassignedTotal.value = data.unassigned_total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载未归属账号失败')
  } finally {
    memberLoading.value = false
  }
}

function searchMembers() {
  memberPagination.current = 1
  void loadMembers()
}

function searchUnassigned() {
  unassignedPagination.current = 1
  void loadUnassigned()
}

function onMemberPageChange(info: PageInfo) {
  memberPagination.current = info.current
  memberPagination.pageSize = info.pageSize
  void loadMembers()
}

function onUnassignedPageChange(info: PageInfo) {
  unassignedPagination.current = info.current
  unassignedPagination.pageSize = info.pageSize
  void loadUnassigned()
}

function onMemberSelect(keys: Array<string | number>) {
  selectedMemberIds.value = keys
}

function onUnassignedSelect(keys: Array<string | number>) {
  selectedUnassignedIds.value = keys
}

function reportSkipped(skipped: Array<{ user_id: number; reason: string }>) {
  if (!skipped?.length) return
  MessagePlugin.warning(skipped.map((item) => item.reason).join('；'))
}

async function assignSelected() {
  if (selectedUnassignedIds.value.length === 0) return
  if (!currentGroup.value) {
    MessagePlugin.warning('请先创建一个代理分组，再把账号纳入该分组')
    return
  }
  submitting.value = true
  try {
    const ids = selectedUnassignedIds.value.map((id) => Number(id))
    const result = await assignAgentGroupMembers(currentGroup.value.id, ids, 'assign')
    MessagePlugin.success(`已纳入 ${result.changed} 个账号`)
    reportSkipped(result.skipped || [])
    selectedUnassignedIds.value = []
    await Promise.all([loadGroups(), loadMembers(), loadUnassigned()])
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '纳入失败')
  } finally {
    submitting.value = false
  }
}

async function removeSelected() {
  if (!currentGroup.value || selectedMemberIds.value.length === 0) return
  submitting.value = true
  try {
    const ids = selectedMemberIds.value.map((id) => Number(id))
    const result = await assignAgentGroupMembers(currentGroup.value.id, ids, 'remove')
    MessagePlugin.success(`已移出 ${result.changed} 个账号（取消代理身份）`)
    reportSkipped(result.skipped || [])
    selectedMemberIds.value = []
    await Promise.all([loadGroups(), loadMembers(), loadUnassigned()])
    emit('changed')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '移出失败')
  } finally {
    submitting.value = false
  }
}

async function loadGroups() {
  loading.value = true
  try {
    // 分组数量是运营手配的档位（个位数），一次取全即可，不做分页控件。
    const data = await getAgentLevelList({ page: 1, page_size: 100 })
    groups.value = data.items || []
    // 未归属总数由成员接口带出；一个分组都没有时用 ID 0 探（未归属分支不校验分组）。
    const probe = await getAgentGroupMembers(groups.value[0]?.id ?? 0, { page: 1, page_size: 1 })
    unassignedTotal.value = probe.unassigned_total
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载代理分组失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadGroups)

defineExpose({ openCreate, reload: loadGroups })
</script>

<style scoped>
.ag-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.surface-card {
  border-radius: var(--hs-radius-lg);
  background: var(--hs-surface-1);
  box-shadow: none;
}

.ag-card {
  padding: 16px 20px;
}

.ag-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.ag-head__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.ag-title {
  margin: 0;
  font-size: 16px;
}

.ag-desc {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.ag-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.muted {
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}

.field-hint {
  display: block;
  margin: 6px 0 12px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.member-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 12px 0;
  flex-wrap: wrap;
}

.member-toolbar > :first-child {
  flex: 1;
  min-width: 220px;
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }

  .ag-head {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
