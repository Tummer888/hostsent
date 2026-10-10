<template>
  <div v-if="product" class="product-detail">
    <div class="site-container">
      <nav class="detail-breadcrumb" aria-label="面包屑">
        <NuxtLink to="/">首页</NuxtLink>
        <span class="detail-breadcrumb__sep">/</span>
        <NuxtLink to="/products">全部产品</NuxtLink>
        <span class="detail-breadcrumb__sep">/</span>
        <span class="detail-breadcrumb__current">{{ product.name }}</span>
      </nav>

      <div class="detail-main">
        <div class="detail-media">
          <img
            v-if="product.coverImage"
            :src="product.coverImage"
            :alt="product.name"
            class="detail-media__img"
          >
          <span v-else class="detail-media__fallback" aria-hidden="true">
            {{ product.name.trim().charAt(0) || 'P' }}
          </span>
        </div>

        <div class="detail-info">
          <h1 class="detail-info__title">{{ product.name }}</h1>
          <p v-if="product.description" class="detail-info__desc">{{ product.description }}</p>

          <div class="detail-price">
            <span class="detail-price__amount">{{ formatPriceAmount(displayPrice) }}</span>
            <span v-if="unit" class="detail-price__unit">{{ unit }}</span>
            <span v-if="selectedSku" class="detail-price__sku">{{ selectedSku.name }}</span>
          </div>

          <!-- 规格与周期只做「选择 + 传递意图」：算价与下单必须在用户中心完成 -->
          <div v-if="product.skus.length" class="detail-option">
            <h3 class="detail-option__title">选择规格</h3>
            <div class="detail-option__list">
              <button
                v-for="sku in product.skus"
                :key="sku.specCode"
                type="button"
                class="detail-option__item"
                :class="{ 'is-active': sku.specCode === selectedSpec }"
                :disabled="sku.stock === 0"
                @click="selectedSpec = sku.specCode"
              >
                <span class="detail-option__name">{{ sku.name }}</span>
                <span class="detail-option__price">
                  {{ sku.stock === 0 ? '已售罄' : formatPriceAmount(sku.price) }}
                </span>
              </button>
            </div>
          </div>

          <div v-if="selectableCycles.length" class="detail-option">
            <h3 class="detail-option__title">计费周期</h3>
            <div class="detail-option__list">
              <button
                v-for="c in selectableCycles"
                :key="c"
                type="button"
                class="detail-option__item"
                :class="{ 'is-active': c === selectedCycle }"
                @click="selectedCycle = c"
              >
                <span class="detail-option__name">{{ cycleLabel(c) }}</span>
              </button>
            </div>
          </div>

          <!--
            配置选项：官网只做「选择 + 传递意图」。加价金额需要按周期与折扣计算，
            只有用户中心才有权威结果，因此这里只把选择拼进跳转链接，不展示算价。
          -->
          <div v-for="opt in allOptions" :key="opt.optionKey" class="detail-option">
            <h3 class="detail-option__title">
              {{ opt.name }}
              <span v-if="opt.required" class="detail-option__required">必选</span>
              <span v-if="opt.unit" class="detail-option__unit">（{{ opt.unit }}）</span>
            </h3>

            <!-- 分组下拉：操作系统按镜像家族（Ubuntu / Windows）分组 -->
            <select
              v-if="opt.widget === 'group_select' || opt.widget === 'select'"
              v-model="selections[opt.optionKey]"
              class="detail-option__select"
            >
              <option value="">{{ opt.required ? `请选择${opt.name}` : '按平台默认' }}</option>
              <template v-if="opt.widget === 'group_select'">
                <optgroup v-for="g in groupedValues(opt)" :key="g.label" :label="g.label">
                  <option v-for="it in g.items" :key="it.value" :value="it.value">
                    {{ it.label }}
                  </option>
                </optgroup>
              </template>
              <template v-else>
                <option v-for="it in opt.values" :key="it.value" :value="it.value">
                  {{ it.label }}
                </option>
              </template>
            </select>

            <!-- 按钮组：CPU 档位这类少量离散取值 -->
            <div v-else-if="opt.widget === 'radio'" class="detail-option__list">
              <button
                v-for="it in opt.values"
                :key="it.value"
                type="button"
                class="detail-option__item"
                :class="{ 'is-active': selections[opt.optionKey] === it.value }"
                @click="selections[opt.optionKey] = it.value"
              >
                <span class="detail-option__name">{{ it.label }}</span>
              </button>
            </div>

            <!-- 数量型：带宽/数据盘/IP 数，区间由配置项给出 -->
            <input
              v-else-if="opt.widget === 'qty'"
              v-model="selections[opt.optionKey]"
              type="number"
              class="detail-option__number"
              :min="opt.minValue ?? 0"
              :max="opt.maxValue ?? undefined"
              :placeholder="opt.defaultValue || '按平台默认'"
            >

            <!-- 开关：二值参数 -->
            <div v-else-if="opt.widget === 'bool'" class="detail-option__list">
              <button
                v-for="it in opt.values"
                :key="it.value"
                type="button"
                class="detail-option__item"
                :class="{ 'is-active': selections[opt.optionKey] === it.value }"
                @click="selections[opt.optionKey] = it.value"
              >
                <span class="detail-option__name">{{ it.label }}</span>
              </button>
            </div>
          </div>

          <div class="detail-actions">
            <a
              v-if="purchaseUrl"
              :href="purchaseUrl"
              class="site-btn site-btn--primary"
              rel="noopener"
            >
              立即选购
            </a>
            <NuxtLink v-else to="/#contact" class="site-btn site-btn--ghost">
              联系咨询
            </NuxtLink>
            <span v-if="purchaseUrl && missingRequired.length" class="detail-option__hint">
              还需选择：{{ missingRequired.map((o) => o.name).join('、') }}
            </span>
            <NuxtLink to="/products" class="site-btn site-btn--ghost">返回产品列表</NuxtLink>
          </div>
        </div>
      </div>

      <section v-if="product.specs.length" class="detail-block">
        <h2 class="detail-block__title">规格参数</h2>
        <dl class="detail-specs">
          <div v-for="spec in product.specs" :key="spec.key" class="detail-specs__row">
            <dt class="detail-specs__label">{{ spec.label }}</dt>
            <dd class="detail-specs__value">{{ spec.value }}</dd>
          </div>
        </dl>
      </section>

      <section v-if="product.description" class="detail-block">
        <h2 class="detail-block__title">产品说明</h2>
        <p class="detail-block__text">{{ product.description }}</p>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatPriceAmount, priceUnit } from '#shared/schemas/product'
import type { ProductOption, ProductOptionItem } from '#shared/schemas/product'

const route = useRoute()
const { content } = useSiteContent()
const { public: publicConfig } = useRuntimeConfig()

/**
 * 先 await 再解构：await 会剥离 Object.assign 附加的属性，
 * 因此要保留 original 引用读取产品与状态。
 * 必须等待的原因：SSR 阶段同步读取会拿到尚未就绪的数据，
 * 从而把正常商品误判为已下线并返回 404。
 */
const detail = useProduct(route.params.id as string)
await detail

const { product, state } = detail

if (state.value !== 'ok' || !product.value) {
  throw createError(
    state.value === 'unavailable'
      ? { statusCode: 503, statusMessage: 'Product service unavailable', fatal: true }
      : { statusCode: 404, statusMessage: 'Product not found', fatal: true },
  )
}

const unit = computed(() => priceUnit(product.value?.priceModel ?? ''))

/**
 * 规格/周期选择。
 *
 * 官网只负责把选中的 spec_code 与 cycle 拼进跳转链接，不做算价也不下单：
 * 价格随用户组折扣、余额、周期矩阵变化，只有登录后的用户中心才有权威结果。
 */
const selectedSpec = ref('')
const selectedCycle = ref('')

const selectedSku = computed(() =>
  product.value?.skus.find((s) => s.specCode === selectedSpec.value),
)

/** 周期来源：选中规格的自有周期优先，否则回落商品级周期。 */
const selectableCycles = computed(() =>
  selectedSku.value?.cycles.length ? selectedSku.value.cycles : product.value?.cycles ?? [],
)

/** 展示价：选了规格就用规格价，否则用商品价。 */
const displayPrice = computed(() => selectedSku.value?.price ?? product.value?.price ?? 0)

function cycleLabel(c: string): string {
  const map: Record<string, string> = {
    hourly: '按小时',
    monthly: '按月',
    quarterly: '按季',
    semiannually: '半年',
    annually: '按年',
    biennially: '两年',
    onetime: '一次性',
  }
  return map[c] || c
}

// 默认选中第一个可售规格与第一个周期，让「立即选购」开箱可用
watchEffect(() => {
  const skus = product.value?.skus ?? []
  if (selectedSpec.value || !skus.length) return
  const preferred = skus.find((s) => s.stock !== 0) ?? skus[0]
  if (preferred) selectedSpec.value = preferred.specCode
})

// ---------- 配置选项：只做选择与意图传递 ----------
/** 选配项选择（参数名 → 取值）；下单时的权威校验与算价在用户中心。 */
const selections = ref<Record<string, string>>({})

/** 展平所有可选项（后端当前只给一个分组）。 */
const allOptions = computed(() => (product.value?.optionGroups ?? []).flatMap((g) => g.options))

/** 分组下拉的取值分组；无 group_label 的归入「其他」，保证不丢项。 */
function groupedValues(opt: ProductOption) {
  const groups: Array<{ label: string; items: ProductOptionItem[] }> = []
  for (const it of opt.values) {
    const label = it.groupLabel || '其他'
    let group = groups.find((g) => g.label === label)
    if (!group) {
      group = { label, items: [] }
      groups.push(group)
    }
    group.items.push(it)
  }
  return groups
}

/**
 * 必选项预填默认值：官网的定位是「把意图带过去」，
 * 未选必选项时用户中心仍会被拦，这里预填只是让跳转后的弹窗开箱可用。
 */
watchEffect(() => {
  const options = allOptions.value
  if (!options.length) return
  const next: Record<string, string> = {}
  for (const opt of options) {
    const current = selections.value[opt.optionKey]
    if (current !== undefined && current !== '') {
      next[opt.optionKey] = current
      continue
    }
    const widget = opt.widget
    if (widget === 'qty') {
      if (opt.defaultValue) next[opt.optionKey] = opt.defaultValue
      continue
    }
    if (opt.required) {
      const preferred =
        opt.values.find((v) => v.value === opt.defaultValue) ??
        opt.values.find((v) => v.isDefault) ??
        opt.values[0]
      if (preferred) next[opt.optionKey] = preferred.value
    } else if (opt.defaultValue) {
      next[opt.optionKey] = opt.defaultValue
    }
  }
  selections.value = next
})

/** 未选中的必选项（枚举型）：缺项时页面给出提示，跳转仍可点（用户中心会拦）。 */
const missingRequired = computed(() =>
  allOptions.value.filter((opt) => {
    if (!opt.required || opt.widget === 'qty') return false
    return !selections.value[opt.optionKey]
  }),
)

watch(
  selectableCycles,
  (cycles) => {
    if (cycles.length && !cycles.includes(selectedCycle.value)) {
      selectedCycle.value = cycles[0] as string
    }
  },
  { immediate: true },
)

/** 控制台地址未配置时不渲染购买入口，避免死链。 */
const purchaseUrl = computed(() => {
  if (!publicConfig.consoleUrl) return ''
  const base = String(publicConfig.consoleUrl).replace(/\/+$/, '')
  const params = new URLSearchParams({ product: String(product.value?.id ?? '') })
  if (selectedSpec.value) params.set('spec', selectedSpec.value)
  if (selectedCycle.value) params.set('cycle', selectedCycle.value)
  // 选配项意图：`config=cpu:4,os:62`（参数名与取值都是平台口径，用户中心按此预填）。
  const picked = Object.entries(selections.value).filter(([, v]) => v !== '')
  if (picked.length) params.set('config', picked.map(([k, v]) => `${k}:${v}`).join(','))
  return `${base}/shop?${params.toString()}`
})

useSeoMeta({
  title: () => product.value?.name ?? '',
  description: () => product.value?.description || content.value.site.slogan,
  ogTitle: () => `${product.value?.name ?? ''} · ${content.value.site.name}`,
  ogDescription: () => product.value?.description || content.value.site.slogan,
  ogType: 'website',
  ogImage: () => product.value?.coverImage || undefined,
})
</script>

<style scoped>
.product-detail {
  padding: 28px 0 72px;
}

.detail-breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--site-text-muted);
}

.detail-breadcrumb a:hover {
  color: var(--site-primary);
}

.detail-breadcrumb__sep {
  color: var(--site-text-subtle);
}

.detail-breadcrumb__current {
  color: var(--site-text);
}

.detail-main {
  display: grid;
  grid-template-columns: minmax(0, 440px) minmax(0, 1fr);
  gap: 40px;
  margin-top: 26px;
}

.detail-media {
  display: flex;
  align-items: center;
  justify-content: center;
  aspect-ratio: 4 / 3;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius-lg);
  background: linear-gradient(135deg, var(--site-primary-soft), rgba(15, 23, 42, 0.04));
  overflow: hidden;
}

.detail-media__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-media__fallback {
  font-size: 64px;
  font-weight: 700;
  color: var(--site-primary-strong);
  opacity: 0.5;
}

.detail-info__title {
  font-size: 28px;
  font-weight: 700;
}

.detail-info__desc {
  margin-top: 14px;
  font-size: 14.5px;
  line-height: 1.9;
  color: var(--site-text-muted);
}

.detail-price {
  display: flex;
  align-items: baseline;
  gap: 4px;
  margin-top: 26px;
  padding: 18px 20px;
  border-radius: var(--site-radius);
  background: var(--site-bg-muted);
  color: var(--site-primary-strong);
}

.detail-price__amount {
  font-size: 32px;
  font-weight: 700;
}

.detail-price__unit {
  font-size: 14px;
  color: var(--site-text-muted);
}

.detail-price__sku {
  margin-left: auto;
  font-size: 13px;
  color: var(--site-text-muted);
}

/* ---------- 规格 / 周期选择 ---------- */
.detail-option {
  margin-top: 24px;
}

.detail-option__title {
  margin-bottom: 10px;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--site-text);
}

.detail-option__list {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.detail-option__item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 120px;
  padding: 10px 14px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  background: #fff;
  color: var(--site-text);
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease;
}

.detail-option__item:hover:not(:disabled) {
  border-color: var(--site-primary);
}

.detail-option__item.is-active {
  border-color: var(--site-primary);
  background: var(--site-primary-soft);
  color: var(--site-primary-strong);
}

.detail-option__item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.detail-option__price {
  font-size: 12px;
  color: var(--site-text-muted);
}

.detail-option__item.is-active .detail-option__price {
  color: var(--site-primary-strong);
}

/* ---------- 配置选项控件 ---------- */
.detail-option__required {
  margin-left: 6px;
  padding: 0 5px;
  border-radius: 3px;
  background: rgba(220, 38, 38, 0.1);
  color: #dc2626;
  font-size: 11px;
  font-weight: 500;
  vertical-align: middle;
}

.detail-option__unit {
  margin-left: 4px;
  color: var(--site-text-muted);
  font-size: 12px;
  font-weight: 400;
}

.detail-option__select,
.detail-option__number {
  width: 100%;
  max-width: 320px;
  padding: 9px 12px;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  background: #fff;
  color: var(--site-text);
  font-size: 13.5px;
}

.detail-option__select:focus,
.detail-option__number:focus {
  border-color: var(--site-primary);
  outline: none;
}

.detail-option__hint {
  align-self: center;
  font-size: 12.5px;
  color: #dc2626;
}

.detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 26px;
}

.detail-block {
  margin-top: 52px;
}

.detail-block__title {
  font-size: 20px;
  font-weight: 600;
}

.detail-block__text {
  margin-top: 14px;
  font-size: 14.5px;
  line-height: 1.9;
  color: var(--site-text-muted);
  white-space: pre-line;
}

.detail-specs {
  margin: 18px 0 0;
  border: 1px solid var(--site-border);
  border-radius: var(--site-radius);
  overflow: hidden;
}

.detail-specs__row {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  border-bottom: 1px solid var(--site-border-soft);
}

.detail-specs__row:last-child {
  border-bottom: none;
}

.detail-specs__label,
.detail-specs__value {
  margin: 0;
  padding: 12px 18px;
  font-size: 14px;
}

.detail-specs__label {
  background: var(--site-bg-muted);
  color: var(--site-text-muted);
}

.detail-specs__value {
  color: var(--site-text);
}

@media (max-width: 900px) {
  .detail-main {
    grid-template-columns: minmax(0, 1fr);
    gap: 26px;
  }

  .detail-info__title {
    font-size: 23px;
  }
}

@media (max-width: 560px) {
  .detail-specs__row {
    grid-template-columns: minmax(0, 1fr);
  }

  .detail-specs__label {
    padding-bottom: 0;
  }
}
</style>
