<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><AppIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">规格模板（配置档）</h2>
          <p class="page-header__desc">
            先选对接平台，再勾选该平台每个参数允许的取值；新建商品时选用档位即生成
            1 个 SKU + 客户可选配置项。
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon /></template>刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon><AddIcon /></template>新增档位
        </t-button>
      </t-space>
    </header>

    <FilterCard>
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input v-model="filters.keyword" placeholder="档位名称" clearable @enter="search" />
      </div>
      <div class="field">
        <span class="field__label">平台</span>
        <t-select v-model="filters.provider_type" clearable placeholder="全部" :options="platformTypeOptions" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部" :options="statusOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="search"><template #icon><SearchIcon /></template>查询</t-button>
          <t-button variant="outline" @click="resetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">档位列表</h3>
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
          <div class="product-cell">
            <span class="cell-strong">{{ row.name }}</span>
            <span class="product-sub">
              {{ row.provider_type || '未绑定平台' }}
              <template v-if="selectionCount(row)"> · {{ selectionCount(row) }} 个可选参数</template>
            </span>
          </div>
        </template>
        <template #specs="{ row }">
          <span>{{ row.cpu }}核 / {{ row.memory }}G / {{ row.disk }}G</span>
        </template>
        <template #bandwidth="{ row }"><span>{{ row.bandwidth === 0 ? '不限' : row.bandwidth + ' Mbps' }}</span></template>
        <template #platform="{ row }">
          <t-tag v-if="row.platform_params" theme="success" variant="light" size="small" shape="round">已配映射</t-tag>
          <t-tag v-else theme="warning" variant="light" size="small" shape="round">缺平台映射</t-tag>
        </template>
        <template #status="{ row }"><t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">{{ row.status === 1 ? '启用' : '停用' }}</t-tag></template>
        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '复制', value: 'clone', theme: 'default' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
              <t-link theme="primary" hover="color" @click="cloneTemplate(row)">复制</t-link>
              <t-link theme="danger" hover="color" @click="remove(row)">删除</t-link>
            </template>
          </div>
        </template>
        <template #empty><t-empty description="暂无档位，请新增" /></template>
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

    <!-- ===== 配置档编辑：左目录勾选、右预览 ===== -->
    <t-dialog
      v-model:visible="dialogVisible"
      :header="form.id ? `编辑档位：${form.name || '未命名'}` : '新增档位'"
      width="1080px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="save"
      @close="closeDialog"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="对接平台" name="providerId">
            <t-select
              v-model="providerId"
              placeholder="选择算力平台渠道"
              :options="platformOptions"
              :loading="platformLoading"
              @change="onPlatformChange"
            />
          </t-form-item>
          <t-form-item label="档位名称" name="name">
            <t-input v-model="form.name" placeholder="如 2核4G / 入门型" />
          </t-form-item>
          <t-form-item label="参考售价（元）" name="price">
            <t-input-number v-model="form.price" :min="0" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="form.sort_order" theme="column" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" :options="statusOptions" />
          </t-form-item>
        </div>

        <t-form-item label="预置档位（一键填基线，再按需改）">
          <t-space size="small" break-line>
            <t-button v-for="p in presets" :key="p.name" size="small" variant="outline" @click="applyPreset(p)">
              {{ p.name }}
            </t-button>
            <t-button size="small" theme="primary" variant="outline" @click="openMultiCpuMem">
              开放多选：CPU 2/4/8核 · 内存 4/8G
            </t-button>
          </t-space>
        </t-form-item>

        <t-alert v-if="!providerId" theme="warning" message="请先选择对接平台，才能读到该平台的配置项目录。" />
        <t-alert v-else-if="!catalog.length" theme="info" message="该平台暂无配置项目录，请先到「产品管理 → 规格管理 → 平台配置项」同步目录。" />

        <template v-if="providerId && catalog.length">
          <t-divider>
            平台配置项
            <span class="divider-hint">已开 {{ enabledCount }} 项 · 多选取值会成为客户可选配置项</span>
          </t-divider>

          <div v-for="grp in catalogGroups" :key="grp.name" class="opt-block">
            <div class="opt-block__head">{{ grp.name || '未分组' }}</div>
            <div v-for="row in grp.rows" :key="row.spec.option_key" class="opt-row" :class="{ 'opt-row--on': row.enabled }">
              <div class="opt-row__head">
                <t-checkbox v-model="row.enabled">
                  <span class="opt-row__name">{{ row.spec.label || row.spec.option_key }}</span>
                </t-checkbox>
                <span class="opt-row__meta">
                  <code>{{ row.spec.option_key }}</code>
                  <t-tag v-if="row.spec.required" theme="error" variant="light" size="small" shape="round">必选</t-tag>
                  <span v-if="row.spec.unit">单位 {{ row.spec.unit }}</span>
                  <span v-if="row.spec.default_value">默认 {{ row.spec.default_value }}</span>
                </span>
              </div>
              <p v-if="row.spec.help" class="opt-row__help">{{ row.spec.help }}</p>

              <div v-if="row.enabled" class="opt-row__body">
                <template v-if="isQtyOption(row.spec)">
                  <div class="opt-inline">
                    <label class="opt-field">
                      最小
                      <t-input-number v-model="row.rangeMin" :min="0" theme="column" />
                    </label>
                    <label class="opt-field">
                      最大
                      <t-input-number v-model="row.rangeMax" :min="0" theme="column" />
                    </label>
                    <label class="opt-field">
                      默认值
                      <t-input-number v-model="row.defaultNumber" theme="column" />
                    </label>
                  </div>
                </template>
                <template v-else>
                  <t-select
                    v-model="row.values"
                    multiple
                    clearable
                    filterable
                    :placeholder="row.spec.value_source === 'manual' ? '手工填写取值（输入后回车添加）' : '选择允许客户选取的值'"
                    :options="valueOptions(row)"
                    :loading="resourceLoading"
                    :creatable="row.spec.value_source === 'manual'"
                    @change="onValuesChange(row)"
                  />
                  <div class="opt-inline">
                    <label class="opt-field">
                      默认值
                      <t-select v-model="row.default" clearable placeholder="默认选中" :options="selectedValueOptions(row)" />
                    </label>
                    <label class="opt-field">
                      分组（用户侧下拉分组）
                      <t-input v-model="row.groupLabel" placeholder="如 Ubuntu / Windows" />
                    </label>
                  </div>
                </template>
              </div>
            </div>
          </div>

          <t-divider>平台映射（生成 SKU 时下发的基线参数）</t-divider>
          <div class="form-grid">
            <t-form-item label="数据中心 area">
              <t-select v-model="platformParams.area" clearable placeholder="平台区域" :options="areaOptions" />
            </t-form-item>
            <t-form-item label="节点 node">
              <t-select v-model="platformParams.node" clearable placeholder="平台节点" :options="nodeOptions" />
            </t-form-item>
            <t-form-item label="存储 store">
              <t-select v-model="platformParams.store" clearable placeholder="系统盘存储（可选）" :options="storeOptions" />
            </t-form-item>
            <t-form-item label="镜像 os">
              <t-select v-model="platformParams.os" clearable placeholder="平台镜像" :options="imageOptions" />
            </t-form-item>
          </div>
          <t-form-item label="其他平台参数（高级 JSON）">
            <t-textarea v-model="extraParamsText" :autosize="{ minRows: 2, maxRows: 5 }"
              placeholder='上面四下拉之外的键，如 {"network_type":"normal","ip_num":1}' />
          </t-form-item>

          <t-divider>名称与描述模板</t-divider>
          <t-form-item label="商品名模板">
            <t-input v-model="form.name_template" placeholder="{cpu}核{memory}G {os}" />
          </t-form-item>
          <t-form-item label="描述模板">
            <t-textarea v-model="form.description_template" :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="{cpu}核{memory}GB 内存 / {disk}GB 系统盘 / {bw}Mbps 带宽" />
          </t-form-item>
          <t-alert theme="info">
            <template #message>
              <div>预览（按基线取值展开）：</div>
              <div class="preview-line">商品名：<strong>{{ previewName || '—' }}</strong></div>
              <div class="preview-line">SKU 编码：<code>{{ previewCode || '—' }}</code></div>
              <div class="preview-line">描述：{{ previewDesc || '—' }}</div>
              <div class="preview-line">平台参数：<code>{{ previewPlatformParams }}</code></div>
            </template>
          </t-alert>
        </template>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createSpecTemplate, deleteSpecTemplate, getOptionCatalog, getSpecTemplateList, updateSpecTemplate } from '@/api/product'
import { getProviderList, getProviderPlatformResources } from '@/api/admin'
import type {
  OptionSpecInfo,
  OptionValueItem,
  PlatformResourceItem,
  ProviderInfo,
  SpecOptionSelections,
  SpecTemplateInfo,
} from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import FilterCard from '@/components/filter-card/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'

defineOptions({ name: 'ProductSpecTemplates' })

const items = ref<SpecTemplateInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()

const filters = reactive<{ keyword?: string; provider_type?: string; status?: number }>({})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(load)

const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const columns: PrimaryTableCol<SpecTemplateInfo>[] = [
  { colKey: 'spec', title: '档位', minWidth: 200 },
  { colKey: 'specs', title: '基线配置', width: 160 },
  { colKey: 'bandwidth', title: '带宽', width: 100 },
  { colKey: 'platform', title: '平台映射', width: 110 },
  { colKey: 'price', title: '参考售价', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'action', title: '操作', width: isMobile.value ? 70 : 170, fixed: 'right' as const, align: 'center' as const },
]

function selectionCount(row: SpecTemplateInfo): number {
  const sel = (row.option_selections || {}) as SpecOptionSelections
  return Object.keys(sel).length
}

async function load() {
  loading.value = true
  try {
    const data = await getSpecTemplateList({
      keyword: filters.keyword,
      provider_type: filters.provider_type,
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
  filters.provider_type = undefined
  filters.status = undefined
  resetPage()
}

// ===== 平台渠道与资源目录 =====
const platformOptions = ref<{ label: string; value: number }[]>([])
const platformTypeOptions = ref<{ label: string; value: string }[]>([])
const platformLoading = ref(false)
const providerId = ref<number | undefined>(undefined)
const providerType = ref('')

const resources = ref<{
  areas: PlatformResourceItem[]
  nodes: PlatformResourceItem[]
  stores: PlatformResourceItem[]
  images: PlatformResourceItem[]
}>({ areas: [], nodes: [], stores: [], images: [] })
const resourceLoading = ref(false)

function toOptions(list: PlatformResourceItem[]): { label: string; value: string }[] {
  return list
    .filter((item) => !item.status || item.status === 'active')
    .map((item) => ({ label: `${item.label}（${item.value}）`, value: item.value }))
}

const areaOptions = computed(() => toOptions(resources.value.areas))
const imageOptions = computed(() => toOptions(resources.value.images))
function filterByArea(list: PlatformResourceItem[], area?: string) {
  if (!area) return list
  return list.filter((item) => !item.parent_id || item.parent_id === area)
}
const nodeOptions = computed(() => toOptions(filterByArea(resources.value.nodes, platformParams.area)))
const storeOptions = computed(() => toOptions(filterByArea(resources.value.stores, platformParams.area)))

async function loadPlatformOptions() {
  platformLoading.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    const compute = data.items.filter((item: ProviderInfo) => item.kind === 'compute')
    platformOptions.value = compute.map((item: ProviderInfo) => ({
      label: `${item.name}（${item.provider_type}）`, value: item.id,
    }))
    const seen = new Set<string>()
    platformTypeOptions.value = compute
      .map((item: ProviderInfo) => item.provider_type)
      .filter((t: string) => (seen.has(t) ? false : (seen.add(t), true)))
      .map((t: string) => ({ label: t, value: t }))
  } catch {
    platformOptions.value = []
    platformTypeOptions.value = []
  } finally {
    platformLoading.value = false
  }
}

async function loadPlatformResources(id?: number) {
  const pid = Number(id)
  resources.value = { areas: [], nodes: [], stores: [], images: [] }
  if (!pid) return
  resourceLoading.value = true
  try {
    resources.value = await getProviderPlatformResources(pid)
  } catch (error) {
    MessagePlugin.warning((error as Error).message || '该渠道未提供平台资源目录，可手工填写平台参数 JSON')
  } finally {
    resourceLoading.value = false
  }
}

// ===== 配置档表单 =====
type OptionRow = {
  spec: OptionSpecInfo
  enabled: boolean
  values: string[]
  rangeMin: number
  rangeMax: number
  defaultNumber: number | null
  default: string
  groupLabel: string
}

const dialogVisible = ref(false)
const catalog = ref<OptionSpecInfo[]>([])
const rows = ref<OptionRow[]>([])
const extraParamsText = ref('')
const platformParams = reactive<{ area?: string; node?: string; store?: string; os?: string }>({
  area: undefined, node: undefined, store: undefined, os: undefined,
})
const PLATFORM_KEYS = ['area', 'node', 'store', 'os'] as const

const form = reactive<{
  id: number
  name: string
  price: number
  sort_order: number
  status: number
  disk: number
  disk_type: string
  os: string
  name_template: string
  description_template: string
}>(emptyForm())

function emptyForm() {
  return {
    id: 0, name: '', price: 0, sort_order: 0, status: 1,
    disk: 40, disk_type: 'ssd', os: '',
    name_template: '{cpu}核{memory}G {os}',
    description_template: '{cpu}核{memory}GB 内存 / {disk}GB 系统盘 / {bw}Mbps 带宽',
  }
}

const presets = [
  { name: '2核4G', cpu: '2', memory: '4096', disk: 40, bw: 5 },
  { name: '4核4G', cpu: '4', memory: '4096', disk: 60, bw: 10 },
  { name: '8核8G', cpu: '8', memory: '8192', disk: 80, bw: 20 },
]

const enabledCount = computed(() => rows.value.filter((r) => r.enabled).length)

const catalogGroups = computed(() => {
  const out: { name: string; rows: OptionRow[] }[] = []
  for (const row of rows.value) {
    const name = row.spec.group_name || ''
    let g = out.find((x) => x.name === name)
    if (!g) {
      g = { name, rows: [] }
      out.push(g)
    }
    g.rows.push(row)
  }
  return out
})

function isQtyOption(spec: OptionSpecInfo): boolean {
  return spec.widget === 'qty' || spec.min_value != null || spec.max_value != null
}

function valueOptions(row: OptionRow): { label: string; value: string }[] {
  const list: OptionValueItem[] = row.spec.values || []
  return list.map((v) => ({
    label: v.group_label ? `${v.group_label} / ${v.label || v.value}` : (v.label || v.value),
    value: v.value,
  }))
}

function selectedValueOptions(row: OptionRow): { label: string; value: string }[] {
  return valueOptions(row).filter((o) => row.values.includes(o.value))
}

function onValuesChange(row: OptionRow) {
  if (row.default && !row.values.includes(row.default)) row.default = ''
  if (!row.default && row.values.length === 1) row.default = row.values[0]
}

function baselineOf(key: string): string | undefined {
  const row = rows.value.find((r) => r.spec.option_key === key)
  if (!row || !row.enabled) return undefined
  if (isQtyOption(row.spec)) {
    return row.defaultNumber != null ? String(row.defaultNumber) : (row.spec.default_value || undefined)
  }
  return row.default || row.values[0]
}

function baselineCpu(): number {
  const v = Number(baselineOf('cpu'))
  return Number.isFinite(v) && v > 0 ? v : 1
}
function baselineMemoryGb(): number {
  const v = Number(baselineOf('memory'))
  // 目录里内存按 MB 存（1024=1G）。
  return Number.isFinite(v) && v > 0 ? Math.max(1, Math.round(v / 1024)) : 1
}
function baselineBw(): number {
  const v = Number(baselineOf('bw') ?? baselineOf('in_bw'))
  return Number.isFinite(v) && v > 0 ? v : 0
}

function renderTemplate(tpl: string): string {
  if (!tpl) return ''
  const osLabel = (() => {
    const osValue = baselineOf('os')
    if (!osValue) return form.os || ''
    const hit = (resources.value.images || []).find((i) => i.value === osValue)
    return hit ? (hit.label.split(' / ').pop() || hit.label) : osValue
  })()
  const vars: Record<string, string> = {
    '{cpu}': String(baselineCpu()),
    '{memory}': String(baselineMemoryGb()),
    '{disk}': String(form.disk || 40),
    '{bw}': String(baselineBw()),
    '{os}': osLabel,
  }
  // 用 split/join 而非 replaceAll：项目 tsconfig 的 target 低于 es2021。
  return Object.entries(vars).reduce((acc, [key, val]) => acc.split(key).join(val), tpl)
}

const previewName = computed(() => renderTemplate(form.name_template))
const previewDesc = computed(() => renderTemplate(form.description_template))
const previewCode = computed(() => {
  const parts: string[] = []
  parts.push(`${baselineCpu()}c${baselineMemoryGb()}g`)
  if (form.disk > 0) parts.push(`${form.disk}g`)
  return `mfy-${parts.join('-')}`
})
const previewPlatformParams = computed(() => {
  const merged = buildPlatformParams()
  return merged ? JSON.stringify(merged) : '—'
})

function applyPreset(p: (typeof presets)[number]) {
  for (const row of rows.value) {
    const key = row.spec.option_key
    if (key === 'cpu' && !isQtyOption(row.spec)) {
      row.enabled = true
      if (!row.values.includes(p.cpu)) row.values = [p.cpu]
      row.default = p.cpu
    } else if (key === 'memory' && !isQtyOption(row.spec)) {
      row.enabled = true
      if (!row.values.includes(p.memory)) row.values = [p.memory]
      row.default = p.memory
    }
  }
  form.disk = p.disk
  form.name = form.name || p.name
  const bw = rows.value.find((r) => r.spec.option_key === 'bw')
  if (bw) {
    bw.enabled = true
    bw.defaultNumber = p.bw
    bw.rangeMin = bw.rangeMin || 1
    bw.rangeMax = bw.rangeMax || 100
  }
  // 必选项默认勾上，省得漏配导致开通被平台拒。
  for (const row of rows.value) {
    if (row.spec.required && !isQtyOption(row.spec) && !row.enabled && row.spec.values?.length) {
      row.enabled = true
      row.values = [row.spec.values[0].value]
      row.default = row.values[0]
    }
  }
}

function openMultiCpuMem() {
  const cpu = rows.value.find((r) => r.spec.option_key === 'cpu')
  if (cpu) {
    cpu.enabled = true
    const preferred = ['2', '4', '8'].filter((v) => (cpu.spec.values || []).some((x) => x.value === v))
    cpu.values = preferred.length ? preferred : cpu.values
    cpu.default = cpu.values[0] || ''
  }
  const mem = rows.value.find((r) => r.spec.option_key === 'memory')
  if (mem) {
    mem.enabled = true
    const preferred = ['4096', '8192'].filter((v) => (mem.spec.values || []).some((x) => x.value === v))
    mem.values = preferred.length ? preferred : mem.values
    mem.default = mem.values[0] || ''
  }
  MessagePlugin.success('已开放 CPU/内存多选，客户侧可在档位内自选')
}

function buildOptionSelections(): SpecOptionSelections {
  const out: SpecOptionSelections = {}
  for (const row of rows.value) {
    if (!row.enabled) continue
    const key = row.spec.option_key
    if (isQtyOption(row.spec)) {
      const min = row.rangeMin ?? row.spec.min_value ?? 0
      const max = row.rangeMax ?? row.spec.max_value ?? 0
      const def = row.defaultNumber != null ? String(row.defaultNumber) : row.spec.default_value
      out[key] = { range: [Number(min), Number(max)], default: def || undefined }
      continue
    }
    if (!row.values.length) continue
    out[key] = { values: [...row.values], default: row.default || row.values[0] }
    if (row.groupLabel.trim()) out[key].group_label = row.groupLabel.trim()
  }
  return out
}

function buildPlatformParams(): Record<string, unknown> | null {
  let extra: Record<string, unknown> = {}
  if (extraParamsText.value.trim()) {
    try {
      const parsed = JSON.parse(extraParamsText.value)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        extra = parsed as Record<string, unknown>
      } else {
        MessagePlugin.warning('「其他平台参数」必须是 JSON 对象')
        return null
      }
    } catch {
      MessagePlugin.warning('「其他平台参数」JSON 格式非法')
      return null
    }
  }
  const merged: Record<string, unknown> = { ...extra }
  for (const key of PLATFORM_KEYS) {
    if (platformParams[key]) merged[key] = platformParams[key]
  }
  return Object.keys(merged).length ? merged : null
}

function buildSpecValues(): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  const cpu = baselineCpu()
  const memMb = Number(baselineOf('memory'))
  const bw = baselineBw()
  if (cpu > 0) out['compute.cpu'] = cpu
  if (Number.isFinite(memMb) && memMb > 0) out['compute.memory'] = memMb
  if (form.disk > 0) out['storage.system.size'] = form.disk
  if (bw > 0) out['network.bandwidth'] = bw
  if (platformParams.area) out['placement.region'] = platformParams.area
  return out
}

function buildRow(spec: OptionSpecInfo, selections: SpecOptionSelections): OptionRow {
  const sel = selections[spec.option_key]
  const qty = isQtyOption(spec)
  const row: OptionRow = {
    spec,
    enabled: !!sel,
    values: sel?.values ? [...sel.values] : [],
    rangeMin: sel?.range?.[0] ?? spec.min_value ?? 0,
    rangeMax: sel?.range?.[1] ?? spec.max_value ?? 0,
    defaultNumber: null,
    default: sel?.default || '',
    groupLabel: sel?.group_label || '',
  }
  if (qty) {
    const defStr = sel?.default || spec.default_value || ''
    const defNum = Number(defStr)
    row.defaultNumber = Number.isFinite(defNum) && defStr !== '' ? defNum : null
  }
  return row
}

async function loadCatalog(type: string): Promise<OptionSpecInfo[]> {
  if (!type) return []
  try {
    return await getOptionCatalog({ provider_type: type, provider_id: providerId.value })
  } catch {
    return []
  }
}

async function openCreate() {
  Object.assign(form, emptyForm())
  rows.value = []
  catalog.value = []
  providerId.value = undefined
  providerType.value = ''
  Object.assign(platformParams, { area: undefined, node: undefined, store: undefined, os: undefined })
  resources.value = { areas: [], nodes: [], stores: [], images: [] }
  extraParamsText.value = ''
  dialogVisible.value = true
}

async function openEdit(row: SpecTemplateInfo) {
  Object.assign(form, {
    id: row.id,
    name: row.name,
    price: row.price,
    sort_order: row.sort_order,
    status: row.status,
    disk: row.disk,
    disk_type: row.disk_type || 'ssd',
    os: row.os || '',
    name_template: row.name_template || '{cpu}核{memory}G {os}',
    description_template: row.description_template || '{cpu}核{memory}GB 内存 / {disk}GB 系统盘 / {bw}Mbps 带宽',
  })
  providerType.value = row.provider_type || ''
  const hit = platformOptions.value.find((o) => o.label.includes(`（${row.provider_type}）`))
  providerId.value = hit?.value
  const params = (row.platform_params || {}) as Record<string, unknown>
  Object.assign(platformParams, {
    area: params.area != null ? String(params.area) : undefined,
    node: params.node != null ? String(params.node) : undefined,
    store: params.store != null ? String(params.store) : undefined,
    os: params.os != null ? String(params.os) : undefined,
  })
  const rest: Record<string, unknown> = { ...params }
  for (const key of PLATFORM_KEYS) delete rest[key]
  extraParamsText.value = Object.keys(rest).length ? JSON.stringify(rest, null, 2) : ''

  if (providerId.value) await loadPlatformResources(providerId.value)
  catalog.value = await loadCatalog(providerType.value)
  const selections = (row.option_selections || {}) as SpecOptionSelections
  rows.value = catalog.value.map((spec) => buildRow(spec, selections))
  dialogVisible.value = true
}

async function cloneTemplate(row: SpecTemplateInfo) {
  await openEdit(row)
  form.id = 0
  form.name = `${row.name}（副本）`
  MessagePlugin.info('已按该档位预填，改名后保存即可')
}

async function onPlatformChange(value?: number | string) {
  const pid = Number(value)
  const hit = platformOptions.value.find((o) => o.value === pid)
  if (hit) {
    const match = hit.label.match(/（([^）]+)）/)
    providerType.value = match ? match[1] : ''
  }
  Object.assign(platformParams, { area: undefined, node: undefined, store: undefined, os: undefined })
  await loadPlatformResources(pid)
  // 换平台后旧勾选对新平台无效，全部重置（避免跨平台脏值）。
  catalog.value = await loadCatalog(providerType.value)
  rows.value = catalog.value.map((spec) => buildRow(spec, {}))
}

function closeDialog() {
  dialogVisible.value = false
}

async function save() {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入档位名称')
    return
  }
  if (!providerType.value) {
    MessagePlugin.warning('请选择对接平台')
    return
  }
  const platformParamsPayload = buildPlatformParams()
  if (platformParamsPayload === null) return
  const selections = buildOptionSelections()
  if (!Object.keys(selections).length) {
    MessagePlugin.warning('请至少勾选一个配置项，否则该档位无法生成客户可选配置')
    return
  }
  if (!platformParamsPayload.area && !platformParamsPayload.node) {
    MessagePlugin.warning('请至少选择数据中心或节点（平台要求二者至少传一个）')
    return
  }
  const payload = {
    name: form.name.trim(),
    provider_type: providerType.value,
    cpu: baselineCpu(),
    memory: baselineMemoryGb(),
    disk: form.disk,
    disk_type: form.disk_type,
    bandwidth: baselineBw(),
    os: form.os,
    price: form.price,
    sort_order: form.sort_order,
    status: form.status,
    spec_values: buildSpecValues(),
    platform_params: platformParamsPayload,
    option_selections: selections,
    name_template: form.name_template,
    description_template: form.description_template,
  }
  try {
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

function remove(row: SpecTemplateInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除档位',
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

function handleMobileAction(value: string | number | Record<string, unknown>, row: SpecTemplateInfo) {
  const action = typeof value === 'string' || typeof value === 'number'
    ? String(value)
    : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'edit':
      void openEdit(row)
      break
    case 'clone':
      void cloneTemplate(row)
      break
    case 'delete':
      remove(row)
      break
  }
}

onMounted(() => {
  load()
  loadPlatformOptions()
})
</script>

<style lang="css">
@import '../../shared.css';
</style>

<style scoped>
.divider-hint {
  margin-left: 8px;
  font-size: 12px;
  font-weight: normal;
  color: var(--td-text-color-secondary, #888);
}

.opt-block {
  margin-bottom: 14px;
}

.opt-block__head {
  font-weight: 600;
  padding: 4px 2px;
  border-bottom: 1px solid var(--td-component-stroke, #e7e7e7);
  margin-bottom: 8px;
}

.opt-row {
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 6px;
  padding: 8px 10px;
  margin-bottom: 8px;
  background: var(--td-bg-color-container, #fff);
}

.opt-row--on {
  border-color: var(--td-brand-color, #0052d9);
  box-shadow: 0 0 0 1px rgba(0, 82, 217, 0.1);
}

.opt-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.opt-row__name {
  font-weight: 600;
}

.opt-row__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.opt-row__meta code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.opt-row__help {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}

.opt-row__body {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.opt-inline {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 8px 12px;
}

.opt-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #666);
}

.preview-line {
  font-size: 13px;
  margin-top: 2px;
}

.preview-line code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}
</style>
