/**
 * 代下单页面的状态与逻辑：选择商品 → 读取可售配置 → 算价 → 提交。
 *
 * 为什么从弹窗里抽成 composable：这套逻辑原先整个长在 ConfigureOrderDialog 里，
 * 改成独立页面后如果继续塞在页面文件里，模板、样式、算价与标签映射会混在一起。
 * 抽出来之后页面只负责布局，逻辑集中在这里。
 *
 * 价格口径必须与后端一致，否则页面显示与实付会对不上：
 *   - 默认价 = pricePipeline.Resolve 的结果（含数量乘算与选配加价、折扣），
 *     页面调 /orders/quote 拿的就是同一条管线的输出；
 *   - 自定义「首次价」在后端是 ManualAmount，直接覆盖最终总额（不再乘数量、
 *     不参与折扣），因此页面把它当总额展示而不是单价。
 */
import { computed, reactive, ref, watch, type Ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  createUserOrder,
  getAdminOrderConfig,
  quoteUserOrder,
  type AdminOrderBrief,
  type AdminOrderConfigOption,
  type AdminOrderConfigOptionVal,
  type AdminOrderConfigResponse,
  type AdminOrderConfigSpec,
  type AdminOrderQuoteResponse,
} from '@/api/user'
import { getProductList as getUcProductList } from '@/api/product'

export type SelectValue = string | number | boolean

export interface ProductOption {
  label: string
  value: number
}

export function cycleLabel(c: string): string {
  const map: Record<string, string> = {
    hourly: '按小时',
    daily: '按天',
    monthly: '月付',
    quarterly: '季付',
    semiannually: '半年付',
    annually: '年付',
    biennially: '两年付',
    triennially: '三年付',
    onetime: '一次性',
  }
  return map[c] || c
}

export function money(v: number): string {
  return (Math.round(v * 100) / 100).toFixed(2)
}

function sourceLabel(source: string): string {
  const map: Record<string, string> = {
    agent: '代理折扣',
    group: '用户组折扣',
    promotion: '促销优惠',
    manual: '人工改价',
    bound: '绑定续费价',
  }
  return map[source] || source || '优惠'
}

/**
 * 生成一个随机主机名：与后端 safeHostname 规则一致（字母开头 + 小写字母数字连字符）。
 *
 * 后缀取 8 位而不是 6 位：带 hs- 前缀共 11 位，满足上游「主机名至少 10 位」的校验；
 * 6 位时总长只有 9 位，提交后会被上游以「主机名长度至少10位」拒绝，白扣一次款。
 */
function randomHostnameValue(): string {
  return `hs-${Math.random().toString(36).slice(2, 10)}`
}

export function useOrderComposer(userId: Ref<number>) {
  const productsLoading = ref(false)
  const configLoading = ref(false)
  const quoteLoading = ref(false)
  const submitting = ref(false)
  const productOptions = ref<ProductOption[]>([])
  const config = ref<AdminOrderConfigResponse | null>(null)
  /** 默认价算价结果；null = 还没算出来或算价失败（不阻断下单）。 */
  const quote = ref<AdminOrderQuoteResponse | null>(null)
  /** 配置项选择：option_key → 取值（统按字符串存，下发口径一致）。 */
  const selection = ref<Record<string, string>>({})

  const form = reactive<{
    product_id: number | undefined
    spec_code: string
    billing_cycle: string
    quantity: number
    /** 首次开通的自定义总额；undefined/0 = 用默认价 */
    price: number | undefined
    /** 绑定续费价；undefined/0 = 不绑定 */
    renewal_price: number | undefined
    pay_mode: 'create' | 'balance'
  }>({
    product_id: undefined,
    spec_code: '',
    billing_cycle: '',
    quantity: 1,
    price: undefined,
    renewal_price: undefined,
    pay_mode: 'create',
  })

  /** 有货规格：售罄的仍然不展示（多规格交给运营显式选择）。 */
  const specs = computed(() => (config.value?.specs ?? []).filter((s) => s.stock !== 0))

  /** 周期选项：优先所选规格自有周期，否则回落商品级周期。 */
  const cycleChoices = computed(() => {
    const spec = config.value?.specs.find((s) => s.spec_code === form.spec_code)
    if (spec?.cycles?.length) return spec.cycles
    return config.value?.cycles ?? []
  })

  const options = computed(() => config.value?.options ?? [])
  const lockedUpstream = computed(() => (config.value?.upstream?.options ?? []).filter((o) => o.locked))
  const upstreamRenewNote = computed(() => {
    const up = config.value?.upstream
    if (!up || up.renewable) return ''
    return up.renew_note || '该商品不支持续费'
  })

  const selectedSpec = computed(() => specs.value.find((s) => s.spec_code === form.spec_code) ?? null)

  /** 未选中的必选枚举项（数量型/文本型有兜底，不拦提交）。 */
  const missingRequired = computed(() =>
    options.value.filter((opt) => {
      if (!opt.required) return false
      if (opt.widget === 'qty' || opt.widget === 'text') return false
      return !selection.value[opt.option_key]
    }),
  )

  /** 已填写的配置项数量（摘要栏「已配置 N/M 项」用）。 */
  const configuredCount = computed(
    () => options.value.filter((opt) => (selection.value[opt.option_key] ?? '') !== '').length,
  )

  /** 兜底默认价：算价未返回时用所选规格价，再退商品基础价，避免价格栏空着。 */
  const defaultUnitPrice = computed(() => {
    const spec = selectedSpec.value
    if (spec && spec.price > 0) return spec.price
    return config.value?.price ?? 0
  })

  /**
   * 预计实付总额。
   * 自定义首次价在后端是 ManualAmount（直接覆盖总额），所以这里不再乘数量；
   * 默认价用算价管线结果（已含数量与选配）。
   */
  const payableAmount = computed(() => {
    if (form.price && form.price > 0) return form.price
    if (quote.value) return quote.value.final_amount
    return defaultUnitPrice.value * form.quantity
  })

  /** 自定义价输入框占位符：把默认价写进去，运营一眼看到「不填就是这么多」。 */
  const defaultPricePlaceholder = computed(() => {
    const amount = quote.value?.final_amount ?? defaultUnitPrice.value * form.quantity
    return amount > 0 ? `默认 ¥${money(amount)}` : '留空按商品价'
  })

  /** 默认价的构成说明：把选配加价与折扣拆出来，避免只有一个数字看不懂。 */
  const quoteNote = computed(() => {
    const q = quote.value
    if (!q) return ''
    const parts: string[] = []
    if (q.option_amount > 0) parts.push(`含选配加价 ¥${money(q.option_amount)}`)
    if (q.discount_amount > 0) {
      parts.push(`优惠 -¥${money(q.discount_amount)}${q.discount_source ? `（${sourceLabel(q.discount_source)}）` : ''}`)
    }
    if (q.discount_amount > 0 && q.original_amount > 0) parts.push(`原价 ¥${money(q.original_amount)}`)
    return parts.join(' · ')
  })

  const canSubmit = computed(() => {
    if (!form.product_id) return false
    if (config.value && config.value.specs.length && !form.spec_code) return false
    return missingRequired.value.length === 0
  })

  // ---------- 加载 ----------

  async function loadProducts() {
    if (productOptions.value.length) return
    productsLoading.value = true
    try {
      const data = await getUcProductList({ page: 1, page_size: 100 })
      productOptions.value = (data?.items || []).map((p) => ({
        label: `${p.name}（¥${p.price}）`,
        value: p.id,
      }))
    } catch (error) {
      MessagePlugin.error((error as Error)?.message || '加载商品列表失败')
    } finally {
      productsLoading.value = false
    }
  }

  /** 选中商品后拉取该商品的可售配置，并预置规格/周期/选配默认值。 */
  async function selectProduct(productId: number | undefined) {
    config.value = null
    quote.value = null
    selection.value = {}
    form.spec_code = ''
    form.billing_cycle = ''
    if (!productId) return
    configLoading.value = true
    try {
      const data = await getAdminOrderConfig(productId)
      config.value = data
      // 只有一个规格时直接选中，省掉一次点击（多规格交给运营显式选择）。
      if (data.specs.length === 1 && data.specs[0].stock !== 0) {
        form.spec_code = data.specs[0].spec_code
      }
      if (cycleChoices.value.length) form.billing_cycle = cycleChoices.value[0]
      applyDefaultSelection()
      void refreshQuote()
    } catch (error) {
      MessagePlugin.error((error as Error)?.message || '加载商品配置失败')
    } finally {
      configLoading.value = false
    }
  }

  /** 切换规格时同步周期选项（规格自有周期可能只有部分档位）。 */
  function syncCycleForSpec() {
    const choices = cycleChoices.value
    if (!choices.length) return
    if (!form.billing_cycle || !choices.includes(form.billing_cycle)) {
      form.billing_cycle = choices[0]
    }
  }

  function selectSpec(sp: AdminOrderConfigSpec) {
    if (sp.stock === 0) return
    form.spec_code = sp.spec_code
  }

  /**
   * 默认选择：枚举项预选一个值，数量型填默认数量。
   *
   * 为什么非必选项也要预选：履约侧对「没选的项」就是取首个可见值
   * （上游 config_groups 的 pickConfigSub、自营的 pickEnumValue 都是这个口径）。
   * 界面上留空并不等于「不配置」，只等于「看不到会开出什么」——
   * 实测商品（网络类型/操作系统）的候选值都没有声明默认，留空时下拉是空白的，
   * 运营必须逐个点开才知道默认开的是 VPC 还是经典网络。
   * 这里把生效值显式预选出来，并以 selection 提交，界面所见即最终下发值。
   */
  function applyDefaultSelection() {
    const next: Record<string, string> = {}
    for (const opt of options.value) {
      if (opt.widget === 'qty') {
        next[opt.option_key] = String(numberValue(opt))
        continue
      }
      // 文本型（主机名）没有候选值，但留空会让运营看不到最终会开成什么名字：
      // 后端 safeHostname 会按商品名/订单号兜底生成，名字不可预测。
      // 这里进页面即随机填一个，运营看着合适就留、不合适再改。
      if (opt.widget === 'text') {
        if (opt.option_key === 'hostname') next[opt.option_key] = randomHostnameValue()
        continue
      }
      // 优先运营声明的默认值，其次首个可见值（与后端履约口径一致）。
      const preferred =
        opt.values.find((v) => v.value === opt.default_value) ??
        opt.values.find((v) => v.is_default) ??
        opt.values[0]
      if (preferred) next[opt.option_key] = preferred.value
    }
    selection.value = next
  }

  // ---------- 算价 ----------

  let quoteTimer: ReturnType<typeof setTimeout> | undefined
  /** 算价请求序号：只认最后一次发出的响应，避免慢响应把价格栏改回旧值。 */
  let quoteSeq = 0

  function debounceQuote() {
    if (quoteTimer) clearTimeout(quoteTimer)
    quoteTimer = setTimeout(() => void refreshQuote(), 300)
  }

  /**
   * 刷新默认价：调后端算价预览，与真正下单同一条管线。
   * 失败时静默清空（页面退回用规格价展示），不弹错误打断操作 ——
   * 算价失败不阻止下单，下单时后端还会再算一次并以那次为准。
   */
  async function refreshQuote() {
    if (!form.product_id || !userId.value) return
    const token = ++quoteSeq
    quoteLoading.value = true
    try {
      const selections = cleanSelections()
      const resp = await quoteUserOrder(userId.value, {
        product_id: form.product_id,
        billing_cycle: form.billing_cycle || undefined,
        spec_code: form.spec_code || undefined,
        quantity: form.quantity,
        config_selections: Object.keys(selections).length ? selections : undefined,
      })
      if (token === quoteSeq) quote.value = resp
    } catch {
      if (token === quoteSeq) quote.value = null
    } finally {
      if (token === quoteSeq) quoteLoading.value = false
    }
  }

  watch(
    () => form.spec_code,
    () => {
      syncCycleForSpec()
      void refreshQuote()
    },
  )
  watch(
    () => form.billing_cycle,
    () => void refreshQuote(),
  )
  // 数量与选配改动同样影响默认价（选项加价并入基数后再打折）。
  // 选配用 300ms 防抖：数量型是输入框，逐键触发会打出一串算价请求。
  watch([() => form.quantity, selection], debounceQuote, { deep: true })

  // ---------- 选配交互 ----------

  function setSelection(opt: AdminOrderConfigOption, value: SelectValue) {
    setSelectionValue(opt, value)
  }

  function setSelectionValue(opt: AdminOrderConfigOption, value: SelectValue | number | null | undefined) {
    selection.value = {
      ...selection.value,
      [opt.option_key]: value === null || value === undefined ? '' : String(value),
    }
  }

  /** 换一个随机主机名（对预填的名字不满意时点「换一个」）。 */
  function randomHostname(opt: AdminOrderConfigOption) {
    setSelectionValue(opt, randomHostnameValue())
  }

  function groupedValues(opt: AdminOrderConfigOption) {
    const groups: Array<{ label: string; items: AdminOrderConfigOptionVal[] }> = []
    for (const it of opt.values) {
      const label = it.group_label || '其他'
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
   * 下拉框最大宽度：选项多/标签长时才给足宽度，选项少就按内容走。
   * 固定满宽会把「网络类型」（三五个选项）撑成一整行的空框，一屏塞不下几个配置项。
   */
  function selectMaxWidth(opt: AdminOrderConfigOption): string {
    if (opt.widget === 'group_select' || opt.values.length > 8) return '100%'
    const longest = opt.values.reduce((max, v) => Math.max(max, v.label.length), 0)
    const px = Math.min(360, Math.max(140, longest * 14 + 64))
    return `${px}px`
  }

  function boolValues(opt: AdminOrderConfigOption): SelectValue[] {
    const values = opt.values.map((v) => v.value)
    if (values.length >= 2) return [values[0], values[1]]
    return [0, 1]
  }

  function numberValue(opt: AdminOrderConfigOption): number {
    const min = opt.min_value ?? undefined
    const max = opt.max_value ?? undefined
    let n: number | undefined
    const raw = selection.value[opt.option_key]
    if (raw !== undefined && raw !== '') {
      const parsed = Number(raw)
      if (!Number.isNaN(parsed)) n = parsed
    }
    if (n === undefined) {
      const def = Number(opt.default_value)
      if (opt.default_value !== '' && !Number.isNaN(def)) n = def
    }
    if (n === undefined) n = min ?? 0
    // 越界兜底：存量脏值已在数据层修正（migrations/111 + 启动期回填），
    // 这里再夹一次，任何来源的默认值都不会被原样预填给客户。
    if (min !== undefined && n < min) n = min
    if (max !== undefined && n > max) n = max
    return n
  }

  /**
   * 数量型选项的可填区间提示（如「可填 80 – 1024 GB，80 GB 以内不加价」）。
   * 这类参数按量计价，代下单的人得知道多少以内不加价、最多能给到多少。
   */
  function qtyRangeHint(opt: AdminOrderConfigOption): string {
    if (opt.widget !== 'qty') return ''
    const unit = opt.unit ? ` ${opt.unit}` : ''
    const parts: string[] = []
    if (opt.min_value !== null && opt.min_value !== undefined && opt.max_value !== null && opt.max_value !== undefined) {
      parts.push(`可填 ${opt.min_value} – ${opt.max_value}${unit}`)
    } else if (opt.min_value !== null && opt.min_value !== undefined) {
      parts.push(`最小 ${opt.min_value}${unit}`)
    } else if (opt.max_value !== null && opt.max_value !== undefined) {
      parts.push(`最大 ${opt.max_value}${unit}`)
    }
    const def = opt.default_value !== '' ? Number(opt.default_value) : NaN
    if (!Number.isNaN(def)) parts.push(`${def}${unit} 以内不加价`)
    // 超出部分的单价随周期变化（上游每周期各一价），写明当前周期的价格。
    const unitPrice = qtyUnitPrice(opt)
    if (unitPrice > 0) {
      const label = form.billing_cycle ? cycleLabel(form.billing_cycle) : ''
      parts.push(`超出部分 ¥${unitPrice.toFixed(2)}/${opt.unit || '单位'}${label ? `（${label}）` : ''}`)
    }
    return parts.join('，')
  }

  function itemLabel(opt: AdminOrderConfigOption, it: AdminOrderConfigOptionVal): string {
    const add = itemAddPrice(opt, it)
    return add > 0 ? `${it.label}（+¥${money(add)}）` : it.label
  }

  function placeholder(opt: AdminOrderConfigOption): string {
    if (opt.help) return opt.help
    return opt.required ? `请选择${opt.name}` : '按平台默认（可选）'
  }

  /** 可自由填写项的占位：一眼看出「这里能直接打字填值」，而不是只能选。 */
  function creatablePlaceholder(opt: AdminOrderConfigOption): string {
    const hint = opt.values.map((v) => v.label).join(' / ')
    return hint ? `可选 ${hint}，也可直接输入取值后回车` : `请输入${opt.name}（可直接输入后回车）`
  }

  function optionHelp(opt: AdminOrderConfigOption): string {
    const parts: string[] = []
    if (opt.help) parts.push(opt.help)
    const raw = selection.value[opt.option_key]
    if (raw) {
      const hit = opt.values.find((v) => v.value === raw)
      if (hit?.help) parts.push(`${hit.label}：${hit.help}`)
    }
    return parts.join(' ')
  }

  /** 某取值在当前周期下的加价（界面估算；真实金额以服务端算价为准）。 */
  function itemAddPrice(opt: AdminOrderConfigOption, it: AdminOrderConfigOptionVal): number {
    switch (form.billing_cycle) {
      case 'quarterly':
        if (it.price_quarterly > 0) return it.price_quarterly
        break
      case 'annually':
        if (it.price_annually > 0) return it.price_annually
        break
      case 'onetime':
        if (it.price_onetime > 0) return it.price_onetime
        break
    }
    void opt
    return it.price_monthly
  }

  /** 某项当前选择的加价估算（口径与后端 ResolveOptionPricing 一致）。 */
  function optionSurcharge(opt: AdminOrderConfigOption): number {
    if (opt.widget === 'qty') {
      // 单位加价必须按当前周期取：上游每个周期各有单价（系统盘 0.8/月、2.4/季、9.6/年），
      // 只按月度算会让年付订单的预览价远低于实付。
      const unit = qtyUnitPrice(opt)
      if (unit <= 0) return 0
      const def = opt.default_value !== '' ? Number(opt.default_value) : (opt.min_value ?? 0)
      const base = Number.isNaN(def) ? (opt.min_value ?? 0) : def
      const billable = numberValue(opt) - base
      return billable > 0 ? billable * unit : 0
    }
    const chosen = selection.value[opt.option_key] || opt.default_value
    const item = opt.values.find((v) => v.value === chosen)
    return item ? itemAddPrice(opt, item) : 0
  }

  /**
   * 数量型选项在当前周期的每单位加价。
   * 该周期无独立单价时回落月度价（与后端 quantityUnitPriceForCycle 同口径）。
   */
  function qtyUnitPrice(opt: AdminOrderConfigOption): number {
    const current = form.billing_cycle
    if (current === 'quarterly') return opt.unit_price_quarterly || opt.unit_price || 0
    if (current === 'annually') return opt.unit_price_annually || opt.unit_price || 0
    if (current === 'onetime') return opt.unit_price_onetime || opt.unit_price || 0
    return opt.unit_price || 0
  }

  /**
   * 把 SKU 的原子取值 JSON 拍平成展示标签（「2核」「4G」「50G SSD」）。
   * 只取几个常用原子：未知 key 原样丢弃，避免界面混进 compute.foo=bar 这类技术字段名。
   */
  function specAtoms(sp: AdminOrderConfigSpec): Array<{ label: string }> {
    if (!sp.specs) return []
    let raw: Record<string, unknown>
    try {
      raw = JSON.parse(sp.specs)
    } catch {
      return []
    }
    if (!raw || typeof raw !== 'object') return []
    const out: Array<{ label: string }> = []
    const cpu = Number(raw['compute.cpu'])
    const memoryMB = Number(raw['compute.memory'])
    const diskGB = Number(raw['storage.system.size'])
    const diskType = String(raw['storage.system.type'] ?? '')
    const bandwidth = Number(raw['network.bandwidth'])
    if (cpu > 0) out.push({ label: `${cpu}核` })
    if (memoryMB > 0) {
      // compute.memory 以 MB 存，小于 1G 才按 MB 展示
      out.push({ label: memoryMB >= 1024 ? `${Math.round(memoryMB / 1024)}G` : `${memoryMB}M` })
    }
    if (diskGB > 0) out.push({ label: diskType ? `${diskGB}G ${diskType.toUpperCase()}` : `${diskGB}G` })
    if (bandwidth > 0) out.push({ label: `${bandwidth}M 带宽` })
    return out
  }

  /** 是否为敏感项（密码类）：渲染成遮蔽输入框，代下单时也不明文示人。 */
  function isSecretOption(opt: AdminOrderConfigOption): boolean {
    return opt.option_key === 'root_password' || opt.option_key === 'password'
  }

  /** 提交给后端的选配：丢掉空值，避免把「未选」当作显式空值下发。 */
  function cleanSelections(): Record<string, string> {
    const out: Record<string, string> = {}
    for (const [k, v] of Object.entries(selection.value)) {
      if (v !== undefined && v !== '') out[k] = String(v)
    }
    return out
  }

  // ---------- 提交 ----------

  /** 创建订单；成功返回订单摘要，由页面决定跳转。 */
  async function submit(): Promise<AdminOrderBrief | null> {
    if (!form.product_id) {
      MessagePlugin.warning('请选择商品')
      return null
    }
    if (config.value && config.value.specs.length && !form.spec_code) {
      MessagePlugin.warning('请选择规格')
      return null
    }
    if (missingRequired.value.length) {
      MessagePlugin.warning(`请选择：${missingRequired.value.map((o) => o.name).join('、')}`)
      return null
    }
    submitting.value = true
    try {
      const selections = cleanSelections()
      const brief = await createUserOrder(userId.value, {
        product_id: form.product_id,
        billing_cycle: form.billing_cycle || undefined,
        spec_code: form.spec_code || undefined,
        quantity: form.quantity,
        price: form.price && form.price > 0 ? form.price : undefined,
        renewal_price: form.renewal_price && form.renewal_price > 0 ? form.renewal_price : undefined,
        pay_mode: form.pay_mode,
        config_selections: Object.keys(selections).length ? selections : undefined,
      })
      MessagePlugin.success(form.pay_mode === 'balance' ? '已支付并投递开通任务' : '订单已创建（待支付）')
      return brief
    } catch (error) {
      MessagePlugin.error((error as Error)?.message || '创建订单失败')
      return null
    } finally {
      submitting.value = false
    }
  }

  return {
    // 状态
    productsLoading,
    configLoading,
    quoteLoading,
    submitting,
    productOptions,
    config,
    quote,
    selection,
    form,
    // 计算属性
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
    // 方法
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
    qtyUnitPrice,
    specAtoms,
    isSecretOption,
    cleanSelections,
    submit,
  }
}
