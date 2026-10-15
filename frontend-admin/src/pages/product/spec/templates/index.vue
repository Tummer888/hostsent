<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip"><AppIcon size="20" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">规格模板（配置档）</h2>
          <p class="page-header__desc">
            先选对接平台，再勾选该平台每个参数允许的取值；新建商品时选用档位即生成 1 个 SKU + 客户可选配置项。
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
        <span class="field__label">机型</span>
        <t-select v-model="filters.machine_kind" clearable placeholder="全部" :options="machineKindFilterOptions" />
      </div>
      <div class="field">
        <span class="field__label">状态</span>
        <t-select v-model="filters.status" clearable placeholder="全部" :options="statusOptions" />
      </div>
      <template #actions>
        <t-space size="small">
          <t-button theme="primary" @click="search"
            ><template #icon><SearchIcon /></template>查询</t-button
          >
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
        <template #machine="{ row }">
          <t-tag v-if="row.machine_kind === 'nat'" theme="warning" variant="light" size="small" shape="round"
            >NAT 机型</t-tag
          >
          <t-tag v-else-if="row.machine_kind === 'vps'" theme="primary" variant="light" size="small" shape="round"
            >独立 IP</t-tag
          >
          <span v-else class="cell-muted">未标注</span>
        </template>
        <template #bandwidth="{ row }"
          ><span>{{ row.bandwidth === 0 ? '不限' : row.bandwidth + ' Mbps' }}</span></template
        >
        <!-- 0 元参考售价要显式标出来：它在建品时会变成 0 元 SKU，是事故前兆，
             显示成空白或 "¥0" 都会被一眼扫过去。 -->
        <template #price="{ row }">
          <span v-if="!row.price || row.price <= 0" class="price-zero">0 元（未定价）</span>
          <span v-else>{{ formatPrice(row.price) }}</span>
        </template>
        <template #platform="{ row }">
          <t-tag v-if="row.platform_params" theme="success" variant="light" size="small" shape="round">已配映射</t-tag>
          <t-tag v-else theme="warning" variant="light" size="small" shape="round">缺平台映射</t-tag>
        </template>
        <template #status="{ row }"
          ><t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">{{
            row.status === 1 ? '启用' : '停用'
          }}</t-tag></template
        >
        <template #action="{ row }">
          <div class="action-cell">
            <MobileAction
              v-if="isMobile"
              :options="
                buildMobileActionOptions([
                  { content: '编辑', value: 'edit', theme: 'default' },
                  { content: '复制', value: 'clone', theme: 'default' },
                  { content: '删除', value: 'delete', theme: 'error' },
                ])
              "
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
      :width="isMobile ? '94vw' : '900px'"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="save"
      @close="closeDialog"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <!--
          弹窗内容按分类切成子页。之前一份配置档的全部内容 —— 基础字段、预置档位、
          4 个分组的几十个配置项、平台映射、模板与预览 —— 顺着一条长表单铺下来，
          找一个参数要滚好几屏。这里用标签页当小导航，一屏只留一类内容。
        -->
        <!--
          导航分两行：第一行是「档位本身 + 各配置项分组」（配什么），
          第二行是「平台映射 + 名称与描述」（映射与输出什么）。7 个标签挤一行时
          彼此贴太近、边界不清，分开之后一眼能分清两类内容。
        -->
        <template v-if="providerId">
          <t-tabs v-model="activeSection" theme="card" size="small" class="dialog-tabs">
            <t-tab-panel value="basic" label="档位信息" />
            <t-tab-panel
              v-for="grp in catalogGroups"
              :key="grpTabValue(grp.name)"
              :value="grpTabValue(grp.name)"
              :label="`${grp.name || '未分组'}（${grp.enabled}/${grp.rows.length}）`"
            />
          </t-tabs>
          <t-tabs v-model="activeSection" theme="card" size="small" class="dialog-tabs dialog-tabs--sub">
            <t-tab-panel value="mapping" label="平台映射" />
            <t-tab-panel value="template" label="名称与描述" />
          </t-tabs>
        </template>

        <!-- 档位信息 -->
        <template v-if="activeSection === 'basic'">
          <!-- 6 个字段刚好排满两列三行；「系统盘基线」同时喂给 {disk} 占位符与
               spec_values.storage.system.size，所以必须和 CPU/内存 放在一起改。 -->
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
            <t-form-item label="系统盘（GB）" name="disk">
              <t-input-number v-model="form.disk" :min="0" theme="column" :style="{ width: '100%' }" />
            </t-form-item>
            <t-form-item label="参考售价（元）" name="price">
              <t-input-number
                v-model="form.price"
                :min="0"
                :precision="2"
                theme="column"
                :style="{ width: '100%' }"
                placeholder="必填，不能为 0"
              />
              <!-- 0 元价的真实风险：建品时它会成为 SKU 售价，误配上架后客户真能 0 元买。
                   这里直接拦住，而不是等上架后才在价格矩阵里发现。 -->
              <span v-if="!form.price || form.price <= 0" class="price-warn">
                参考售价为 0：建品时该档位会生成 0 元 SKU，上架后客户可 0 元下单，请填写真实售价
              </span>
            </t-form-item>
            <t-form-item label="排序" name="sort_order">
              <t-input-number
                v-model="form.sort_order"
                theme="column"
                :style="{ width: '100%' }"
                placeholder="留空自动接在最后"
              />
              <span class="form-hint">新建时自动取当前最大值 +10，便于后续插入调整。</span>
            </t-form-item>
            <t-form-item label="状态" name="status">
              <t-select v-model="form.status" :options="statusOptions" />
            </t-form-item>
          </div>

          <t-form-item label="机型" name="machineKind">
            <t-radio-group v-model="machineKind" variant="default-filled" @change="onMachineKindChange">
              <t-radio-button v-for="opt in machineKindOptions" :key="opt.value" :value="opt.value">{{
                opt.label
              }}</t-radio-button>
            </t-radio-group>
            <p class="opt-row__help">
              {{ machineKindOptions.find((o) => o.value === machineKind)?.description || '' }}
              <template v-if="machineKind === 'nat'">
                NAT 机型不分配独立 IP，客户通过节点转发地址（nat_host_ip:ext_port）访问； 开通参数里 IP 数量固定为 0。
              </template>
            </p>
          </t-form-item>

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

          <div class="section-hint">
            已开 <strong>{{ enabledCount }}</strong> 个配置项；多选取值会生成客户可选配置项。
          </div>
        </template>

        <!-- 各分组的平台配置项：一次只渲染当前分组 -->
        <template v-for="grp in catalogGroups" :key="grpTabValue(grp.name)">
          <template v-if="activeSection === grpTabValue(grp.name)">
            <div class="section-hint">
              本分组已开 <strong>{{ grp.enabled }}</strong> / {{ grp.rows.length }} 项 · 多选取值会成为客户可选配置项
            </div>

            <div
              v-for="row in grp.rows"
              :key="row.spec.option_key"
              class="opt-row"
              :class="{
                'opt-row--on': row.enabled,
                'opt-row--collapsed': !isExpanded(row),
                'opt-row--error': highlightKey === row.spec.option_key,
              }"
              :data-opt-key="row.spec.option_key"
            >
              <div class="opt-row__head">
                <t-checkbox v-model="row.enabled" @change="onEnabledChange(row)">
                  <span class="opt-row__name">{{ row.spec.label || row.spec.option_key }}</span>
                </t-checkbox>
                <span class="opt-row__meta">
                  <code>{{ row.spec.option_key }}</code>
                  <t-tag v-if="row.spec.required" theme="error" variant="light" size="small" shape="round">必选</t-tag>
                  <span v-if="row.spec.unit">单位 {{ row.spec.unit }}</span>
                  <span v-if="row.spec.default_value">平台默认 {{ row.spec.default_value }}</span>
                  <t-link class="opt-row__toggle" size="small" @click="toggleExpand(row)">
                    {{ isExpanded(row) ? '收起' : '展开' }}
                  </t-link>
                </span>
              </div>

              <!--
                收起态只留一行「这一项配了什么」。34 个配置项每个都有一段说明文字，
                全铺出来光说明就占掉好几屏，所以说明随控件一起收在展开态里。
              -->
              <p v-if="!isExpanded(row) && row.enabled" class="opt-row__summary">{{ rowSummary(row) }}</p>

              <template v-else-if="isExpanded(row)">
                <p v-if="row.spec.help" class="opt-row__help">{{ row.spec.help }}</p>

                <div v-if="row.enabled" class="opt-row__body">
                  <template v-if="isQtyOption(row.spec)">
                    <!--
                      固定值 / 动态范围 二选一（用户要求）：
                      这类项多数时候是定死的（如"所有商品系统盘 30G"），只有偶尔才要让客户在区间内调。
                      原来恒显示"最小/最大/默认值"三个框，填固定值得把同一个数填两遍，
                      且默认值能填到区间之外（实测故障：带宽区间 [10,100]、默认值 0
                      → 下单表单预填出 0Mbps）。改成先选模式，固定值只填一个数。
                    -->
                    <div class="opt-inline opt-inline--mode">
                      <label class="opt-field">
                        取值方式
                        <t-radio-group
                          :model-value="isFixedQty(row) ? 'fixed' : 'range'"
                          variant="default-filled"
                          size="small"
                          @change="(v: string | number | boolean) => setQtyMode(row, String(v))"
                        >
                          <t-radio-button value="fixed">固定值</t-radio-button>
                          <t-radio-button value="range">动态范围</t-radio-button>
                        </t-radio-group>
                      </label>
                    </div>
                    <div class="opt-inline">
                      <template v-if="isFixedQty(row)">
                        <label class="opt-field">
                          固定值
                          <t-input-number v-model="row.rangeMin" :min="0" theme="column" />
                          <span class="opt-field__unit">{{ row.spec.unit || '' }}</span>
                        </label>
                      </template>
                      <template v-else>
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
                          <t-input-number
                            v-model="row.defaultNumber"
                            :min="row.rangeMin ?? 0"
                            :max="row.rangeMax ?? undefined"
                            theme="column"
                          />
                        </label>
                      </template>
                    </div>
                    <!-- 默认值必须在区间内：超界会让下单表单预填出客户看不懂的值 -->
                    <p v-if="qtyRangeError(row)" class="opt-row__error">{{ qtyRangeError(row) }}</p>
                  </template>
                  <template v-else>
                    <t-select
                      v-model="row.values"
                      multiple
                      clearable
                      filterable
                      :placeholder="
                        row.spec.value_source === 'manual' ? '手工填写取值（输入后回车添加）' : '选择允许客户选取的值'
                      "
                      :options="valueOptions(row)"
                      :loading="resourceLoading"
                      :creatable="row.spec.value_source === 'manual'"
                      @change="onValuesChange(row)"
                    />
                    <div class="opt-inline">
                      <label class="opt-field">
                        默认值
                        <t-select
                          v-model="row.default"
                          clearable
                          placeholder="默认选中"
                          :options="selectedValueOptions(row)"
                        />
                      </label>
                      <label class="opt-field">
                        分组（用户侧下拉分组）
                        <t-input v-model="row.groupLabel" placeholder="如 Ubuntu / Windows" />
                      </label>
                    </div>
                    <!-- 取值含义：
                         -1/0/auto 这类值光看取值看不出含义，选完就把目录里的说明列出来；
                         数量型没有候选取值，改提示文档给的平台默认口径。 -->
                    <div v-if="rowValuesHelp(row).length" class="opt-valuehint">
                      <span v-for="e in rowValuesHelp(row)" :key="e.value" class="opt-valuehint__item">
                        <code>{{ e.value }}</code> {{ e.text }}
                      </span>
                    </div>
                    <p v-else-if="row.spec.default_value" class="opt-row__help">
                      平台未传递时的默认：{{ row.spec.default_value }}
                    </p>
                  </template>
                </div>
              </template>
            </div>
          </template>
        </template>

        <!-- 平台映射 -->
        <template v-if="activeSection === 'mapping'">
          <div class="section-hint">生成 SKU 时下发给平台的基线参数（区域与节点至少填一个）。</div>
          <div class="form-grid">
            <t-form-item label="数据中心 area" :class="{ 'field-error': highlightKey === 'area' }">
              <t-select
                id="field-area"
                v-model="platformParams.area"
                clearable
                placeholder="平台区域"
                :options="areaOptions"
                @change="onAreaChange"
              />
            </t-form-item>
            <t-form-item label="节点 node" :class="{ 'field-error': highlightKey === 'node' }">
              <t-select
                id="field-node"
                v-model="platformParams.node"
                clearable
                placeholder="平台节点"
                :options="nodeOptions"
              />
            </t-form-item>
            <t-form-item label="存储 store">
              <t-select
                v-model="platformParams.store"
                clearable
                placeholder="系统盘存储（可选）"
                :options="storeOptions"
              />
            </t-form-item>
            <t-form-item label="镜像 os">
              <t-select v-model="platformParams.os" clearable placeholder="平台镜像" :options="imageOptions" />
            </t-form-item>
          </div>

          <!-- 机型参数：决定这条档位走独立 IP 还是 NAT 链路，必须显式下发 -->
          <div class="section-hint section-hint--sub">机型参数（决定开通链路：独立 IP 还是 NAT 转发）</div>
          <div class="form-grid">
            <t-form-item label="IP 数量 ip_num">
              <t-input-number
                v-if="machineKind !== 'nat'"
                v-model="platformParams.ip_num"
                :min="1"
                :max="20"
                theme="column"
                :style="{ width: '100%' }"
              />
              <div v-else class="machine-locked">
                <strong>0</strong>
                <span>NAT 机型不分配独立 IP（固定为 0，不可改）</span>
              </div>
            </t-form-item>
            <t-form-item label="NAT 端口转发 nat_acl_limit">
              <t-select
                v-model="platformParams.nat_acl_limit"
                :disabled="machineKind !== 'nat'"
                :options="natLimitOptions"
                placeholder="-1 不支持"
              />
            </t-form-item>
            <t-form-item label="共享建站 nat_web_limit">
              <t-select
                v-model="platformParams.nat_web_limit"
                :disabled="machineKind !== 'nat'"
                :options="natLimitOptions"
                placeholder="-1 不支持"
              />
            </t-form-item>
          </div>

          <t-form-item label="其他平台参数（高级 JSON）">
            <t-textarea
              v-model="extraParamsText"
              :autosize="{ minRows: 2, maxRows: 5 }"
              placeholder='上面下拉之外的键，如 {"network_type":"normal","snapshot_num":1}'
            />
          </t-form-item>
        </template>

        <!-- 名称与描述 -->
        <template v-if="activeSection === 'template'">
          <div class="section-hint">模板里的 {cpu}/{memory} 等占位符按「平台映射」与配置项的基线取值展开。</div>
          <t-form-item label="商品名模板">
            <t-input v-model="form.name_template" placeholder="{cpu}核{memory}G {os}" />
          </t-form-item>
          <t-form-item label="描述模板">
            <t-textarea
              v-model="form.description_template"
              :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="{cpu}核{memory}GB 内存 / {disk}GB 系统盘 / {bw}Mbps 带宽"
            />
          </t-form-item>
          <t-alert theme="info">
            <template #message>
              <div>预览（按基线取值展开）：</div>
              <div class="preview-line">
                商品名：<strong>{{ previewName || '—' }}</strong>
              </div>
              <div class="preview-line">
                SKU 编码：<code>{{ previewCode || '—' }}</code>
              </div>
              <div class="preview-line">描述：{{ previewDesc || '—' }}</div>
              <div class="preview-line">
                平台参数：<code>{{ previewPlatformParams }}</code>
              </div>
            </template>
          </t-alert>
        </template>

        <t-alert v-if="!providerId" theme="warning" message="请先选择对接平台，才能读到该平台的配置项目录。" />
        <t-alert
          v-else-if="!catalog.length"
          theme="info"
          message="该平台暂无配置项目录，请先到「产品管理 → 规格管理 → 平台配置项」同步目录。"
        />
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'

import { AddIcon, AppIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  createSpecTemplate,
  deleteSpecTemplate,
  getOptionCatalog,
  getSpecTemplateList,
  updateSpecTemplate,
} from '@/api/product'
import { getProviderList, getProviderPlatformResources } from '@/api/admin'
import { formatPrice } from '@/pages/product/constants'
import type {
  OptionSpecInfo,
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

const filters = reactive<{ keyword?: string; provider_type?: string; machine_kind?: string; status?: number }>({})

const machineKindFilterOptions = [
  { label: '独立 IP 机型', value: 'vps' },
  { label: 'NAT 机型', value: 'nat' },
  { label: '未标注', value: 'unset' },
]

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(load)

const statusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const columns: PrimaryTableCol<SpecTemplateInfo>[] = [
  { colKey: 'spec', title: '档位', minWidth: 200 },
  { colKey: 'machine', title: '机型', width: 110 },
  { colKey: 'specs', title: '基线配置', width: 160 },
  { colKey: 'bandwidth', title: '带宽', width: 100 },
  { colKey: 'platform', title: '平台映射', width: 110 },
  { colKey: 'price', title: '参考售价', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 170,
    fixed: 'right' as const,
    align: 'center' as const,
  },
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
      machine_kind: filters.machine_kind,
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

/**
 * nextSortOrder 计算新建档位的默认排序号。
 *
 * 为什么不能恒为 0：排序 0 的档位会挤在一起，列表顺序只能靠 id 兜底，
 * 运营想调顺序也没有"先排谁"的数值空间。取当前最大值 +10 留出插入余量
 * （想插两个档位之间时，中间还有 1~9 可用）。
 *
 * 取数走一次 page_size=100 的轻量查询而不是翻遍分页：档位通常远少于 100 条，
 * 超出的极端情况下算不准也只是排序号偏小，不影响功能。
 */
async function nextSortOrder(): Promise<number> {
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100 })
    const max = data.items.reduce((acc, item) => Math.max(acc, item.sort_order || 0), 0)
    return max + 10
  } catch {
    return 10
  }
}
function resetFilters() {
  filters.keyword = undefined
  filters.provider_type = undefined
  filters.machine_kind = undefined
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
      label: `${item.name}（${item.provider_type}）`,
      value: item.id,
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
// 弹窗内的子页导航：'basic' / 'group:<分组名>' / 'mapping' / 'template'。
// 第一次选平台后落到「档位信息」，换平台会重置回这里（旧的 group:xxx 对新平台无意义）。
const activeSection = ref('basic')
// 已展开编辑的配置项（option_key）。默认全部收起，一屏能看到全部分组内容；
// 展开状态是纯 UI 状态，不进 rows，避免混进保存载荷。
const expandedKeys = ref<string[]>([])
const extraParamsText = ref('')
// highlightKey：校验失败后要高亮哪个字段（红框）。配合 focusSection 使用，
// 3 秒后自动清除；置于此处是因为它同时被「平台映射」与配置项行消费。
const highlightKey = ref('')
const platformParams = reactive<{
  area?: string
  node?: string
  store?: string
  os?: string
  ip_num?: number
  nat_acl_limit?: string
  nat_web_limit?: string
}>({
  area: undefined,
  node: undefined,
  store: undefined,
  os: undefined,
  ip_num: undefined,
  nat_acl_limit: undefined,
  nat_web_limit: undefined,
})
const PLATFORM_KEYS = ['area', 'node', 'store', 'os'] as const
// 机型参数（doc122）：与 PLATFORM_KEYS 一样是「平台上的一等字段」，写进 platform_params。
// 单列出来是因为它们决定机型，NAT 的 ip_num 必须显式下发 0，不能靠高级 JSON 手写。
const MACHINE_KEYS = ['ip_num', 'nat_acl_limit', 'nat_web_limit'] as const

// ===== 机型（doc122）=====
// 魔方云两条开通链路共用同一接口，机型完全由参数取值决定，面板不校验组合：
//   vps：ip_num >= 1 且 nat_*_limit = -1；
//   nat：ip_num = 0 且 nat_acl_limit / nat_web_limit 至少一个 >= 0。
// 后端同样规则兜底（spec/service/machine_kind.go），这里先给出可读提示。
type MachineKind = '' | 'vps' | 'nat'
const machineKind = ref<MachineKind>('vps')
const machineKindOptions = [
  { label: '独立 IP 机型', value: 'vps', description: '分配独立公网 IP，客户直连' },
  { label: 'NAT 机型', value: 'nat', description: '不分配独立 IP，经节点转发地址访问' },
]
const natLimitOptions = [
  { label: '不支持（-1）', value: '-1' },
  { label: '不限制（0）', value: '0' },
  { label: '限 10 个（10）', value: '10' },
]

const form = reactive<{
  id: number
  name: string
  price: number | undefined
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
    // price 初始 undefined（不是 0）：0 元参考售价会生成 0 元 SKU，
    // 显示成空 + 必填校验，比预填 0 再拦住更不容易漏。
    id: 0,
    name: '',
    price: undefined as number | undefined,
    sort_order: 0,
    status: 1,
    disk: 40,
    disk_type: 'ssd',
    os: '',
    name_template: '{cpu}核{memory}G {os}',
    description_template: '{cpu}核{memory}GB 内存 / {disk}GB 系统盘 / {bw}Mbps 带宽',
  }
}

// 两套预置档位（doc122，规格按运营常用配置补齐）：选机型后只显示对应那一组，
// 点一下同时设定机型与参数基线。系统盘/带宽取该规格的常见搭配，按需再改。
const vpsPresets = [
  { name: '2核4G', cpu: '2', memory: '4096', disk: 40, bw: 5 },
  { name: '4核4G', cpu: '4', memory: '4096', disk: 50, bw: 10 },
  { name: '4核8G', cpu: '4', memory: '8192', disk: 60, bw: 10 },
  { name: '8核8G', cpu: '8', memory: '8192', disk: 80, bw: 20 },
  { name: '8核16G', cpu: '8', memory: '16384', disk: 100, bw: 20 },
  { name: '16核16G', cpu: '16', memory: '16384', disk: 100, bw: 30 },
  { name: '16核32G', cpu: '16', memory: '32768', disk: 120, bw: 30 },
]
// NAT 机型规格普遍更小（共享节点出口，成本低），整数核起步。
const natPresets = [
  { name: '1核1G', cpu: '1', memory: '1024', disk: 20, bw: 5 },
  { name: '2核2G', cpu: '2', memory: '2048', disk: 30, bw: 10 },
  { name: '2核4G', cpu: '2', memory: '4096', disk: 40, bw: 10 },
  { name: '4核4G', cpu: '4', memory: '4096', disk: 50, bw: 10 },
  { name: '4核8G', cpu: '4', memory: '8192', disk: 60, bw: 20 },
  { name: '8核8G', cpu: '8', memory: '8192', disk: 80, bw: 20 },
]
const presets = computed(() => (machineKind.value === 'nat' ? natPresets : vpsPresets))

const enabledCount = computed(() => rows.value.filter((r) => r.enabled).length)

const catalogGroups = computed(() => {
  const out: { name: string; rows: OptionRow[]; enabled: number }[] = []
  for (const row of rows.value) {
    const name = row.spec.group_name || ''
    let g = out.find((x) => x.name === name)
    if (!g) {
      g = { name, rows: [], enabled: 0 }
      out.push(g)
    }
    g.rows.push(row)
    if (row.enabled) g.enabled += 1
  }
  return out
})

/** 分组名 → 标签页 value。分组名可能为空串（未分组），空值不能当 tab value 用。 */
function grpTabValue(name: string): string {
  return `group:${name}`
}

// ===== 配置项行的展开/收起 =====
// 勾上勾选时自动展开，省得「勾了却看不到要配什么」再点一次；取消勾选则收起。
function isExpanded(row: OptionRow): boolean {
  return expandedKeys.value.includes(row.spec.option_key)
}

function toggleExpand(row: OptionRow) {
  const key = row.spec.option_key
  expandedKeys.value = isExpanded(row) ? expandedKeys.value.filter((k) => k !== key) : [...expandedKeys.value, key]
}

function onEnabledChange(row: OptionRow) {
  if (row.enabled) {
    if (!isExpanded(row)) expandedKeys.value = [...expandedKeys.value, row.spec.option_key]
  } else {
    expandedKeys.value = expandedKeys.value.filter((k) => k !== row.spec.option_key)
  }
}

function resetExpanded() {
  expandedKeys.value = []
}

/** 收起态的一行摘要：数量型给范围，多选型给标签名。 */
function rowSummary(row: OptionRow): string {
  if (isQtyOption(row.spec)) {
    const unit = row.spec.unit ? ` ${row.spec.unit}` : ''
    const def = row.defaultNumber != null ? `，默认 ${row.defaultNumber}${unit}` : ''
    return `取值范围 ${row.rangeMin} – ${row.rangeMax}${unit}${def}`
  }
  if (!row.values.length) return '已开但未选值，保存时会跳过'
  const labelOf = new Map(valueOptions(row).map((o) => [o.value, o.label]))
  const names = row.values.map((v) => labelOf.get(v) || v)
  const head = names.slice(0, 4).join('、')
  const more = names.length > 4 ? ` 等 ${names.length} 项` : ''
  const def = row.default ? `，默认 ${labelOf.get(row.default) || row.default}` : ''
  return `可选：${head}${more}${def}`
}

/** rowValuesHelp 已选取值里能查到含义的那些（-1 不开通 / 0 不限量 / auto 随机端口）。 */
function rowValuesHelp(row: OptionRow): { value: string; text: string }[] {
  const map = (row.spec.options_help || {}) as Record<string, string>
  return row.values.filter((v) => map[v]).map((v) => ({ value: v, text: String(map[v]) }))
}

function isQtyOption(spec: OptionSpecInfo): boolean {
  return spec.widget === 'qty' || spec.min_value != null || spec.max_value != null
}

/**
 * 数量型是否按「固定值」配置：最小 == 最大。
 *
 * 用区间相等来表达固定值，而不是新增一个持久化字段——后端 option_selections
 * 的契约就是 range: [min, max]，相等即固定，不引入第二套口径（也就不用改后端与存量数据）。
 */
function isFixedQty(row: OptionRow): boolean {
  const min = row.rangeMin
  const max = row.rangeMax
  return min != null && max != null && Number(min) === Number(max)
}

/** 切换数量型取值方式：切到「固定值」时把最大对齐到最小（默认值也拉回区间内）。 */
function setQtyMode(row: OptionRow, mode: string) {
  if (mode === 'fixed') {
    row.rangeMax = row.rangeMin ?? 0
    row.defaultNumber = row.rangeMin ?? 0
  } else if (row.rangeMin != null && row.rangeMax != null && Number(row.rangeMin) === Number(row.rangeMax)) {
    // 动态范围：把上限放开一档，否则刚切过去还是固定值，看起来像没生效。
    row.rangeMax = Number(row.rangeMin) + 1
  }
}

/**
 * 数量型的区间/默认值校验（保存前拦，避免脏配置流到下单表单）。
 *
 * 关键一条是默认值必须落在区间内：实测故障里模板带宽区间 [10,100]、默认值却是 0，
 * 下单表单照默认值预填出 0Mbps，客户看到的带宽等于无效值。
 */
function qtyRangeError(row: OptionRow): string {
  const min = row.rangeMin
  const max = row.rangeMax
  if (min == null || max == null) return ''
  if (Number(min) > Number(max)) return '最小值不能大于最大值'
  const def = row.defaultNumber
  if (def != null && (Number(def) < Number(min) || Number(def) > Number(max))) {
    return `默认值 ${def} 不在区间 [${min}, ${max}] 内，客户下单时会预填出无效值`
  }
  return ''
}

/** 是否存在数量型配置错误（保存前统一拦一次）。 */
function firstQtyRangeError(): { name: string; message: string } | null {
  for (const row of rows.value) {
    if (!row.enabled || !isQtyOption(row.spec)) continue
    const err = qtyRangeError(row)
    if (err) return { name: row.spec.label || row.spec.option_key, message: err }
  }
  return null
}

/**
 * valueOptions 配置档里的候选取值。
 * - static：目录声明里的 options（cpu/memory 这类静态枚举）；
 * - manual：库里/目录里带候选值的（port 的 auto），运营还能在控件里新建；
 * - 平台来源（images/areas/nodes/stores）：库里的取值（含分组）。
 * 三处合并去重，不只看 values，否则 manual 项的 options 会整个丢掉（port 就只剩空）。
 */
/**
 * parseRawValueLabel 解析平台原始取值名「传值|地区^展示名」→ 展示名。
 *
 * 平台目录里的取值名是原始形态（实测样本）：
 *   `1|CN^香港CN2`      → 取值 1，地区 CN，展示名 香港CN2
 *   `auto`               → 无竖线，展示名即原值
 *   `12|CentOS^CentOS-7.6.1810-x64` → 取值 12，展示名 CentOS-7.6.1810-x64
 *
 * 与后端 product_customer_options.upstreamSubLabel 同一套口径：
 * 先取竖线后半段，再取 `^` 后的明细（没明细时取 `^` 前那段）。
 * 不解析的话编辑器里会显示成「1|CN^默认（1）」，运营既看不懂也没法核对。
 */
function parseRawValueLabel(raw: string): string {
  let name = String(raw || '').trim()
  if (!name) return ''
  const bar = name.indexOf('|')
  if (bar >= 0) name = name.slice(bar + 1).trim()
  const caret = name.indexOf('^')
  if (caret >= 0) {
    const detail = name.slice(caret + 1).trim()
    name = detail || name.slice(0, caret).trim()
  }
  return name
}

function valueOptions(row: OptionRow): { label: string; value: string }[] {
  // 展示名与传值都要露出来：运营核对的是"传给平台什么、客户看到什么"两件事，
  // 只显示其一会让对不上的问题查不出来。
  const labelOf = (label: string, value: string) => {
    const pretty = parseRawValueLabel(label)
    if (!pretty) return value
    return pretty !== value ? `${pretty}（传值 ${value}）` : pretty
  }
  const out = new Map<string, string>()
  const push = (value: string, label: string) => {
    if (!value || out.has(value)) return
    out.set(value, labelOf(label, value))
  }
  for (const v of row.spec.values || []) {
    push(v.value, v.group_label ? `${v.group_label} / ${v.label || v.value}` : v.label || v.value)
  }
  for (const o of specStaticOptions(row.spec)) {
    push(o.value, o.label)
  }
  return [...out.entries()].map(([value, label]) => ({ label, value }))
}

/** specStaticOptions 解析目录项自带的 options（可能是数组或 JSON 字符串，或已解析对象）。 */
function specStaticOptions(spec: OptionSpecInfo): { value: string; label: string }[] {
  const raw = spec.options as unknown
  if (!raw) return []
  if (Array.isArray(raw)) {
    return raw
      .map((o) => {
        const item = o as { value?: unknown; label?: unknown }
        const value = item.value != null ? String(item.value) : ''
        return { value, label: item.label != null ? String(item.label) : value }
      })
      .filter((o) => o.value)
  }
  if (typeof raw === 'string') {
    try {
      return specStaticOptions({ ...spec, options: JSON.parse(raw) })
    } catch {
      return []
    }
  }
  return []
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
    return row.defaultNumber != null ? String(row.defaultNumber) : row.spec.default_value || undefined
  }
  return row.default || row.values[0]
}

/**
 * resolveBaseline 取某参数的「基线值」，两处来源合并：
 *   1. 平台映射下拉（platformParams）—— 运营显式设定的出站基线；
 *   2. 配置项目录里的同名项（rows）—— 在「位置与调度」等分组勾选并设定默认值时。
 *
 * 为什么两处都要认：area/node/os/store 既是「平台映射」的基线字段，也是配置项目录里的
 * 条目（客户可选）。运营在配置项里勾选并设了默认值，生成的 SKU 同样需要这个值，
 * 只认下拉会造成"选了却提示未选"、且生成的档位真的缺参数（平台拒绝开通）。
 */
function resolveBaseline(key: string): string {
  const fromDropdown = (platformParams as Record<string, unknown>)[key]
  if (fromDropdown != null && String(fromDropdown).trim() !== '') return String(fromDropdown)
  return baselineOf(key) || ''
}

/**
 * focusSection 把弹窗切到出错的分组并高亮该字段（doc122 交互优化）。
 *
 * 配置项有几十个、分在多页签里，不跳转的话运营要自己逐页找"哪个没填"。
 */
function focusSection(section: string, fieldKey?: string) {
  activeSection.value = section
  if (!fieldKey) return
  // 高亮：先把之前的高亮清掉，再在下一帧标记，保证二次触发也有视觉反馈。
  highlightKey.value = ''
  nextTick(() => {
    highlightKey.value = fieldKey
    // 滚动到该字段所在行（配置项行有 data-opt-key，映射区用 id 定位）。
    const el = document.querySelector(`[data-opt-key="${fieldKey}"]`) || document.getElementById(`field-${fieldKey}`)
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    window.setTimeout(() => {
      if (highlightKey.value === fieldKey) highlightKey.value = ''
    }, 3000)
  })
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
    return hit ? hit.label.split(' / ').pop() || hit.label : osValue
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

function applyPreset(p: { name: string; cpu: string; memory: string; disk: number; bw: number }) {
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
  // 机型参数基线一并填上：少了它档位要么被后端互斥校验拒，要么开出无法访问的机器。
  applyMachineBaseline()
  // 必选项默认勾上，省得漏配导致开通被平台拒。
  for (const row of rows.value) {
    if (row.spec.required && !isQtyOption(row.spec) && !row.enabled && row.spec.values?.length) {
      row.enabled = true
      row.values = [row.spec.values[0].value]
      row.default = row.values[0]
    }
  }
  expandKeys(['cpu', 'memory', 'bw'])
}

/**
 * applyMachineBaseline 按当前机型写入 ip_num / nat_*_limit 基线。
 *
 * 这两个值必须进 platform_params 而不能只留在界面上：NAT 的 ip_num=0 一旦缺失，
 * 平台会按必填校验拒绝开通；而若被默认成 >=1，客户付 NAT 的价钱却拿到独立 IP。
 */
function applyMachineBaseline() {
  if (machineKind.value === 'nat') {
    platformParams.ip_num = 0
    if (platformParams.nat_acl_limit == null) platformParams.nat_acl_limit = '0'
    if (platformParams.nat_web_limit == null) platformParams.nat_web_limit = '-1'
  } else if (machineKind.value === 'vps') {
    if (platformParams.ip_num == null || platformParams.ip_num < 1) platformParams.ip_num = 1
    platformParams.nat_acl_limit = '-1'
    platformParams.nat_web_limit = '-1'
  }
}

/** onMachineKindChange 切换机型：立即刷新参数基线，并拦掉与机型冲突的客户配置项。 */
function onMachineKindChange() {
  applyMachineBaseline()
  // NAT 机型的 ip_num 固定为 0，不能同时作为客户可选配置项：
  // 客户选配优先级高于档位基线，选了非 0 会把 0 覆盖掉。
  if (machineKind.value === 'nat') {
    const row = rows.value.find((r) => r.spec.option_key === 'ip_num')
    if (row?.enabled) {
      row.enabled = false
      MessagePlugin.warning('NAT 机型的 ip_num 固定为 0，已关闭「IP 数量」配置项')
    }
  }
}

/** 把指定配置项加入展开态（预置填充后让改动可见）。 */
function expandKeys(keys: string[]) {
  expandedKeys.value = [...new Set([...expandedKeys.value, ...keys])]
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
  expandKeys(['cpu', 'memory'])
}

function buildOptionSelections(): SpecOptionSelections {
  const out: SpecOptionSelections = {}
  for (const row of rows.value) {
    if (!row.enabled) continue
    const key = row.spec.option_key
    if (isQtyOption(row.spec)) {
      const min = row.rangeMin ?? row.spec.min_value ?? 0
      const max = row.rangeMax ?? row.spec.max_value ?? 0
      // 固定值时 min == max，默认值直接取该值：不让"默认值"再成为独立输入，
      // 从源头消除"默认值落在区间外"（那是下单表单预填出 0 的根因）。
      const fixed = Number(min) === Number(max)
      const def = fixed ? String(min) : row.defaultNumber != null ? String(row.defaultNumber) : row.spec.default_value
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
  // 机型参数：ip_num 的 0 是有效值，不能用真值判断（0 会被跳过 → NAT 下发不出 0）。
  if (platformParams.ip_num != null) merged.ip_num = Number(platformParams.ip_num)
  for (const key of ['nat_acl_limit', 'nat_web_limit'] as const) {
    const v = platformParams[key]
    if (v != null && String(v).trim() !== '') merged[key] = Number(v)
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
  machineKind.value = 'vps'
  activeSection.value = 'basic'
  resetExpanded()
  Object.assign(platformParams, {
    area: undefined,
    node: undefined,
    store: undefined,
    os: undefined,
    ip_num: undefined,
    nat_acl_limit: undefined,
    nat_web_limit: undefined,
  })
  resources.value = { areas: [], nodes: [], stores: [], images: [] }
  extraParamsText.value = ''
  // 排序号默认接在现有档位之后（恒 0 会让新档位与老档位挤在同一序位）。
  form.sort_order = await nextSortOrder()
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
    // 机型参数提升为界面字段：老档位把它们写在高级 JSON 里，打开时收编到控件上，
    // 否则运营看不到当前机型设置，一保存就按默认值覆盖（NAT 的 0 会被改回 1）。
    ip_num: params.ip_num != null ? Number(params.ip_num) : undefined,
    nat_acl_limit: params.nat_acl_limit != null ? String(params.nat_acl_limit) : undefined,
    nat_web_limit: params.nat_web_limit != null ? String(params.nat_web_limit) : undefined,
  })
  const rest: Record<string, unknown> = { ...params }
  for (const key of PLATFORM_KEYS) delete rest[key]
  for (const key of MACHINE_KEYS) delete rest[key]
  extraParamsText.value = Object.keys(rest).length ? JSON.stringify(rest, null, 2) : ''
  machineKind.value = (row.machine_kind as MachineKind) || ''

  if (providerId.value) await loadPlatformResources(providerId.value)
  catalog.value = await loadCatalog(providerType.value)
  const selections = (row.option_selections || {}) as SpecOptionSelections
  rows.value = catalog.value.map((spec) => buildRow(spec, selections))
  activeSection.value = 'basic'
  // 打开时全部收起：一屏能看到这个分组所有的配置项，每行带一句摘要说明配了什么；
  // 要改哪项点「展开」即可。默认把已开的项全展开反而又把弹窗撑长了。
  resetExpanded()
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
  Object.assign(platformParams, {
    area: undefined,
    node: undefined,
    store: undefined,
    os: undefined,
    ip_num: undefined,
    nat_acl_limit: undefined,
    nat_web_limit: undefined,
  })
  activeSection.value = 'basic'
  resetExpanded()
  await loadPlatformResources(pid)
  // 换平台后旧勾选对新平台无效，全部重置（避免跨平台脏值）。
  catalog.value = await loadCatalog(providerType.value)
  rows.value = catalog.value.map((spec) => buildRow(spec, {}))
}

function closeDialog() {
  dialogVisible.value = false
}

/**
 * onAreaChange 换区域时清掉节点：节点选项按区域过滤（filterByArea），
 * 留着上一个区域的节点会变成"选了个在新区域里不存在的节点"，开通即失败。
 */
function onAreaChange() {
  if (platformParams.node && !nodeOptions.value.some((o) => o.value === platformParams.node)) {
    platformParams.node = undefined
  }
  if (platformParams.store && !storeOptions.value.some((o) => o.value === platformParams.store)) {
    platformParams.store = undefined
  }
}

async function save() {
  if (!form.name.trim()) {
    focusSection('basic')
    MessagePlugin.warning('请输入档位名称')
    return
  }
  if (!providerType.value) {
    focusSection('basic')
    MessagePlugin.warning('请选择对接平台')
    return
  }
  // 0 元参考售价会变成 0 元 SKU，误上架后客户能 0 元下单 —— 拦住而不是事后补救。
  if (!form.price || form.price <= 0) {
    focusSection('basic')
    MessagePlugin.warning('参考售价必须大于 0：0 元档位生成 0 元 SKU，上架后客户可 0 元下单')
    return
  }
  const platformParamsPayload = buildPlatformParams()
  if (platformParamsPayload === null) return
  const selections = buildOptionSelections()
  if (!Object.keys(selections).length) {
    // 跳到第一个配置项分组，省得运营在 7 个页签里找"该去哪勾"。
    const first = catalogGroups.value[0]
    focusSection(first ? grpTabValue(first.name) : 'basic')
    MessagePlugin.warning('请至少勾选一个配置项，否则该档位无法生成客户可选配置')
    return
  }
  // 数量型的区间/默认值自检：默认值超出区间时下单表单会预填出无效值
  // （实测故障：带宽区间 [10,100] 默认 0 → 客户看到 0Mbps）。后端也会拦，这里先给可读提示。
  const qtyErr = firstQtyRangeError()
  if (qtyErr) {
    MessagePlugin.warning(`「${qtyErr.name}」：${qtyErr.message}`)
    return
  }
  // 区域/节点可能配在两处：①「平台映射」的下拉（基线）；②「位置与调度」分组的配置项（客户可选）。
  // 只在②里勾选时，基线仍是空的，而生成 SKU 用的是基线——两边都要认，否则运营明明选了
  // 却被提示"未选"，且生成的档位确实缺位置参数。缺位置参数平台会直接拒绝开通。
  const areaValue = resolveBaseline('area')
  const nodeValue = resolveBaseline('node')
  if (!areaValue && !nodeValue) {
    focusSection('mapping', 'area')
    MessagePlugin.warning('请至少选择数据中心或节点（平台要求二者至少传一个）')
    return
  }
  // 只在配置项里选了区域/节点时，把它们补进基线，否则生成的 SKU 缺位置参数。
  if (!platformParamsPayload.area && areaValue) platformParamsPayload.area = areaValue
  if (!platformParamsPayload.node && nodeValue) platformParamsPayload.node = nodeValue
  if (!platformParamsPayload.os) {
    const osValue = resolveBaseline('os')
    if (osValue) platformParamsPayload.os = osValue
  }
  if (!platformParamsPayload.store) {
    const storeValue = resolveBaseline('store')
    if (storeValue) platformParamsPayload.store = storeValue
  }
  // 机型与本档位参数必须自洽（doc122）。后端同样校验，这里先给可读提示：
  // 最危险的组合是 NAT 机型没开任何转发入口 —— 机器开得出来但客户连不上，
  // 面板不会报错，等客户投诉才发现。
  const kindError = machineKindError(selections)
  if (kindError) {
    focusSection('mapping', machineKind.value === 'nat' ? 'ip_num' : 'nat_acl_limit')
    MessagePlugin.warning(kindError)
    return
  }
  const payload = {
    name: form.name.trim(),
    provider_type: providerType.value,
    machine_kind: machineKind.value || '',
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

/**
 * machineKindError 机型与参数的互斥校验，规则与后端 normalizeAndValidateMachineKind 一致。
 * 未标注机型（''）跳过：存量档位没有这个维度，不能因为新增校验就让它们改不动。
 */
function machineKindError(selections: SpecOptionSelections): string {
  const kind = machineKind.value
  if (!kind) return ''
  const ipNum = platformParams.ip_num
  const acl = numOrNull(platformParams.nat_acl_limit)
  const web = numOrNull(platformParams.nat_web_limit)

  if (kind === 'nat') {
    if (ipNum == null) return 'NAT 机型请设置「IP 数量」为 0（不分配独立 IP）'
    if (ipNum !== 0) return 'NAT 机型的「IP 数量」必须为 0；需要独立 IP 请把机型改为「独立 IP 机型」'
    if ((acl == null || acl < 0) && (web == null || web < 0)) {
      return 'NAT 机型至少开通一种访问入口：NAT 端口转发或共享建站需 >= 0；否则机器没有独立 IP 也没有转发入口，客户将无法连接'
    }
    if (selections.ip_num) return 'NAT 机型的 IP 数量固定为 0，请取消勾选「IP 数量」配置项'
    return ''
  }
  if (ipNum != null && ipNum < 1) return '独立 IP 机型的「IP 数量」必须 >= 1；不分配独立 IP 请把机型改为「NAT 机型」'
  if (acl != null && acl >= 0) return '独立 IP 机型的「NAT 端口转发」必须为 -1（与独立 IP 互斥）'
  if (web != null && web >= 0) return '独立 IP 机型的「共享建站」必须为 -1（与独立 IP 互斥）'
  return ''
}

function numOrNull(v?: string | number): number | null {
  if (v == null || String(v).trim() === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
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
  const action =
    typeof value === 'string' || typeof value === 'number'
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
.dialog-tabs {
  margin-bottom: 8px;
}

/* 第二行导航（平台映射 / 名称与描述）与上一行之间留一截空白，
   让「配什么」和「映射/输出成什么」一眼分得开。 */
.dialog-tabs--sub {
  margin-bottom: 14px;
}

.section-hint {
  margin-bottom: 10px;
  font-size: 12.5px;
  color: var(--td-text-color-secondary, #888);
}

/* 平台映射页里的二级小标题（区域/节点/存储/镜像 与 机型参数 之间）。 */
.section-hint--sub {
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px dashed var(--td-component-border, #e0e0e0);
  font-weight: 600;
  color: var(--td-text-color-primary, #333);
}

/* NAT 机型的 ip_num 是固定只读值，用纯文本呈现而非禁用输入框，避免像"可以改但被锁定"。 */
.machine-locked {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.4;
}

.machine-locked strong {
  font-size: 15px;
}

.machine-locked span {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.cell-muted {
  color: var(--td-text-color-placeholder, #bbb);
}

.opt-row {
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 6px;
  padding: 6px 10px;
  margin-bottom: 6px;
  background: var(--td-bg-color-container, #fff);
}

.opt-row--on {
  border-color: var(--td-brand-color, #0052d9);
  box-shadow: 0 0 0 1px rgba(0, 82, 217, 0.1);
}

/* 校验失败定位：红框 + 浅红底，配合 focusSection 的滚动让"哪个没填"一眼可见。
   配置项有几十个分在多页签里，不标出来的话运营只能逐页翻找。 */
.opt-row--error {
  border-color: var(--td-error-color, #d54941);
  background: rgba(213, 73, 65, 0.06);
  box-shadow: 0 0 0 2px rgba(213, 73, 65, 0.18);
}

/* 0 元参考售价：建品时会变成 0 元 SKU，是事故前兆，用警示色标出来。 */
.price-zero {
  color: var(--td-warning-color, #e37318);
  font-weight: 600;
}

/* 表单内的警示文案（如 0 元价提示），紧跟在输入框下方。 */
.price-warn {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--td-warning-color, #e37318);
}

/* 表单内的普通说明文字。 */
.form-hint {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--td-text-color-secondary, #888);
}

.field-error {
  border-radius: 6px;
  background: rgba(213, 73, 65, 0.06);
  box-shadow: 0 0 0 1px var(--td-error-color, #d54941) inset;
  padding: 4px 8px;
  margin: -4px -8px;
}

/* 收起态收紧行高：一屏能多看几条，展开编辑时再回到常规间距。 */
.opt-row--collapsed {
  padding: 4px 10px;
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

.opt-row__toggle {
  font-size: 12px;
}

.opt-row__summary {
  margin: 2px 0 0 24px;
  font-size: 12.5px;
  color: var(--td-text-color-placeholder, #999);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.opt-row__help {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}

.opt-valuehint {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 12px;
}

.opt-valuehint__item {
  font-size: 12px;
  color: var(--td-text-color-placeholder, #999);
}

.opt-valuehint__item code {
  margin-right: 4px;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
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

/* 取值方式这一行只有一个单选框组，不该按多列网格铺满整行 */
.opt-inline--mode {
  grid-template-columns: 1fr;
}

.opt-field__unit {
  margin-left: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

/* 区间/默认值校验失败：与输入框同列，一眼看到是哪一项配错了 */
.opt-row__error {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--td-error-color-6, #d54941);
}

.preview-line {
  font-size: 13px;
  margin-top: 2px;
}

.preview-line code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

/* 窄屏下弹窗几乎满宽，配置项内的多列网格塌成一列。 */
@media (max-width: 768px) {
  .opt-inline {
    grid-template-columns: 1fr;
  }

  .opt-row__summary {
    white-space: normal;
  }
}
</style>
