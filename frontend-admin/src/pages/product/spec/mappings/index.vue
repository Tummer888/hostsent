<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">规格映射</h2>
          <p class="page-header__desc">将上游供应商的规格编号映射到平台规格模板，保证商品规格统一识别。</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadMappings">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建映射
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选条件</h3>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">供应商类型</span>
          <t-select v-model="filters.provider_type" clearable placeholder="全部供应商" :options="providerTypeOptions" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="上游规格 ID / 规格名称" clearable @enter="handleSearch" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusOptions" />
        </div>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">映射列表</h3>
        <span class="table-card__meta">共 {{ total }} 条映射</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        bordered
        table-layout="fixed"
        cell-empty-content="—"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #provider_type="{ row }">
          <span>{{ providerTypeLabel(row.provider_type) }}</span>
        </template>

        <template #upstream="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.upstream_spec_id }}</span>
            <span class="product-sub">{{ row.upstream_name }}</span>
          </div>
        </template>

        <template #platform="{ row }">
          <div class="product-cell">
            <span class="cell-strong">{{ row.platform_name || '—' }}</span>
            <span class="product-sub">模板 #{{ row.platform_spec_id || '—' }}</span>
          </div>
        </template>

        <template #spec="{ row }">
          <span>{{ row.cpu }}C / {{ row.memory }}G / {{ row.disk }}G</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
            {{ statusTag(row.status).text }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link theme="primary" hover="color" @click="openBind(row)">绑定平台规格</t-link>
            <t-link
              v-if="row.status === 1"
              theme="warning"
              hover="color"
              @click="handleToggleStatus(row)"
            >停用</t-link>
            <t-link
              v-else
              theme="success"
              hover="color"
              @click="handleToggleStatus(row)"
            >启用</t-link>
            <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无规格映射，请先新建映射或绑定平台规格" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="formVisible"
      :header="editing ? '编辑规格映射' : '新建规格映射'"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="formVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="供应商类型" name="provider_type">
          <t-select v-model="form.provider_type" :options="providerTypeOptions" placeholder="请选择供应商类型" />
        </t-form-item>
        <t-form-item label="上游规格 ID" name="upstream_spec_id">
          <t-input v-model="form.upstream_spec_id" placeholder="如 cloud.s3.xlarge" />
        </t-form-item>
        <t-form-item label="上游规格名称" name="upstream_name">
          <t-input v-model="form.upstream_name" placeholder="选填，上游规格展示名称" />
        </t-form-item>
        <t-form-item label="CPU（核）" name="cpu">
          <t-input-number v-model="form.cpu" :min="0" theme="column" placeholder="CPU 核数" />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="内存（GB）" name="memory">
            <t-input-number v-model="form.memory" :min="0" theme="column" placeholder="内存 GB" />
          </t-form-item>
          <t-form-item label="磁盘（GB）" name="disk">
            <t-input-number v-model="form.disk" :min="0" theme="column" placeholder="磁盘 GB" />
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="bindVisible"
      header="绑定平台规格"
      width="460px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleBind"
      @close="bindVisible = false"
    >
      <p style="margin-bottom: var(--space-md); font-size: 13px; color: var(--color-muted-foreground);">
        为「{{ bindForm.upstreamName }}」({{ bindForm.upstreamSpecId }}) 绑定对应的平台规格模板。
      </p>
      <t-form label-align="top" @submit.prevent>
        <t-form-item label="平台规格模板" name="platform_spec_id">
          <t-select
            v-model="bindForm.platformSpecId"
            :options="templateOptions"
            placeholder="请选择平台规格模板"
            filterable
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProviderTypes } from '@/api/admin'
import {
  bindSpecMapping,
  createSpecMapping,
  deleteSpecMapping,
  getSpecMappingList,
  getSpecTemplateList,
  updateSpecMapping,
} from '@/api/product'
import { formatTime } from '@/pages/product/constants'
import type { ProviderTypeItem, SpecMappingInfo, SpecTemplateInfo } from '@/types/interface'

defineOptions({ name: 'ProductSpecMappings' })

const loading = ref(false)
const list = ref<SpecMappingInfo[]>([])
const total = ref(0)

const providerTypeOptions = ref<{ label: string; value: string }[]>([])
const statusOptions = ref<{ label: string; value: number }[]>([
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
])

const filters = reactive<{ provider_type: string | undefined; keyword: string | undefined; status: number | undefined }>({
  provider_type: undefined,
  keyword: undefined,
  status: undefined,
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const columns: PrimaryTableCol<SpecMappingInfo>[] = [
  { colKey: 'provider_type', title: '供应商类型', width: 120 },
  { colKey: 'upstream', title: '上游规格', minWidth: 160 },
  { colKey: 'platform', title: '平台规格', minWidth: 160 },
  { colKey: 'spec', title: '规格', width: 130 },
  { colKey: 'status', title: '状态', width: 80 },
  { colKey: 'created_at', title: '创建时间', width: 160 },
  {
    colKey: 'action',
    title: '操作',
    width: 220,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

function providerTypeLabel(type: string): string {
  return providerTypeOptions.value.find((item) => item.value === type)?.label || type || '—'
}

function statusTag(status: number): { theme: 'success' | 'danger' | 'default'; text: string } {
  return status === 1 ? { theme: 'success', text: '启用' } : { theme: 'default', text: '停用' }
}

async function loadProviderTypes() {
  try {
    const data = await getProviderTypes()
    providerTypeOptions.value = data.map((item: ProviderTypeItem) => ({ label: item.name, value: item.type }))
  } catch {
    /* 忽略供应商类型加载失败 */
  }
}

async function loadMappings() {
  loading.value = true
  try {
    const data = await getSpecMappingList({
      provider_type: filters.provider_type,
      keyword: filters.keyword,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    list.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载规格映射失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadMappings()
}

function handleSearch() {
  pagination.current = 1
  loadMappings()
}

function handleResetFilters() {
  filters.provider_type = undefined
  filters.keyword = undefined
  filters.status = undefined
  pagination.current = 1
  loadMappings()
}

// ---- 新建 / 编辑 ----
const formVisible = ref(false)
const editing = ref(false)
let editingId = 0
const form = reactive<{
  provider_type: string | undefined
  upstream_spec_id: string
  upstream_name: string
  cpu: number
  memory: number
  disk: number
}>({
  provider_type: undefined,
  upstream_spec_id: '',
  upstream_name: '',
  cpu: 0,
  memory: 0,
  disk: 0,
})

function openCreate() {
  editing.value = false
  editingId = 0
  form.provider_type = undefined
  form.upstream_spec_id = ''
  form.upstream_name = ''
  form.cpu = 0
  form.memory = 0
  form.disk = 0
  formVisible.value = true
}

function openEdit(row: SpecMappingInfo) {
  editing.value = true
  editingId = row.id
  form.provider_type = row.provider_type
  form.upstream_spec_id = row.upstream_spec_id
  form.upstream_name = row.upstream_name
  form.cpu = row.cpu
  form.memory = row.memory
  form.disk = row.disk
  formVisible.value = true
}

async function handleSave() {
  if (!form.provider_type || !form.upstream_spec_id) {
    MessagePlugin.warning('请填写供应商类型与上游规格 ID')
    return
  }
  const payload = {
    provider_type: form.provider_type,
    upstream_spec_id: form.upstream_spec_id,
    upstream_name: form.upstream_name || undefined,
    cpu: form.cpu,
    memory: form.memory,
    disk: form.disk,
  }
  try {
    if (editing.value) {
      await updateSpecMapping(editingId, payload)
    } else {
      await createSpecMapping(payload)
    }
    MessagePlugin.success('已保存')
    formVisible.value = false
    loadMappings()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  }
}

// ---- 绑定平台规格 ----
const bindVisible = ref(false)
let bindId = 0
const bindForm = reactive<{ platformSpecId: number | undefined; upstreamSpecId: string; upstreamName: string }>({
  platformSpecId: undefined,
  upstreamSpecId: '',
  upstreamName: '',
})
const templateOptions = ref<{ label: string; value: number }[]>([])

async function loadTemplates() {
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 200 })
    templateOptions.value = data.items.map((item: SpecTemplateInfo) => ({ label: item.name, value: item.id }))
  } catch {
    /* 忽略模板加载失败 */
  }
}

function openBind(row: SpecMappingInfo) {
  bindId = row.id
  bindForm.platformSpecId = row.platform_spec_id || undefined
  bindForm.upstreamSpecId = row.upstream_spec_id
  bindForm.upstreamName = row.upstream_name
  loadTemplates()
  bindVisible.value = true
}

async function handleBind() {
  if (!bindForm.platformSpecId) {
    MessagePlugin.warning('请选择平台规格模板')
    return
  }
  const template = templateOptions.value.find((item) => item.value === bindForm.platformSpecId)
  try {
    await bindSpecMapping(bindId, {
      platform_spec_id: bindForm.platformSpecId,
      platform_name: template?.label,
    })
    MessagePlugin.success('绑定成功')
    bindVisible.value = false
    loadMappings()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '绑定失败')
  }
}

// ---- 启用 / 停用 / 删除 ----
async function handleToggleStatus(row: SpecMappingInfo) {
  try {
    await updateSpecMapping(row.id, {
      provider_type: row.provider_type,
      upstream_spec_id: row.upstream_spec_id,
      upstream_name: row.upstream_name,
      cpu: row.cpu,
      memory: row.memory,
      disk: row.disk,
      status: row.status === 1 ? 0 : 1,
    })
    MessagePlugin.success('状态已更新')
    loadMappings()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新状态失败')
  }
}

async function handleDelete(row: SpecMappingInfo) {
  try {
    await deleteSpecMapping(row.id)
    MessagePlugin.success('已删除')
    loadMappings()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除失败')
  }
}

onMounted(() => {
  loadProviderTypes()
  loadMappings()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>
