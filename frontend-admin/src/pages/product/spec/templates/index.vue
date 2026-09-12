<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><AppIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">规格模板</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon /></template>刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon /></template>新增模板
        </t-button>
      </t-space>
    </header>

    <section class="filter-card surface-card">
      <div class="filter-card__head">
        <h3 class="card-title">筛选</h3>
      </div>
      <div class="filter-card__grid">
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" placeholder="规格名称" clearable @enter="search" />
        </div>
        <div class="field">
          <span class="field__label">规格族</span>
          <t-select v-model="filters.spec_family" clearable placeholder="全部" :options="specFamilyOptions" />
        </div>
        <div class="field">
          <span class="field__label">状态</span>
          <t-select v-model="filters.status" clearable placeholder="全部" :options="statusOptions" />
        </div>
      </div>
      <div class="filter-card__actions">
        <t-space size="small">
          <t-button theme="primary" @click="search"><template #icon><SearchIcon /></template>查询</t-button>
          <t-button variant="outline" @click="resetFilters">重置</t-button>
        </t-space>
      </div>
    </section>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">规格模板列表</h3>
        <span class="table-card__meta">共 {{ pagination.total }} 条</span>
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
        <template #spec="{ row }">
          <div class="product-cell"><span class="cell-strong">{{ row.name }}</span><span class="product-sub">{{ familyLabel(row.spec_family) }}</span></div>
        </template>
        <template #specs="{ row }">
          <span>{{ row.cpu }}核 / {{ row.memory }}G / {{ row.disk }}G</span>
        </template>
        <template #bandwidth="{ row }"><span>{{ row.bandwidth === 0 ? '不限' : row.bandwidth + ' Mbps' }}</span></template>
        <template #status="{ row }"><t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">{{ row.status === 1 ? '启用' : '停用' }}</t-tag></template>
        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="danger" hover="color" @click="remove(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty><t-empty description="暂无规格模板，请新增" /></template>
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

    <t-dialog v-model:visible="dialogVisible" :header="form.id ? '编辑规格模板' : '新增规格模板'" width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary' }" :cancel-btn="{ content: '取消' }" @confirm="save" @close="closeDialog">
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="规格名称" name="name"><t-input v-model="form.name" placeholder="如：通用型-2核4G" /></t-form-item>
          <t-form-item label="规格族" name="spec_family"><t-select v-model="form.spec_family" :options="specFamilyOptions" /></t-form-item>
          <t-form-item label="CPU（核）" name="cpu"><t-input-number v-model="form.cpu" :min="1" theme="column" /></t-form-item>
          <t-form-item label="内存（GB）" name="memory"><t-input-number v-model="form.memory" :min="1" :precision="1" theme="column" /></t-form-item>
          <t-form-item label="系统盘（GB）" name="disk"><t-input-number v-model="form.disk" :min="1" theme="column" /></t-form-item>
          <t-form-item label="带宽（Mbps）" name="bandwidth"><t-input-number v-model="form.bandwidth" :min="0" theme="column" /></t-form-item>
          <t-form-item label="磁盘类型" name="disk_type"><t-select v-model="form.disk_type" :options="diskTypeOptions" /></t-form-item>
          <t-form-item label="参考售价（元）" name="price"><t-input-number v-model="form.price" :min="0" :precision="2" theme="column" /></t-form-item>
          <t-form-item label="排序" name="sort_order"><t-input-number v-model="form.sort_order" theme="column" /></t-form-item>
          <t-form-item label="状态" name="status"><t-select v-model="form.status" :options="statusOptions" /></t-form-item>
        </div>
        <t-form-item label="操作系统" name="os"><t-input v-model="form.os" placeholder="如：Linux / Windows" /></t-form-item>
        <t-form-item label="适用场景" name="description"><t-textarea v-model="form.description" :autosize="{ minRows: 2, maxRows: 4 }" /></t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createSpecTemplate, deleteSpecTemplate, getSpecTemplateList, updateSpecTemplate } from '@/api/product'
import type { SpecTemplateInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'

defineOptions({ name: 'ProductSpecTemplates' })

const items = ref<SpecTemplateInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()

const filters = reactive<{ keyword?: string; spec_family?: string; status?: number }>({})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(load)

const specFamilyOptions = [
  { label: '通用型', value: 'general' },
  { label: '计算型', value: 'compute' },
  { label: '内存型', value: 'memory' },
  { label: '存储型', value: 'storage' },
  { label: 'GPU型', value: 'gpu' },
]
const diskTypeOptions = [
  { label: 'SSD', value: 'ssd' },
  { label: 'HDD', value: 'hdd' },
]
const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const columns: PrimaryTableCol<SpecTemplateInfo>[] = [
  { colKey: 'spec', title: '规格', minWidth: 180 },
  { colKey: 'specs', title: '配置', width: 150 },
  { colKey: 'bandwidth', title: '带宽', width: 100 },
  { colKey: 'os', title: '操作系统', width: 120 },
  { colKey: 'price', title: '参考售价', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 120, fixed: 'right' as const, align: 'center' as const },
]

function familyLabel(v: string): string {
  return specFamilyOptions.find((o) => o.value === v)?.label || v
}

async function load() {
  loading.value = true
  try {
    const data = await getSpecTemplateList({
      keyword: filters.keyword,
      spec_family: filters.spec_family,
      status: filters.status,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    items.value = data.items
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}

function search() {
  resetPage()
}
function resetFilters() {
  filters.keyword = undefined
  filters.spec_family = undefined
  filters.status = undefined
  resetPage()
}

const dialogVisible = ref(false)
const form = reactive<{ id: number; name: string; spec_family: string; cpu: number; memory: number; disk: number; bandwidth: number; disk_type: string; os: string; price: number; sort_order: number; description: string; status: number }>(emptyForm())

function emptyForm() {
  return { id: 0, name: '', spec_family: 'general', cpu: 2, memory: 4, disk: 50, bandwidth: 5, disk_type: 'ssd', os: '', price: 0, sort_order: 0, description: '', status: 1 }
}
function openCreate() {
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}
function openEdit(row: SpecTemplateInfo) {
  Object.assign(form, {
    id: row.id, name: row.name, spec_family: row.spec_family, cpu: row.cpu, memory: row.memory,
    disk: row.disk, bandwidth: row.bandwidth, disk_type: row.disk_type, os: row.os,
    price: row.price, sort_order: row.sort_order, description: row.description, status: row.status,
  })
  dialogVisible.value = true
}
function closeDialog() {
  dialogVisible.value = false
}
async function save() {
  try {
    const payload = {
      name: form.name, spec_family: form.spec_family, cpu: form.cpu, memory: form.memory, disk: form.disk,
      bandwidth: form.bandwidth, disk_type: form.disk_type, os: form.os, price: form.price,
      sort_order: form.sort_order, description: form.description, status: form.status,
    }
    if (form.id) {
      await updateSpecTemplate(form.id, payload)
    } else {
      await createSpecTemplate(payload)
    }
    MessagePlugin.success('已保存')
    closeDialog()
    load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  }
}
async function remove(row: SpecTemplateInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除规格模板',
    body: `确认删除「${row.name}」？删除后不可恢复。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteSpecTemplate(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        load()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

onMounted(load)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: SpecTemplateInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      void remove(row)
      break
  }
}
</script>

<style lang="css">
@import '../../shared.css';
</style>
