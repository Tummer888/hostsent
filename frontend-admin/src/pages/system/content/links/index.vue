<template>
  <div class="page-body content-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <LinkIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">友情链接</h2>
          <p class="page-header__desc">展示在官网页脚的合作伙伴链接</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadData">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon aria-hidden="true" /></template>
          新建链接
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">名称</span>
        <t-input v-model="filters.keyword" placeholder="链接名称" clearable @enter="handleSearch" />
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
          <t-button variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">链接列表</h3>
        <span class="table-card__meta">共 {{ total }} 条 · 展示顺序按排序值升序</span>
      </div>
      <t-table
        row-key="id"
        :data="list"
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
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">{{ row.url }}</span>
          </div>
        </template>
        <template #open_in_new="{ row }">
          <t-tag v-if="row.open_in_new" theme="primary" variant="light" size="small" shape="round">新窗口</t-tag>
          <t-tag v-else theme="default" variant="light" size="small" shape="round">当前窗口</t-tag>
        </template>
        <template #sort_order="{ row }">
          <span class="cell-muted">{{ row.sort_order }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="categoryStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ categoryStatusLabel(row.status) }}
          </t-tag>
        </template>
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
              <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty>
          <t-empty description="暂无友情链接" />
        </template>
      </t-table>

      <MobilePagination
        v-if="isMobile"
        :current="page.current"
        :page-size="page.size"
        :total="total"
        @go="goMobilePage"
        @page-size="handleMobilePageSizeChange"
      />
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editingId ? '编辑链接' : '新建链接'"
      width="520px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      :loading="saving"
      @confirm="handleSubmit"
      @close="dialogVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="名称" name="name" :rules="[{ required: true, message: '请输入链接名称' }]">
          <t-input v-model="form.name" placeholder="请输入链接名称" :maxlength="64" />
        </t-form-item>
        <t-form-item label="链接地址" name="url" :rules="[{ required: true, message: '请输入链接地址' }]">
          <t-input v-model="form.url" placeholder="https://example.com" />
        </t-form-item>
        <t-form-item label="Logo 地址" name="logo">
          <t-input v-model="form.logo" placeholder="可选，https://..." />
        </t-form-item>
        <t-form-item label="描述" name="description">
          <t-input v-model="form.description" placeholder="可选" :maxlength="255" />
        </t-form-item>
        <t-form-item label="排序" name="sort_order">
          <t-input-number v-model="form.sort_order" :min="0" placeholder="数值越小越靠前" />
        </t-form-item>
        <t-form-item label="打开方式" name="open_in_new">
          <t-switch v-model="form.open_in_new" />
          <p class="field-help">开启后点击链接在新窗口打开（外链建议开启）。</p>
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="form.status">
            <t-radio value="active">启用</t-radio>
            <t-radio value="disabled">停用</t-radio>
          </t-radio-group>
          <p class="field-help">停用后不在官网页脚展示。</p>
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { AddIcon, LinkIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createFriendlyLink,
  deleteFriendlyLink,
  getFriendlyLinks,
  updateFriendlyLink,
  type CategoryStatus,
  type FriendlyLinkItem,
} from '@/api/content'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

import { categoryStatusLabel, categoryStatusOptions, categoryStatusTheme } from '../constants'

defineOptions({ name: 'ContentLinks' })

const { isMobile } = useIsMobile()
const loading = ref(false)
const saving = ref(false)
const list = ref<FriendlyLinkItem[]>([])
const total = ref(0)
const filters = reactive({ keyword: '', status: '' })
const page = reactive({ current: 1, size: 10 })

const statusOptions = categoryStatusOptions()

const columns = computed<PrimaryTableCol[]>(() => [
  { colKey: 'name', title: '名称', minWidth: 200 },
  { colKey: 'open_in_new', title: '打开方式', width: 110, align: 'center' },
  { colKey: 'sort_order', title: '排序', width: 80, align: 'center' },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 150, fixed: 'right', align: 'center' },
])

const pagination = computed(() => ({
  current: page.current,
  pageSize: page.size,
  total: total.value,
  showJumper: true,
}))

async function loadData() {
  loading.value = true
  try {
    const resp = await getFriendlyLinks({
      keyword: filters.keyword || undefined,
      status: filters.status || undefined,
      page: page.current,
      page_size: page.size,
    })
    list.value = resp.list || []
    total.value = resp.total ?? 0
  } catch (e) {
    MessagePlugin.error((e as Error).message || '加载友情链接失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.current = 1
  loadData()
}

function handleReset() {
  filters.keyword = ''
  filters.status = ''
  page.current = 1
  loadData()
}

function handlePageChange(info: PageInfo) {
  page.current = info.current
  page.size = info.pageSize
  loadData()
}

function goMobilePage(target: number) {
  const totalPages = Math.max(1, Math.ceil(total.value / page.size))
  const clamped = Math.min(Math.max(target, 1), totalPages)
  if (clamped === page.current) return
  void applyMobilePage(clamped, page.size)
}

async function applyMobilePage(current: number, pageSize: number) {
  page.current = current
  page.size = pageSize
  await loadData()
}

function handleMobilePageSizeChange(pageSize: number) {
  void applyMobilePage(1, pageSize)
}

// —— 新增 / 编辑 ——
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<{
  name: string
  url: string
  logo: string
  description: string
  sort_order: number
  open_in_new: boolean
  status: CategoryStatus
}>({
  name: '',
  url: '',
  logo: '',
  description: '',
  sort_order: 0,
  open_in_new: true,
  status: 'active',
})

function resetForm() {
  form.name = ''
  form.url = ''
  form.logo = ''
  form.description = ''
  form.sort_order = 0
  form.open_in_new = true
  form.status = 'active'
}

function openCreate() {
  editingId.value = null
  resetForm()
  dialogVisible.value = true
}

function openEdit(row: FriendlyLinkItem) {
  editingId.value = row.id
  form.name = row.name
  form.url = row.url
  form.logo = row.logo
  form.description = row.description
  form.sort_order = row.sort_order
  form.open_in_new = !!row.open_in_new
  form.status = row.status
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入链接名称')
    return
  }
  if (!form.url.trim()) {
    MessagePlugin.warning('请输入链接地址')
    return
  }
  saving.value = true
  try {
    const payload = {
      name: form.name.trim(),
      url: form.url.trim(),
      logo: form.logo.trim(),
      description: form.description.trim(),
      sort_order: form.sort_order,
      open_in_new: form.open_in_new,
      status: form.status,
    }
    if (editingId.value) {
      await updateFriendlyLink(editingId.value, payload)
      MessagePlugin.success('链接已更新')
    } else {
      await createFriendlyLink(payload)
      MessagePlugin.success('链接已创建')
    }
    dialogVisible.value = false
    loadData()
  } catch (e) {
    MessagePlugin.error((e as Error).message || '保存链接失败')
  } finally {
    saving.value = false
  }
}

function handleDelete(row: FriendlyLinkItem) {
  const dialog = DialogPlugin.confirm({
    header: '删除友情链接',
    body: `确认删除「${row.name}」吗？删除后官网页脚将不再展示。`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteFriendlyLink(row.id)
        MessagePlugin.success('链接已删除')
        dialog.destroy()
        loadData()
      } catch (e) {
        MessagePlugin.error((e as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

function handleMobileAction(value: string | number | Record<string, unknown>, row: FriendlyLinkItem) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  if (action === 'edit') openEdit(row)
  if (action === 'delete') handleDelete(row)
}

onMounted(loadData)
</script>

<style lang="css">
@import '../shared.css';
</style>
