<template>
  <div class="page-body product-module">
    <!--
      商品详情（与编辑页合并后的单页形态）。

      为什么合并：原来「详情」只读、「编辑」才能改，运营改一个字段要跳一次页；
      而且两边各渲染了一遍商品信息（详情用只读描述列表、编辑用表单），同样的字段
      在两个页面两种样式。现在一页承载全部：顶部是页头 + 页级「保存」，
      下方用子页导航（页签）分区，每区只保留**一处**编辑入口。
    -->
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AppIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ product?.name || '产品详情' }}</h2>
          <p class="page-header__desc">
            SKU: {{ product?.code || '—' }}
            <template v-if="product">
              · <t-tag :theme="sourceModeTag(product.source_mode).theme" variant="light" size="small" shape="round">
                {{ sourceModeTag(product.source_mode).text }}
              </t-tag>
              <t-tag :theme="statusTag(product.status).theme" variant="light" size="small" shape="round">
                {{ statusTag(product.status).text }}
              </t-tag>
            </template>
          </p>
        </div>
      </div>
      <t-space size="small" class="page-header__actions">
        <t-button variant="outline" :loading="loading" @click="reloadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button variant="outline" @click="goBack">返回列表</t-button>
        <!--
          页级「保存」：一次提交本页所有改动（配置项 → 基础信息 → 价格）。
          为什么合并成一个：原来详情页只读、编辑页才有保存，合并后如果每个子页各带按钮，
          运营改了三处要点三次；而三块写的是同一个商品的字段，"一并保存"才符合直觉。
        -->
        <t-button v-if="canUpdate" theme="primary" :loading="saving" @click="handleSave">保存</t-button>
      </t-space>
    </header>

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <!-- ══════════ 基本信息 ══════════ -->
        <t-tab-panel value="basic" label="基本信息">
          <div class="tabs-section">
            <!-- 可编辑的本体字段：与「商品列表 → 编辑」用的是同一份表单实现（口径只有一处）。 -->
            <ProductForm ref="basicRef" mode="edit" :initial="product" embedded />

            <!--
              只读的链路与审计信息：这些字段**不可编辑**（由导入/建品流程决定），
              但运营改价、对账时要看，所以单独列一块，不与上面的表单重复。
              原来详情页把可编辑字段也列在只读表里（与编辑表单两套渲染），已去掉。
            -->
            <t-descriptions v-if="product" :column="2" bordered size="medium" class="detail-desc">
              <t-descriptions-item label="链路">
                <t-tag :theme="sourceModeTag(product.source_mode).theme" variant="light" size="small" shape="round">
                  {{ sourceModeTag(product.source_mode).text }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="价格模型">{{ priceModelLabel(product.price_model) }}</t-descriptions-item>
              <!-- 上游专属：提供商 / 资源商品 / 加价规则（自营商品没有这三项，直接不渲染） -->
              <template v-if="isUpstreamChain">
                <t-descriptions-item label="上游提供商">{{ providerDisplay }}</t-descriptions-item>
                <t-descriptions-item label="关联上游商品">
                  <span>{{ product.source_product_id ? `#${product.source_product_id}` : '—' }}</span>
                </t-descriptions-item>
                <t-descriptions-item label="上游加价规则">
                  <span>{{ markupLabel(product.upstream_markup_type, product.upstream_markup_value) }}</span>
                  <span class="inline-hint">在「价格」子页设百分比规则或逐周期改价</span>
                </t-descriptions-item>
                <t-descriptions-item label="仅透传上游参数">
                  <t-tag
                    :theme="product.spec_passthrough ? 'warning' : 'default'"
                    variant="light"
                    size="small"
                    shape="round"
                  >
                    {{ product.spec_passthrough ? '是' : '否' }}
                  </t-tag>
                </t-descriptions-item>
              </template>
              <t-descriptions-item label="创建时间">{{ formatTime(product.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="更新时间">{{ formatTime(product.updated_at) }}</t-descriptions-item>
              <t-descriptions-item v-if="product.specs" label="规格快照 JSON" :span="2">
                <pre class="spec-pre">{{ product.specs }}</pre>
              </t-descriptions-item>
            </t-descriptions>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <!-- ══════════ 价格 ══════════
             两条链路的改价方式不同，这里按链路给对应的编辑器：
               · 上游转售：价格由「上游成本 + 加价规则」推导（UpstreamPriceEditor：
                 百分比规则跟随上游，或逐周期自定义转人工价，并可同步已开通主机的续费价）；
               · 自营：直接维护「周期 × 价格」矩阵（PriceMatrixEditor，商品调价页同款）。
             不把两个编辑器叠在一起：它们改的是同一批周期行，同时出现会让"到底以哪个为准"更模糊。 -->
        <t-tab-panel value="price" label="价格">
          <div class="tabs-section">
            <template v-if="product">
              <UpstreamPriceEditor
                v-if="isUpstreamChain"
                ref="priceRef"
                embedded
                :product-id="product.id"
                :product-name="product.name"
                :source-product-id="product.source_product_id"
                :upstream="product.upstream || null"
                :initial-markup-type="product.upstream_markup_type || ''"
                :initial-markup-value="product.upstream_markup_value || 0"
              />
              <template v-else>
                <!-- 自营商品可按 SKU 分别定价：先选要维护的规格，默认商品级（不区分规格）。 -->
                <div class="price-scope">
                  <span class="price-scope__label">定价范围</span>
                  <t-select
                    v-model="priceSpecId"
                    class="price-scope__select"
                    :options="priceSpecOptions"
                    :disabled="!priceSpecOptions.length"
                    @change="reloadPriceMatrix"
                  />
                  <span class="price-scope__hint">
                    选具体规格则只维护该 SKU 的周期价（其价格优先于商品级）；未拆 SKU 的商品只有商品级。
                  </span>
                </div>
                <PriceMatrixEditor ref="matrixRef" :product-id="product.id" :spec-id="priceSpecId" />
              </template>
            </template>
            <t-skeleton v-else :row-col="[{ width: '100%', height: '160px' }]" animation="gradient" />
          </div>
        </t-tab-panel>

        <!-- ══════════ 规格变体（SKU） ══════════ -->
        <t-tab-panel value="specs" label="规格变体（SKU）">
          <div class="tabs-section">
            <!--
              ══════════ 上游商品的 SKU 维度（T4.8）══════════
              对接魔方财务的商品的差异都写在「配置项」里（CPU 4核/8核、内存 8G/16G），
              上游没有本地规格模板。这里让运营声明哪些配置项属于"规格维度"，
              按取值组合展开成多个独立 SKU（可分别定价与设库存）。
              不设维度则维持原行为：一个商品 + 客户选配项，下单时就地选配置。
            -->
            <div v-if="isUpstreamChain" class="axis-panel">
              <div class="axis-panel__head">
                <div class="axis-panel__title">
                  <span>SKU 维度</span>
                  <t-tag
                    v-if="axisInfo && axisInfo.selected_keys.length"
                    theme="primary"
                    variant="light"
                    size="small"
                    shape="round"
                  >
                    已选 {{ axisInfo.selected_keys.length }} 项 → {{ axisInfo.combination_count }} 个规格
                  </t-tag>
                </div>
                <span class="axis-panel__hint">
                  勾选"每个取值都是一台不同机器"的配置项（CPU、内存这类离散规格）；带宽、系统盘这类连续可调量不要勾
                  —— 那应由客户下单时选。
                </span>
              </div>
              <t-loading v-if="axisLoading" size="small" />
              <template v-else-if="axisInfo">
                <p v-if="axisInfo.notice" class="axis-panel__warn">{{ axisInfo.notice }}</p>
                <p v-if="!axisInfo.candidates.length" class="axis-panel__empty">
                  该商品的上游配置项里没有可做维度的离散多值项（单值项做维度只会产出 1 个规格，数量型项客户填的是连续值）。
                </p>
                <div v-else class="axis-list">
                  <label
                    v-for="cand in axisInfo.candidates"
                    :key="cand.key"
                    class="axis-item"
                    :class="{ 'axis-item--on': axisSelected.includes(cand.key), 'axis-item--locked': cand.locked }"
                  >
                    <t-checkbox
                      :checked="axisSelected.includes(cand.key)"
                      :disabled="cand.locked"
                      @change="(v: boolean) => toggleAxis(cand.key, v)"
                    />
                    <div class="axis-item__body">
                      <div class="axis-item__line">
                        <span class="axis-item__label">{{ cand.label }}</span>
                        <code class="axis-item__key">{{ cand.key }}</code>
                        <span class="axis-item__count">{{ cand.values.length }} 个取值</span>
                        <t-tag v-if="cand.locked" variant="light" size="small" shape="round">上游锁定</t-tag>
                      </div>
                      <div class="axis-item__values">{{ cand.values.map((v) => v.label).join('、') }}</div>
                    </div>
                  </label>
                </div>
                <div class="axis-panel__foot">
                  <span class="axis-panel__count">
                    将生成
                    <b :class="{ 'axis-panel__count--over': axisOverLimit }">{{ axisCombination }}</b>
                    个规格（上限 {{ axisInfo.max_combinations }}）
                    <template v-if="axisInfo.existing_spec_count">· 现有 {{ axisInfo.existing_spec_count }} 个</template>
                  </span>
                  <t-space size="small">
                    <t-button size="small" variant="outline" :loading="axisSaving" @click="saveAxis">保存维度</t-button>
                    <t-button
                      size="small"
                      theme="primary"
                      :loading="axisGenerating"
                      :disabled="!axisSelected.length || axisOverLimit"
                      @click="generateAxisSpecs"
                    >
                      生成规格
                    </t-button>
                  </t-space>
                </div>
              </template>
            </div>

            <div class="table-card__head">
              <t-space size="small">
                <span class="table-card__meta">共 {{ specs.length }} 个规格</span>
                <!-- 上游商品的规格按上游开通，本地不建 SKU（建了也不参与履约）：只展示。 -->
                <template v-if="canEditSpecs">
                  <t-button size="small" variant="outline" @click="openGenerateDialog">按规格模板生成</t-button>
                  <t-button size="small" theme="primary" @click="openSpecDialog()">新增规格</t-button>
                </template>
              </t-space>
            </div>
            <t-alert
              theme="info"
              :message="
                isUpstreamChain
                  ? '代理商品按上游规格开通；此处规格仅供展示，本地不做增删改。上架门禁要求上游规格完成平台绑定或开启「仅透传」。'
                  : '自营商品上架前，每个启用 SKU 都需满足必填原子且完成平台绑定（绑定状态为「已确认」）。'
              "
            />
            <t-table
              row-key="id"
              :data="specs"
              :columns="isUpstreamChain ? specColumnsReadOnly : specColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #binding_status="{ row }">
                <t-tag :theme="bindingStatusTag(row.binding_status).theme" variant="light" size="small" shape="round">
                  {{ bindingStatusTag(row.binding_status).text }}
                </t-tag>
              </template>
              <template #platform_params="{ row }">
                <span class="cell-muted">{{ row.platform_params || '—' }}</span>
              </template>
              <template #status="{ row }">
                <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">
                  {{ row.status === 1 ? '启用' : '停用' }}
                </t-tag>
              </template>
              <template #op="{ row }">
                <t-space size="small">
                  <t-link theme="primary" @click="openSpecDialog(row)">编辑</t-link>
                  <t-link theme="primary" @click="openBindingDialog(row)">平台绑定</t-link>
                  <t-link theme="danger" @click="removeSpec(row)">删除</t-link>
                </t-space>
              </template>
              <template #empty>
                <t-empty
                  :description="
                    isUpstreamChain ? '上游商品由上游下发规格，本地不建 SKU' : '暂无规格变体，可点「新增规格」或按模板生成'
                  "
                />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <!-- ══════════ 可选配置项 ══════════
             自营与上游两种链路共用同一个编辑器（ConfigOptionsEditor），
             它按行的 source 区分"本地参数"与"上游镜像项"，并对上游行只读映射键。 -->
        <t-tab-panel value="config" label="可选配置项">
          <div class="tabs-section">
            <ConfigOptionsEditor
              v-if="product"
              ref="configRef"
              :product-id="product.id"
              :upstream="isUpstreamChain"
              :provider-id="product.source_provider_id"
            />
            <t-skeleton v-else :row-col="[{ width: '100%', height: '160px' }]" animation="gradient" />
          </div>
        </t-tab-panel>

        <!-- ══════════ 上游对接（仅对接商品） ══════════ -->
        <t-tab-panel v-if="isUpstreamChain" value="upstream" label="上游对接">
          <div class="tabs-section">
            <div class="table-card__head">
              <span class="table-card__meta">上游商品 ID：{{ product?.upstream?.upstream_id || '—' }}</span>
            </div>

            <t-alert
              v-if="product?.upstream"
              :theme="product.upstream.fixed_config ? 'warning' : 'info'"
              :message="
                product.upstream.fixed_config
                  ? '该商品由上游固定配置（规格不可修改）：配置箱按上游商品参数开通，本地改动不会生效，改动锁定项还会导致开通失败。'
                  : '该商品配置来自上游商品：未锁定项（多个取值可选）可在上游允许范围内选择，锁定项为上游固定配置不可修改。'
              "
            />
            <t-empty v-else description="未读取到上游商品镜像（可能未同步或上游已删除）" />

            <template v-if="product?.upstream">
              <t-descriptions :column="2" bordered size="medium" class="detail-desc">
                <t-descriptions-item label="上游提供商">
                  {{ product.upstream.provider_name || '—' }}
                  <t-tag
                    v-if="product.upstream.provider_type"
                    class="inline-tag"
                    theme="default"
                    variant="light"
                    size="small"
                    shape="round"
                  >
                    {{ product.upstream.provider_type }}
                  </t-tag>
                </t-descriptions-item>
                <t-descriptions-item label="上游商品 ID">{{ product.upstream.upstream_id || '—' }}</t-descriptions-item>
                <t-descriptions-item label="上游分组（分类）">
                  <t-tag v-if="product.upstream.group_name" theme="primary" variant="light" size="small" shape="round">
                    {{ product.upstream.group_name }}
                  </t-tag>
                  <span v-else>未分类（上游未公示分组）</span>
                </t-descriptions-item>
                <t-descriptions-item label="上游付费类型">
                  <span>{{ upstreamPayTypeLabel(product.upstream.pay_type) }}</span>
                </t-descriptions-item>
                <t-descriptions-item label="上游标价">
                  <span v-if="product.upstream.list_price > 0">¥{{ formatPrice(product.upstream.list_price) }}</span>
                  <span v-else class="upstream-price-hint">上游未定价</span>
                  <span v-if="product.upstream.price_source === 'catalog'" class="upstream-price-hint">
                    取自上游前台展示价（{{ upstreamCatalogCycleText }}，该商品价格挂在配置项上）
                  </span>
                </t-descriptions-item>
                <t-descriptions-item label="代理价（采购价）"
                  >¥{{ formatPrice(product.upstream.cost_price) }}</t-descriptions-item
                >
                <t-descriptions-item label="折扣">{{ product.upstream.discount_desc || '—' }}</t-descriptions-item>
                <t-descriptions-item label="毛利率">
                  <span>{{ grossMarginText }}</span>
                </t-descriptions-item>
                <t-descriptions-item label="是否可续费">
                  <t-tag
                    :theme="product.upstream.renewable ? 'success' : 'danger'"
                    variant="light"
                    size="small"
                    shape="round"
                  >
                    {{ product.upstream.renewable ? '可续费' : '不可续费' }}
                  </t-tag>
                  <span v-if="!product.upstream.renewable" class="inline-hint">{{ product.upstream.renew_note }}</span>
                </t-descriptions-item>
                <t-descriptions-item label="配置形态">
                  <t-tag
                    :theme="product.upstream.fixed_config ? 'warning' : 'primary'"
                    variant="light"
                    size="small"
                    shape="round"
                  >
                    {{ product.upstream.fixed_config ? '整机固定配置' : '部分可选' }}
                  </t-tag>
                </t-descriptions-item>
              </t-descriptions>

              <div class="table-card__head">
                <h3 class="card-title">付费周期（上游）</h3>
                <span class="table-card__meta">标价即上游公示价；代理价已含上游给本账号的折扣</span>
              </div>
              <t-table
                row-key="cycle"
                :data="upstreamCycleRows"
                :columns="upstreamCycleColumns"
                size="small"
                hover
                cell-empty-content="—"
              >
                <template #empty><t-empty description="上游未下发周期价格（可能只有一次性或免费周期）" /></template>
              </t-table>

              <div class="table-card__head">
                <h3 class="card-title">配置箱参数（上游）</h3>
                <span class="table-card__meta">
                  锁定项为上游固定配置，本地只读；未锁定项有多个取值，可在上游允许范围内选择
                </span>
              </div>
              <t-table
                row-key="key"
                :data="product.upstream.options"
                :columns="upstreamOptionColumns"
                size="small"
                hover
                cell-empty-content="—"
              >
                <template #up_status="{ row }">
                  <t-tag :theme="row.locked ? 'warning' : 'success'" variant="light" size="small" shape="round">
                    {{ row.locked ? '固定不可改' : `${row.sub_count} 个取值可选` }}
                  </t-tag>
                </template>
                <template #up_value="{ row }">
                  <span>{{ row.value || '—' }}</span>
                  <span v-if="!row.locked" class="inline-hint">以下单时选择为准</span>
                </template>
                <template #empty><t-empty description="上游商品未下发配置项" /></template>
              </t-table>

              <template v-if="product.upstream.description">
                <div class="table-card__head">
                  <h3 class="card-title">上游商品描述</h3>
                  <span class="table-card__meta">
                    上游下发的富文本原文（含样式标签）；前台按纯文本展示，避免把上游内容当可信 HTML
                  </span>
                </div>
                <!-- 不渲染上游 HTML（v-html 会把供应商内容当可信 HTML 引入后台会话），
                     这里转成纯文本预览：与用户侧看到的内容一致。 -->
                <pre class="upstream-desc">{{ decodedUpstreamDescription }}</pre>
              </template>
            </template>
          </div>
        </t-tab-panel>

        <!-- ══════════ 变更历史 ══════════ -->
        <t-tab-panel value="history" label="变更历史">
          <div class="tabs-section">
            <div class="table-card__head">
              <span class="table-card__meta">共 {{ history.length }} 条记录</span>
            </div>
            <t-table
              row-key="id"
              :data="history"
              :columns="historyColumns"
              :loading="loading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
            >
              <template #change_type="{ row }">
                <t-tag theme="primary" variant="light" size="small" shape="round">
                  {{ changeTypeLabel(row.change_type) }}
                </t-tag>
              </template>
              <template #created_at="{ row }">
                <span class="cell-muted">{{ formatTime(row.created_at) }}</span>
              </template>
              <template #empty>
                <t-empty description="暂无变更历史" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>

    <!-- SKU 新增/编辑 -->
    <t-dialog
      v-model:visible="specDialogVisible"
      :header="specForm.id ? '编辑规格变体' : '新增规格变体'"
      :width="isMobile ? '94vw' : '640px'"
      :confirm-btn="{ loading: specSaving }"
      @confirm="submitSpec"
    >
      <t-form label-align="top" :data="specForm">
        <div class="form-grid">
          <t-form-item label="规格编码" required>
            <t-input v-model="specForm.spec_code" placeholder="如 2c4g-30m" />
          </t-form-item>
          <t-form-item label="规格名称" required>
            <t-input v-model="specForm.name" placeholder="如 2核4G 30M" />
          </t-form-item>
          <t-form-item label="价格模型">
            <t-select v-model="specForm.price_model" :options="priceModelOptions" />
          </t-form-item>
          <t-form-item label="销售价（元）">
            <t-input-number v-model="specForm.price" :min="0" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="成本价（元）">
            <t-input-number v-model="specForm.cost_price" :min="0" :precision="2" theme="column" />
          </t-form-item>
          <t-form-item label="库存（-1 不限）">
            <t-input-number v-model="specForm.stock" :min="-1" theme="column" />
          </t-form-item>
          <t-form-item label="规格模板">
            <t-select
              v-model="specForm.spec_template_id"
              clearable
              placeholder="选模板可自动带出规格编码/名称/取值（仍可改）"
              :options="templateOptions"
              @change="onSpecTemplateChange"
            />
            <span class="form-hint">选中模板会自动填好规格编码、名称与原子取值，你只需微调。</span>
          </t-form-item>
          <t-form-item label="排序">
            <t-input-number v-model="specForm.sort_order" :min="0" theme="column" />
          </t-form-item>
          <t-form-item label="状态">
            <t-select v-model="specForm.status" :options="specStatusOptions" />
          </t-form-item>
        </div>
        <t-form-item label="规格取值 JSON（原子 key → 取值，如 compute.cpu / placement.region）">
          <t-textarea
            v-model="specForm.specs"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder='{"compute.cpu":2,"compute.memory":4096,"storage.system.size":60,"placement.region":"hk"}'
          />
        </t-form-item>
        <t-alert v-if="specAtoms.length" theme="info">
          <template #message>
            <div class="atom-hints">
              <span v-for="atom in specAtoms" :key="atom.key" class="atom-hint">
                {{ atom.key }}<em v-if="atom.required">*</em>({{ atom.name }}{{ atom.unit ? '/' + atom.unit : '' }})
              </span>
            </div>
          </template>
        </t-alert>
      </t-form>
    </t-dialog>

    <!-- 按规格模板生成 SKU（自营链路打通：模板含平台写参数 → SKU + 已确认绑定） -->
    <t-dialog
      v-model:visible="generateVisible"
      header="按规格模板生成规格"
      :width="isMobile ? '94vw' : '600px'"
      :confirm-btn="{ content: '生成', theme: 'primary', loading: generateSaving }"
      @confirm="submitGenerate"
    >
      <t-form label-align="top" :data="generateForm">
        <t-form-item label="规格模板" required>
          <t-select
            v-model="generateForm.spec_template_id"
            placeholder="请选择已配置平台参数的规格模板"
            :options="templateOptions"
            @change="onTemplateChange"
          />
        </t-form-item>
        <div class="form-grid">
          <t-form-item label="规格编码（留空自动生成）">
            <t-input v-model="generateForm.spec_code" placeholder="如 general-2c4g-50g" />
          </t-form-item>
          <t-form-item label="规格名称（留空取模板名）">
            <t-input v-model="generateForm.name" placeholder="如 通用型 2核4G" />
          </t-form-item>
          <t-form-item label="销售价（元）">
            <t-input-number
              v-model="generateForm.price"
              :min="0"
              :precision="2"
              theme="column"
              placeholder="0 = 取模板参考售价"
            />
          </t-form-item>
          <t-form-item label="成本价（元）">
            <t-input-number v-model="generateForm.cost_price" :min="0" :precision="2" theme="column" />
          </t-form-item>
        </div>
        <t-form-item label="平台参数（可覆盖模板值）">
          <t-textarea
            v-model="generateForm.platform_params"
            :autosize="{ minRows: 2, maxRows: 6 }"
            placeholder='留空即用模板参数，如 {"area":"1","node":"2","os":"12"}'
          />
        </t-form-item>
        <t-alert v-if="selectedTemplateHint" theme="info" :message="selectedTemplateHint" />
      </t-form>
    </t-dialog>

    <!-- 平台绑定 -->
    <t-dialog
      v-model:visible="bindingDialogVisible"
      :header="`平台绑定 · ${bindingSpec?.spec_code || ''}`"
      :width="isMobile ? '94vw' : '680px'"
      :footer="false"
    >
      <div class="binding-panel">
        <div class="binding-panel__head">
          <span>已确认的出站参数将在开通时覆盖推导值（优先级最高）。</span>
          <t-button size="small" variant="outline" @click="newBinding">新建绑定</t-button>
        </div>
        <t-table
          row-key="id"
          :data="bindings"
          :columns="bindingColumns"
          :loading="bindingLoading"
          size="small"
          table-layout="fixed"
          cell-empty-content="—"
        >
          <template #status="{ row }">
            <t-tag :theme="bindingStatusTag(row.status).theme" variant="light" size="small" shape="round">
              {{ bindingStatusTag(row.status).text }}
            </t-tag>
          </template>
          <template #op="{ row }">
            <t-space size="small">
              <t-link theme="primary" @click="editBinding(row)">编辑</t-link>
              <t-link v-if="row.status !== 'confirmed'" theme="primary" @click="confirmBinding(row)">确认</t-link>
            </t-space>
          </template>
          <template #empty>
            <t-empty description="暂无绑定，点击「新建绑定」建立平台参数映射" />
          </template>
        </t-table>

        <t-divider>绑定编辑</t-divider>
        <t-form label-align="top" :data="bindingForm">
          <div class="form-grid">
            <t-form-item label="方向">
              <t-select v-model="bindingForm.direction" :options="bindingDirectionOptions" />
            </t-form-item>
            <t-form-item label="匹配方式">
              <t-select v-model="bindingForm.match_type" :options="bindingMatchOptions" />
            </t-form-item>
            <t-form-item label="状态">
              <t-select v-model="bindingForm.status" :options="bindingStatusOptions" />
            </t-form-item>
            <t-form-item label="优先级">
              <t-input-number v-model="bindingForm.priority" :min="0" theme="column" />
            </t-form-item>
            <t-form-item label="置信度">
              <t-input-number v-model="bindingForm.confidence" :min="0" :max="100" theme="column" />
            </t-form-item>
            <t-form-item label="规格模板 ID">
              <t-input-number v-model="bindingForm.spec_template_id" :min="0" theme="column" />
            </t-form-item>
          </div>
          <t-form-item label="平台参数 JSON（area/node/store/pid/gid…）">
            <t-textarea
              v-model="bindingForm.platform_params"
              :autosize="{ minRows: 3, maxRows: 8 }"
              placeholder='{"area":"10|CN^襄阳电信","node":"1","store":"1"}'
            />
          </t-form-item>
          <t-form-item label="备注">
            <t-input v-model="bindingForm.remark" placeholder="选填" />
          </t-form-item>
          <div class="form-footer">
            <t-button theme="primary" :loading="bindingSaving" @click="submitBinding">
              {{ bindingForm.id ? '保存绑定' : '创建绑定' }}
            </t-button>
          </div>
        </t-form>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { AppIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  confirmSpecBinding,
  createProductSpec,
  deleteProductSpec,
  generateProductSpecFromTemplate,
  getProductDetail,
  getProductHistory,
  getProductSpecs,
  getSpecAtomList,
  getSpecBindingList,
  getSpecTemplateList,
  getUpstreamAxisOptions,
  generateUpstreamAxisSpecs,
  saveUpstreamAxisOptions,
  updateProduct,
  updateProductSpec,
  upsertSpecBinding,
} from '@/api/product'
import { getProviderList } from '@/api/admin'
import {
  bindingStatusTag,
  billingCycleLabel,
  changeTypeLabel,
  cycleEntries,
  formatPrice,
  formatTime,
  markupLabel,
  priceModelLabel,
  priceModelOptions,
  sourceModeTag,
  statusTag,
} from '@/pages/product/constants'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePermission } from '@/composables/usePermission'
import ConfigOptionsEditor from './ConfigOptionsEditor.vue'
import PriceMatrixEditor from './PriceMatrixEditor.vue'
import ProductForm from './ProductForm.vue'
import UpstreamPriceEditor from './UpstreamPriceEditor.vue'
import type {
  ProviderInfo,
  SaleProductHistoryInfo,
  SaleProductInfo,
  SaleProductSpecInfo,
  SaleProductSpecRequest,
  SaleProductUpdateRequest,
  SaleProductUpstreamOption,
  SpecAtomInfo,
  SpecBindingInfo,
  UpstreamAxisOptionsInfo,
  SpecBindingUpsertRequest,
  SpecTemplateInfo,
} from '@/types/interface'

defineOptions({ name: 'ProductProductsDetail' })

const route = useRoute()
const router = useRouter()
const { has } = usePermission()
const { isMobile } = useIsMobile()

/** 写权限：没有 product:update 时不渲染保存按钮与规格增删改入口（后端另有权限校验）。 */
const canUpdate = has('product:update')

const product = ref<SaleProductInfo | null>(null)
const specs = ref<SaleProductSpecInfo[]>([])
const history = ref<SaleProductHistoryInfo[]>([])
const specAtoms = ref<SpecAtomInfo[]>([])
const templateOptions = ref<{ label: string; value: number }[]>([])
// 模板详情（选模板时带出规格参数用）：只用来回填表单，不作为落库依据。
const specTemplateDetails = ref<SpecTemplateInfo[]>([])
const loading = ref(false)
const saving = ref(false)
const activeTab = ref('basic')

// 分类加载：基础信息表单里的「分类」下拉用它（categoryName 由表单自己展示选项文字，此处不需要）。
const { loadCategories } = useCategoryOptions()

const isUpstreamChain = computed(() => product.value?.source_mode === 'upstream')

// ===== 上游商品的 SKU 维度（T4.8）=====
// 声明哪些上游配置项属于"规格维度"，按取值组合展开成多个独立 SKU。
// 与自营的区别：上游没有规格模板，维度选择存在商品行上（products.sku_axes），
// 因为上游配置项是覆盖式重写的，标记放那里会被下次同步抹掉。
const axisInfo = ref<UpstreamAxisOptionsInfo | null>(null)
const axisLoading = ref(false)
const axisSaving = ref(false)
const axisGenerating = ref(false)
/** 界面上当前的勾选（可能尚未保存）；保存后与 axisInfo.selected_keys 一致。 */
const axisSelected = ref<string[]>([])

/** 按当前勾选实时算组合数（不必等保存就能看到会生成多少个规格）。 */
const axisCombination = computed(() => {
  const info = axisInfo.value
  if (!info || !axisSelected.value.length) return 0
  let total = 1
  for (const key of axisSelected.value) {
    const cand = info.candidates.find((c) => c.key === key)
    if (!cand || !cand.values.length) continue
    total *= cand.values.length
  }
  return total
})

const axisOverLimit = computed(
  () => !!axisInfo.value && axisCombination.value > axisInfo.value.max_combinations,
)

async function loadAxisOptions() {
  if (!isUpstreamChain.value) return
  axisLoading.value = true
  try {
    const info = await getUpstreamAxisOptions(Number(route.params.id))
    axisInfo.value = info
    axisSelected.value = [...info.selected_keys]
  } catch {
    // 拉不到就不展示该面板（老商品/上游未同步都可能失败），不打断整页。
    axisInfo.value = null
  } finally {
    axisLoading.value = false
  }
}

function toggleAxis(key: string, checked: boolean) {
  const next = new Set(axisSelected.value)
  if (checked) next.add(key)
  else next.delete(key)
  axisSelected.value = Array.from(next).sort()
}

async function saveAxis() {
  axisSaving.value = true
  try {
    const info = await saveUpstreamAxisOptions(Number(route.params.id), { keys: axisSelected.value })
    axisInfo.value = info
    axisSelected.value = [...info.selected_keys]
    MessagePlugin.success('SKU 维度已保存')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    axisSaving.value = false
  }
}

async function generateAxisSpecs() {
  axisGenerating.value = true
  try {
    // 先保存再生成：避免运营改了勾选直接点生成、结果按旧选择跑。
    await saveUpstreamAxisOptions(Number(route.params.id), { keys: axisSelected.value })
    const result = await generateUpstreamAxisSpecs(Number(route.params.id))
    MessagePlugin.success(
      `已生成 ${result.combination_count} 个规格（新增 ${result.created}、更新 ${result.updated}）`,
    )
    await Promise.all([loadSpecs(), loadAxisOptions()])
  } catch (error) {
    MessagePlugin.error((error as Error).message || '生成失败')
  } finally {
    axisGenerating.value = false
  }
}
/** 上游商品的规格由上游下发，本地只读展示（与规格页签的提示文案一致）。 */
const canEditSpecs = computed(() => canUpdate && !isUpstreamChain.value)

// ---------- 子页组件引用（页级保存按顺序调度它们） ----------
const basicRef = ref<{ prepareBasicPayload: () => (SaleProductUpdateRequest & { code?: string }) | null } | null>(null)
const configRef = ref<{ reload: () => Promise<void>; save: () => Promise<boolean> } | null>(null)
const priceRef = ref<{ reload: () => Promise<void>; save: () => Promise<boolean> } | null>(null)
const matrixRef = ref<{ reload: () => Promise<void>; save: () => Promise<boolean> } | null>(null)

// 自营商品的定价范围（商品级 / 指定 SKU）：与「商品调价 → 周期价格」同口径。
const priceSpecId = ref<number | undefined>(undefined)
const priceSpecOptions = computed(() => [
  { label: '商品级（默认价）', value: 0 },
  ...specs.value.filter((s) => s.status === 1).map((s) => ({ label: s.name || s.spec_code, value: s.id })),
])
function reloadPriceMatrix() {
  void matrixRef.value?.reload()
}

/**
 * 上游提供商展示名：商品详情里只有 source_provider_id（裸 ID），
 * 运营看不出这条链路接到哪家渠道。这里解析成「名称（类型） · #ID」。
 */
const providerNames = ref<Record<number, string>>({})
const providerDisplay = computed(() => {
  const id = product.value?.source_provider_id
  if (!id) return '—'
  const info = providerNames.value[id]
  return info ? `${info} · #${id}` : `#${id}`
})

/** 拉取提供商列表建 ID→展示名映射（失败只影响展示，回落为 #ID）。 */
async function loadProviderNames() {
  try {
    const data = await getProviderList({ page_size: 100 })
    const map: Record<number, string> = {}
    for (const item of data.items || []) {
      const type = item.provider_type ? `（${item.provider_type}）` : ''
      map[item.id] = `${item.name}${type}`
    }
    providerNames.value = map
  } catch {
    // 展示名解析失败不影响详情页主流程。
  }
}

// ===== 上游对接页签（仅对接商品）=====
const upstreamCycleColumns: PrimaryTableCol<{ cycle: string; label: string; amount: number }>[] = [
  { colKey: 'label', title: '周期', width: 120 },
  { colKey: 'amount', title: '上游标价（元）', width: 160 },
  { colKey: 'cost', title: '代理价（元）', width: 160 },
]

const upstreamOptionColumns: PrimaryTableCol<SaleProductUpstreamOption>[] = [
  { colKey: 'option_name', title: '上游配置项', minWidth: 150 },
  { colKey: 'up_status', title: '可变性', width: 130 },
  { colKey: 'up_value', title: '取值', minWidth: 180 },
]

/** 上游各周期标价/代理价合并成一张表（周期 → 标价 + 代理价）。 */
const upstreamCycleRows = computed(() => {
  const up = product.value?.upstream
  if (!up) return []
  const list = up.list_cycles || {}
  const cost = up.cost_cycles || {}
  const cycles = Array.from(new Set([...Object.keys(list), ...Object.keys(cost)]))
  return cycleEntries(
    cycles.reduce<Record<string, number>>((acc, cycle) => {
      acc[cycle] = list[cycle] ?? 0
      return acc
    }, {}),
  ).map((row) => ({ ...row, cost: cost[row.cycle] ?? 0 }))
})

/** 毛利率：本地售价 vs 代理价（保本线）。代理价缺失时不展示。 */
const grossMarginText = computed(() => {
  const up = product.value?.upstream
  const price = product.value?.price ?? 0
  if (!up || up.cost_price <= 0 || price <= 0) return '—'
  const margin = ((price - up.cost_price) / price) * 100
  return `${margin.toFixed(1)}%（售价 ¥${formatPrice(price)} / 代理价 ¥${formatPrice(up.cost_price)}）`
})

/** 上游描述（转义 HTML 原文）→ 纯文本预览：解实体、去标签、压缩空行。 */
const decodedUpstreamDescription = computed(() => decodeHtmlTextToPlainText(product.value?.upstream?.description))

function decodeHtmlTextToPlainText(raw?: string): string {
  if (!raw) return ''
  const decoded = String(raw)
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#0?39;/g, "'")
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
  return (
    decoded
      // 样式/脚本连同内容丢弃，其余标签换成空串（标签本身不是卖点内容）
      .replace(/<(script|style)\b[^>]*>[\s\S]*?<\/\1>/gi, '')
      .replace(/<br\s*\/?>/gi, '\n')
      .replace(/<\/(div|p|li|tr|h[1-4])>/gi, '\n')
      .replace(/<[^>]*>/g, '')
      .split('\n')
      .map((line) => line.replace(/[ \t\u00a0]+/g, ' ').trim())
      .filter((line) => line !== '')
      .join('\n')
  )
}

/** 上游前台展示价回填时的周期文案（catalog_cycle → 「年付」）。 */
const upstreamCatalogCycleText = computed(() => {
  const cycle = product.value?.upstream?.catalog_cycle
  return cycle ? billingCycleLabel(cycle) : '上游前台展示价'
})

function upstreamPayTypeLabel(payType: string): string {
  const found = String(payType || '').match(/"pay_type"\s*:\s*"([^"]+)"/)
  const value = found ? found[1] : payType
  return billingCycleLabel(value)
}

// ---------- 规格变体 ----------

/** 自营：可增删改。 */
const specColumns: PrimaryTableCol<SaleProductSpecInfo>[] = [
  { colKey: 'spec_code', title: '规格编码', minWidth: 120 },
  { colKey: 'name', title: '规格名称', minWidth: 120 },
  { colKey: 'price', title: '售价', width: 90 },
  { colKey: 'cost_price', title: '成本价', width: 90 },
  { colKey: 'stock', title: '库存', width: 70 },
  { colKey: 'binding_status', title: '平台绑定', width: 100 },
  { colKey: 'platform_params', title: '已确认参数', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 70 },
  { colKey: 'op', title: '操作', width: 190 },
]

/** 上游转售：只读（去掉操作列，避免"说是仅供参考、实际能删"的自相矛盾）。 */
const specColumnsReadOnly: PrimaryTableCol<SaleProductSpecInfo>[] = specColumns.filter((c) => c.colKey !== 'op')

const bindingColumns: PrimaryTableCol<SpecBindingInfo>[] = [
  { colKey: 'direction', title: '方向', width: 80 },
  { colKey: 'platform_params', title: '平台参数', minWidth: 220 },
  { colKey: 'match_type', title: '匹配', width: 100 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'priority', title: '优先级', width: 80 },
  { colKey: 'op', title: '操作', width: 130 },
]

const historyColumns: PrimaryTableCol<SaleProductHistoryInfo>[] = [
  { colKey: 'change_type', title: '操作类型', width: 100 },
  { colKey: 'old_value', title: '变更前', minWidth: 140 },
  { colKey: 'new_value', title: '变更后', minWidth: 140 },
  { colKey: 'operator_name', title: '操作人', width: 110 },
  { colKey: 'remark', title: '备注', minWidth: 120 },
  { colKey: 'created_at', title: '时间', width: 170 },
]

const specStatusOptions = [
  { label: '启用', value: 1 },
  { label: '停用', value: 0 },
]

const bindingDirectionOptions = [
  { label: '出站（开通参数）', value: 'outbound' },
  { label: '入站（归一展示）', value: 'inbound' },
]

const bindingMatchOptions = [
  { label: '人工', value: 'manual' },
  { label: '精确匹配', value: 'auto_exact' },
  { label: '区间匹配', value: 'auto_range' },
]

const bindingStatusOptions = [
  { label: '未映射', value: 'unmapped' },
  { label: '自动映射', value: 'auto_mapped' },
  { label: '已确认', value: 'confirmed' },
  { label: '已失效', value: 'stale' },
]

// ---------- SKU 编辑 ----------

const specDialogVisible = ref(false)
const specSaving = ref(false)
const specForm = reactive({
  id: 0,
  spec_code: '',
  name: '',
  specs: '',
  price_model: 'fixed',
  price: 0,
  cost_price: 0,
  stock: -1,
  spec_template_id: undefined as number | undefined,
  sort_order: 0,
  status: 1,
})

function openSpecDialog(row?: SaleProductSpecInfo) {
  specForm.id = row?.id || 0
  specForm.spec_code = row?.spec_code || ''
  specForm.name = row?.name || ''
  specForm.specs = row?.specs || ''
  specForm.price_model = row?.price_model || 'fixed'
  specForm.price = row?.price || 0
  specForm.cost_price = row?.cost_price || 0
  specForm.stock = row?.stock ?? -1
  specForm.spec_template_id = row?.spec_template_id || undefined
  specForm.sort_order = row?.sort_order || 0
  specForm.status = row?.status ?? 1
  specDialogVisible.value = true
}

/**
 * 选中规格模板时自动带出规格编码/名称/原子取值（可改）——此前模板下拉只是个"引用标记"，
 * 选了之后下面还要手写 JSON，模板等于白配。
 */
function onSpecTemplateChange(value?: number | string) {
  const id = Number(value)
  if (!id) return
  const tpl = specTemplateDetails.value.find((item) => item.id === id)
  if (!tpl) return
  if (!specForm.spec_code.trim()) specForm.spec_code = deriveSpecCodeFromTemplate(tpl)
  if (!specForm.name.trim()) specForm.name = tpl.name
  if (!specForm.price && tpl.price > 0) specForm.price = tpl.price
  if (!specForm.specs.trim()) specForm.specs = JSON.stringify(deriveSpecValuesFromTemplate(tpl), null, 2)
  MessagePlugin.info(`已按模板「${tpl.name}」带出规格参数，可按需修改`)
}

/** 由模板派生 SKU 编码：规格族-核数/内存/磁盘（与后端 deriveSpecCode 同口径）。 */
function deriveSpecCodeFromTemplate(tpl: SpecTemplateInfo): string {
  const parts: string[] = []
  if (tpl.spec_family) parts.push(tpl.spec_family)
  parts.push(`${tpl.cpu || 1}c${Math.round(tpl.memory || 1)}g`)
  if (tpl.disk > 0) parts.push(`${tpl.disk}g`)
  return parts.join('-')
}

/** 模板未填 spec_values 时按结构化字段推导原子取值（内存 GB→MB，与字典口径一致）。 */
function deriveSpecValuesFromTemplate(tpl: SpecTemplateInfo): Record<string, unknown> {
  if (tpl.spec_values && Object.keys(tpl.spec_values).length) return tpl.spec_values
  const out: Record<string, unknown> = {}
  if (tpl.cpu > 0) out['compute.cpu'] = tpl.cpu
  if (tpl.memory > 0) out['compute.memory'] = Math.round(tpl.memory * 1024)
  if (tpl.disk > 0) out['storage.system.size'] = tpl.disk
  if (tpl.bandwidth > 0) out['network.bandwidth'] = tpl.bandwidth
  if (tpl.disk_type) out['storage.system.type'] = tpl.disk_type
  if (tpl.os) out['image.os'] = tpl.os
  return out
}

async function submitSpec() {
  if (!specForm.spec_code.trim() || !specForm.name.trim()) {
    MessagePlugin.warning('请填写规格编码与名称')
    return
  }
  if (specForm.specs.trim()) {
    try {
      JSON.parse(specForm.specs)
    } catch {
      MessagePlugin.warning('规格取值必须是合法 JSON')
      return
    }
  }
  const payload: SaleProductSpecRequest = {
    spec_code: specForm.spec_code.trim(),
    name: specForm.name.trim(),
    specs: specForm.specs,
    price_model: specForm.price_model,
    price: specForm.price,
    cost_price: specForm.cost_price,
    stock: specForm.stock,
    spec_template_id: specForm.spec_template_id || 0,
    sort_order: specForm.sort_order,
    status: specForm.status,
  }
  specSaving.value = true
  const id = Number(route.params.id)
  try {
    if (specForm.id) {
      await updateProductSpec(id, specForm.id, payload)
      MessagePlugin.success('规格已更新')
    } else {
      await createProductSpec(id, payload)
      MessagePlugin.success('规格已新增')
    }
    specDialogVisible.value = false
    await loadSpecs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存规格失败')
  } finally {
    specSaving.value = false
  }
}

async function removeSpec(row: SaleProductSpecInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除规格',
    body: `确认删除规格「${row.name}」？此操作不影响历史订单。`,
    theme: 'warning',
    confirmBtn: { content: '删除', theme: 'danger' },
    onConfirm: async () => {
      try {
        await deleteProductSpec(Number(route.params.id), row.id)
        MessagePlugin.success('规格已删除')
        dialog.hide()
        await loadSpecs()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
  })
}

// ---------- 平台绑定 ----------

const bindingDialogVisible = ref(false)
const bindingLoading = ref(false)
const bindingSaving = ref(false)
const bindingSpec = ref<SaleProductSpecInfo | null>(null)
const bindings = ref<SpecBindingInfo[]>([])
const bindingForm = reactive({
  id: 0,
  direction: 'outbound',
  match_type: 'manual',
  status: 'unmapped',
  priority: 0,
  confidence: 0,
  spec_template_id: 0,
  platform_params: '',
  remark: '',
})

async function openBindingDialog(row: SaleProductSpecInfo) {
  bindingSpec.value = row
  bindingDialogVisible.value = true
  resetBindingForm()
  await loadBindings()
}

function resetBindingForm() {
  bindingForm.id = 0
  bindingForm.direction = 'outbound'
  bindingForm.match_type = 'manual'
  bindingForm.status = 'unmapped'
  bindingForm.priority = 0
  bindingForm.confidence = 0
  bindingForm.spec_template_id = 0
  bindingForm.platform_params = ''
  bindingForm.remark = ''
}

function newBinding() {
  resetBindingForm()
}

function editBinding(row: SpecBindingInfo) {
  bindingForm.id = row.id
  bindingForm.direction = row.direction || 'outbound'
  bindingForm.match_type = row.match_type || 'manual'
  bindingForm.status = row.status || 'unmapped'
  bindingForm.priority = row.priority || 0
  bindingForm.confidence = row.confidence || 0
  bindingForm.spec_template_id = row.spec_template_id || 0
  bindingForm.platform_params = stringifyParams(row.platform_params)
  bindingForm.remark = row.remark || ''
}

function stringifyParams(params: SpecBindingInfo['platform_params']): string {
  if (!params) return ''
  if (typeof params === 'string') return params
  return JSON.stringify(params, null, 2)
}

async function loadBindings() {
  if (!bindingSpec.value) return
  bindingLoading.value = true
  try {
    bindings.value = await getSpecBindingList({ product_spec_id: bindingSpec.value.id })
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载绑定失败')
  } finally {
    bindingLoading.value = false
  }
}

async function submitBinding() {
  if (!bindingSpec.value) return
  let params: Record<string, unknown> | undefined
  if (bindingForm.platform_params.trim()) {
    try {
      params = JSON.parse(bindingForm.platform_params)
    } catch {
      MessagePlugin.warning('平台参数必须是合法 JSON')
      return
    }
  }
  const payload: SpecBindingUpsertRequest = {
    product_spec_id: bindingSpec.value.id,
    direction: bindingForm.direction,
    match_type: bindingForm.match_type,
    status: bindingForm.status,
    priority: bindingForm.priority,
    confidence: bindingForm.confidence,
    spec_template_id: bindingForm.spec_template_id || 0,
    platform_params: params,
    remark: bindingForm.remark,
  }
  bindingSaving.value = true
  try {
    await upsertSpecBinding(payload)
    MessagePlugin.success('绑定已保存')
    resetBindingForm()
    await loadBindings()
    await loadSpecs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存绑定失败')
  } finally {
    bindingSaving.value = false
  }
}

async function confirmBinding(row: SpecBindingInfo) {
  try {
    await confirmSpecBinding(row.id, { remark: '人工确认（后台）' })
    MessagePlugin.success('绑定已确认')
    await loadBindings()
    await loadSpecs()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '确认失败')
  }
}

// ---------- 按规格模板生成 SKU（自营链路打通） ----------

const generateVisible = ref(false)
const generateSaving = ref(false)
const generateForm = reactive<{
  spec_template_id: number | undefined
  spec_code: string
  name: string
  price: number
  cost_price: number
  platform_params: string
}>({ spec_template_id: undefined, spec_code: '', name: '', price: 0, cost_price: 0, platform_params: '' })
// 模板列表（生成下拉用）：带平台参数与参考售价，便于选择时给出提示。
const generateTemplates = ref<SpecTemplateInfo[]>([])

const selectedTemplateHint = computed(() => {
  const tpl = generateTemplates.value.find((item) => item.id === generateForm.spec_template_id)
  if (!tpl) return ''
  const platform = tpl.platform_params ? JSON.stringify(tpl.platform_params) : '未配置'
  return `模板「${tpl.name}」：原子取值 ${tpl.spec_values ? JSON.stringify(tpl.spec_values) : '按 CPU/内存/磁盘推导'}；平台参数 ${platform}。`
})

async function openGenerateDialog() {
  generateForm.spec_template_id = undefined
  generateForm.spec_code = ''
  generateForm.name = ''
  generateForm.price = 0
  generateForm.cost_price = 0
  generateForm.platform_params = ''
  generateVisible.value = true
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100, status: 1 })
    generateTemplates.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载规格模板失败')
  }
}

function onTemplateChange(value?: number | string) {
  const tpl = generateTemplates.value.find((item) => item.id === Number(value))
  if (!tpl) return
  if (!generateForm.name) generateForm.name = tpl.name
  if (!generateForm.price && tpl.price > 0) generateForm.price = tpl.price
}

async function submitGenerate() {
  if (!generateForm.spec_template_id) {
    MessagePlugin.warning('请选择规格模板')
    return
  }
  let platformParams: Record<string, unknown> | undefined
  if (generateForm.platform_params.trim()) {
    try {
      platformParams = JSON.parse(generateForm.platform_params)
    } catch {
      MessagePlugin.warning('平台参数必须是合法 JSON')
      return
    }
  }
  generateSaving.value = true
  try {
    const result = await generateProductSpecFromTemplate(Number(route.params.id), {
      spec_template_id: generateForm.spec_template_id,
      spec_code: generateForm.spec_code.trim() || undefined,
      name: generateForm.name.trim() || undefined,
      price: generateForm.price,
      cost_price: generateForm.cost_price,
      platform_params: platformParams,
    })
    if (result.notice) {
      MessagePlugin.warning(result.notice)
    } else {
      MessagePlugin.success('规格已生成并完成平台绑定')
    }
    generateVisible.value = false
    await loadSpecs()
    await loadHistory()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '生成规格失败')
  } finally {
    generateSaving.value = false
  }
}

// ---------- 基础数据 ----------

async function loadSpecs() {
  specs.value = await getProductSpecs(Number(route.params.id))
}

async function loadHistory() {
  history.value = await getProductHistory(Number(route.params.id))
}

async function loadAtomsAndTemplates() {
  try {
    specAtoms.value = (await getSpecAtomList()).filter((atom) => atom.status === 1)
  } catch {
    specAtoms.value = []
  }
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100 })
    // 只列启用中的模板：停用模板不该再被选来生成新规格。
    specTemplateDetails.value = data.items.filter((item) => item.status === 1)
    templateOptions.value = specTemplateDetails.value.map((item) => ({ label: item.name, value: item.id }))
  } catch {
    specTemplateDetails.value = []
    templateOptions.value = []
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  loading.value = true
  try {
    product.value = await getProductDetail(id)
    await Promise.all([loadSpecs(), loadHistory()])
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载产品详情失败')
  } finally {
    loading.value = false
  }
}

/** 刷新：商品本体 + 规格/历史，各子页编辑器各自重载（它们的数据不在 product 里）。 */
async function reloadAll() {
  await loadDetail()
  await Promise.all([configRef.value?.reload(), priceRef.value?.reload(), matrixRef.value?.reload()])
}

/**
 * 页级保存：按「配置项 → 基础信息 → 价格」顺序落库，任一步失败即停并说明**已经存了什么**。
 *
 * 顺序的理由：价格放最后。上游商品的加价规则会重算 `products.price`（以及周期矩阵），
 * 而基础信息表单里的销售价是"进页面那一刻"读到的值；若反过来先存价格、再回传基础信息，
 * 那个旧值会把刚算好的价格覆盖回去（实测过的"改价保存后又变回去"）。
 * 先写基础信息、再由价格编辑器覆盖，最终值永远以价格为准 —— 与「矩阵价优先于商品展示价」
 * 的算价口径一致。
 */
async function handleSave() {
  if (saving.value || !product.value) return
  const id = Number(route.params.id)

  // 先校验取值（此时用户还看着表单，提示能对应上他在改的东西）。
  const payload = basicRef.value?.prepareBasicPayload() ?? null
  if (!payload) {
    activeTab.value = 'basic'
    return
  }

  saving.value = true
  try {
    // 1) 可选配置项（独立接口，整表覆盖该商品的配置项行）
    if (configRef.value) {
      const ok = await configRef.value.save()
      if (!ok) {
        activeTab.value = 'config'
        MessagePlugin.error('可选配置项未保存成功，基础信息与价格均未提交')
        return
      }
    }

    // 2) 基础信息（更新接口不接受 code，SKU 编码不可变）
    const { code: _code, ...rest } = payload
    void _code
    await updateProduct(id, rest)

    // 3) 价格：上游走加价规则/逐周期自定义，自营走周期矩阵
    const priceEditor = priceRef.value || matrixRef.value
    if (priceEditor) {
      const ok = await priceEditor.save()
      if (!ok) {
        activeTab.value = 'price'
        await loadDetail()
        MessagePlugin.warning('基础信息与配置项已保存，但价格未保存成功；请修正「价格」子页后重试')
        return
      }
    }

    MessagePlugin.success('已保存')
    // 保存成功后以服务端数据为准刷新一次：编辑表单与各只读区块都回到落库后的状态。
    await loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

function goBack() {
  router.push('/product/products')
}

onMounted(() => {
  loadCategories()
  loadAtomsAndTemplates()
  void loadProviderNames()
  loadDetail()
  void loadAxisOptions()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.tabs-section {
  padding-top: var(--space-md);
}

.spec-pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 12px;
  color: var(--hs-text-tech);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

/* 自营商品的价格子页：定价范围（商品级 / 指定 SKU）筛选行 */
.price-scope {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-bottom: 12px;
}

.price-scope__label {
  font-size: 13px;
  font-weight: 600;
}

.price-scope__select {
  width: 240px;
  max-width: 100%;
}

.price-scope__hint {
  flex: 1 1 260px;
  min-width: 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--td-text-color-secondary, #6b7280);
}

.atom-hints {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.atom-hint {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 12px;
}

.atom-hint em {
  color: var(--td-error-color, #d54941);
  font-style: normal;
}

.binding-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.tabs-section :deep(.t-alert) {
  margin-bottom: 12px;
}

.tabs-section > .form-footer {
  margin-top: 12px;
}
.upstream-price-hint {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: var(--td-text-color-placeholder, #999);
}

/* 详情页只读信息与可编辑表单之间留出分隔，避免看成一份表单的两半 */
.detail-desc {
  margin-top: 16px;
}

/* 页头按钮：窄屏换行，不与标题挤在一行 */
@media (max-width: 768px) {
  .page-header__actions {
    width: 100%;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .price-scope__select {
    width: 100%;
  }
}

/* ===== 上游商品的 SKU 维度面板（T4.8）===== */
.axis-panel {
  margin-bottom: 12px;
  padding: 12px;
  border-left: 3px solid var(--td-brand-color, #0052d9);
  border-radius: 0 8px 8px 0;
  background: var(--td-bg-color-container-hover, #f3f3f3);
}
.axis-panel__head {
  margin-bottom: 10px;
}
.axis-panel__title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}
.axis-panel__hint {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--td-text-color-placeholder, #8b8b8b);
}
.axis-panel__warn {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--td-error-color-6, #d54941);
}
.axis-panel__empty {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder, #8b8b8b);
}
.axis-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.axis-item {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--td-bg-color-container, #fff);
  cursor: pointer;
  transition: box-shadow 0.15s;
}
.axis-item--on {
  box-shadow: inset 0 0 0 1px var(--td-brand-color, #0052d9);
}
.axis-item--locked {
  opacity: 0.72;
  cursor: not-allowed;
}
.axis-item__body {
  flex: 1;
  min-width: 0;
}
.axis-item__line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.axis-item__label {
  font-weight: 500;
}
.axis-item__key {
  font-size: 12px;
  color: var(--td-text-color-placeholder, #8b8b8b);
}
.axis-item__count {
  font-size: 12px;
  color: var(--td-text-color-placeholder, #8b8b8b);
}
.axis-item__values {
  margin-top: 3px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #666);
  overflow-wrap: anywhere;
}
.axis-panel__foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 10px;
}
.axis-panel__count {
  font-size: 13px;
}
.axis-panel__count b {
  font-weight: 500;
}
.axis-panel__count--over {
  color: var(--td-error-color-6, #d54941);
}
</style>
