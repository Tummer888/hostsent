<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><AppIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">平台配置项</h2>
          <p class="page-header__desc">
            按对接平台（魔方云）维护它支持的配置项：改中文名/必选/默认值，导入镜像与自定义取值，
            供「规格模板（配置档）」勾选后生成商品的可选配置项。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="reload">
          <template #icon><RefreshIcon /></template>刷新
        </t-button>
        <t-button theme="primary" variant="outline" :loading="syncing" @click="syncCatalog">
          同步适配器目录
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon /></template>新增配置项
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">平台渠道</span>
        <t-select
          v-model="providerId"
          placeholder="选择算力平台渠道"
          :options="providerOptions"
          :loading="providerLoading"
          @change="onProviderChange"
        />
      </div>
      <div class="field">
        <span class="field__label">平台类型</span>
        <t-input v-model="providerType" placeholder="如 mofangyun" @enter="load" />
      </div>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="keyword" placeholder="参数名 / 中文名" clearable />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="load">查询</t-button>
          <t-button
            variant="outline"
            :disabled="!providerId"
            :loading="refreshing"
            @click="refreshValues"
          >
            从平台刷新取值
          </t-button>
        </t-space>
      </template>
    </FilterCard>

    <t-alert v-if="!providerType" theme="warning" message="请先选择平台渠道或填写平台类型，目录按平台声明。" />

    <section v-else class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">可配置项目录（{{ filtered.length }} 项）</h3>
        <span class="table-card__meta">
          已入库 {{ inDbCount }} 项 · 待同步 {{ filtered.length - inDbCount }} 项
        </span>
      </div>

      <div v-for="group in groupedItems" :key="group.name" class="opt-group">
        <div class="opt-group__head">
          <span class="opt-group__name">{{ group.name || '未分组' }}</span>
          <span class="opt-group__count">{{ group.items.length }} 项</span>
        </div>
        <t-table
          row-key="option_key"
          :data="group.items"
          :columns="columns"
          size="small"
          hover
          cell-empty-content="—"
        >
          <template #option="{ row }">
            <div class="product-cell">
              <span class="cell-strong">{{ row.label || row.option_key }}</span>
              <span class="product-sub">
                <code>{{ row.option_key }}</code>
                <t-tag v-if="row.required" theme="error" variant="light" size="small" shape="round">必选</t-tag>
                <t-tag v-if="row.multi_value" theme="primary" variant="light" size="small" shape="round">可多选</t-tag>
                <t-tag v-if="row.hidden" theme="default" variant="light" size="small" shape="round">已隐藏</t-tag>
                <t-tag v-if="!row.id" theme="warning" variant="light" size="small" shape="round">未入库</t-tag>
              </span>
            </div>
          </template>
          <template #widget="{ row }">
            {{ widgetLabel(row.widget) }}
            <span v-if="row.unit" class="opt-unit">/ {{ row.unit }}</span>
          </template>
          <template #values="{ row }">
            <t-link theme="primary" hover="color" @click="openValues(row)">
              {{ (row.values || []).length }} 个取值
            </t-link>
            <span v-if="row.min_value != null || row.max_value != null" class="product-sub">
              范围 {{ row.min_value ?? '—' }} ~ {{ row.max_value ?? '—' }}
            </span>
          </template>
          <template #default_value="{ row }">
            <span>{{ row.default_value || '—' }}</span>
          </template>
          <template #source="{ row }">
            <t-tag :theme="row.source === 'custom' ? 'primary' : 'default'" variant="light" size="small" shape="round">
              {{ row.source === 'custom' ? '自定义' : '适配器' }}
            </t-tag>
          </template>
          <template #action="{ row }">
            <div class="action-cell">
              <MobileAction
                v-if="isMobile"
                :options="buildMobileActionOptions([
                  { content: '编辑', value: 'edit', theme: 'default' },
                  { content: '取值', value: 'values', theme: 'default' },
                  { content: '删除', value: 'delete', theme: 'error' },
                ])"
                @select="(value) => handleMobileAction(value, row)"
              />
              <template v-else>
                <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
                <t-link theme="primary" hover="color" @click="openValues(row)">取值</t-link>
                <t-link v-if="row.id" theme="danger" hover="color" @click="removeSpec(row)">删除</t-link>
              </template>
            </div>
          </template>
          <template #empty><t-empty description="暂无配置项" /></template>
        </t-table>
      </div>
    </section>

    <!-- 编辑配置项 -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="form.id ? '编辑配置项' : '新增配置项'"
      width="720px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="saveSpec"
      @close="closeDialog"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="参数名（平台写键）" name="option_key">
            <t-input
              v-model="form.option_key"
              :disabled="!!form.id"
              placeholder="如 cpu / memory / system_disk_size"
            />
          </t-form-item>
          <t-form-item label="中文名" name="label">
            <t-input v-model="form.label" placeholder="如 CPU / 操作系统" />
          </t-form-item>
          <t-form-item label="分组" name="group_name">
            <t-input v-model="form.group_name" placeholder="如 基础配置 / 网络 / 高级" />
          </t-form-item>
          <t-form-item label="控件" name="widget">
            <t-select v-model="form.widget" :options="widgetOptions" />
          </t-form-item>
          <t-form-item label="取值来源" name="value_source">
            <t-select v-model="form.value_source" :options="valueSourceOptions" />
          </t-form-item>
          <t-form-item label="单位" name="unit">
            <t-input v-model="form.unit" placeholder="核 / GB / Mbps / 个" />
          </t-form-item>
          <t-form-item label="默认值" name="default_value">
            <t-input v-model="form.default_value" placeholder="未传递时平台的默认值" />
          </t-form-item>
          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="form.sort_order" theme="column" />
          </t-form-item>
          <t-form-item label="数量下限" name="min_value">
            <t-input-number v-model="form.min_value" :allow-input="onlyNumber" theme="column" />
          </t-form-item>
          <t-form-item label="数量上限" name="max_value">
            <t-input-number v-model="form.max_value" :allow-input="onlyNumber" theme="column" />
          </t-form-item>
        </div>
        <t-form-item label="必选">
          <t-switch v-model="form.required" />
          <span class="form-hint">平台不传会拒绝开通的参数（如 area/os/cpu/memory）。</span>
        </t-form-item>
        <t-form-item label="允许多选（配置档里可勾多个取值）">
          <t-switch v-model="form.multi_value" />
        </t-form-item>
        <t-form-item label="在目录中隐藏">
          <t-switch v-model="form.hidden" />
        </t-form-item>
        <t-form-item label="说明">
          <t-textarea v-model="form.help" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="取值从哪来、怎么填" />
        </t-form-item>
        <t-form-item v-if="form.value_source === 'static'" label="静态枚举（每行一个：值|显示名）">
          <t-textarea
            v-model="optionsText"
            :autosize="{ minRows: 3, maxRows: 10 }"
            placeholder="2|2核&#10;4|4核&#10;8|8核"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 取值库 -->
    <t-dialog
      v-model:visible="valuesVisible"
      :header="`取值库：${current?.label || current?.option_key || ''}`"
      width="800px"
      :footer="false"
      @close="valuesVisible = false"
    >
      <t-alert theme="info" message="取值 = 下发给平台的原始值；镜像/区域/节点/存储可用「从平台刷新取值」批量灌入，也可手工添加或批量导入。" />
      <div class="values-toolbar">
        <t-input v-model="newValue" placeholder="值|显示名（如 12|Ubuntu 22.04）" @enter="addValue" />
        <t-input v-model="newGroup" placeholder="分组（Ubuntu/Windows，选填）" />
        <t-button theme="primary" :loading="valueSaving" @click="addValue">添加</t-button>
      </div>
      <t-table
        row-key="value"
        :data="currentValues"
        :columns="valueColumns"
        size="small"
        hover
        max-height="420"
        cell-empty-content="—"
      >
        <template #ivalue="{ row }">
          <code>{{ row.value }}</code>
        </template>
        <template #istatus="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '停用' }}
          </t-tag>
        </template>
        <template #iorigin="{ row }">
          {{ row.origin === 'platform' ? '平台拉取' : row.origin === 'adapter' ? '适配器' : '手工' }}
        </template>
        <template #iaction="{ row }">
          <t-link
            v-if="row.status === 'active'"
            theme="danger"
            hover="color"
            @click="offlineValue(row)"
          >停用</t-link>
          <span v-else class="product-sub">已停用</span>
        </template>
        <template #empty><t-empty description="暂无取值，点上方「从平台刷新取值」或手工添加" /></template>
      </t-table>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createOptionSpec,
  deleteOptionSpec,
  deleteOptionValue,
  getOptionCatalog,
  getOptionValues,
  refreshOptionValues,
  syncOptionCatalog,
  updateOptionSpec,
  upsertOptionValue,
} from '@/api/product'
import { getProviderList } from '@/api/admin'
import type { OptionSpecInfo, OptionValueItem, ProviderInfo } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import FilterCard from '@/components/filter-card/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'

defineOptions({ name: 'ProductSpecOptionCatalog' })

const { isMobile } = useIsMobile()

const providerOptions = ref<{ label: string; value: number }[]>([])
const providerLoading = ref(false)
const providerId = ref<number | undefined>(undefined)
const providerType = ref('')
const keyword = ref('')
const loading = ref(false)
const syncing = ref(false)
const refreshing = ref(false)

const items = ref<OptionSpecInfo[]>([])

const widgetOptions = [
  { label: '下拉（可分组）', value: 'select' },
  { label: '单选（按钮组）', value: 'radio' },
  { label: '数量（步进器）', value: 'qty' },
  { label: '开关', value: 'bool' },
]
const valueSourceOptions = [
  { label: '静态枚举（手写候选值）', value: 'static' },
  { label: '手工填 ID（平台面板复制）', value: 'manual' },
  { label: '平台区域', value: 'areas' },
  { label: '平台节点', value: 'nodes' },
  { label: '平台存储', value: 'stores' },
  { label: '平台镜像', value: 'images' },
]

const columns: PrimaryTableCol<OptionSpecInfo>[] = [
  { colKey: 'option', title: '配置项', minWidth: 200 },
  { colKey: 'widget', title: '控件', width: 110 },
  { colKey: 'values', title: '可选值', minWidth: 150 },
  { colKey: 'default_value', title: '默认值', width: 110 },
  { colKey: 'source', title: '来源', width: 90 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 170, fixed: 'right' as const, align: 'center' as const },
]

const valueColumns: PrimaryTableCol<OptionValueItem>[] = [
  { colKey: 'ivalue', title: '值', width: 100 },
  { colKey: 'label', title: '显示名', minWidth: 200 },
  { colKey: 'group_label', title: '分组', width: 110 },
  { colKey: 'istatus', title: '状态', width: 80 },
  { colKey: 'iorigin', title: '来源', width: 90 },
  { colKey: 'iaction', title: '操作', width: 80, align: 'center' as const },
]

function widgetLabel(w: string): string {
  return widgetOptions.find((o) => o.value === w)?.label || w || '下拉'
}

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return items.value
  return items.value.filter(
    (it) =>
      it.option_key.toLowerCase().includes(kw) ||
      (it.label || '').toLowerCase().includes(kw),
  )
})

const inDbCount = computed(() => filtered.value.filter((it) => !!it.id).length)

/** 按 group_name 分组展示，保持后端排序（分组名 + sort_order）。 */
const groupedItems = computed(() => {
  const groups: { name: string; items: OptionSpecInfo[] }[] = []
  for (const it of filtered.value) {
    const name = it.group_name || ''
    let g = groups.find((x) => x.name === name)
    if (!g) {
      g = { name, items: [] }
      groups.push(g)
    }
    g.items.push(it)
  }
  return groups
})

async function loadProviders() {
  providerLoading.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    providerOptions.value = data.items
      .filter((item: ProviderInfo) => item.kind === 'compute')
      .map((item: ProviderInfo) => ({ label: `${item.name}（${item.provider_type}）`, value: item.id }))
  } catch {
    providerOptions.value = []
  } finally {
    providerLoading.value = false
  }
}

function onProviderChange(value?: number | string) {
  const id = Number(value)
  if (!id) return
  const hit = providerOptions.value.find((o) => o.value === id)
  if (hit) {
    // 从渠道名里取出平台类型（"魔方云测试接口（mofangyun）" → mofangyun）。
    const match = hit.label.match(/（([^）]+)）/)
    if (match) providerType.value = match[1]
  }
  void load()
}

async function load() {
  if (!providerType.value.trim() && !providerId.value) {
    items.value = []
    return
  }
  loading.value = true
  try {
    items.value = await getOptionCatalog({
      provider_type: providerType.value.trim() || undefined,
      provider_id: providerId.value,
    })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载平台配置项目录失败')
    items.value = []
  } finally {
    loading.value = false
  }
}

function reload() {
  void load()
}

async function syncCatalog() {
  const type = providerType.value.trim()
  if (!type) {
    MessagePlugin.warning('请先选择平台渠道或填写平台类型')
    return
  }
  syncing.value = true
  try {
    const res = await syncOptionCatalog(type)
    MessagePlugin.success(`已同步 ${res.declared} 项：新增 ${res.created}，跳过 ${res.skipped}`)
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '同步失败')
  } finally {
    syncing.value = false
  }
}

// 目前平台声明里带平台来源的配置项：魔方云口径
const PLATFORM_VALUE_KEYS = ['os', 'area', 'node', 'store']

async function refreshValues() {
  if (!providerId.value) {
    MessagePlugin.warning('请先选择平台渠道（需要渠道才能读平台资源）')
    return
  }
  refreshing.value = true
  try {
    const res = await refreshOptionValues({
      provider_type: providerType.value.trim() || undefined,
      provider_id: providerId.value,
      option_keys: PLATFORM_VALUE_KEYS,
    })
    MessagePlugin.success(`已刷新取值：新增 ${res.created}，更新 ${res.updated}，停用 ${res.offlined}`)
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '刷新失败')
  } finally {
    refreshing.value = false
  }
}

// ===== 编辑配置项 =====
const dialogVisible = ref(false)
const optionsText = ref('')
const form = reactive<{
  id: number
  option_key: string
  label: string
  group_name: string
  widget: string
  value_source: string
  unit: string
  default_value: string
  sort_order: number
  min_value: number | null
  max_value: number | null
  required: boolean
  multi_value: boolean
  hidden: boolean
  help: string
}>(emptyForm())

function emptyForm() {
  return {
    id: 0, option_key: '', label: '', group_name: '', widget: 'select',
    value_source: 'static', unit: '', default_value: '', sort_order: 0,
    min_value: null, max_value: null, required: false, multi_value: false,
    hidden: false, help: '',
  }
}

function onlyNumber(v: string): boolean {
  return /^\d*\.?\d*$/.test(v)
}

function openCreate() {
  Object.assign(form, emptyForm())
  optionsText.value = ''
  dialogVisible.value = true
}

function openEdit(row: OptionSpecInfo) {
  Object.assign(form, {
    id: row.id || 0,
    option_key: row.option_key,
    label: row.label,
    group_name: row.group_name,
    widget: row.widget,
    value_source: row.value_source,
    unit: row.unit,
    default_value: row.default_value,
    sort_order: row.sort_order,
    min_value: row.min_value ?? null,
    max_value: row.max_value ?? null,
    required: row.required,
    multi_value: row.multi_value,
    hidden: row.hidden,
    help: row.help,
  })
  // 静态枚举回填成「值|显示名」多行文本。
  const opts = Array.isArray(row.options) ? (row.options as { value: string; label: string }[]) : []
  optionsText.value = opts.map((o) => `${o.value}|${o.label}`).join('\n')
  dialogVisible.value = true
}

function closeDialog() {
  dialogVisible.value = false
}

/** 解析「值|显示名」多行文本为静态枚举数组。 */
function parseOptionsText(): { value: string; label: string }[] | undefined {
  const lines = optionsText.value.split('\n').map((l) => l.trim()).filter(Boolean)
  if (!lines.length) return undefined
  return lines.map((line) => {
    const idx = line.indexOf('|')
    if (idx < 0) return { value: line, label: line }
    return { value: line.slice(0, idx).trim(), label: line.slice(idx + 1).trim() }
  })
}

async function saveSpec() {
  if (!form.option_key.trim()) {
    MessagePlugin.warning('请填写参数名')
    return
  }
  const payload = {
    provider_type: providerType.value.trim() || undefined,
    option_key: form.option_key.trim(),
    label: form.label,
    group_name: form.group_name,
    widget: form.widget,
    value_source: form.value_source,
    unit: form.unit,
    default_value: form.default_value,
    sort_order: form.sort_order,
    min_value: form.min_value,
    max_value: form.max_value,
    required: form.required,
    multi_value: form.multi_value,
    hidden: form.hidden,
    help: form.help,
    options: form.value_source === 'static' ? parseOptionsText() : undefined,
  }
  try {
    if (form.id) {
      await updateOptionSpec(form.id, payload)
    } else {
      await createOptionSpec(payload)
    }
    MessagePlugin.success('已保存')
    closeDialog()
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  }
}

function removeSpec(row: OptionSpecInfo) {
  if (!row.id) return
  const dialog = DialogPlugin.confirm({
    header: '删除配置项',
    body: `确认删除「${row.label || row.option_key}」及其全部取值？`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteOptionSpec(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        await load()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

// ===== 取值库 =====
const valuesVisible = ref(false)
const current = ref<OptionSpecInfo | null>(null)
const currentValues = ref<OptionValueItem[]>([])
const newValue = ref('')
const newGroup = ref('')
const valueSaving = ref(false)

async function openValues(row: OptionSpecInfo) {
  current.value = row
  valuesVisible.value = true
  await loadValues()
  // 平台来源的配置项若库里没有取值，自动从平台拉一次（否则弹窗是空的）。
  if (!currentValues.value.length && providerId.value) {
    try {
      await refreshOptionValues({
        provider_type: providerType.value.trim() || undefined,
        provider_id: providerId.value,
        option_keys: [row.option_key],
      })
      await loadValues()
    } catch {
      // 平台不可达时保持空列表，由运营手工添加。
    }
  }
}

async function loadValues() {
  if (!current.value) return
  try {
    const list = await getOptionValues({
      provider_type: providerType.value.trim() || undefined,
      provider_id: providerId.value,
      option_key: current.value.option_key,
      include_offline: true,
    })
    currentValues.value = list || []
  } catch {
    currentValues.value = []
  }
}

async function addValue() {
  if (!current.value) return
  const raw = newValue.value.trim()
  if (!raw) {
    MessagePlugin.warning('请输入取值')
    return
  }
  const idx = raw.indexOf('|')
  const value = idx < 0 ? raw : raw.slice(0, idx).trim()
  const label = idx < 0 ? raw : raw.slice(idx + 1).trim()
  valueSaving.value = true
  try {
    await upsertOptionValue({
      provider_type: providerType.value.trim() || undefined,
      option_key: current.value.option_key,
      value,
      label,
      group_label: newGroup.value.trim(),
    })
    MessagePlugin.success('已添加')
    newValue.value = ''
    await loadValues()
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '添加失败')
  } finally {
    valueSaving.value = false
  }
}

async function offlineValue(row: OptionValueItem) {
  // 停用而非删除：配置项里已有的档位可能已引用该取值，删掉会让历史配置对不上账。
  try {
    await deleteOptionValue(Number(row.value) || 0)
  } catch {
    /* 取值没有 id 时走 upsert 停用 */
  }
  try {
    await upsertOptionValue({
      provider_type: providerType.value.trim() || undefined,
      option_key: current.value?.option_key || '',
      value: row.value,
      label: row.label,
      group_label: row.group_label,
      status: 'offline',
    })
    MessagePlugin.success('已停用')
    await loadValues()
    await load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '停用失败')
  }
}

function handleMobileAction(value: string | number | Record<string, unknown>, row: OptionSpecInfo) {
  const action = typeof value === 'string' || typeof value === 'number'
    ? String(value)
    : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      openEdit(row)
      break
    case 'values':
      void openValues(row)
      break
    case 'delete':
      removeSpec(row)
      break
  }
}

onMounted(async () => {
  await loadProviders()
  // 默认选第一个算力渠道，省得运营每次都要手点。
  const first = providerOptions.value[0]
  if (first) {
    providerId.value = first.value
    onProviderChange(first.value)
  }
})
</script>

<style lang="css">
@import '../../shared.css';
</style>

<style scoped>
.opt-group {
  margin-bottom: 18px;
}

.opt-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 2px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  margin-bottom: 8px;
}

.opt-group__name {
  font-weight: 600;
}

.opt-group__count {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.opt-unit {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.product-sub code {
  margin-right: 6px;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.values-toolbar {
  display: grid;
  grid-template-columns: 2fr 1fr auto;
  gap: 8px;
  margin: 12px 0;
}
</style>
