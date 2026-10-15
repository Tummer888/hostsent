<template>
  <div class="page-body users-module order-create">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <CartIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">为用户创建订单</h2>
          <p class="page-header__desc">
            代客下单：选商品 → 选规格与配置 → 确认价格。
            提交后按所选开通方式处理，规格与选配口径与用户端自助购买一致。
          </p>
        </div>
      </div>
      <div class="detail-actions">
        <t-button variant="outline" @click="goBack">
          <template #icon><ChevronLeftIcon aria-hidden="true" /></template>
          返回
        </t-button>
        <t-button variant="outline" :loading="productsLoading" @click="reloadProducts">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新商品
        </t-button>
      </div>
    </header>

    <!-- 下单成功后的结果态：没有订单查看权限的角色（后端 order:list 与 user:create 是
         两个独立权限码）跳不进订单详情，此时留在本页给出订单号与后续动作，
         而不是把人丢到一个会被权限守卫弹走的页面。 -->
    <section v-if="createdOrder" class="table-card surface-card result-card">
      <span class="result-card__icon">
        <CheckCircleIcon size="22" aria-hidden="true" />
      </span>
      <div class="result-card__text">
        <h3 class="result-card__title">订单创建成功</h3>
        <p class="result-card__desc">
          订单号 <strong class="result-card__no">{{ createdOrder.order_no }}</strong>
          · {{ createdOrder.product_name }} · ¥{{ money(createdOrder.total_amount) }}
          <template v-if="createdOrder.status === 'pending'">（待支付）</template>
        </p>
      </div>
      <div class="result-card__actions">
        <t-button v-if="canViewOrder" theme="primary" @click="goOrderDetail">查看订单详情</t-button>
        <t-button variant="outline" @click="startAnother">再下一单</t-button>
        <t-button variant="outline" @click="goBack">返回</t-button>
      </div>
    </section>

    <div v-else class="composer">
      <div class="composer__main">
        <!-- 第一步：选商品。单列铺满：这是唯一的入口决策，独占一行读起来最清楚。 -->
        <section class="table-card surface-card">
          <h3 class="card-title">选择商品</h3>
          <div class="pick-row">
            <t-select
              v-model="form.product_id"
              class="pick-row__select"
              :options="productOptions"
              filterable
              clearable
              placeholder="请选择要开通的商品"
              :loading="productsLoading"
              @change="(v: unknown) => selectProduct(v as number | undefined)"
            />
            <span class="pick-row__hint">
              <template v-if="config">
                {{ config.name }} · {{ priceModelLabel(config.price_model) }}
                <template v-if="config.source_mode === 'upstream'"> · 上游转售</template>
              </template>
              <template v-else>选择商品后自动读取可售规格与配置项</template>
            </span>
          </div>
        </section>

        <!-- 未选商品：给出明确的空态，而不是一片空白 -->
        <section v-if="!form.product_id" class="table-card surface-card empty-card">
          <t-empty description="请先选择商品，随后在此配置规格、周期与开通参数" />
        </section>

        <t-loading v-else :loading="configLoading" size="small" show-overlay>
          <template v-if="config">
            <!-- 规格：卡片态展示原子取值与价格，比纯文字单选一眼可辨 -->
            <section class="table-card surface-card">
              <h3 class="card-title">
                规格
                <em class="card-hint">决定 CPU / 内存 / 硬盘与基准价</em>
              </h3>
              <div v-if="specs.length" class="specs">
                <button
                  v-for="sp in specs"
                  :key="sp.spec_code"
                  type="button"
                  class="spec"
                  :class="{ 'is-active': form.spec_code === sp.spec_code }"
                  @click="selectSpec(sp)"
                >
                  <span class="spec__name">{{ sp.name }}</span>
                  <span v-if="specAtoms(sp).length" class="spec__atoms">
                    <span v-for="a in specAtoms(sp)" :key="a.label" class="spec__atom">{{ a.label }}</span>
                  </span>
                  <span class="spec__foot">
                    <em class="spec__price">¥{{ money(sp.price) }}</em>
                    <em v-if="sp.stock > 0" class="spec__stock">剩 {{ sp.stock }}</em>
                  </span>
                </button>
              </div>
              <p v-else class="section-empty">该商品未挂规格，按商品级价格下单。</p>
            </section>

            <section v-if="cycleChoices.length" class="table-card surface-card">
              <h3 class="card-title">计费周期</h3>
              <t-radio-group v-model="form.billing_cycle" variant="default-filled" size="small">
                <t-radio-button v-for="c in cycleChoices" :key="c" :value="c">{{ cycleLabel(c) }}</t-radio-button>
              </t-radio-group>
            </section>

            <!-- 配置项：两列卡片。加配项是「多列表格型」内容，单列会把页面拉得很长。 -->
            <section v-if="options.length" class="table-card surface-card">
              <h3 class="card-title">
                配置选项
                <em class="card-hint">已按默认配置预选，需改动再调</em>
              </h3>
              <div class="options">
                <div v-for="opt in options" :key="opt.option_key" class="option">
                  <div class="option__head">
                    <span class="option__name">
                      {{ opt.name }}
                      <em v-if="opt.required" class="option__required">必选</em>
                      <em v-if="opt.unit" class="option__unit">{{ opt.unit }}</em>
                    </span>
                    <span v-if="optionSurcharge(opt) > 0" class="option__add">+¥{{ money(optionSurcharge(opt)) }}</span>
                  </div>

                  <!-- 分组下拉：操作系统按镜像家族 -->
                  <t-select
                    v-if="opt.widget === 'group_select'"
                    :model-value="selection[opt.option_key] || undefined"
                    :style="{ maxWidth: selectMaxWidth(opt) }"
                    :placeholder="placeholder(opt)"
                    :clearable="!opt.required"
                    filterable
                    @change="(v: SelectValue) => setSelection(opt, v)"
                  >
                    <t-option-group v-for="g in groupedValues(opt)" :key="g.label" :label="g.label" divider>
                      <t-option
                        v-for="it in g.items"
                        :key="it.value"
                        :value="it.value"
                        :label="itemLabel(opt, it)"
                      />
                    </t-option-group>
                  </t-select>

                  <!-- 枚举型取值少（≤6）→ 小卡片直接点选，比下拉少两次交互 -->
                  <div v-else-if="opt.widget === 'radio' && opt.values.length <= 6" class="cards">
                    <button
                      v-for="it in opt.values"
                      :key="it.value"
                      type="button"
                      class="card"
                      :class="{ 'is-active': selection[opt.option_key] === it.value }"
                      @click="setSelectionValue(opt, it.value)"
                    >
                      <span class="card__label">{{ it.label }}</span>
                      <span v-if="itemAddPrice(opt, it) > 0" class="card__price">
                        +¥{{ money(itemAddPrice(opt, it)) }}
                      </span>
                      <span v-else class="card__price card__price--free">不加价</span>
                    </button>
                  </div>
                  <!--
                    creatable：平台只给了示例值但允许填其它值（端口可填 1024~30000）。
                    渲染成可输入的下拉，既能选示例值也能直接填；
                    不允许自由填写时保持纯下拉，避免填出平台不认的值。
                  -->
                  <t-select
                    v-else-if="(opt.widget === 'radio' || opt.widget === 'select') && opt.creatable"
                    :model-value="selection[opt.option_key] || undefined"
                    :style="{ maxWidth: selectMaxWidth(opt) }"
                    :placeholder="creatablePlaceholder(opt)"
                    :clearable="!opt.required"
                    filterable
                    creatable
                    @create="(v: string) => setSelectionValue(opt, v)"
                    @change="(v: SelectValue) => setSelection(opt, v)"
                  >
                    <t-option v-for="it in opt.values" :key="it.value" :value="it.value" :label="itemLabel(opt, it)" />
                  </t-select>

                  <t-select
                    v-else-if="opt.widget === 'radio' || opt.widget === 'select'"
                    :model-value="selection[opt.option_key] || undefined"
                    :style="{ maxWidth: selectMaxWidth(opt) }"
                    :placeholder="placeholder(opt)"
                    :clearable="!opt.required"
                    filterable
                    @change="(v: SelectValue) => setSelection(opt, v)"
                  >
                    <t-option v-for="it in opt.values" :key="it.value" :value="it.value" :label="itemLabel(opt, it)" />
                  </t-select>

                  <t-switch
                    v-else-if="opt.widget === 'bool'"
                    :model-value="selection[opt.option_key]"
                    :custom-value="boolValues(opt)"
                    @change="(v: string | number | boolean) => setSelectionValue(opt, v)"
                  />

                  <!-- 数量型：滑块 + 数值输入（上游按量计价） -->
                  <div v-else-if="opt.widget === 'qty'" class="qty">
                    <t-slider
                      class="qty__slider"
                      :model-value="numberValue(opt)"
                      :min="opt.min_value ?? 0"
                      :max="opt.max_value ?? undefined"
                      :step="1"
                      :label="false"
                      @change="(v: number | number[]) => setSelectionValue(opt, Array.isArray(v) ? v[0] : v)"
                    />
                    <div class="qty__input">
                      <t-input-number
                        :model-value="numberValue(opt)"
                        :min="opt.min_value ?? 0"
                        :max="opt.max_value ?? undefined"
                        :step="1"
                        theme="normal"
                        @change="(v: number | string | undefined) => setSelectionValue(opt, v ?? '')"
                      />
                      <span v-if="opt.unit" class="qty__unit">{{ opt.unit }}</span>
                    </div>
                    <span v-if="qtyRangeHint(opt)" class="qty__range">{{ qtyRangeHint(opt) }}</span>
                  </div>

                  <!-- 文本型：主机名/登录密码等自由填写项；密码用遮蔽输入 -->
                  <div v-else-if="opt.widget === 'text'" class="text-field">
                    <t-input
                      :model-value="selection[opt.option_key] || ''"
                      :type="isSecretOption(opt) ? 'password' : 'text'"
                      :placeholder="placeholder(opt)"
                      clearable
                      @change="(v: string) => setSelectionValue(opt, v)"
                    />
                    <t-button
                      v-if="opt.option_key === 'hostname'"
                      size="small"
                      variant="outline"
                      @click="randomHostname(opt)"
                    >
                      换一个
                    </t-button>
                  </div>

                  <p v-if="optionHelp(opt)" class="option__help">{{ optionHelp(opt) }}</p>
                </div>
              </div>
              <p v-if="missingRequired.length" class="section-warn">
                请选择：{{ missingRequired.map((o) => o.name).join('、') }}
              </p>
            </section>

            <!-- 上游固定配置：只读，说明「这些项定死了，改也开不出来」 -->
            <section v-if="lockedUpstream.length" class="table-card surface-card">
              <h3 class="card-title">
                上游固定配置
                <em class="card-hint">上游已定规格，本地不可修改</em>
              </h3>
              <div class="fixed">
                <span v-for="o in lockedUpstream" :key="o.key" class="fixed__item">
                  {{ o.name }}：<strong>{{ o.value || '—' }}</strong>
                </span>
              </div>
              <p v-if="upstreamRenewNote" class="section-warn">{{ upstreamRenewNote }}</p>
            </section>

            <!-- 价格：先展示默认价（算价管线结果），再让运营按需覆盖首次价与续费价 -->
            <section class="table-card surface-card">
              <h3 class="card-title">
                价格
                <em class="card-hint">留空即按默认价下单；填了则以所填价为准，不参与折扣算价</em>
              </h3>
              <div class="price">
                <div class="price__fields">
                  <label class="price__field">
                    <span class="price__field-name">首次价（元）</span>
                    <t-input-number
                      v-model="form.price"
                      :min="0"
                      :precision="2"
                      theme="normal"
                      :placeholder="defaultPricePlaceholder"
                      clearable
                    />
                    <em class="price__field-hint">
                      留空 = ¥{{ money(quote?.final_amount ?? defaultUnitPrice * form.quantity) }}
                    </em>
                  </label>
                  <label class="price__field">
                    <span class="price__field-name">
                      续费价（元/周期）
                      <em class="price__field-tag">长期有效</em>
                    </span>
                    <t-input-number
                      v-model="form.renewal_price"
                      :min="0"
                      :precision="2"
                      theme="normal"
                      placeholder="留空 = 按商品定价续费"
                      clearable
                    />
                    <em class="price__field-hint">填写后此机器每次续费都按它结算，商品调价不影响</em>
                  </label>
                </div>
              </div>
            </section>
          </template>
        </t-loading>
      </div>

      <!-- 右侧摘要栏：始终可见，提交前不必回滚页面核对「给谁开、开什么、多少钱」 -->
      <aside class="composer__aside">
        <div class="table-card surface-card aside-card">
          <h3 class="card-title">下单用户</h3>
          <div class="buyer">
            <span class="buyer__avatar">
              <img v-if="profile?.avatar" :src="profile.avatar" :alt="username" />
              <UserIcon v-else size="18" aria-hidden="true" />
            </span>
            <div class="buyer__text">
              <span class="buyer__name">
                {{ profile?.username || username || '—' }}
                <t-tag v-if="profile && !isActive" theme="warning" variant="light" size="small" shape="round">
                  {{ userStatusLabel(profile.status) }}
                </t-tag>
              </span>
              <span class="buyer__id">ID {{ userId || '—' }}</span>
            </div>
          </div>
          <!-- 余额只在「余额支付并开通」下才是关键信息，其它情况弱化展示 -->
          <div class="buyer__balance">
            <span class="buyer__balance-label">账户余额</span>
            <span class="buyer__balance-value">¥{{ money(profile?.balance ?? 0) }}</span>
          </div>
          <p v-if="form.pay_mode === 'balance' && balanceShort" class="aside-warn">
            余额不足 ¥{{ money(payableAmount - (profile?.balance ?? 0)) }}，余额支付会失败。
          </p>
        </div>

        <div class="table-card surface-card aside-card">
          <h3 class="card-title">订单摘要</h3>
          <dl class="summary">
            <div class="summary__row">
              <dt>商品</dt>
              <dd>{{ config?.name || '未选择' }}</dd>
            </div>
            <div class="summary__row">
              <dt>规格</dt>
              <dd>{{ selectedSpec?.name || (config && !config.specs.length ? '商品级' : '未选择') }}</dd>
            </div>
            <div class="summary__row">
              <dt>周期</dt>
              <dd>{{ form.billing_cycle ? cycleLabel(form.billing_cycle) : '—' }}</dd>
            </div>
            <div class="summary__row">
              <dt>数量</dt>
              <dd>
                <t-input-number v-model="form.quantity" size="small" :min="1" :max="99" theme="normal" />
              </dd>
            </div>
            <div v-if="options.length" class="summary__row">
              <dt>配置项</dt>
              <dd>已填 {{ configuredCount }}/{{ options.length }} 项</dd>
            </div>
            <div v-if="form.renewal_price" class="summary__row">
              <dt>续费价</dt>
              <dd>¥{{ money(form.renewal_price) }} / {{ cycleLabel(form.billing_cycle) }}</dd>
            </div>
          </dl>

          <div class="payable">
            <span class="payable__label">
              应付金额
              <em v-if="quoteLoading" class="payable__loading">计算中…</em>
            </span>
            <span class="payable__value">¥{{ money(payableAmount) }}</span>
            <span v-if="form.price && form.price > 0" class="payable__tag">按自定义价</span>
            <span v-else-if="quoteNote" class="payable__note">{{ quoteNote }}</span>
          </div>

          <div class="pay-mode">
            <span class="pay-mode__label">开通方式</span>
            <t-radio-group v-model="form.pay_mode" variant="default-filled" size="small">
              <t-radio-button value="create">创建订单（待支付）</t-radio-button>
              <t-radio-button value="balance">余额支付并开通</t-radio-button>
            </t-radio-group>
            <p class="pay-mode__hint">
              {{
                form.pay_mode === 'balance'
                  ? '从用户余额扣款后立即投递开通任务，扣款与开通一步完成。'
                  : '只落一张待支付订单，由用户自行付款。'
              }}
            </p>
          </div>

          <div class="aside-actions">
            <t-button
              theme="primary"
              block
              :loading="submitting"
              :disabled="!canSubmit"
              @click="handleSubmit"
            >
              创建订单
            </t-button>
            <t-button variant="outline" block :disabled="submitting" @click="goBack">取消</t-button>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 代客下单页（原「为用户创建订单」弹窗改版）。
 *
 * 为什么从弹窗改成独立页面：这个表单的内容量（商品/规格/周期/多组配置项/上游固定项/价格）
 * 早已超出弹窗能承载的规模——弹窗里被迫把宽度拉到 1180px 再内部滚动，运营仍然要
 * 反复上下翻找，且弹窗一关就丢失全部填写内容。独立页面有稳定的整屏高度，
 * 右侧摘要栏常驻显示「给谁开、开什么、多少钱」，提交前不必回滚核对。
 *
 * 页面结构：
 *   左列 = 自上而下的配置流程（商品 → 规格 → 周期 → 配置项 → 上游固定 → 价格）；
 *   右列 = 常驻摘要（用户与余额、订单摘要、应付金额、开通方式、提交）。
 *
 * 逻辑集中在 useOrderComposer（算价走 /users/:id/orders/quote，与下单同一条管线）。
 */
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { CartIcon, CheckCircleIcon, ChevronLeftIcon, RefreshIcon, UserIcon } from 'tdesign-icons-vue-next'

import { getUserDetailAggregate, type AdminOrderBrief, type UserInfo } from '@/api/user'
import { priceModelLabel } from '@/pages/product/constants'
import { userStatusLabel } from '@/pages/users/constants'
import { useUserStore } from '@/store'

import { cycleLabel, money, useOrderComposer, type SelectValue } from './useOrderComposer'

defineOptions({ name: 'UserAccountOrderCreate' })

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const userId = computed(() => Number(route.query.id || 0))
/** 用户名从入口带进来，详情接口失败时页头仍能显示是谁 */
const username = ref(String(route.query.name || ''))
const profile = ref<UserInfo | null>(null)
const createdOrder = ref<AdminOrderBrief | null>(null)

const {
  productsLoading,
  configLoading,
  quoteLoading,
  submitting,
  productOptions,
  config,
  quote,
  selection,
  form,
  specs,
  cycleChoices,
  options,
  lockedUpstream,
  upstreamRenewNote,
  selectedSpec,
  missingRequired,
  configuredCount,
  defaultUnitPrice,
  payableAmount,
  defaultPricePlaceholder,
  quoteNote,
  canSubmit,
  loadProducts,
  selectProduct,
  selectSpec,
  setSelection,
  setSelectionValue,
  randomHostname,
  groupedValues,
  selectMaxWidth,
  boolValues,
  numberValue,
  qtyRangeHint,
  itemLabel,
  placeholder,
  creatablePlaceholder,
  optionHelp,
  itemAddPrice,
  optionSurcharge,
  specAtoms,
  isSecretOption,
  submit,
} = useOrderComposer(userId)

const isActive = computed(() => (profile.value?.status ?? 'active') === 'active')

/** 余额是否不足以覆盖本次应付（仅余额支付模式下拦提示）。 */
const balanceShort = computed(() => payableAmount.value > (profile.value?.balance ?? 0))

// 订单详情是订单域页面（守卫用 order:list），与代下单用的 user:create 是两个独立权限码：
// 有下单权但没有订单查看权的角色不该被送进一个会被弹走的页面。
const canViewOrder = computed(() => userStore.hasPermission('order:list'))

/**
 * 读取下单用户资料。
 *
 * 纯展示用途：失败只降级（页头退回入口带过来的用户名），不阻断下单 ——
 * 下单接口只认用户 ID，不依赖这里是否读到资料。
 */
async function loadProfile() {
  if (!userId.value) return
  try {
    const data = await getUserDetailAggregate(userId.value)
    profile.value = data.profile
    if (data.profile?.username) username.value = data.profile.username
  } catch {
    profile.value = null
  }
}

function reloadProducts() {
  productOptions.value = []
  void loadProducts()
}

async function handleSubmit() {
  const brief = await submit()
  if (!brief) return
  // 有订单查看权限就直接进订单详情；没有的留在本页看结果，避免被权限守卫弹回。
  if (canViewOrder.value) {
    void router.replace(`/orders/detail/${brief.id}`)
    return
  }
  createdOrder.value = brief
}

function goOrderDetail() {
  if (!createdOrder.value) return
  void router.replace(`/orders/detail/${createdOrder.value.id}`)
}

/** 结果态下「再下一单」：清空上一单的商品与配置，留在本页重新开始。 */
function startAnother() {
  createdOrder.value = null
  form.product_id = undefined
  form.spec_code = ''
  form.billing_cycle = ''
  form.quantity = 1
  form.price = undefined
  form.renewal_price = undefined
  form.pay_mode = 'create'
  void selectProduct(undefined)
}

function goBack() {
  const from = String(route.query.from || '')
  // 只接受站内相对路径，避免把外部地址当成跳转目标。
  if (from.startsWith('/') && !from.startsWith('//')) {
    void router.push(from)
    return
  }
  void router.push('/users/accounts/list')
}

onMounted(() => {
  if (!userId.value) {
    MessagePlugin.warning('缺少用户 ID，无法代下单')
    void router.replace('/users/accounts/list')
    return
  }
  void loadProducts()
  void loadProfile()
})
</script>

<style scoped>
/* 两列：左配置、右摘要。摘要在桌面端定宽并吸附，滚配置时不跟着走丢。 */
.composer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: var(--space-lg);
  align-items: start;
}

.composer__main {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  min-width: 0;
}

.composer__aside {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  position: sticky;
  /* 吸附在吸顶头部（56px）之下，与商品页的 aside 同一口径 */
  top: 76px;
}

.aside-card {
  padding: var(--space-md) var(--space-md);
}

/* ---------- 选商品 ---------- */
.pick-row {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.pick-row__select {
  width: 380px;
  max-width: 100%;
}

.pick-row__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.empty-card {
  padding: var(--space-xl) 20px;
}

/* ---------- 区块标题补充说明 ---------- */
.card-hint {
  margin-left: 8px;
  font-size: 12px;
  font-style: normal;
  font-weight: 400;
  color: var(--color-muted-foreground);
}

.section-empty {
  margin: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.section-warn {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--td-error-color-6);
}

/* ---------- 规格卡片 ---------- */
.specs {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
}

.spec {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-1);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast);
}

.spec:hover {
  border-color: var(--td-brand-color-3);
}

.spec.is-active {
  border-color: var(--td-brand-color);
  box-shadow: inset 0 0 0 1px var(--td-brand-color);
}

.spec__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.spec__atoms {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 6px;
}

.spec__atom {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--hs-surface-2);
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.spec__foot {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.spec__price {
  font-style: normal;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-error-color-6);
}

.spec__stock {
  font-style: normal;
  font-size: 11px;
  color: var(--color-muted-foreground);
}

/* ---------- 配置项 ---------- */
.options {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
}

.option {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-1);
}

.option__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.option__name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-foreground);
}

.option__required {
  margin-left: 6px;
  font-style: normal;
  font-size: 11px;
  color: var(--td-error-color-6);
}

/* 单位是「该填什么」的关键信息，做成带底色的标签而不是小灰字 */
.option__unit {
  margin-left: 6px;
  padding: 0 5px;
  border-radius: 3px;
  background: var(--hs-surface-2);
  font-style: normal;
  font-size: 11px;
  font-weight: 500;
  color: var(--color-muted-foreground);
}

.option__add {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-warning-color-6);
}

.option__help {
  margin: 0;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground);
}

/*
  枚举取值小卡片：取值少时替代下拉，「4核/8核」这类应当能直接点选。
  与规格卡片同一套视觉语言（选中主色描边），避免出现第三种选中态。
*/
.cards {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

/*
  卡片内边距放宽（用户反馈「有点拥挤」）：内容与边框留出呼吸空间，
  卡片之间也拉开间距，点选时不容易误触相邻项。
*/
.card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-width: 96px;
  padding: 11px 15px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-1);
  cursor: pointer;
  transition:
    border-color var(--hs-duration-fast),
    box-shadow var(--hs-duration-fast);
}

.card:hover {
  border-color: var(--td-brand-color-3);
}

.card.is-active {
  border-color: var(--td-brand-color);
  box-shadow: inset 0 0 0 1px var(--td-brand-color);
}

.card__label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-foreground);
}

.card__price {
  font-size: 11px;
  color: var(--td-warning-color-6);
}

/* 「不加价」用弱化色：它是「无需关注」的信息，不该与加价抢注意力 */
.card__price--free {
  color: var(--color-muted-foreground);
}

/* 数量型：滑块 + 数值输入 */
.qty {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.qty__slider {
  width: 100%;
}

.qty__input {
  display: flex;
  align-items: center;
  gap: 8px;
}

.qty__input :deep(.t-input-number) {
  width: 150px;
}

.qty__unit,
.qty__range {
  font-size: 11.5px;
  color: var(--color-muted-foreground);
}

.text-field {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ---------- 上游固定配置 ---------- */
.fixed {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  padding: 10px 12px;
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-2);
  font-size: 12px;
}

.fixed__item strong {
  color: var(--color-foreground);
}

/* ---------- 价格 ---------- */
.price__fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.price__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.price__field-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-foreground);
}

.price__field-tag {
  margin-left: 4px;
  padding: 0 5px;
  border-radius: 3px;
  background: var(--td-brand-color-1);
  font-style: normal;
  font-size: 10px;
  font-weight: 400;
  color: var(--td-brand-color);
}

.price__field-hint {
  font-style: normal;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground);
}

/* ---------- 右侧摘要 ---------- */
.buyer {
  display: flex;
  align-items: center;
  gap: 10px;
}

.buyer__avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 36px;
  height: 36px;
  overflow: hidden;
  border-radius: 50%;
  background: var(--hs-surface-2);
  color: var(--color-muted-foreground);
}

.buyer__avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.buyer__text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.buyer__name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.buyer__id {
  font-family: var(--hs-font-mono);
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.buyer__balance {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--td-component-border);
}

.buyer__balance-label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.buyer__balance-value {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-foreground);
}

.aside-warn {
  margin: 8px 0 0;
  font-size: 11.5px;
  line-height: 1.5;
  color: var(--td-error-color-6);
}

.summary {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
}

.summary__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
}

.summary__row dt {
  flex: none;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.summary__row dd {
  min-width: 0;
  margin: 0;
  font-size: 13px;
  color: var(--color-foreground);
  text-align: right;
  overflow-wrap: anywhere;
}

.payable {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 6px 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--td-component-border);
}

.payable__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.payable__loading {
  margin-left: 4px;
  font-style: normal;
  color: var(--td-brand-color);
}

.payable__value {
  margin-left: auto;
  font-size: 22px;
  font-weight: 700;
  color: var(--td-error-color-6);
}

.payable__tag {
  padding: 0 6px;
  border-radius: 3px;
  background: var(--td-warning-color-1);
  font-size: 11px;
  color: var(--td-warning-color-6);
}

.payable__note {
  flex: 1 1 100%;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground);
}

.pay-mode {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--td-component-border);
}

.pay-mode__label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.pay-mode__hint {
  margin: 8px 0 0;
  font-size: 11px;
  line-height: 1.5;
  color: var(--color-muted-foreground);
}

.aside-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 14px;
}

/* ---------- 结果态 ---------- */
.result-card {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.result-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--td-success-color-1);
  color: var(--td-success-color);
}

.result-card__text {
  flex: 1;
  min-width: 220px;
}

.result-card__title {
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

.result-card__desc {
  margin: 0;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.result-card__no {
  font-family: var(--hs-font-mono);
  color: var(--color-foreground);
}

.result-card__actions {
  display: flex;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

/*
  窄屏：摘要栏从右侧落下。顺序上放不进 DOM 前面（它在模板末尾），
  因此用 order 把它提到配置区之后、但仍随文档流 —— 提交按钮在手边，
  不必滚到最后才能下单。
*/
@media (max-width: 1100px) {
  .composer {
    grid-template-columns: minmax(0, 1fr);
  }

  .composer__aside {
    position: static;
    top: auto;
  }

  .price__fields {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .options {
    grid-template-columns: 1fr;
  }

  .pick-row__select {
    width: 100%;
  }

  .composer__aside {
    gap: var(--space-md);
  }
}
</style>
