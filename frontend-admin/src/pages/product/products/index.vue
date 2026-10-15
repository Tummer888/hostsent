<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">产品列表</h2>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="loadProducts">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="handleCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建产品
        </t-button>
        <t-button variant="outline" @click="handleClone">
          <template #icon>
            <CloudIcon aria-hidden="true" />
          </template>
          导入上游商品
        </t-button>
      </t-space>
    </header>

    <div class="products-layout">
      <!-- 分类导航：点节点 = 过滤该分类及其整棵子树（后端已支持子树展开）；移动端隐藏，用筛选卡里的分类下拉 -->
      <aside v-if="!isMobile" class="products-aside surface-card">
        <div class="products-aside__head">
          <h3 class="card-title">分类导航</h3>
          <t-link theme="primary" hover="color" class="products-aside__all" @click="selectAllCategories"
            >全部产品</t-link
          >
        </div>
        <t-input v-model="treeKeyword" size="small" clearable placeholder="搜索分类">
          <template #prefix-icon>
            <SearchIcon aria-hidden="true" />
          </template>
        </t-input>
        <div class="products-aside__tree">
          <t-tree
            v-model:expanded="asideExpanded"
            :data="filteredAsideTree"
            :keys="{ label: 'name', value: 'id', children: 'children' }"
            activable
            :actived="activeTreeKeys"
            line
            :expand-level="1"
            @active="onTreeActive"
          >
            <template #label="{ node }">
              <span class="aside-node" :class="{ 'aside-node--disabled': node.data.status !== 1 }">
                {{ node.data.name }}{{ node.data.status !== 1 ? '（停用）' : '' }}
              </span>
              <!--
                每个分类的商品数：没有这个数，导航只是棵树 —— 运营看不出哪个分类是空的，
                也无法判断某类要不要合并/删除。数字为 0 时也显示（"空分类"本身就是信息）。
              -->
              <span v-if="node.data.status === 1" class="aside-node__count">{{ node.data.count ?? 0 }}</span>
            </template>
          </t-tree>

          <!--
            「未分类」不是真实分类，没有 ID 可用，因此不进树上、单列一行：
            它的计数来自后端单独给的桶（category_id 为空或指向已删除分类）。
            没有这一行，这些商品在导航里完全不可见 —— 点遍所有分类都找不到它们。
          -->
          <div
            v-if="uncategorizedCount"
            class="aside-unclassified"
            :class="{ 'is-active': onlyUncategorized }"
            @click="selectUncategorized"
          >
            <span class="aside-unclassified__name">未分类</span>
            <span class="aside-node__count">{{ uncategorizedCount }}</span>
          </div>
        </div>
      </aside>

      <div class="products-main">
        <FilterCard>
          <div class="field">
            <span class="field__label">关键词</span>
            <t-input v-model="filters.keyword" placeholder="产品名称 / SKU 编码" clearable @enter="handleSearch" />
          </div>
          <div v-if="isMobile" class="field">
            <span class="field__label">分类</span>
            <t-select v-model="filters.category_id" clearable placeholder="全部分类" :options="categoryOptions" />
          </div>
          <div class="field">
            <span class="field__label">状态</span>
            <t-select v-model="filters.status" clearable placeholder="全部状态" :options="productStatusOptions" />
          </div>
          <div class="field">
            <span class="field__label">链路</span>
            <t-select v-model="filters.source_mode" clearable placeholder="全部链路" :options="sourceModeOptions" />
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
            <div class="table-card__meta products-crumb-wrap">
              <span v-if="activeCategoryPath.length" class="products-crumb">
                分类：{{ activeCategoryPath.map((n) => n.name).join(' / ') }}
                <t-link theme="primary" hover="color" @click="selectAllCategories">查看全部</t-link>
              </span>
              <!-- 未分类视图也要有面包屑：否则筛选生效了却看不出"在看哪一批"。 -->
              <span v-else-if="onlyUncategorized" class="products-crumb">
                分类：未分类
                <t-link theme="primary" hover="color" @click="selectAllCategories">查看全部</t-link>
              </span>
              <span>共 {{ pagination.total }} 个产品</span>
            </div>
          </div>

          <!-- 批量操作条：勾选后出现，支持跨页累计选择（selectedCount 由选区状态给出） -->
          <div class="batch-bar" :class="{ 'batch-bar--active': selectedRowKeys.length > 0 }">
            <span class="batch-bar__count">
              <template v-if="selectedRowKeys.length">已选 {{ selectedRowKeys.length }} 个产品</template>
              <template v-else>勾选商品后可批量上架 / 下架 / 删除</template>
            </span>
            <t-space size="small">
              <t-button
                size="small"
                variant="outline"
                :disabled="!selectedRowKeys.length || batchLoading"
                @click="handleBatchPublish"
              >
                批量上架
              </t-button>
              <t-button
                size="small"
                variant="outline"
                :disabled="!selectedRowKeys.length || batchLoading"
                @click="handleBatchUnpublish"
              >
                批量下架
              </t-button>
              <t-button
                size="small"
                theme="danger"
                variant="outline"
                :disabled="!selectedRowKeys.length || batchLoading"
                @click="handleBatchDelete"
              >
                批量删除
              </t-button>
              <t-link v-if="selectedRowKeys.length" theme="default" hover="color" @click="clearSelection"
                >清空选择</t-link
              >
            </t-space>
          </div>
          <t-table
            row-key="id"
            :data="productList"
            :columns="columns"
            :loading="loading"
            size="small"
            hover
            table-layout="fixed"
            cell-empty-content="—"
            :row-selection="rowSelection"
            :reserve-selected-row-on-paginate="true"
            :pagination="isMobile ? undefined : pagination"
            @page-change="handlePageChange"
            @select-change="handleSelectChange"
          >
            <template #name="{ row }">
              <div class="product-cell">
                <span class="cell-strong">{{ row.name }}</span>
                <span class="product-sub">SKU: {{ row.code }}</span>
              </div>
            </template>

            <template #category="{ row }">
              <span>{{ categoryName(row.category_id) }}</span>
            </template>

            <template #source_mode="{ row }">
              <t-tag :theme="sourceModeTag(row.source_mode).theme" variant="light" size="small" shape="round">
                {{ sourceModeTag(row.source_mode).text }}
              </t-tag>
            </template>

            <template #price="{ row }">
              <div class="price-cell">
                <span class="price-main">¥{{ formatPrice(row.price) }}</span>
                <span class="price-sub"
                  >{{ priceModelLabel(row.price_model) }} / 成本 ¥{{ formatPrice(row.cost_price) }}</span
                >
              </div>
            </template>

            <template #stock="{ row }">
              <span>{{ row.stock === -1 ? '不限' : row.stock }}</span>
            </template>

            <template #status="{ row }">
              <t-tag :theme="statusTag(row.status).theme" variant="light" size="small" shape="round">
                {{ statusTag(row.status).text }}
              </t-tag>
            </template>

            <template #action="{ row }">
              <div class="action-cell">
                <MobileAction
                  v-if="isMobile"
                  :options="
                    buildMobileActionOptions([
                      { content: '详情', value: 'detail', theme: 'default' },
                      { content: '改价', value: 'price', theme: 'default' },
                      { content: '上架', value: 'publish', hidden: () => row.status === 1, theme: 'success' },
                      { content: '下架', value: 'unpublish', hidden: () => row.status !== 1, theme: 'warning' },
                      { content: '删除', value: 'delete', theme: 'error' },
                    ])
                  "
                  @select="(value) => handleMobileAction(value, row)"
                />
                <template v-else>
                  <!--
                    「详情」即编辑入口：详情页与编辑页已合并（页面子页导航），
                    原来并列的「详情 / 编辑」会把同一份内容指向两个地址，
                    运营点哪个都行、改完还得自己判断该回哪。
                  -->
                  <t-link theme="primary" hover="color" @click="openDetail(row)">详情</t-link>
                  <t-link theme="primary" hover="color" @click="openPriceDialog(row)">改价</t-link>
                  <t-link v-if="row.status !== 1" theme="primary" hover="color" @click="handlePublish(row)"
                    >上架</t-link
                  >
                  <t-link v-else theme="warning" hover="color" @click="handleUnpublish(row)">下架</t-link>
                  <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
                </template>
              </div>
            </template>

            <template #empty>
              <t-empty description="暂无产品数据，请先新建产品" />
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
      </div>
    </div>

    <!--
      改价：对接商品的价格是「周期 × 价格」的矩阵（上游按周期定价并已给折扣），
      只填一个单价会让季付/年付档位算错，因此改用共用编辑器（百分比规则 / 逐周期自定义）。
      自营商品仍用下面的简单弹窗（它是单一售价模型）。
    -->
    <t-dialog
      v-model:visible="priceVisible"
      :header="priceProduct ? `改价 · ${priceProduct.name}` : '改价'"
      :width="isMobile ? '94vw' : '760px'"
      :footer="false"
      @close="priceVisible = false"
    >
      <UpstreamPriceEditor
        v-if="priceProduct && isUpstreamProduct(priceProduct)"
        :product-id="priceProduct.id"
        :product-name="priceProduct.name"
        :source-product-id="priceProduct.source_product_id"
        :upstream="priceUpstream"
        @saved="handlePriceSaved"
        @cancel="priceVisible = false"
      />
      <!-- 自营商品：单一售价模型，保持原有的两字段弹窗语义 -->
      <div v-else-if="priceProduct" class="self-price-form">
        <t-form label-align="top" :data="priceForm" @submit.prevent>
          <t-form-item label="销售价（元）" name="price">
            <t-input-number
              v-model="priceForm.price"
              :min="0"
              :precision="2"
              theme="column"
              placeholder="请输入销售价"
            />
          </t-form-item>
          <t-form-item label="成本价（元）" name="cost_price">
            <t-input-number
              v-model="priceForm.cost_price"
              :min="0"
              :precision="2"
              theme="column"
              placeholder="请输入成本价"
            />
          </t-form-item>
          <t-form-item label="备注" name="remark">
            <t-textarea
              v-model="priceForm.remark"
              :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="选填，记录本次调价说明"
            />
          </t-form-item>
        </t-form>
        <p class="self-price-form__hint">
          自营商品按单一售价与周期矩阵定价；如需按周期分别定价，请到「周期价格」页维护。
        </p>
        <div class="self-price-form__footer">
          <t-button variant="outline" @click="priceVisible = false">取消</t-button>
          <t-button theme="primary" @click="handleSavePrice">保存</t-button>
        </div>
      </div>
    </t-dialog>

    <t-dialog
      v-model:visible="cloneVisible"
      header="从上游商品导入"
      :width="isMobile ? '94vw' : '960px'"
      :confirm-btn="{
        content: `导入选中的 ${selectedProductIds.size} 个商品`,
        theme: 'primary',
        disabled: !selectedProductIds.size,
        loading: cloneSubmitting,
      }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleCloneConfirm"
      @close="cloneVisible = false"
    >
      <t-form label-align="top" :data="cloneForm" @submit.prevent>
        <!--
          导入设置：每个字段用「控件 + 说明」的纵向包裹（.clone-field）。
          说明必须放在自己的块里，不能直接塞进 t-form-item —— TDesign 的
          .t-form__controls-content 是不换行的 flex 行，中文的 min-content 宽度
          只有 1 个字，说明会被挤成竖排单字（实测截图故障）。
        -->
        <div class="clone-grid">
          <div class="clone-field clone-field--required">
            <span class="clone-field__label">上游提供商</span>
            <t-select
              v-model="cloneForm.source_provider_id"
              :options="providerOptions"
              placeholder="请选择上游提供商"
              @change="handleProviderChange"
            />
            <span class="clone-tip">只列可对接商品目录的渠道，自营平台不在列。</span>
          </div>
          <div class="clone-field clone-field--required">
            <span class="clone-field__label">归入本地分类</span>
            <!--
              分类必填（用户要求）：导入动辄几十上百个商品，留空会整批落进「未分类」——
              列表那一列显示不出归属、分类导航也统计不到，事后逐个回头补的成本远高于现在选一次。
              所以去掉 clearable 并在提交前拦一道（服务端同样校验，绕过界面也拦得住）。
            -->
            <t-select v-model="cloneForm.category_id" :options="categoryOptions" placeholder="请选择商品分类" />
            <span class="clone-tip">导入后按该分类归属展示与统计；一个分类下可放多个商品。</span>
          </div>
          <div class="clone-field">
            <span class="clone-field__label">定价（按上游标价）</span>
            <t-input-number v-model="cloneForm.price_percent" class="clone-percent" :min="1" :step="5" suffix="%" />
            <span class="clone-tip">售价 = 上游标价 × N%；100 按原价，&gt;100 加价，&lt;100 让利。</span>
          </div>
          <div class="clone-field">
            <span class="clone-field__label">导入后</span>
            <t-radio-group v-model="cloneForm.publish_now" class="clone-publish">
              <t-radio-button :value="true">直接上架</t-radio-button>
              <t-radio-button :value="false">先存草稿</t-radio-button>
            </t-radio-group>
            <span class="clone-tip"
              >财务型上游的开通参数由上游自带（原样转发），导入即可上架；选草稿则先编辑再上架。</span
            >
          </div>
        </div>

        <div class="clone-summary">
          <span
            >上游商品 {{ filteredProducts.length }} 个<template v-if="cloneKeyword"
              >（全部 {{ providerProducts.length }} 个）</template
            ></span
          >
          <span class="clone-summary__split">
            在售 {{ onSaleCount }} · 未在售 {{ filteredProducts.length - onSaleCount }}
            <template v-if="importedCount"> · 已导入 {{ importedCount }}</template>
          </span>
          <span v-if="discountHint" class="clone-summary__hint">{{ discountHint }}</span>
          <span class="clone-summary__pick">
            <template v-if="selectedProductIds.size"
              >已选 {{ selectedProductIds.size }} 个 · 预计毛利 {{ formatPrice(selectedProfit) }}/月</template
            >
            <template v-else>共 {{ selectableCount }} 个可导入</template>
          </span>
        </div>

        <div class="clone-search">
          <t-input v-model="cloneKeyword" clearable placeholder="搜索商品名称 / 规格 / 上游 ID / 分类名，快速定位">
            <template #prefix-icon>
              <SearchIcon size="14" aria-hidden="true" />
            </template>
          </t-input>
          <span class="clone-search__hint">
            <template v-if="cloneKeyword">
              匹配 {{ filteredProducts.length }} 个商品，分布在 {{ visibleGroups.length }} 个分类
              <t-link theme="primary" hover="color" @click="cloneKeyword = ''">清空</t-link>
            </template>
            <template v-else>输入关键词可按分类与商品快速定位（上游商品常有几百个，靠滚动很难找）</template>
          </span>
        </div>

        <p class="clone-hint">
          「隐藏」= 上游商铺未展示（分类被上游隐藏 / 商品隐藏 / 已下架），仍可导入开通； 「已导入」=
          该上游商品已经导入过，不再重复勾选 —— 点击可直接打开本地商品详情。
        </p>

        <div class="clone-groups">
          <div v-if="cloneLoading" class="clone-empty">正在加载上游商品…</div>
          <div v-else-if="!providerProducts.length" class="clone-empty">请先选择上游提供商，加载商品分组</div>
          <div v-else-if="!visibleGroups.length" class="clone-empty">
            没有匹配「{{ cloneKeyword }}」的上游商品，换个关键词试试
          </div>
          <div v-for="group in visibleGroups" :key="group.id" class="clone-group">
            <div class="clone-group__head">
              <t-checkbox
                :checked="isGroupChecked(group)"
                :disabled="!hasSelectable(group)"
                @change="() => toggleGroup(group)"
              >
                {{ group.name }}
                <t-tag v-if="group.derived" size="small" variant="light-outline" theme="warning">派生</t-tag>
              </t-checkbox>
              <span class="clone-group__count">
                {{ group.items.length }} 个
                <template v-if="groupImportedCount(group)"> · 已导入 {{ groupImportedCount(group) }}</template>
              </span>
            </div>
            <div class="clone-table">
              <div class="clone-table__head clone-row">
                <span class="clone-col--check" />
                <span class="clone-col--name">商品</span>
                <span class="clone-col--spec">规格</span>
                <span class="clone-col--num clone-col--uplist">上游标价</span>
                <span class="clone-col--num clone-col--cost">代理价</span>
                <span class="clone-col--num clone-col--sale">售价</span>
                <span class="clone-col--num clone-col--margin">毛利</span>
                <span class="clone-col--num clone-col--stock">库存</span>
                <span class="clone-col--status">状态</span>
              </div>
              <label
                v-for="item in group.items"
                :key="item.id"
                class="clone-row clone-row--item"
                :class="{ 'clone-row--off': !isProductOnSale(item), 'clone-row--done': item.imported }"
              >
                <t-checkbox
                  class="clone-col--check"
                  :checked="isProductChecked(item.id)"
                  :disabled="Boolean(item.imported)"
                  @change="() => toggleProduct(item.id)"
                />
                <span class="clone-cell clone-cell--name clone-col--name" :title="item.name">{{ item.name }}</span>
                <span class="clone-cell clone-cell--spec clone-col--spec">{{ formatProductSpec(item) }}</span>
                <span class="clone-cell clone-col--num clone-col--uplist">{{ priceText(item.sale_price) }}</span>
                <span class="clone-cell clone-col--num clone-col--cost">{{ priceText(item.cost_price) }}</span>
                <span class="clone-cell clone-col--num clone-cell--sale clone-col--sale">{{
                  priceText(salePriceOf(item))
                }}</span>
                <span class="clone-cell clone-col--num clone-col--margin" :class="marginClass(item)">{{
                  marginText(item)
                }}</span>
                <span class="clone-cell clone-col--num clone-col--stock" :class="stockClass(item)">{{
                  stockText(item)
                }}</span>
                <span class="clone-cell clone-cell--status clone-col--status">
                  <t-tooltip v-if="item.imported" :content="importedTooltip(item)">
                    <t-link theme="success" hover="color" @click="openImportedProduct(item)">
                      已导入 {{ importedStatusText(item) }}
                    </t-link>
                  </t-tooltip>
                  <t-tooltip v-else :content="statusTooltipOf(item)">
                    <t-tag size="small" variant="light" :theme="statusThemeOf(item)">{{ statusTextOf(item) }}</t-tag>
                  </t-tooltip>
                </span>
              </label>
            </div>
          </div>
        </div>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import FilterCard from '@/components/filter-card/index.vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AddIcon, AppIcon, CloudIcon, RefreshIcon, SearchIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  batchCloneProductFromUpstream,
  batchDeleteProducts,
  batchPublishProducts,
  batchUnpublishProducts,
  deleteProduct,
  getProductCategoryList,
  getProductDetail,
  getProductList,
  publishProduct,
  setProductSpecPassthrough,
  unpublishProduct,
  updateProductPrice,
  type SaleProductBatchActionResponse,
} from '@/api/product'
import { getProductList as getResourceProductList, getProviderList } from '@/api/admin'
import {
  formatPrice,
  priceModelLabel,
  productStatusOptions,
  sourceModeOptions,
  sourceModeTag,
  statusTag,
} from '@/pages/product/constants'
import type { ProductInfo, ProviderInfo, SaleProductCategoryInfo, SaleProductInfo } from '@/types/interface'
import type { SaleProductUpstreamInfo } from '@/types/product'
import MobileAction from '@/components/mobile-action/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import UpstreamPriceEditor from './UpstreamPriceEditor.vue'
import { buildMobileActionOptions } from '@/composables/useMobileActions'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { useIsMobile } from '@/composables/useIsMobile'
import { useMobilePagination } from '@/composables/useMobilePagination'
import { formatDisk, formatMemory } from '@/utils/spec-format'

defineOptions({ name: 'ProductProducts' })

const router = useRouter()
const route = useRoute()

const productList = ref<SaleProductInfo[]>([])
const loading = ref(false)
const { isMobile } = useIsMobile()
const { categoryOptions, categoryName, categoryCounts, loadCategories } = useCategoryOptions()

const filters = reactive<{
  keyword: string | undefined
  category_id: number | undefined
  /** 只看未分类（与 category_id 互斥）：未分类不是一个真实分类 ID */
  uncategorized: boolean | undefined
  status: number | undefined
  source_mode: string | undefined
}>({
  keyword: undefined,
  category_id: undefined,
  uncategorized: undefined,
  status: undefined,
  source_mode: undefined,
})

const { pagination, mobilePage, applyTotal, handlePageChange, goMobilePage, handleMobilePageSizeChange, resetPage } =
  useMobilePagination(loadProducts)

const columns: PrimaryTableCol<SaleProductInfo>[] = [
  // 勾选列（批量操作入口）；row-selection 通过 :row-selection 传入
  { colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const },
  { colKey: 'name', title: '产品', minWidth: 180 },
  { colKey: 'source_mode', title: '链路', width: 110 },
  { colKey: 'category', title: '分类', width: 120 },
  { colKey: 'price', title: '价格', width: 170 },
  { colKey: 'stock', title: '库存', width: 80 },
  { colKey: 'status', title: '状态', width: 90 },
  {
    colKey: 'action',
    title: '操作',
    width: isMobile.value ? 70 : 260,
    fixed: 'right' as const,
    align: 'center' as const,
  },
]

// ===== 分类导航（左树右表）：树只负责定位，表格保持服务端分页 =====
const asideTree = ref<SaleProductCategoryInfo[]>([])
const asideExpanded = ref<Array<string | number>>([])
const treeKeyword = ref('')

/**
 * 分类导航的计数与未分类数：与 useCategoryOptions 同一次请求取回，
 * 但导航树要的是「带计数的节点」（t-tree 从 node.data 读），
 * 所以这里把计数合并进树节点，而不是让模板去查 map。
 */
const uncategorizedCount = ref(0)
/** 是否正在看「未分类」——它不是真实分类 ID，得单独一个状态位。 */
const onlyUncategorized = computed(() => filters.uncategorized === true)

/** 递归把「分类 ID → 商品数」写进树节点（字段名 count，供 t-tree 的 slot 读）。 */
function mergeCategoryCounts(nodes: SaleProductCategoryInfo[]): SaleProductCategoryInfo[] {
  return nodes.map((node) => ({
    ...node,
    count: categoryCounts.value[node.id] ?? 0,
    children: node.children?.length ? mergeCategoryCounts(node.children) : node.children,
  }))
}

// 按名称过滤导航树：仅后代命中时保留「祖先 → 命中项」链
const filteredAsideTree = computed(() => {
  const kw = treeKeyword.value.trim().toLowerCase()
  if (!kw) return asideTree.value
  const walk = (nodes: SaleProductCategoryInfo[]): SaleProductCategoryInfo[] => {
    const out: SaleProductCategoryInfo[] = []
    for (const node of nodes) {
      const children = node.children?.length ? walk(node.children) : []
      if (node.name.toLowerCase().includes(kw)) out.push(node)
      else if (children.length) out.push({ ...node, children })
    }
    return out
  }
  return walk(asideTree.value)
})

// 从根到当前选中分类的路径（面包屑用）
const activeCategoryPath = computed<SaleProductCategoryInfo[]>(() => {
  const target = filters.category_id
  if (!target) return []
  const path: SaleProductCategoryInfo[] = []
  const walk = (nodes: SaleProductCategoryInfo[]): boolean => {
    for (const node of nodes) {
      path.push(node)
      if (node.id === target) return true
      if (node.children?.length && walk(node.children)) return true
      path.pop()
    }
    return false
  }
  walk(asideTree.value)
  return path
})

const activeTreeKeys = computed(() => (filters.category_id ? [filters.category_id] : []))

async function loadCategoryTree() {
  try {
    const data = await getProductCategoryList()
    // 计数随树一起下发，但 t-tree 从 node.data 读值，这里并进节点。
    asideTree.value = mergeCategoryCounts(data.items || [])
    uncategorizedCount.value = data.uncategorized_count ?? 0
    // 从 URL 直进时展开选中节点的祖先链，否则深层节点看不见
    if (filters.category_id) {
      asideExpanded.value = activeCategoryPath.value.map((n) => n.id)
    }
  } catch {
    // 导航树加载失败不阻断列表：分类下拉（移动端）与「全部产品」仍可用
  }
}

function onTreeActive(value: Array<string | number>) {
  const id = value.length ? Number(value[value.length - 1]) : undefined
  selectCategory(id && id > 0 ? id : undefined)
}

function selectCategory(id: number | undefined) {
  filters.category_id = id
  // 选具体分类时清掉「未分类」：两者互斥，留着会让筛选条件自相矛盾
  filters.uncategorized = undefined
  resetPage()
  router.replace({
    query: {
      ...route.query,
      category_id: id ? String(id) : undefined,
      uncategorized: undefined,
    },
  })
}

function selectAllCategories() {
  selectCategory(undefined)
}

/** 只看未分类商品：这些商品没有有效分类归属，不在任何树节点下。 */
function selectUncategorized() {
  filters.category_id = undefined
  filters.uncategorized = true
  resetPage()
  router.replace({ query: { ...route.query, category_id: undefined, uncategorized: '1' } })
}

async function loadProducts() {
  loading.value = true
  try {
    const data = await getProductList({
      keyword: filters.keyword,
      category_id: filters.category_id,
      uncategorized: filters.uncategorized,
      status: filters.status,
      source_mode: filters.source_mode,
      page: pagination.current,
      page_size: pagination.pageSize,
    })
    productList.value = data.items
    applyTotal(data.meta.total)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  resetPage()
}

function handleResetFilters() {
  filters.keyword = undefined
  filters.category_id = undefined
  filters.uncategorized = undefined
  filters.status = undefined
  filters.source_mode = undefined
  resetPage()
}

function handleCreate() {
  router.push('/product/products/create')
}

// ===== 批量操作（多选上架/下架/删除） =====
// 选区由 TDesign 的 row-selection 维护：selectedRowKeys 只含当前勾选的商品 ID，
// 翻页会让未勾选的页离开视图，但已勾选的键保留在集合里（跨页累计，删除前会再确认）。
const selectedRowKeys = ref<Array<string | number>>([])
const batchLoading = ref(false)

// 多选列配置：跨页保留已勾选项（reserve-selected-row-on-paginate 在表格上开启），
// 全选只作用于当前页 —— 提示文案里明确写了"已选 N 个产品"。
const rowSelection = { type: 'multiple' as const, width: 46 }

function handleSelectChange(keys: Array<string | number>) {
  selectedRowKeys.value = keys
}

function clearSelection() {
  selectedRowKeys.value = []
}

function selectedIds(): number[] {
  return selectedRowKeys.value.map((key) => Number(key)).filter((id) => id > 0)
}

/** 批量结果提示：成功数 + 前几条失败原因（逐条回报，不能只报"部分失败"）。 */
function reportBatchResult(
  kind: string,
  result: {
    succeeded?: number[]
    failed?: Array<{ name?: string; product_id: number; reason: string }>
    message?: string
  },
) {
  const ok = result.succeeded?.length ?? 0
  const failed = result.failed ?? []
  if (ok > 0) MessagePlugin.success(`${kind}成功 ${ok} 个`)
  if (failed.length > 0) {
    const detail = failed
      .slice(0, 3)
      .map((f) => `${f.name || `#${f.product_id}`}：${f.reason}`)
      .join('；')
    MessagePlugin.warning(`${failed.length} 个未${kind}：${detail}${failed.length > 3 ? ' 等' : ''}`)
  }
  if (ok === 0 && failed.length === 0) MessagePlugin.warning(result.message || `没有可${kind}的产品`)
}

async function runBatch(kind: string, action: (ids: number[]) => Promise<SaleProductBatchActionResponse>) {
  const ids = selectedIds()
  if (!ids.length) {
    MessagePlugin.warning('请先勾选产品')
    return
  }
  batchLoading.value = true
  try {
    const result = await action(ids)
    reportBatchResult(kind, result || {})
    clearSelection()
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `批量${kind}失败`)
  } finally {
    batchLoading.value = false
  }
}

function handleBatchPublish() {
  const ids = selectedIds()
  if (!ids.length) {
    MessagePlugin.warning('请先勾选产品')
    return
  }
  // 上架走自己的一条路径（而不是通用 runBatch）：门禁拦截时要能从结果里
  // 认出"哪些是仅透传可放行的"，给一次覆盖全选的一键操作，而不是让运营
  // 逐个商品回到单条上架去点确认弹窗。
  batchLoading.value = true
  void (async () => {
    try {
      const result = await batchPublishProducts(ids)
      reportBatchResult('上架', result || {})
      const blocked = (result?.failed ?? []).filter((f) => f.reason.includes('仅透传') || f.reason.includes('平台绑定'))
      if (blocked.length > 0) {
        offerBatchPassthroughAndPublish(blocked.map((f) => f.product_id))
      }
      clearSelection()
      loadProducts()
    } catch (error) {
      MessagePlugin.error((error as Error).message || '批量上架失败')
    } finally {
      batchLoading.value = false
    }
  })()
}

/**
 * 批量上架被门禁拦下时的一键处理：对这批商品统一开启「仅透传上游参数」再上架。
 *
 * 适用于上游给的整机固定配置（参数原样转发）。财务型上游的商品在服务端已按
 * config_groups 自动放行；这里覆盖的是"上游不给配置组、但确实按原样转发"的少数情形。
 */
function offerBatchPassthroughAndPublish(ids: number[]) {
  if (!ids.length) return
  const dialog = DialogPlugin.confirm({
    header: `有 ${ids.length} 个商品需要确认参数来源`,
    body: `这批上游转售商品未完成"本地规格 → 上游开通参数"的平台绑定确认。
若它们都是整机固定配置（参数原样转发、不改写），可一次性开启「仅透传上游参数」放行；
否则请到商品详情 → 规格页签逐个完成绑定确认。`,
    confirmBtn: { content: '全部开启仅透传并上架', theme: 'primary' },
    cancelBtn: { content: '稍后处理' },
    onConfirm: async () => {
      dialog.hide()
      try {
        await Promise.all(ids.map((id) => setProductSpecPassthrough(id, true)))
        const retry = await batchPublishProducts(ids)
        reportBatchResult('上架', retry || {})
        loadProducts()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '开启仅透传后仍上架失败')
      }
    },
  })
}

function handleBatchUnpublish() {
  void runBatch('下架', (ids) => batchUnpublishProducts(ids))
}

function handleBatchDelete() {
  const ids = selectedIds()
  if (!ids.length) {
    MessagePlugin.warning('请先勾选产品')
    return
  }
  const names = productList.value
    .filter((item) => ids.includes(item.id))
    .slice(0, 3)
    .map((item) => item.name)
    .join('、')
  const dialog = DialogPlugin.confirm({
    header: `删除 ${ids.length} 个产品？`,
    body: `删除后不可恢复（含规格与价格矩阵）。${names ? `包含：${names}${ids.length > 3 ? ' 等' : ''}` : ''}`,
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      dialog.hide()
      await runBatch('删除', (list) => batchDeleteProducts(list))
    },
  })
}

// ===== 上游商品导入（克隆：按上游分组多选 + 本地分类 + 按上游标价百分比定价） =====
// 入口参数：?import=<渠道ID>&preselect=<上游资源商品ID>（上游商品页「去导入」跳转过来时用）
const cloneVisible = ref(false)
const cloneSubmitting = ref(false)
const cloneLoading = ref(false)
const providerOptions = ref<{ label: string; value: number }[]>([])
const providerProducts = ref<ProductInfo[]>([])
const selectedProductIds = ref<Set<number>>(new Set())
// 导入弹窗搜索关键词：上游商品常有几百个，靠滚动定位分类/商品不现实。
const cloneKeyword = ref('')
const cloneForm = reactive<{
  source_provider_id: number | undefined
  category_id: number | undefined
  price_percent: number
  /** 导入后是否直接上架（默认是）。财务型上游开通参数自带，无需逐个确认映射。 */
  publish_now: boolean
}>({
  source_provider_id: undefined,
  category_id: undefined,
  price_percent: 100,
  publish_now: true,
})

/** 优惠提示：上游给本账号的代理折扣说明（取自商品镜像的折扣描述）。 */
const discountHint = computed(() => {
  const hit = providerProducts.value.find((item) => item.cost_price > 0 && item.sale_price > item.cost_price)
  if (!hit) return ''
  const off = Math.round((hit.cost_price / hit.sale_price) * 100)
  return `该渠道代理价约为标价的 ${off}%（成本按代理价计）`
})

/** 在售商品数：上游隐藏分组/隐藏商品/已下架都不算在售（status 由同步按真实可见性写入）。 */
const onSaleCount = computed(() => filteredProducts.value.filter((item) => isProductOnSale(item)).length)

/** 已导入数：上游商品已导入为本地商品的数量（这些不允许重复导入）。 */
const importedCount = computed(() => filteredProducts.value.filter((item) => item.imported).length)

/** 可导入数（未导入的）——汇总行给运营一个"还能导几个"的准数。 */
const selectableCount = computed(() => filteredProducts.value.filter((item) => !item.imported).length)

/**
 * 搜索过滤后的上游商品：按商品名/规格/上游 ID/分类名匹配（大小写不敏感）。
 *
 * 分类名也参与匹配是为了"先搜分类名、再整组勾选"这个高频动作：
 * 运营常按「香港|六区」这类分类名找货，而不是逐个记商品名。
 */
const filteredProducts = computed(() => {
  const kw = cloneKeyword.value.trim().toLowerCase()
  if (!kw) return providerProducts.value
  return providerProducts.value.filter((item) => {
    const haystack = [item.name, item.group_name || '', item.upstream_id || '', formatProductSpec(item)]
      .join(' ')
      .toLowerCase()
    return haystack.includes(kw)
  })
})

/** 搜索时分组列表按过滤结果重建；清空搜索后回到全量分组。 */
const visibleGroups = computed(() => buildCloneGroups(filteredProducts.value))

/** 已选商品的预计毛利（售价 − 代理价，按标价可见的商品相加）。 */
const selectedProfit = computed(() => {
  let total = 0
  // 用全量而非过滤结果求和：搜索只是视图过滤，已勾选项不应因为关键词变化而从毛利里"消失"。
  for (const item of providerProducts.value) {
    if (!selectedProductIds.value.has(item.id)) continue
    const sale = salePriceOf(item)
    if (sale > 0 && item.cost_price > 0) total += sale - item.cost_price
  }
  return Math.round(total * 100) / 100
})

/** 本次定价下的售价：上游标价 × 百分比（未定价商品返回 0）。 */
function salePriceOf(item: ProductInfo): number {
  if (!(item.sale_price > 0)) return 0
  const pct = cloneForm.price_percent > 0 ? cloneForm.price_percent : 100
  return Math.round(item.sale_price * pct) / 100
}

/** 价目单元格：未定价商品显示「未定价」而不是 ¥0.00（活动款价格常只挂在配置项里）。 */
function priceText(v: number): string {
  return v > 0 ? `¥${formatPrice(v)}` : '未定价'
}

/** 毛利单元格：售价与代理价的差额与毛利率；缺价时给「—」。 */
function marginText(item: ProductInfo): string {
  const sale = salePriceOf(item)
  if (sale <= 0 || !(item.cost_price > 0)) return '—'
  const profit = sale - item.cost_price
  const pct = Math.round((profit / sale) * 100)
  return `${formatPrice(profit)} / ${pct}%`
}

function marginClass(item: ProductInfo): string {
  const sale = salePriceOf(item)
  if (sale <= 0 || !(item.cost_price > 0)) return 'clone-cell--muted'
  return sale >= item.cost_price ? 'clone-cell--up' : 'clone-cell--down'
}

/** 是否在售：以同步写入的真实可见性为准（旧数据无 visibility 时退回 status）。 */
function isProductOnSale(item: ProductInfo): boolean {
  if (item.visibility) return item.visibility === 'visible'
  return item.status === 1
}

/** 上游状态：只显示两态（正常 / 隐藏），具体原因放悬停说明。 */
function statusTextOf(item: ProductInfo): string {
  return isProductOnSale(item) ? '正常' : '隐藏'
}

function statusTooltipOf(item: ProductInfo): string {
  switch (item.visibility) {
    case 'group_hidden':
      return '隐藏：上游把整个分类从商铺隐藏（仍可导入开通）'
    case 'product_hidden':
      return '隐藏：上游单独隐藏了这个商品'
    case 'retired':
      return '隐藏：上游已下架该商品（不建议导入）'
    default:
      return '正常：上游商铺在展示'
  }
}

/** 库存文案：不限量显示「不限」；上游限量显示余量，0 显示「售罄」。 */
function stockText(item: ProductInfo): string {
  if (item.stock === undefined || item.stock === null || item.stock < 0) return '不限'
  return item.stock === 0 ? '售罄' : String(item.stock)
}

/** 库存着色：售罄危险色、紧张（≤5）警示色，其余默认。 */
function stockClass(item: ProductInfo): string {
  if (item.stock === undefined || item.stock === null || item.stock < 0) return 'clone-cell--muted'
  if (item.stock === 0) return 'stock-text--out'
  if (item.stock <= 5) return 'stock-text--low'
  return ''
}

function statusThemeOf(item: ProductInfo): 'success' | 'danger' | 'warning' | 'default' {
  if (isProductOnSale(item)) return 'success'
  return item.visibility === 'retired' ? 'danger' : 'warning'
}

async function handleClone() {
  cloneForm.source_provider_id = undefined
  cloneForm.category_id = undefined
  cloneForm.price_percent = 100
  cloneForm.publish_now = true
  selectedProductIds.value = new Set()
  cloneKeyword.value = ''
  cloneVisible.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    // 只列出可做商品对接的渠道：自营算力平台（魔方云等）没有商品目录，
    // 之前混在列表里选中后什么也拉不到。
    providerOptions.value = data.items
      .filter((item: ProviderInfo) => item.catalog_supported !== false)
      .map((item: ProviderInfo) => ({ label: item.name, value: item.id }))
  } catch {
    MessagePlugin.error('加载上游提供商失败')
  }
}

async function handleProviderChange() {
  providerProducts.value = []
  selectedProductIds.value = new Set()
  cloneKeyword.value = ''
  if (!cloneForm.source_provider_id) return
  cloneLoading.value = true
  try {
    // 一次拉全量（列表接口上限 500，且不再携带 raw_specs 大字段）：
    // 此前固定 page_size=100 导致上游商品多于 100 时后一半根本选不到。
    const data = await getResourceProductList({ provider_id: cloneForm.source_provider_id, page_size: 500 })
    providerProducts.value = data.items
  } catch {
    MessagePlugin.error('加载上游商品失败')
  } finally {
    cloneLoading.value = false
  }
}

function buildCloneGroups(
  items: ProductInfo[],
): { id: number; name: string; derived: boolean; items: ProductInfo[] }[] {
  const map = new Map<number, { id: number; name: string; derived: boolean; items: ProductInfo[] }>()
  for (const item of items) {
    const gid = item.group_id || 0
    if (!map.has(gid)) {
      map.set(gid, {
        id: gid,
        // 隐藏分组的上游不公示分类名，同步按商品名前缀派生了近似标签（带「派生」标记）；
        // 两者都没有时给「分组 #id」，而不是含糊的「未分类」。
        name: item.group_name || `分组 #${gid}`,
        derived: Boolean(item.group_name_derived),
        items: [],
      })
    }
    map.get(gid)!.items.push(item)
  }
  // 在售组优先、组名稳定排序，避免每次加载顺序乱跳。
  return Array.from(map.values()).sort((a, b) => {
    const aOn = a.items.some((item) => isProductOnSale(item)) ? 0 : 1
    const bOn = b.items.some((item) => isProductOnSale(item)) ? 0 : 1
    if (aOn !== bOn) return aOn - bOn
    return a.name.localeCompare(b.name, 'zh-Hans-CN')
  })
}

function isProductChecked(id: number): boolean {
  return selectedProductIds.value.has(id)
}

function toggleProduct(id: number) {
  const item = providerProducts.value.find((p) => p.id === id)
  // 已导入的上游商品不允许再选：重复导入只会拿到"商品编码重复"的失败。
  if (item?.imported) return
  const next = new Set(selectedProductIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedProductIds.value = next
}

/** 组内未导入的商品（勾选操作只在这些上生效）。 */
function selectableItems(group: { items: ProductInfo[] }): ProductInfo[] {
  return group.items.filter((item) => !item.imported)
}

function groupImportedCount(group: { items: ProductInfo[] }): number {
  return group.items.length - selectableItems(group).length
}

function hasSelectable(group: { items: ProductInfo[] }): boolean {
  return selectableItems(group).length > 0
}

function isGroupChecked(group: { items: ProductInfo[] }): boolean {
  const selectable = selectableItems(group)
  return selectable.length > 0 && selectable.every((item) => selectedProductIds.value.has(item.id))
}

function toggleGroup(group: { items: ProductInfo[] }) {
  const selectable = selectableItems(group)
  if (selectable.length === 0) return
  const next = new Set(selectedProductIds.value)
  const allChecked = isGroupChecked(group)
  if (allChecked) {
    selectable.forEach((item) => next.delete(item.id))
  } else {
    selectable.forEach((item) => next.add(item.id))
  }
  selectedProductIds.value = next
}

/** 已导入商品在本地商品表的去向与状态（弹窗里直接说明"导到哪儿了"）。 */
function importedStatusText(item: ProductInfo): string {
  const suffix = item.imported_product_id ? ` #${item.imported_product_id}` : ''
  switch (item.imported_status) {
    case 1:
      return `已上架${suffix}`
    case 2:
      return `已下架${suffix}`
    default:
      return `草稿${suffix}`
  }
}

/**
 * 「草稿」是什么、在哪儿改：导入默认落草稿（前台不展示、不售卖）。
 * 悬停直接说清入口，避免"显示草稿但不知道去哪看"。
 */
function importedTooltip(item: ProductInfo): string {
  const target = item.imported_product_id ? `本地商品 #${item.imported_product_id}` : '本地商品'
  const how = '点击打开该商品详情：可改分类/描述/价格，改完在详情页或列表页「上架」'
  switch (item.imported_status) {
    case 1:
      return `已导入为${target}（已上架）。${how}`
    case 2:
      return `已导入为${target}（已下架，前台不可购买）。${how}`
    default:
      return `已导入为${target}（草稿：未上架，前台不展示也不售卖）。${how}`
  }
}

/** 跳到已导入的本地商品详情（草稿的编辑/上架入口就在那里）。 */
function openImportedProduct(item: ProductInfo) {
  if (!item.imported_product_id) {
    MessagePlugin.warning('未取到本地商品 ID，请到「产品管理 → 产品列表」按名称查找')
    return
  }
  router.push(`/product/products/${item.imported_product_id}`)
}

function formatProductSpec(item: ProductInfo): string {
  // 规格单位统一走 utils/spec-format（此前直接拼 `${memory}M` 会显示「8192M」）。
  const parts: string[] = []
  if (item.cpu) parts.push(`${item.cpu}核`)
  const mem = formatMemory(item.memory || 0)
  if (mem !== '—') parts.push(mem)
  const disk = formatDisk(item.disk || 0)
  if (disk !== '—') parts.push(disk)
  if (item.os) parts.push(item.os)
  return parts.length ? parts.join(' / ') : '规格未同步'
}

async function handleCloneConfirm() {
  if (!cloneForm.source_provider_id || selectedProductIds.value.size === 0) {
    MessagePlugin.warning('请选择上游提供商并勾选至少一个上游商品')
    return
  }
  // 分类必填（用户要求）：服务端同样会拦（见 CloneFromUpstreamBatch），
  // 这里先挡一道是为了把提示落在用户正看着的那个字段上，而不是发完请求才报错。
  if (!cloneForm.category_id) {
    MessagePlugin.warning('请选择「归入本地分类」——留空会让整批商品落进「未分类」')
    return
  }
  // 上游未定价的商品（价格只挂在配置项、上游还没配活动价）仍可导入，
  // 但要在导入前说清楚：导入后本地售价为 0，需人工补价。
  const unpriced = providerProducts.value.filter(
    (item) => selectedProductIds.value.has(item.id) && !(item.sale_price > 0) && !(item.cost_price > 0),
  )
  // 提交前剔除已导入的（弹窗可能开着很久，期间这些可能已被别处导入）：
  // 带上只会得到"商品编码重复"的失败，直接在客户端挡掉。
  const selectedItems = providerProducts.value.filter((item) => selectedProductIds.value.has(item.id))
  const importable = selectedItems.filter((item) => !item.imported)
  if (importable.length === 0) {
    MessagePlugin.warning('选中的商品都已导入过，无需重复导入')
    return
  }
  cloneSubmitting.value = true
  try {
    const result = await batchCloneProductFromUpstream({
      source_provider_id: cloneForm.source_provider_id,
      source_product_ids: importable.map((item) => item.id),
      category_id: cloneForm.category_id,
      price_percent: cloneForm.price_percent,
      // 1=导入即上架（默认），0=草稿（先编辑再上架）
      status: cloneForm.publish_now ? 1 : 0,
    })
    const imported = result?.items?.length ?? 0
    const failed = result?.failed ?? []
    if (imported > 0) {
      MessagePlugin.success(`已导入 ${imported} 个上游商品`)
    }
    if (unpriced.length > 0) {
      MessagePlugin.warning(
        `${unpriced.length} 个商品上游未定价（如 ${unpriced[0].name}），导入后售价为 0，请到商品详情页补价`,
      )
    }
    if (failed.length > 0) {
      // 逐条失败此前被静默吞掉（接口一律返回成功），这里把原因摊开：
      // 最常见的是「该上游商品已导入过」（商品编码 cpy-<渠道>-<上游商品> 已存在）。
      const detail = failed
        .slice(0, 3)
        .map((f) => `${f.name || `#${f.source_product_id}`}：${f.reason}`)
        .join('；')
      MessagePlugin.warning(
        imported > 0
          ? `${failed.length} 个未导入：${detail}${failed.length > 3 ? ' 等' : ''}`
          : `导入失败：${detail}${failed.length > 3 ? ' 等' : ''}`,
      )
    }
    if (imported === 0 && failed.length === 0) {
      MessagePlugin.warning('没有可导入的商品，请检查选择')
    }
    if (imported > 0) {
      cloneVisible.value = false
      loadProducts()
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '导入失败')
  } finally {
    cloneSubmitting.value = false
  }
}

function openDetail(row: SaleProductInfo) {
  router.push(`/product/products/${row.id}`)
}

/**
 * 编辑入口已并入详情页（详情/编辑合并为单页 + 子页导航）：
 * 这里保留一个跳详情页的转发，供历史调用点（如旧代码里的 openEdit）复用，
 * 避免再拼出 `/edit` 这个已废弃的地址。
 */
function openEdit(row: SaleProductInfo) {
  openDetail(row)
}

async function handlePublish(row: SaleProductInfo) {
  try {
    await publishProduct(row.id)
    MessagePlugin.success('已上架')
    loadProducts()
  } catch (error) {
    const message = (error as Error).message || '上架失败'
    // 上架门禁（上游转售商品）：把"为什么被拦"和"怎么放行"讲清楚，并给一键操作，
    // 避免运营看到一句报错就卡住（此前只能去猜「仅透传」在哪里开）。
    if (message.includes('仅透传') || message.includes('平台绑定')) {
      offerPassthroughAndPublish(row)
      return
    }
    MessagePlugin.error(message)
  }
}

/**
 * 上架被门禁拦下时的引导：说明原理（本地规格→上游开通参数没确认过），
 * 若该上游商品是整机固定配置，可直接开启「仅透传」再上架。
 */
function offerPassthroughAndPublish(row: SaleProductInfo) {
  const dialog = DialogPlugin.confirm({
    header: '上架前需要确认参数来源',
    body: `「${row.name}」是上游转售商品：上架后客户下单会把参数发给上游开通，所以系统先要求确认"本地规格 → 上游开通参数"的对应关系。
若该上游商品是整机固定配置（参数原样转发、不改写），可开启「仅透传上游参数」直接放行；否则请到商品详情 → 规格页签完成平台绑定确认。`,
    confirmBtn: { content: '开启仅透传并上架', theme: 'primary' },
    cancelBtn: { content: '稍后处理' },
    onConfirm: async () => {
      dialog.hide()
      try {
        await setProductSpecPassthrough(row.id, true)
        await publishProduct(row.id)
        MessagePlugin.success('已开启仅透传并上架')
        loadProducts()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '开启仅透传后仍上架失败')
      }
    },
  })
}

async function handleUnpublish(row: SaleProductInfo) {
  try {
    await unpublishProduct(row.id)
    MessagePlugin.success('已下架')
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '下架失败')
  }
}

async function handleDelete(row: SaleProductInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除产品',
    body: `确认删除「${row.name}」？删除后不可恢复。`,
    theme: 'danger',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteProduct(row.id)
        MessagePlugin.success('已删除')
        dialog.hide()
        loadProducts()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

const priceVisible = ref(false)
const priceForm = reactive<{ price: number; cost_price: number; remark: string }>({
  price: 0,
  cost_price: 0,
  remark: '',
})
// 当前改价的商品行：对接商品把它交给共用价格编辑器（需要 id/上游信息），
// 自营商品仍走下面的两字段表单。
const priceProduct = ref<SaleProductInfo | null>(null)
// 上游对接信息（各周期成本/标价/折扣说明）：对接商品的编辑器要用它算各周期预览，
// 列表接口不带该字段，因此按需单独拉详情。
const priceUpstream = ref<SaleProductUpstreamInfo | null>(null)
const priceDetailLoading = ref(false)

/** 是否对接（上游转售）商品：改价入口据此分流两套编辑器。 */
function isUpstreamProduct(row: SaleProductInfo): boolean {
  return row.source_mode === 'upstream'
}

async function openPriceDialog(row: SaleProductInfo) {
  priceProduct.value = row
  priceForm.price = row.price
  priceForm.cost_price = row.cost_price
  priceForm.remark = ''
  priceUpstream.value = null
  priceVisible.value = true
  if (!isUpstreamProduct(row)) return
  // 对接商品：补拉详情拿上游各周期价，编辑器据此渲染"各周期售价预览"。
  priceDetailLoading.value = true
  try {
    const detail = await getProductDetail(row.id)
    priceUpstream.value = detail.upstream || null
  } catch (error) {
    MessagePlugin.warning((error as Error).message || '读取上游价格失败，各周期预览将为空')
  } finally {
    priceDetailLoading.value = false
  }
}

/** 共用编辑器保存完成：这里只负责收起弹窗并刷新列表。 */
function handlePriceSaved() {
  priceVisible.value = false
  loadProducts()
}

async function handleSavePrice() {
  if (!priceProduct.value) return
  try {
    await updateProductPrice(priceProduct.value.id, {
      price: priceForm.price,
      cost_price: priceForm.cost_price,
      remark: priceForm.remark || undefined,
    })
    MessagePlugin.success('价格已更新')
    priceVisible.value = false
    loadProducts()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新价格失败')
  }
}

onMounted(() => {
  const q = Number(route.query.category_id)
  if (q > 0) filters.category_id = q
  // 「未分类」是布尔状态（不是分类 ID），从 URL 直进时单独还原。
  if (route.query.uncategorized === '1') filters.uncategorized = true
  loadCategories()
  loadCategoryTree()
  loadProducts()
  void openCloneFromQuery()
})

/**
 * 从上游商品页「去导入」跳转时自动开弹窗并预选：
 *   ?import=<渠道ID>（&preselect=<上游商品ID>）
 * 处理完把参数从地址栏清掉，避免刷新或返回时反复弹窗。
 */
async function openCloneFromQuery() {
  const providerId = Number(route.query.import)
  if (!providerId) return
  const preselectId = Number(route.query.preselect)
  const query = { ...route.query }
  delete query.import
  delete query.preselect
  router.replace({ query })
  await handleClone()
  cloneForm.source_provider_id = providerId
  await handleProviderChange()
  if (preselectId > 0) {
    const target = providerProducts.value.find((item) => item.id === preselectId)
    if (target?.imported) {
      MessagePlugin.warning(`${target.name} 已导入过，无需重复导入`)
    } else if (target) {
      selectedProductIds.value = new Set([preselectId])
    }
  }
}

// 浏览器前进/后退时同步分类过滤（selectCategory 的 replace 会触发本 watch，但值相同直接跳过）
watch(
  () => route.query.category_id,
  (value) => {
    const id = Number(value)
    const next = value && id > 0 ? id : undefined
    if (next !== filters.category_id) {
      filters.category_id = next
      resetPage()
    }
  },
)

// 「未分类」同样要跟地址栏双向同步：它是页面状态，后退/刷新都该回到同一视图。
watch(
  () => route.query.uncategorized,
  (value) => {
    const next = value === '1'
    if (next !== !!filters.uncategorized) {
      filters.uncategorized = next
      resetPage()
    }
  },
)

// 移动端操作下拉分发
function handleMobileAction(value: string | number | Record<string, unknown>, row: SaleProductInfo) {
  const action =
    typeof value === 'string' || typeof value === 'number'
      ? String(value)
      : String((value as { value?: string })?.value ?? '')
  switch (action) {
    case 'detail':
      openDetail(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'price':
      openPriceDialog(row)
      break
    case 'publish':
      void handlePublish(row)
      break
    case 'unpublish':
      void handleUnpublish(row)
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

<style scoped>
/*
  导入设置区：自适应列宽（窄屏自动塌成一列），每列内是「标签 / 控件 / 说明」纵向排列。
  为什么不用 t-form-item：它的 controls-content 是不换行的 flex 行，说明文案与控件并排时
  会把说明挤成竖排单字（中文 min-content 宽度 = 1 个字，实测截图故障）。
*/
.clone-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px 16px;
  align-items: start;
  margin-bottom: 12px;
}
.clone-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.clone-field__label {
  font-size: 14px;
  font-weight: 400;
  line-height: 22px;
  color: var(--td-text-color-primary, #000);
}
/* 必填项：与 TDesign 的必填星号语义一致，视觉上提示"这一项不能空" */
.clone-field--required .clone-field__label::after {
  content: '*';
  margin-left: 4px;
  color: var(--td-error-color, #d54941);
}
.clone-field :deep(.t-input__wrap),
.clone-field :deep(.t-select__wrap) {
  width: 100%;
}
/* 百分比输入框默认宽度很窄，拉满一列才不会与下拉框参差 */
.clone-percent {
  width: 100%;
}
/* 单选组窄屏换行，避免两个按钮被压到溢出 */
.clone-publish {
  flex-wrap: wrap;
  gap: 8px;
}
.clone-tip {
  display: block;
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-secondary, #999);
  /* 说明可能较长：允许正常换行，不再固定高度造成截断或撑高 */
  overflow-wrap: break-word;
}
.clone-hint {
  margin: 0 0 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--td-text-color-secondary, #999);
}
/* 自营商品改价：两字段表单 + 底部按钮（对接商品走 UpstreamPriceEditor 自带 footer） */
.self-price-form__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--td-text-color-secondary, #999);
}
.self-price-form__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}

/* 导入弹窗搜索：上游商品常有几百个，靠滚动很难定位到目标分类/商品 */
.clone-search {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}
.clone-search :deep(.t-input__wrap) {
  width: 320px;
  flex: none;
}
.clone-search__hint {
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
/* 商品面板固定高度：加载/切换服务商时弹窗不再忽高忽低 */
.clone-groups {
  height: 420px;
  overflow: auto;
  border: 1px solid var(--td-component-border, #ddd);
  border-radius: 6px;
  padding: 8px;
}
.clone-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--td-text-color-secondary, #999);
  font-size: 13px;
}
.clone-group {
  margin-bottom: 12px;
}
.clone-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  background: var(--td-bg-color-secondarycontainer, #f5f5f5);
  border-radius: 4px;
  font-weight: 600;
}
.clone-group__count {
  color: var(--td-text-color-secondary, #999);
  font-weight: 400;
  font-size: 12px;
}
/* 商品表：固定列宽对齐，价格各占一列，避免长名挤在一起看不清 */
.clone-table {
  padding: 4px 8px 0 8px;
}
.clone-row {
  display: grid;
  grid-template-columns: 28px minmax(180px, 1.6fr) 140px 76px 76px 76px 88px 64px 76px;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font-size: 12px;
}
.clone-cell--muted {
  color: var(--td-text-color-placeholder, #bbb);
}
.stock-text--low {
  color: var(--td-warning-color, #e37318);
  font-weight: 600;
}
.stock-text--out {
  color: var(--td-error-color, #d54941);
  font-weight: 600;
}
.clone-table__head.clone-row {
  color: var(--td-text-color-placeholder, #bbb);
  border-bottom: 1px solid var(--td-component-border, #eee);
  padding-bottom: 6px;
}
.clone-row--item {
  border-bottom: 1px dashed var(--td-component-border, #f0f0f0);
  cursor: pointer;
}
.clone-row--item:hover {
  background: var(--td-bg-color-container-hover, #fafafa);
}
.clone-row--off .clone-cell--name,
.clone-row--off .clone-cell--spec {
  color: var(--td-text-color-placeholder, #bbb);
}
/* 已导入：整行压暗 + 勾选框禁用，一眼区分"还能导的"和"已经导过的" */
.clone-row--done {
  opacity: 0.72;
  cursor: not-allowed;
}
.clone-row--done:hover {
  background: transparent;
}
.clone-cell--status {
  overflow: visible;
}
.clone-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.clone-cell--spec {
  color: var(--td-text-color-secondary, #999);
}
.clone-cell--sale {
  color: var(--td-brand-color, #0052d9);
  font-weight: 600;
}
.clone-col--num {
  text-align: right;
}
.clone-cell--up {
  color: var(--td-success-color, #2ba471);
}
.clone-cell--down {
  color: var(--td-error-color, #d54941);
}
.clone-cell--muted {
  color: var(--td-text-color-placeholder, #bbb);
}
.clone-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 10px;
  margin-bottom: 8px;
  min-height: 24px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}
.clone-summary__split {
  color: var(--td-text-color-primary, #333);
}
.clone-summary__hint {
  padding: 1px 8px;
  border-radius: 10px;
  background: var(--td-success-color-1, #e8f8f2);
  color: var(--td-success-color, #2ba471);
}
.clone-summary__pick {
  margin-left: auto;
  color: var(--td-brand-color, #0052d9);
}
/* 批量操作条：常态是灰色提示，勾选后高亮，避免误以为"没选也能点" */
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  margin-bottom: 8px;
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer, #f5f5f5);
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}

.batch-bar--active {
  background: rgba(0, 82, 217, 0.06);
  color: var(--td-text-color-primary, #333);
}

.batch-bar__count {
  font-weight: 500;
}

.products-layout {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.products-aside {
  flex: 0 0 248px;
  position: sticky;
  top: 76px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 14px;
  max-height: calc(100vh - 100px);
}

.products-aside__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.products-aside__all {
  font-size: 13px;
}

.products-aside__tree {
  overflow: auto;
  min-height: 120px;
}

.aside-node {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.aside-node--disabled {
  color: var(--color-muted-foreground);
}

/*
  节点行：名称在左、计数在右。
  t-tree 的 .t-tree__label 不是 flex（只有 flex:1 + overflow:hidden），
  而槽内容外面还被 TDesign 又套了一层无类名的 span —— 两处都要设成 flex 行，
  否则两个 span 会紧挨着排、计数跟着长名字一起被 ellipsis 截掉。
*/
.products-aside__tree :deep(.t-tree__label) {
  display: flex;
  align-items: center;
}

/* 槽内容的外层包裹（TDesign 生成，无类名，只能按结构选） */
.products-aside__tree :deep(.t-tree__label > span) {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

/* 名称独占剩余宽度，把计数推到最右 */
.products-aside__tree :deep(.t-tree__label) .aside-node {
  flex: 1;
  min-width: 0;
}

/*
  分类商品数：小号胶囊、等宽数字（多行数字纵向对齐，扫读时不会跳动）。
  与实例列表的 .kind-nav__count 同款观感 —— 同一后台里"数量"应该是同一种视觉语言。
  计数为 0 时也渲染：空分类本身就是运营要看到的信息。
*/
.aside-node__count {
  flex: none;
  min-width: 20px;
  padding: 0 6px;
  border-radius: 999px;
  background: var(--td-bg-color-container-hover, #f3f4f6);
  color: var(--td-text-color-secondary, #6b7280);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-align: center;
  line-height: 18px;
}

/*
  「未分类」行：不是分类树里的节点，所以手动补一行，缩进对齐树的根节点
  （t-tree 根节点 label 左侧是展开箭头 + margin，这里用 padding 对齐）。
  没有它，未分类商品在导航里完全不可见 —— 点遍所有分类都找不到。
*/
.aside-unclassified {
  display: flex;
  align-items: center;
  gap: 8px;
  /* 左内边距对齐树上根节点的文字起始位置（箭头宽度 + label 的 margin-left） */
  padding: 6px 12px 6px 30px;
  border-radius: var(--td-radius-default, 3px);
  cursor: pointer;
  transition: background var(--hs-duration-fast, 0.2s) var(--hs-ease-out, ease);
}

.aside-unclassified:hover {
  background: var(--td-bg-color-container-hover, #f3f4f6);
}

/* 选中态：与树上节点的 t-is-active 观感一致（浅底 + 主色字 + 加重），
   只靠底色区分对色觉障碍用户不友好，字重一起变。 */
.aside-unclassified.is-active {
  background: var(--td-brand-color-light, #f2f3ff);
  color: var(--td-brand-color, #0052d9);
  font-weight: 500;
}

.aside-unclassified__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.products-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-lg, 16px);
}

.products-crumb-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.products-crumb {
  font-weight: 600;
  color: var(--color-foreground);
}

@media (max-width: 900px) {
  .products-layout {
    display: block;
  }

  .products-aside {
    display: none;
  }
}

/*
  窄屏（弹窗 94vw）：设置区塌成一列；商品表只保留「商品 / 售价 / 状态」，
  其余列（规格/标价/代理价/毛利）在手机上挤不进一屏，隐藏比横滚更易读。
*/
@media (max-width: 768px) {
  .clone-grid {
    grid-template-columns: 1fr;
  }
  .clone-search {
    flex-direction: column;
    align-items: stretch;
  }
  .clone-search :deep(.t-input__wrap) {
    width: 100%;
  }
  /*
    窄屏两行布局：第一行 勾选 + 商品名（占满剩余宽度），第二行 售价 / 库存 / 状态。
    单行塞 5 列在手机上会把商品名压到看不清；分两行后关键信息都保留且可读。
  */
  .clone-row {
    grid-template-columns: 24px minmax(0, 1fr) auto auto auto;
    grid-template-areas:
      'check name name name name'
      'check sale stock status status';
    row-gap: 4px;
    column-gap: 8px;
  }
  .clone-col--check {
    grid-area: check;
    align-self: center;
  }
  .clone-col--name {
    grid-area: name;
  }
  .clone-col--sale {
    grid-area: sale;
  }
  .clone-col--stock {
    grid-area: stock;
  }
  .clone-col--status {
    grid-area: status;
    justify-self: start;
  }
  .clone-col--spec,
  .clone-col--uplist,
  .clone-col--cost,
  .clone-col--margin {
    display: none;
  }
  .clone-groups {
    height: 320px;
  }
}
</style>
