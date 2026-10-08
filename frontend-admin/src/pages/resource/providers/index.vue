<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <component :is="pageIcon" size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ pageTitle }}</h2>
          <p class="page-header__desc">{{ pageDesc }}</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="handleRefreshList">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="goCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          {{ createLabel }}
        </t-button>
      </t-space>
    </header>

    <FilterCard :columns="4">
      <div class="field">
        <span class="field__label">关键词</span>
        <t-input
          v-model="filters.keyword"
          clearable
          placeholder="搜索名称 / API 地址"
          @enter="handleSearch"
        >
          <template #prefix-icon>
            <SearchIcon size="14" aria-hidden="true" />
          </template>
        </t-input>
      </div>
      <div class="field">
        <span class="field__label">提供商类型</span>
        <t-select v-model="filters.provider_type" clearable placeholder="全部类型" :options="typeOptions" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部状态" :options="statusFilterOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button variant="outline" @click="handleResetFilters">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">提供商列表</h3>
        <span class="table-card__meta">共 {{ total }} 条记录</span>
      </div>
      <t-table
        row-key="id"
        :data="providerList"
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
          <div class="name-cell">
            <span class="name-cell__text">{{ row.name }}</span>
            <t-tooltip v-if="row.credential_error" :content="row.credential_error" placement="top">
              <t-tag theme="danger" variant="light" size="small" shape="round">凭证异常</t-tag>
            </t-tooltip>
          </div>
        </template>

        <template #provider_type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ typeLabel(row.provider_type) }}</t-tag>
        </template>

        <template #connectivity="{ row }">
          <t-tooltip :content="connTooltip(connState(row.id), row)" :disabled="connState(row.id).status === 'idle'" placement="top">
            <span class="conn-cell">
              <span class="conn-signal" :class="`conn-signal--${signalTier(connState(row.id))}`">
                <i v-for="n in 3" :key="n" :class="{ 'is-on': n <= signalBars(connState(row.id)) }" />
              </span>
              <t-tag
                v-if="connState(row.id).status === 'testing'"
                theme="primary"
                variant="light"
                size="small"
                shape="round"
              >
                测试中…
              </t-tag>
              <t-tag
                v-else-if="connState(row.id).status === 'ok'"
                theme="success"
                variant="light"
                size="small"
                shape="round"
              >
                {{ connLatencyLabel(connState(row.id)) }}
              </t-tag>
              <t-tag v-else-if="connState(row.id).status === 'fail'" theme="danger" variant="light" size="small" shape="round">
                连接失败
              </t-tag>
              <t-tag v-else theme="default" variant="light" size="small" shape="round">未测试</t-tag>
            </span>
          </t-tooltip>
        </template>

        <!-- 列头即「一键测试」入口：并发测试当前页全部渠道，运行中以旋转圆圈 + 进度提示。
             注意 TDesign 的列头插槽名 = col.title 字面量（useTableHeader.renderTitle 用
             slots[col.title] 查找），与单元格插槽用 colKey 不同；改列标题时必须同步改这里。 -->
        <template #连通性>
          <span class="conn-head">
            <span class="conn-head__text">连通性</span>
            <t-tooltip :content="testAllHint" placement="top">
              <t-button
                class="conn-head__btn"
                size="small"
                variant="text"
                :disabled="testingAll || !providerList.length"
                @click="handleTestAll"
              >
                <template #icon>
                  <LoadingIcon v-if="testingAll" class="conn-head__spin" />
                  <LinkIcon v-else />
                </template>
                <span class="conn-head__label">{{ testingAll ? `${testDone}/${testTotal}` : '一键测试' }}</span>
              </t-button>
            </t-tooltip>
          </span>
        </template>

        <template #ops_url="{ row }">
          <t-link v-if="row.ops_console_url" theme="primary" hover="color" :href="row.ops_console_url" target="_blank">
            运维平台
          </t-link>
          <span v-else class="muted">—</span>
        </template>

        <template #resources="{ row }">
          <div class="resource-cell">
            <div class="resource-row">
              <span class="resource-label">CPU</span>
              <t-progress :percentage="usagePercent(row.used_cpu, row.total_cpu)" :color="usageColor(row.used_cpu, row.total_cpu)" :stroke-width="6" theme="line" />
            </div>
            <div class="resource-row">
              <span class="resource-label">内存</span>
              <t-progress :percentage="usagePercent(row.used_memory, row.total_memory)" :color="usageColor(row.used_memory, row.total_memory)" :stroke-width="6" theme="line" />
            </div>
            <div class="resource-row">
              <span class="resource-label">磁盘</span>
              <t-progress :percentage="usagePercent(row.used_disk, row.total_disk)" :color="usageColor(row.used_disk, row.total_disk)" :stroke-width="6" theme="line" />
            </div>
          </div>
        </template>

        <template #status="{ row }">
          <t-tag :theme="resolveStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ statusLabelMap[row.status] || '未知' }}
          </t-tag>
        </template>

        <template #last_sync_at="{ row }">
          <span class="time-text">{{ row.last_sync_at ? formatTime(row.last_sync_at) : '从未同步' }}</span>
        </template>

        <template #sync_state="{ row }">
          <t-tooltip v-if="row.sync_paused" :content="row.last_sync_error || '同步已暂停'" placement="top">
            <t-tag theme="warning" variant="light" size="small" shape="round">
              {{ row.last_sync_error === '适配器未实现' ? '未接入' : '已暂停' }}
            </t-tag>
          </t-tooltip>
          <t-tag v-else-if="row.consecutive_failures > 0" theme="danger" variant="light" size="small" shape="round">
            失败 {{ row.consecutive_failures }} 次
          </t-tag>
          <t-tag v-else theme="success" variant="light" size="small" shape="round">正常</t-tag>
        </template>

        <template #action="{ row }">
<div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="buildMobileActionOptions([
                { content: '详情', value: 'detail', theme: 'default' },
                { content: '编辑', value: 'edit', theme: 'default' },
                { content: '测试连接', value: 'test', theme: 'default' },
                { content: '运维平台', value: 'ops', hidden: () => !row.ops_console_url, theme: 'default' },
                { content: '恢复同步', value: 'resume', hidden: () => !(row.sync_paused), theme: 'warning' },
                { content: '删除', value: 'delete', theme: 'error' },
              ])"
              @select="(value) => handleMobileAction(value, row)"
            />
            <template v-else>
              <t-link theme="primary" hover="color" @click="router.push(`/resource/providers/${row.id}`)">详情</t-link>
              <t-link theme="primary" hover="color" @click="openEditDialog(row)">编辑</t-link>
              <t-link theme="primary" hover="color" :disabled="connState(row.id).status === 'testing'" @click="handleTestConnection(row)">
                测试连接
              </t-link>
              <t-link
                v-if="row.sync_paused"
                theme="warning"
                hover="color"
                :disabled="resumingId === row.id"
                @click="handleResumeSync(row)"
              >
                恢复同步
              </t-link>
              <t-popconfirm content="删除后该渠道的同步配置将一并移除，确认删除？" @confirm="handleDelete(row)">
                <t-link theme="danger" hover="color">删除</t-link>
              </t-popconfirm>
            </template>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无提供商数据" />
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
      header="编辑提供商"
      width="680px"
      :confirm-btn="{ content: '保存', theme: 'success', loading: submitting }"
      cancel-btn="取消"
      :on-confirm="handleSaveDialog"
      @close="handleDialogClose"
    >
      <!-- 分块编辑：原先把基本信息/凭证/能力矩阵堆在一屏，680px 宽下必须上下滚动才能看全。
           改为标题切换后每次只显示一块，弹窗高度稳定，也避免"漏看凭证区"这类误操作。
           t-form 仍包在 tabs 外层：t-form-item 通过 provide/inject 注册到祖先表单，
           面板设 destroy-on-hide=false 保证切走的面板不卸载，否则在别的标签页点保存
           会跳过名称/地址这类必填校验（t-form 只校验已挂载的表单项）。 -->
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top">
        <t-tabs v-model="editTab" theme="card" size="small" class="edit-tabs">
          <t-tab-panel value="basic" label="基本信息" :destroy-on-hide="false">
            <div class="form-grid">
              <t-form-item label="提供商名称" name="name">
                <t-input v-model="formData.name" placeholder="例如：华东 OpenStack" maxlength="50" />
              </t-form-item>
              <t-form-item label="类型" name="provider_type">
                <t-input :model-value="typeLabel(formData.provider_type)" disabled />
              </t-form-item>
              <t-form-item label="API 地址" name="api_endpoint">
                <t-input v-model="formData.api_endpoint" placeholder="https://api.example.com" />
              </t-form-item>
              <t-form-item label="区域" name="region">
                <t-input v-model="formData.region" placeholder="例如：cn-east-1" />
              </t-form-item>
              <t-form-item label="运维平台地址" name="ops_console_url" class="form-item--full">
                <t-input v-model="formData.ops_console_url" placeholder="上游/平台运维控制台地址，留空则不显示跳转入口" />
              </t-form-item>
            </div>
          </t-tab-panel>

          <t-tab-panel value="sync" label="同步与调度" :destroy-on-hide="false">
            <div class="form-grid">
              <t-form-item label="同步间隔（秒）" name="sync_interval">
                <t-input-number v-model="formData.sync_interval" :min="0" :step="60" placeholder="默认 3600" />
              </t-form-item>
              <t-form-item label="启用实例同步" name="sync_enabled">
                <t-switch v-model="formData.sync_enabled" />
              </t-form-item>
            </div>
            <t-alert
              theme="info"
              message="同步间隔决定调度器多久拉起一次该渠道的同步任务；关闭「启用实例同步」后实例范围不再参与调度。"
            />
          </t-tab-panel>

          <t-tab-panel value="credential" label="凭证" :destroy-on-hide="false">
            <CredentialFormFields
              v-if="editingCredentialFields.length"
              v-model="formData.credentials"
              :fields="editingCredentialFields"
            />
            <div v-else class="form-grid">
              <t-form-item label="API 密钥" name="api_key">
                <t-input v-model="formData.api_key" type="password" placeholder="留空表示不修改" />
              </t-form-item>
              <t-form-item label="API 密码" name="api_secret">
                <t-input v-model="formData.api_secret" type="password" placeholder="留空表示不修改" />
              </t-form-item>
            </div>
            <t-alert
              theme="warning"
              class="edit-tab-alert"
              :message="editingCredentialFields.length
                ? '凭证字段留空或保持脱敏回显表示不修改；修改后将重新加密落库。'
                : 'API 密钥/密码留空表示保持原值不变。'"
            />
          </t-tab-panel>

          <t-tab-panel value="capability" label="能力矩阵" :destroy-on-hide="false">
            <CapabilityMatrix
              v-if="editingDescriptor"
              :descriptor="editingDescriptor"
              :title="`${typeLabel(formData.provider_type)} 能力矩阵`"
            />
            <t-empty v-else description="该类型暂无能力描述符" />
          </t-tab-panel>
        </t-tabs>
      </t-form>
    </t-dialog>

  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AddIcon, CloudIcon, LinkIcon, LoadingIcon, RefreshIcon, SearchIcon, ServerIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormRule, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  deleteProvider,
  getProviderList,
  getProviderTypes,
  resumeProviderSync,
  testConnection,
  updateProvider,
} from '@/api/admin'
import type { CapabilityDescriptor, ProviderInfo, ProviderTypeItem } from '@/types/interface'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useIsMobile } from '@/composables/useIsMobile'
import CapabilityMatrix from './components/CapabilityMatrix.vue'
import CredentialFormFields from './components/CredentialFormFields.vue'

defineOptions({ name: 'ResourceProviders' })

const route = useRoute()
const router = useRouter()

// 双链路拆页（本轮 S1）：同一组件服务两个路由，链路由路由 meta.channelKind 固定。
// upstream = 上游转售渠道（对接上游商家，目录/定价/生命周期在上游）；
// compute  = 自营平台对接（对接资源平台作为自营执行器，本地售卖）。
const channelKind = computed(() => (route.meta.channelKind as string) || 'upstream')
const isCompute = computed(() => channelKind.value === 'compute')
const pageTitle = computed(() => (isCompute.value ? '自营平台对接' : '上游转售渠道'))
const pageDesc = computed(() =>
  isCompute.value
    ? '对接资源平台作为自营产品执行器：本地定价与售卖，平台侧提供开通与电源控制'
    : '对接上游商家转售其资源：商品目录、成本价与生命周期以对上为准',
)
const createLabel = computed(() => (isCompute.value ? '添加平台' : '添加渠道'))
const pageIcon = computed(() => (isCompute.value ? ServerIcon : CloudIcon))

const providerList = ref<ProviderInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const submitting = ref(false)
const resumingId = ref<number | null>(null)
const total = ref(0)
const typeOptions = ref<{ label: string; value: string }[]>([])
const typeNameMap = ref<Record<string, string>>({})
const typeKindMap = ref<Record<string, string>>({})

// 行内连接测试状态：把「连接测试」页的职责收敛到列表一行一组状态。
type ConnStatus = 'idle' | 'testing' | 'ok' | 'fail'
type ConnResult = { status: ConnStatus; message: string; latency?: number }
const connResults = ref<Record<number, ConnResult>>({})

// 一键测试的运行态：done/total 驱动列头的进度文案，testingAll 期间禁用重复点击与行内单测。
const testingAll = ref(false)
const testDone = ref(0)
const testTotal = ref(0)
// 并发上限：上游 HealthCheck 是登录往返，渠道数通常个位数，4 路足够把总耗时压到单次往返量级，
// 又不至于把上游登录接口打出限流（渠道凭证可能指向同一台上游）。
const TEST_CONCURRENCY = 4

function connState(id: number): ConnResult {
  return connResults.value[id] || { status: 'idle' as ConnStatus, message: '' }
}

// 信号格：0 格未测试，1-3 格按延迟档位，直观表达"连通质量"而不只是通/不通；
// 失败给满格但按 down 配色（红），与"未测试"的全灰区分开。
function signalBars(conn: ConnResult): number {
  if (conn.status === 'fail') return 3
  if (conn.status !== 'ok' || typeof conn.latency !== 'number') return 0
  if (conn.latency < 500) return 3
  if (conn.latency < 1500) return 2
  return 1
}

function signalTier(conn: ConnResult): 'idle' | 'good' | 'fair' | 'poor' | 'down' {
  if (conn.status === 'fail') return 'down'
  if (conn.status === 'testing') return 'idle'
  switch (signalBars(conn)) {
    case 3:
      return 'good'
    case 2:
      return 'fair'
    case 1:
      return 'poor'
    default:
      return 'idle'
  }
}

function connLatencyLabel(conn: ConnResult): string {
  return typeof conn.latency === 'number' ? `${conn.latency} ms` : '连通正常'
}

function connTooltip(conn: ConnResult, row: ProviderInfo): string {
  switch (conn.status) {
    case 'testing':
      return `正在测试「${row.name}」…`
    case 'ok':
      return `「${row.name}」连通正常${typeof conn.latency === 'number' ? `，握手耗时 ${conn.latency} ms` : ''}`
    case 'fail':
      return `「${row.name}」连接失败：${conn.message}`
    default:
      return ''
  }
}

const testAllHint = computed(() => {
  if (testingAll.value) return `正在并发测试（${testDone.value}/${testTotal.value}），最多 ${TEST_CONCURRENCY} 路并行`
  if (!providerList.value.length) return '当前页没有可测试的渠道'
  return `并发测试当前页 ${providerList.value.length} 个渠道的连接`
})

const statusLabelMap: Record<number, string> = {
  1: '启用',
  0: '禁用',
}

const statusFilterOptions = [
  { label: '启用', value: 1 },
  { label: '禁用', value: 0 },
]

const filters = reactive({
  keyword: '',
  provider_type: '',
  // 默认必须是空串（= 不按状态筛），不能写 0：后端 status 是 int，`status != 0`
  // 才当作筛选条件，传 0 等于不筛。默认 0 会让下拉显示「禁用」为已选值、
  // 列表却按「全部」返回，看上去像筛选失效。
  status: '' as number | '',
})

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

// 移动端分页状态：与桌面端 pagination 同步维护
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})
function typeLabel(type: string): string {
  return typeNameMap.value[type] || type
}

// 可选类型按当前链路过滤：上游渠道只列出 kind=upstream 的类型，自营平台只列 kind=compute。
const visibleTypes = computed(() => {
  const types = typeOptions.value
  const known = Object.keys(typeKindMap.value)
  if (!known.length) return types
  return types.filter((item) => (typeKindMap.value[item.value] || 'upstream') === channelKind.value)
})

function resolveStatusTheme(status: number) {
  if (status === 1) return 'success'
  return 'default'
}

function usagePercent(used: number, total: number): number {
  if (!total) return 0
  return Math.min(100, Math.round((used / total) * 100))
}

function usageColor(used: number, total: number): string {
  const pct = usagePercent(used, total)
  if (pct >= 80) return '#dc2626'
  if (pct >= 60) return '#d97706'
  return '#16a34a'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

// 列的可见性依赖链路：资源概览（CPU/内存/磁盘配额）只对自营平台有意义——自营类型在能力
// 描述符里声明 pool/region 同步范围，容量由资源池同步写入；上游转售渠道的目录、定价与生命
// 周期全在上游，本地没有容量字段（描述符只声明 catalog/price/instance），渲染出来恒为 0/0。
const columns = computed<PrimaryTableCol<ProviderInfo>[]>(() => {
  const base: PrimaryTableCol<ProviderInfo>[] = [
    { colKey: 'name', title: '名称', minWidth: 200 },
    { colKey: 'provider_type', title: '类型', width: 120 },
    { colKey: 'api_endpoint', title: 'API 地址', minWidth: 220, ellipsis: true },
    // 宽度按列头（「连通性」+ 一键测试按钮）取值，行内只有信号格 + 延迟标签，列头更宽。
    { colKey: 'connectivity', title: '连通性', width: 200 },
    { colKey: 'status', title: '状态', width: 90 },
    { colKey: 'sync_state', title: '同步状态', width: 110 },
  ]
  if (isCompute.value) {
    base.push({ colKey: 'resources', title: '资源概览', minWidth: 220 })
  }
  base.push(
    { colKey: 'ops_url', title: '运维入口', width: 100 },
    { colKey: 'last_sync_at', title: '最后同步', width: 160 },
    {
      colKey: 'action',
      title: '操作',
      width: isMobile.value ? 70 : 300,
      fixed: 'right' as const,
      align: 'center' as const,
    },
  )
  return base
})

async function loadTypes() {
  try {
    const types = await getProviderTypes()
    typeNameMap.value = Object.fromEntries(types.map((item: ProviderTypeItem) => [item.type, item.name]))
    typeKindMap.value = Object.fromEntries(types.map((item: ProviderTypeItem) => [item.type, item.kind || 'upstream']))
    // 类型下拉按当前链路过滤（上游转售 / 自营平台），避免跨链路选错。
    typeOptions.value = types
      .filter((item: ProviderTypeItem) => (item.kind || 'upstream') === channelKind.value)
      .map((item: ProviderTypeItem) => ({ label: item.name, value: item.type }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商类型失败')
  }
}

async function loadProviders() {
  loading.value = true
  try {
    const data = await getProviderList({
      page: pagination.current,
      page_size: pagination.pageSize,
      keyword: filters.keyword || undefined,
      provider_type: filters.provider_type || undefined,
      // 链路由当前页面决定，用户不可跨链路查看/操作。
      kind: channelKind.value,
      status: filters.status === '' ? undefined : filters.status,
    })
    providerList.value = data.items
    total.value = data.meta.total
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商列表失败')
  } finally {
    loading.value = false
  }
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  loadProviders()
  mobilePage.current = pageInfo?.current ?? pagination.current
  mobilePage.pageSize = pageInfo?.pageSize ?? pagination.pageSize
  mobilePage.total = pagination.total
}

// —— 移动端分页交互 ——
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

function handleSearch() {
  pagination.current = 1
  loadProviders()
}

function handleResetFilters() {
  filters.keyword = ''
  filters.provider_type = ''
  filters.status = ''
  pagination.current = 1
  loadProviders()
}

function handleRefreshList() {
  loadProviders()
}

function goCreate() {
  router.push({ path: '/resource/providers/create', query: { kind: channelKind.value } })
}

function setConnResult(id: number, result: ConnResult) {
  // 整体替换而不是原地改键：connResults 是普通对象 ref，新增键不会触发依赖收集。
  connResults.value = { ...connResults.value, [id]: result }
}

// 单渠道测试：返回结果供调用方汇总，不在这里弹提示（一键测试只要一条汇总提示）。
// latency 只统计真实请求往返，MIN_TEST_VISIBLE_MS 只用于让「测试中」图标可被肉眼看到，
// 不作为延迟计入信号档位。
const MIN_TEST_VISIBLE_MS = 350

async function runConnectionTest(row: ProviderInfo): Promise<ConnResult> {
  setConnResult(row.id, { status: 'testing', message: '' })
  const startedAt = Date.now()
  try {
    const result = await testConnection(row.id)
    const latency = Date.now() - startedAt
    await holdTestingVisible(startedAt)
    if (result.success) {
      const ok: ConnResult = { status: 'ok', message: result.message || 'ok', latency }
      setConnResult(row.id, ok)
      return ok
    }
    const fail: ConnResult = { status: 'fail', message: result.message, latency }
    setConnResult(row.id, fail)
    return fail
  } catch (error) {
    await holdTestingVisible(startedAt)
    const fail: ConnResult = { status: 'fail', message: (error as Error).message || '连接测试失败' }
    setConnResult(row.id, fail)
    return fail
  }
}

function holdTestingVisible(startedAt: number) {
  const rest = MIN_TEST_VISIBLE_MS - (Date.now() - startedAt)
  return rest > 0 ? new Promise((resolve) => setTimeout(resolve, rest)) : Promise.resolve()
}

async function handleTestConnection(row: ProviderInfo) {
  if (testingAll.value) return
  const result = await runConnectionTest(row)
  if (result.status === 'ok') {
    MessagePlugin.success(`「${row.name}」连接正常（${result.latency} ms）`)
  } else {
    MessagePlugin.error(`「${row.name}」连接失败：${result.message}`)
  }
}

// 一键测试：并发跑完当前页全部渠道，逐行回填状态与信号格，最后给一条汇总。
async function handleTestAll() {
  if (testingAll.value) return
  const rows = [...providerList.value]
  if (!rows.length) return
  testingAll.value = true
  testDone.value = 0
  testTotal.value = rows.length
  const results: Record<number, ConnResult> = {}
  let cursor = 0
  // 工作池：N 个 worker 抢同一个游标。cursor++ 与读取之间没有 await，单线程下不会重复取号。
  const workers = Array.from({ length: Math.min(TEST_CONCURRENCY, rows.length) }, async () => {
    while (cursor < rows.length) {
      const row = rows[cursor++]
      results[row.id] = await runConnectionTest(row)
      testDone.value += 1
    }
  })
  try {
    await Promise.all(workers)
  } finally {
    testingAll.value = false
  }
  const okCount = Object.values(results).filter((item) => item.status === 'ok').length
  const failCount = rows.length - okCount
  if (failCount === 0) {
    MessagePlugin.success(`一键测试完成：${rows.length} 个渠道全部连通正常`)
  } else {
    MessagePlugin.warning(`一键测试完成：${okCount} 个连通，${failCount} 个失败（悬停「连接失败」查看原因）`)
  }
}

async function handleResumeSync(row: ProviderInfo) {
  resumingId.value = row.id
  try {
    await resumeProviderSync(row.id)
    MessagePlugin.success('已恢复同步，下次调度将重新尝试')
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复同步失败')
  } finally {
    resumingId.value = null
  }
}

async function handleDelete(row: ProviderInfo) {  try {
    await deleteProvider(row.id)
    MessagePlugin.success('提供商已删除')
    if (providerList.value.length === 1 && pagination.current > 1) {
      pagination.current -= 1
    }
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '删除提供商失败')
  }
}

type ProviderForm = {
  name: string
  provider_type: string
  api_endpoint: string
  region: string
  api_key: string
  api_secret: string
  credentials: Record<string, string>
  sync_interval: number
  sync_enabled: boolean
  status: number
  ops_console_url: string
}

const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const editingDescriptor = ref<CapabilityDescriptor | null>(null)
const formRef = ref<FormInstanceFunctions | null>(null)
// 编辑弹窗内的分块标签页：每次只渲染一块，避免 680px 弹窗需要上下滚动。
const editTab = ref('basic')

const editingCredentialFields = computed(() => editingDescriptor.value?.credential_schema || [])

const formData = reactive<ProviderForm>({
  name: '',
  provider_type: '',
  api_endpoint: '',
  region: '',
  api_key: '',
  api_secret: '',
  credentials: {},
  sync_interval: 3600,
  sync_enabled: false,
  status: 1,
  ops_console_url: '',
})

const rules: Record<string, FormRule[]> = {
  name: [{ required: true, message: '请输入提供商名称', type: 'error', trigger: 'blur' }],
  api_endpoint: [
    { required: true, message: '请输入 API 地址', type: 'error', trigger: 'blur' },
    { pattern: /^https?:\/\//, message: 'API 地址需以 http(s):// 开头', type: 'error', trigger: 'blur' },
  ],
  ops_console_url: [
    { pattern: /^https?:\/\//, message: '运维地址需以 http(s):// 开头', type: 'error', trigger: 'blur' },
  ],
}

function openEditDialog(row: ProviderInfo) {
  editingId.value = row.id
  editingDescriptor.value = row.capabilities || null
  // 每次打开都回到第一块，避免上次停留在「能力矩阵」时打开以为内容缺失。
  editTab.value = 'basic'
  Object.assign(formData, {
    name: row.name,
    provider_type: row.provider_type,
    api_endpoint: row.api_endpoint,
    region: row.region,
    api_key: '',
    api_secret: '',
    credentials: { ...(row.credentials || {}) },
    sync_interval: row.sync_interval,
    sync_enabled: row.sync_enabled,
    status: row.status,
    ops_console_url: row.ops_console_url || '',
  })
  formRef.value?.clearValidate?.()
  dialogVisible.value = true
}

async function handleSaveDialog() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return
  if (editingId.value === null) return
  submitting.value = true
  try {
    const payload = {
      name: formData.name,
      api_endpoint: formData.api_endpoint,
      region: formData.region,
      sync_enabled: formData.sync_enabled,
      sync_interval: formData.sync_interval,
      status: formData.status,
      ops_console_url: formData.ops_console_url || '',
    } as Parameters<typeof updateProvider>[1]
    if (editingCredentialFields.value.length) {
      payload.credentials = formData.credentials
    } else {
      payload.api_key = formData.api_key || undefined
      payload.api_secret = formData.api_secret || undefined
    }
    await updateProvider(editingId.value, payload)
    MessagePlugin.success('提供商已保存')
    dialogVisible.value = false
    loadProviders()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存提供商失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  formRef.value?.clearValidate?.()
}

onMounted(() => {
  loadTypes()
  loadProviders()
})

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, any>, row: ProviderInfo) {
  const action = typeof value === 'string' || typeof value === 'number' ? String(value) : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      router.push(`/resource/providers/${row.id}`)
      break
    case 'edit':
      openEditDialog(row)
      break
    case 'test':
      void handleTestConnection(row)
      break
    case 'ops':
      if (row.ops_console_url) window.open(row.ops_console_url, '_blank', 'noopener')
      break
    case 'resume':
      void handleResumeSync(row)
      break
    case 'delete':
      void handleDelete(row)
      break
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, var(--color-primary), var(--td-brand-color-8));
}

/* 桌面 4 列、中屏 2 列、窄屏 1 列由 FilterCard 的 columns 属性给出。 */

/* —— 连通性：信号格 + 状态标签，列头挂「一键测试」 —— */
.conn-head {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.conn-head__text {
  font-weight: 600;
}

.conn-head__btn {
  --td-brand-color: var(--color-primary);
  height: auto;
  padding: 0 2px;
  font-size: 12px;
}

.conn-head__label {
  font-size: 12px;
  font-weight: 400;
}

/* 测试中：圆圈持续旋转，作为"正在跑"的直观信号 */
.conn-head__spin {
  animation: conn-spin 0.9s linear infinite;
}

@keyframes conn-spin {
  to {
    transform: rotate(360deg);
  }
}

.conn-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

/* 三格信号条：由矮到高，点亮格数代表延迟档位；未测试全灰，失败满格但走红（down 配色）。 */
.conn-signal {
  display: inline-flex;
  align-items: flex-end;
  gap: 2px;
  height: 12px;
}

.conn-signal i {
  width: 3px;
  border-radius: 1px;
  background: var(--color-border);
  transition: background-color 0.2s ease;
}

.conn-signal i:nth-child(1) {
  height: 4px;
}

.conn-signal i:nth-child(2) {
  height: 8px;
}

.conn-signal i:nth-child(3) {
  height: 12px;
}

.conn-signal--good i.is-on {
  background: #16a34a;
}

.conn-signal--fair i.is-on {
  background: #d97706;
}

.conn-signal--poor i.is-on {
  background: #ea580c;
}

.conn-signal--down i.is-on {
  background: var(--color-destructive);
}

.resource-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.resource-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.resource-label {
  width: 34px;
  flex-shrink: 0;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.form-item--full {
  grid-column: 1 / -1;
}

/* 编辑弹窗分块：标题切换下方内容固定高度，避免弹窗随内容长短跳动。 */
.edit-tabs :deep(.t-tabs__content) {
  min-height: 260px;
  padding-top: var(--space-md);
}

.edit-tab-alert {
  margin-top: var(--space-md);
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.name-cell__text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
