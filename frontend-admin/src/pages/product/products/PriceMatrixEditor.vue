<template>
  <!--
    周期价格矩阵编辑器（doc25）：商品 × 规格 × 周期 的价格维护。

    抽成组件的原因：这张表有两个入口 ——
      1) 「商品调价 → 周期价格」独立页（带商品/规格筛选器）；
      2) 商品详情页的「价格」页签（商品已定，只需编辑）。
    两处都用这一份实现，避免"同一张矩阵两套保存口径"（历史上就出现过口径漂移）。

    保存按钮由调用方渲染：矩阵页放在页头，详情页与其它页签合并成页级「保存」。
    本组件只暴露 save()/reload()，不自己决定按钮位置。
  -->
  <div class="price-matrix-editor">
    <div class="table-card__head">
      <h3 class="card-title">
        {{ matrix?.product_name || '周期价格矩阵' }}
        <t-tag v-if="isUpstream" theme="warning" variant="light" size="small" shape="round" class="head-tag">
          上游转售
        </t-tag>
      </h3>
      <span class="table-card__meta">{{ cycleHint }}</span>
    </div>

    <t-alert
      v-if="isUpstream"
      theme="info"
      class="matrix-alert"
      message="上游商品的价格由上游成本 + 加价规则推导；此处可启停档位，显式改价会转为人工价（不再随上游同步）。要整批按规则改价，用「价格」页签的百分比方案。"
    />

    <t-table
      row-key="cycle"
      :data="items"
      :columns="columns"
      :loading="loading"
      size="small"
      hover
      table-layout="fixed"
      cell-empty-content="—"
    >
      <template #cycle="{ row }">
        <div class="matrix-cell">
          <span class="cell-strong">{{ row.cycle_name }}</span>
          <span class="cell-muted">{{ row.cycle }}</span>
        </div>
      </template>

      <template #cost_setup_fee="{ row }">
        <t-input-number
          v-model="row.cost_setup_fee"
          :min="0"
          :precision="2"
          :disabled="!row.editable"
          theme="normal"
          size="small"
        />
      </template>

      <template #setup_fee="{ row }">
        <t-input-number
          v-model="row.setup_fee"
          :min="0"
          :precision="2"
          :disabled="!row.editable"
          theme="normal"
          size="small"
        />
      </template>

      <template #cost_price="{ row }">
        <t-input-number
          v-model="row.cost_price"
          :min="0"
          :precision="2"
          :disabled="!row.editable"
          theme="normal"
          size="small"
        />
      </template>

      <template #price="{ row }">
        <t-input-number
          v-model="row.price"
          :min="0"
          :precision="2"
          :disabled="!row.editable"
          theme="normal"
          size="small"
        />
      </template>

      <template #source="{ row }">
        <t-tag :theme="sourceTheme(row.source)" variant="light" size="small" shape="round">
          {{ sourceLabel(row.source) }}
        </t-tag>
      </template>

      <template #enabled="{ row }">
        <t-switch v-model="row.status" :custom-value="[1, 0]" @change="() => onToggle(row)" />
      </template>

      <template #empty>
        <t-empty description="该商品尚未建立周期价格，请先同步上游商品目录或在建品时勾选可售周期" />
      </template>
    </t-table>

    <p class="matrix-footnote">停用的档位不会出现在用户端/开放平台的可选周期里，下单时会直接拒绝该周期。</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProductPriceMatrix, saveProductPriceMatrix } from '@/api/product'
import type { ProductPriceItem, ProductPriceMatrix } from '@/types/interface'

defineOptions({ name: 'PriceMatrixEditor' })

const props = defineProps<{
  /** 要维护矩阵的商品；0/未选时不加载 */
  productId: number
  /** 指定 SKU（0 = 商品级，不区分规格） */
  specId?: number
  /** 首次挂载即加载（独立矩阵页由筛选器驱动，用默认 true 即可） */
  autoLoad?: boolean
}>()

// 把加载/保存状态透出去：独立矩阵页的「刷新 / 保存矩阵」按钮在页头，不在本组件内，
// 需要跟着转 loading（否则点保存后按钮无反馈，运营会重复点）。
const emit = defineEmits<{
  (e: 'update:loading', value: boolean): void
  (e: 'update:saving', value: boolean): void
}>()

const loading = ref(false)
const saving = ref(false)
const currency = ref('CNY')
const matrix = ref<ProductPriceMatrix | null>(null)
const items = ref<ProductPriceItem[]>([])

const isUpstream = computed(() => matrix.value?.source_mode === 'upstream')

const cycleHint = computed(() => {
  if (!matrix.value) return ''
  if (isUpstream.value) {
    return `上游可提供 ${matrix.value.upstream_cycles.length} 档周期`
  }
  return `平台全部 ${matrix.value.upstream_cycles.length} 档周期可维护`
})

const columns: PrimaryTableCol<ProductPriceItem>[] = [
  { colKey: 'cycle', title: '周期', width: 150 },
  { colKey: 'cost_setup_fee', title: '初装费成本', width: 140 },
  { colKey: 'setup_fee', title: '初装费售价', width: 140 },
  { colKey: 'cost_price', title: '价格成本', width: 140 },
  { colKey: 'price', title: '售价（折后）', width: 150 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'enabled', title: '启用', width: 90, align: 'center' as const },
]

function sourceLabel(source: string): string {
  return { upstream: '上游同步', markup: '加价推导', manual: '人工填写' }[source] || '人工填写'
}
function sourceTheme(source: string): 'success' | 'warning' | 'primary' | 'default' {
  switch (source) {
    case 'upstream':
      return 'primary'
    case 'markup':
      return 'warning'
    case 'manual':
      return 'success'
    default:
      return 'default'
  }
}

async function reload() {
  if (!props.productId) {
    matrix.value = null
    items.value = []
    baseline.value = ''
    return
  }
  loading.value = true
  emit('update:loading', true)
  try {
    const data = await getProductPriceMatrix({
      product_id: props.productId,
      spec_id: props.specId,
      currency: currency.value,
    })
    matrix.value = data
    items.value = data.items || []
    baseline.value = signature()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载周期价格矩阵失败')
  } finally {
    loading.value = false
    emit('update:loading', false)
  }
}

/**
 * 表格当前值的签名，用于判断"有没有真的改过"。
 *
 * 为什么要这个：商品详情页把各区块合并成一个页级「保存」，没碰过的表格也提交一次
 * 等于让别的页签的保存顺带整表覆盖一遍价格 —— 上游派生行虽然由后端保护（值没变就
 * 保持派生），但每次保存都刷新 updated_at 且会掩盖"到底改了什么"。
 * 签名比对天然幂等：改了又改回去 = 没改。
 */
function signature(): string {
  return JSON.stringify(
    items.value.map((item) => ({
      c: item.cycle,
      p: Number(item.price) || 0,
      cp: Number(item.cost_price) || 0,
      sf: Number(item.setup_fee) || 0,
      csf: Number(item.cost_setup_fee) || 0,
      st: item.status ? 1 : 0,
    })),
  )
}
const baseline = ref('')
const isDirty = computed(() => signature() !== baseline.value)

// t-switch 绑定 1/0 数值时组件内部会把布尔语义转换一次，这里统一回写为 1/0 保证提交口径稳定。
function onToggle(row: ProductPriceItem) {
  row.status = row.status ? 1 : 0
}

/**
 * 保存矩阵，返回是否成功。
 *
 * 返回值是必须的：详情页把「价格」与其它页签合并成页级保存，价格存失败时
 * 不能继续后续步骤（否则出现"提示成功、价格其实没存"的错觉）。
 * 表格没动过时直接返回成功、不发请求（见 signature 的说明）。
 */
async function save(): Promise<boolean> {
  if (!props.productId || !items.value.length) return true
  if (!isDirty.value) return true
  saving.value = true
  emit('update:saving', true)
  try {
    const data = await saveProductPriceMatrix({
      product_id: props.productId,
      spec_id: props.specId,
      currency: currency.value,
      items: items.value.map((item) => ({
        cycle: item.cycle,
        price: Number(item.price) || 0,
        cost_price: Number(item.cost_price) || 0,
        setup_fee: Number(item.setup_fee) || 0,
        cost_setup_fee: Number(item.cost_setup_fee) || 0,
        status: item.status ? 1 : 0,
      })),
    })
    matrix.value = data
    items.value = data.items || []
    baseline.value = signature()
    MessagePlugin.success('周期价格已保存')
    return true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
    return false
  } finally {
    saving.value = false
    emit('update:saving', false)
  }
}

// 商品/规格变化即重载（独立页切筛选器、详情页首次拿到 id 都走这条）。
watch(
  () => [props.productId, props.specId] as const,
  () => {
    if (props.autoLoad === false) return
    void reload()
  },
  { immediate: true },
)

defineExpose({ reload, save })
</script>

<style scoped>
.head-tag {
  margin-left: 8px;
}

.matrix-alert {
  margin: 0 0 12px;
}

.matrix-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.matrix-footnote {
  margin: 12px 0 4px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
</style>
