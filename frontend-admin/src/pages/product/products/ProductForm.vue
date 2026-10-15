<template>
  <div class="page-body product-module">
    <header v-if="!embedded" class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AddIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ pageTitle }}</h2>
          <p class="page-header__desc">{{ headerDesc }}</p>
        </div>
      </div>
    </header>

    <!-- 代理（上游转售）商品：链路与上游绑定由导入决定（只读），基础信息（名称/分类/描述等）可改。 -->
    <section v-if="isUpstream" :class="embedded ? 'form-plain' : 'form-card surface-card'">
      <t-alert theme="info">
        <template #message>
          <div class="upstream-readonly">
            <p>
              这是<b>上游转售商品</b>：规格、开通参数与价格推导都由上游决定，
              <b>链路与上游绑定不可改</b>；本页可改名称、分类、描述、封面、库存、排序与「仅透传」。
            </p>
            <p class="upstream-readonly__meta">
              {{ providerDisplay }} · 上游资源商品 {{ upstreamProductDisplay }}
            </p>
          </div>
        </template>
      </t-alert>

      <!--
        合并到商品详情页后，这一段是详情页「基本信息」子页的内容，不再自带页签：
        价格编辑在详情页的「价格」子页（见 UpstreamPriceEditor / PriceMatrixEditor），
        两层次页签会让人分不清"到底在哪个标签下"。独立建品页（create）走下方自营分支，
        与这里无关。
      -->
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="产品名称" name="name" :rules="[{ required: true, message: '产品名称不能为空' }]">
            <t-input v-model="form.name" placeholder="请输入产品名称" clearable />
          </t-form-item>
          <t-form-item
            label="分类"
            name="category_id"
            :rules="[{ required: true, message: '请选择商品分类' }]"
          >
            <!-- 分类必填：留空会让商品落进「未分类」——列表显示不出归属、导航也统计不到。 -->
            <t-select v-model="form.category_id" placeholder="请选择分类" :options="categoryOptions" />
          </t-form-item>
          <t-form-item label="可售库存">
            <t-input-number v-model="form.stock" :min="-1" theme="column" placeholder="-1 表示不限" />
            <span class="form-hint">{{ stockHint }}</span>
          </t-form-item>
          <t-form-item label="排序">
            <t-input-number v-model="form.sort_order" :min="0" theme="column" placeholder="数值越小越靠前" />
          </t-form-item>
          <t-form-item label="封面图" class="form-grid__full">
            <t-input v-model="form.cover_image" placeholder="选填，官网产品卡与详情页展示用" clearable />
          </t-form-item>
          <t-form-item label="产品描述" class="form-grid__full">
            <t-textarea
              v-model="form.description"
              :autosize="{ minRows: 3, maxRows: 8 }"
              placeholder="选填；导入时已自动带入上游描述，可在此改写"
            />
          </t-form-item>
        </div>

        <!-- 仅透传：上架门禁的显式放行项。文案必须写清"通常不用开"，否则会与"导入即可上架"矛盾。 -->
        <t-form-item label="仅透传上游参数（上架门禁放行项）">
          <t-switch v-model="form.spec_passthrough" />
          <span class="clone-tip" style="display: block; margin-top: 6px; line-height: 1.7">
            <b>先说结论：</b>对接财务型上游（魔方财务等）时，开通参数由上游商品自带，
            系统上架时会<b>自动识别并放行</b>，这个开关<b>通常不需要开</b>。<br />
            <b>它是什么：</b>上架门禁要求确认「本地规格 → 上游开通参数」的对应关系；开启本开关
            表示"参数原样转发、无需确认"，用于门禁<b>无法自动判定</b>的少数情况（如上游没下发配置项）。<br />
            <b>什么时候该开：</b>该上游商品是整机固定配置，且上架时系统提示需要开启才能上架。
          </span>
        </t-form-item>
      </t-form>

      <div v-if="!embedded" class="form-footer">
        <t-button variant="outline" @click="handleCancel">取消</t-button>
        <t-button theme="primary" :loading="submitting" @click="handleSubmit">保存</t-button>
      </div>
    </section>

    <section v-else :class="embedded ? 'form-plain' : 'form-card surface-card'">
      <!--
        内嵌到商品详情页（「基础信息」子页）：自营商品只保留**可编辑的商品本体字段**。

        为什么不复用下面的建品向导：向导是"选档位 → 规格明细 → 基础信息 → 平台参数"的顺序流程，
        其中规格与周期价格在合并后的详情页各自是独立子页（规格变体 / 价格）。把向导整段搬进来
        会让同一件事出现两个入口，运营分不清该在哪个页签改。这里只留本体字段 + 平台默认参数，
        与后端的 products 表字段一一对应。
      -->
      <t-form v-if="embedded" label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="产品名称" name="name" :rules="[{ required: true, message: '产品名称不能为空' }]">
            <t-input v-model="form.name" placeholder="请输入产品名称" clearable />
          </t-form-item>
          <t-form-item label="SKU 编码" name="code">
            <!-- 编码是系统与上游的标识，创建后不可更改（改动会让历史订单与绑定对不上）。 -->
            <t-input v-model="form.code" disabled />
            <span class="form-hint">系统与上游用的标识，创建后不可更改。</span>
          </t-form-item>
          <t-form-item
            label="分类"
            name="category_id"
            :rules="[{ required: true, message: '请选择商品分类' }]"
          >
            <!-- 分类必填：留空会让商品落进「未分类」——列表显示不出归属、导航也统计不到。 -->
            <t-select v-model="form.category_id" placeholder="请选择分类" :options="categoryOptions" />
          </t-form-item>
          <t-form-item label="产品类型" name="product_type">
            <t-select v-model="form.product_type" clearable placeholder="请选择类型" :options="productTypeOptions" />
          </t-form-item>
          <t-form-item label="平台渠道" name="source_provider_id" :rules="platformRules">
            <t-select
              v-model="form.source_provider_id"
              placeholder="请选择自营平台渠道（魔方云等）"
              :options="platformOptions"
              :loading="platformLoading"
              clearable
              @change="onPlatformChange"
            />
            <span class="form-hint">云主机必须绑定一个「算力平台」渠道，开通/暂停/销毁将下发到该平台。</span>
          </t-form-item>
          <t-form-item label="价格模型" name="price_model">
            <t-select v-model="form.price_model" placeholder="请选择价格模型" :options="priceModelOptions" />
            <span class="form-hint">无显式周期时的兜底计费方式；各周期价格见本页「价格」子页。</span>
          </t-form-item>
          <t-form-item label="库存" name="stock">
            <t-input-number v-model="form.stock" :min="-1" theme="column" placeholder="-1 表示不限" />
            <span class="form-hint">{{ stockHint }}</span>
          </t-form-item>
          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="form.sort_order" :min="0" theme="column" placeholder="数值越小越靠前" />
          </t-form-item>
          <t-form-item label="销售价（元）" name="price">
            <t-input-number v-model="form.price" :min="0" :precision="2" theme="column" placeholder="请输入销售价" />
            <span class="form-hint">{{ priceHint }}</span>
          </t-form-item>
          <t-form-item label="成本价（元）" name="cost_price">
            <t-input-number
              v-model="form.cost_price"
              :min="0"
              :precision="2"
              theme="column"
              placeholder="请输入成本价"
            />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-select v-model="form.status" placeholder="请选择状态" :options="productStatusOptions" />
          </t-form-item>
          <t-form-item label="封面图 URL" name="cover_image">
            <t-input v-model="form.cover_image" placeholder="选填，官网产品卡与详情页展示用" clearable />
          </t-form-item>
          <t-form-item label="产品描述" name="description" class="form-grid__full">
            <t-textarea
              v-model="form.description"
              :autosize="{ minRows: 2, maxRows: 6 }"
              placeholder="选填，产品简介"
            />
          </t-form-item>
        </div>

        <div v-if="form.cover_image" class="cover-preview">
          <span class="cover-preview__label">封面预览</span>
          <img class="cover-preview__img" :src="form.cover_image" alt="封面预览" />
        </div>

        <!-- 平台默认参数（商品级）：档位未指定时生效，是开通的兜底映射。 -->
        <div class="form-step">
          <div class="form-step__head form-step__head--toggle" @click="platformSectionOpen = !platformSectionOpen">
            <span class="form-step__no">·</span>
            <div class="form-step__text">
              <div class="form-step__title">
                平台默认参数
                <t-tag theme="default" variant="light" size="small" shape="round">选填</t-tag>
              </div>
              <div class="form-step__desc">
                商品级默认值，仅当档位未指定对应项时生效（档位自身的配置优先级更高）。
                <template v-if="!platformSectionOpen">需要时展开。</template>
              </div>
            </div>
            <t-link theme="primary" hover="color">{{ platformSectionOpen ? '收起' : '展开' }}</t-link>
          </div>

          <template v-if="platformSectionOpen">
            <t-alert
              theme="info"
              message="此处是商品级默认参数；「规格变体」里每个 SKU 的平台绑定优先级更高。选好平台渠道后可下拉选取平台上真实存在的区域/节点/存储/镜像。"
            />
            <div class="form-grid">
              <t-form-item label="区域 area">
                <t-select
                  v-model="platformParams.area"
                  clearable
                  placeholder="平台区域"
                  :options="areaOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
              <t-form-item label="节点 node">
                <t-select
                  v-model="platformParams.node"
                  clearable
                  placeholder="平台节点"
                  :options="nodeOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
              <t-form-item label="存储 store">
                <t-select
                  v-model="platformParams.store"
                  clearable
                  placeholder="系统盘所在存储（可选）"
                  :options="storeOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
              <t-form-item label="镜像 os">
                <t-select
                  v-model="platformParams.os"
                  clearable
                  filterable
                  placeholder="平台镜像（默认开通用）"
                  :options="imageOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
            </div>

            <template v-if="catalogDefaults.length">
              <div class="defaults-hint">平台默认取值（未显式设置时按这些值开通）</div>
              <div class="defaults-list">
                <div v-for="d in catalogDefaults" :key="d.key" class="defaults-item">
                  <span class="defaults-item__label">{{ d.label }}</span>
                  <code class="defaults-item__value">{{ d.value }}</code>
                  <span v-if="d.help" class="defaults-item__help">{{ d.help }}</span>
                </div>
              </div>
            </template>
            <t-form-item label="其他平台参数（高级，JSON）">
              <t-textarea
                v-model="extraParamsText"
                :autosize="{ minRows: 2, maxRows: 6 }"
                placeholder='上面四个下拉之外的键，如 {"network_type":"normal","ip_num":1,"traffic_quota":0}'
              />
            </t-form-item>
          </template>
        </div>
      </t-form>

      <t-form v-else label-align="top" :data="form" @submit.prevent>
        <!-- ══════════ 步骤 1：选规格档位（建品入口） ══════════
             为什么提到最前：建品的实际决策顺序是「先定卖什么规格，再补名称与描述」。
             原先把档位选择放在表单最底部，运营得先跳过一堆与规格无关的字段
             （分类/渠道/封面/描述/平台参数）才看得到档位，选完还会在原地展开一个
             子表单，与上面的基础信息重复一遍，看着像两份表单。
             现在：档位在最前，选完即回填下方基础信息；档位自身的细节在「规格明细」统一改。 -->
        <template v-if="mode === 'create'">
          <div class="form-step">
            <div class="form-step__head">
              <span class="form-step__no">1</span>
              <div class="form-step__text">
                <div class="form-step__title">选规格档位</div>
                <div class="form-step__desc">
                  勾选要开通的规格（一个档位 = 一个 SKU）。选好后「基础信息」会按档位自动填好名称与售价， 档位自身的 CPU
                  / 内存 / 盘 / 镜像在「规格明细」里逐档微调。
                </div>
              </div>
              <t-link v-if="specMode === 'template'" theme="default" hover="color" @click="specMode = 'later'">
                暂不选规格
              </t-link>
              <t-link v-else theme="primary" hover="color" @click="specMode = 'template'">改为按档位配置</t-link>
            </div>

            <template v-if="specMode === 'template'">
              <t-alert
                v-if="!templateRows.length"
                theme="warning"
                message="还没有规格模板。请先到「产品管理 → 规格管理 → 规格模板」新建模板并配好平台映射。"
              />
              <template v-else>
                <div class="tpl-picker">
                  <div class="tpl-picker__bar">
                    <!-- 机型筛选：独立 IP 与 NAT 的访问方式不同，分开看避免混选 -->
                    <t-radio-group v-model="machineKindFilter" variant="default-filled" size="small">
                      <t-radio-button v-for="opt in machineKindFilterOptions" :key="opt.value" :value="opt.value">{{
                        opt.label
                      }}</t-radio-button>
                    </t-radio-group>
                    <t-space size="small">
                      <span class="tpl-list__count">已选 {{ selectedTemplateCount }} / {{ templateRows.length }}</span>
                      <t-link theme="primary" hover="color" @click="selectAllTemplates">全选</t-link>
                      <t-link theme="primary" hover="color" @click="clearAllTemplates">清空</t-link>
                    </t-space>
                  </div>

                  <t-alert
                    v-if="hasMixedKinds"
                    theme="warning"
                    class="tpl-mix-warn"
                    message="已同时勾选「独立 IP」与「NAT 机型」档位：同一商品下的规格访问方式不一致（前者直连独立 IP，后者经节点转发地址），请确认这是有意为之。"
                  />

                  <div v-for="grp in machineKindGroups" :key="grp.kind" class="tpl-group">
                    <div class="tpl-group__head">
                      <t-tag
                        :theme="grp.kind === 'nat' ? 'warning' : grp.kind === 'vps' ? 'primary' : 'default'"
                        variant="light"
                        size="small"
                        shape="round"
                        >{{ grp.label }}</t-tag
                      >
                      <span class="tpl-group__desc">{{ machineKindGroupDesc(grp.kind) }}</span>
                    </div>
                    <!-- 紧凑卡片：一眼看清档位规格与价格，勾选即选中；细节在「规格明细」改，
                         不在这里展开 —— 展开会让同一批字段出现两处，是"重复表单"的来源。 -->
                    <div class="tpl-grid">
                      <label
                        v-for="row in grp.rows"
                        :key="row.tpl.id"
                        class="tpl-chip"
                        :class="{ 'tpl-chip--on': row.selected }"
                      >
                        <t-checkbox v-model="row.selected" @change="onRowToggle(row)" />
                        <span class="tpl-chip__body">
                          <span class="tpl-chip__name">{{ row.tpl.name }}</span>
                          <span class="tpl-chip__spec">
                            {{ row.cpu }} 核 · {{ row.memoryGb }}G 内存 · {{ row.disk }}G 系统盘<template
                              v-if="row.bandwidth"
                            >
                              · {{ row.bandwidth }}M</template
                            >
                          </span>
                          <span class="tpl-chip__meta">
                            <em v-if="!row.tpl.platform_params" class="tpl-row__warn">缺平台映射</em>
                            <template v-else>已配平台映射</template>
                            <template v-if="row.options.length"> · {{ row.options.length }} 个可选参数</template>
                          </span>
                        </span>
                        <span class="tpl-chip__price">{{ row.price > 0 ? `¥${row.price}` : '未定价' }}</span>
                      </label>
                    </div>
                  </div>

                  <!--
                    多选的影响要说清楚：勾 N 个档位 = 生成 N 个 SKU，而「客户可选配置项」
                    是按**参数名跨档位合并**的（各档位开放的同名参数取并集）。
                    不说清楚的话，运营会以为参数是逐档位隔离的，看到合并后的取值列表就乱了。
                  -->
                  <t-alert v-if="selectedTemplateCount > 1" theme="info" class="tpl-multi-tip">
                    <template #message>
                      已选 <b>{{ selectedTemplateCount }}</b> 个档位 → 将生成 <b>{{ totalSkuCount }}</b> 个 SKU<template
                        v-if="totalSkuCount !== selectedTemplateCount"
                        >（含按「SKU 维度」取值组合展开的规格）</template
                      >。
                      客户可选配置项按<b>参数名跨档位合并</b>：如同一参数在多个档位都开放，客户可选取值取并集；
                      各档位的<b>基线值</b>（CPU/内存/盘）仍按各自档位下发，互不影响。
                    </template>
                  </t-alert>
                  <t-alert v-else-if="totalSkuCount > 1" theme="info" class="tpl-multi-tip">
                    <template #message>
                      已选 1 个档位，该档位含「SKU 维度」参数 → 将按取值组合生成
                      <b>{{ totalSkuCount }}</b> 个独立规格，可分别定价与设库存。
                    </template>
                  </t-alert>
                </div>
              </template>
            </template>
            <t-alert
              v-else
              theme="info"
              message="商品建好后，到详情页「规格变体 → 按规格模板生成」逐个生成；未生成 SKU 前无法上架。"
            />
          </div>
        </template>

        <!-- ══════════ 步骤 2：规格明细（仅已选档位） ══════════
             只列出被勾选的档位，逐档改 CPU/内存/盘/镜像与开放给客户的选配项。

             为什么不留在原位置（原为步骤 3、在基础信息之后）：运营的实际动作是
             "勾一个档位 → 核对这个档位配了什么"，隔着一屏基础信息就得来回滚动对照。
             紧贴档位选择之后，勾选与核对在同一屏内完成。 -->
        <div v-if="mode === 'create' && specMode === 'template' && selectedRows.length" class="form-step">
          <div class="form-step__head">
            <span class="form-step__no">2</span>
            <div class="form-step__text">
              <div class="form-step__title">规格明细</div>
              <div class="form-step__desc">
                逐档微调基线值与「客户可选配置项」。不改则按档位模板原值生成 SKU。
                可选配置项默认全部开放给客户（模板里配了几个取值就开放几个），可逐值设加价；
                用「基线值」标记客户默认拿到的那个，也可整项关闭。
              </div>
            </div>
            <span class="form-step__meta">{{ selectedRows.length }} 个档位</span>
          </div>

          <div v-for="row in selectedRows" :key="row.tpl.id" class="spec-detail">
            <div class="spec-detail__head">
              <t-tag
                :theme="kindOfRow(row) === 'nat' ? 'warning' : kindOfRow(row) === 'vps' ? 'primary' : 'default'"
                variant="light"
                size="small"
                shape="round"
                >{{ machineKindLabel(row) }}</t-tag
              >
              <span class="spec-detail__name">{{ row.tpl.name }}</span>
              <span class="spec-detail__code">
                SKU <code>{{ previewSpecCode(row) }}</code>
              </span>
              <t-tag
                v-if="axisCombinationCount(row) > 1"
                theme="primary"
                variant="light"
                size="small"
                shape="round"
              >
                SKU 维度 → {{ axisCombinationCount(row) }} 个规格
              </t-tag>
              <t-tag v-if="!row.tpl.platform_params" theme="danger" variant="light" size="small" shape="round">
                缺平台映射，无法上架
              </t-tag>
            </div>

            <t-alert
              v-if="isNatRow(row)"
              theme="info"
              class="tpl-row__notice"
              message="NAT 机型不分配独立 IP，客户通过节点转发地址（IP:端口）访问；如需客户自己配置端口映射，请确认平台侧已开启 NAT 转发。"
            />

            <div class="tpl-row__fields">
              <label class="tpl-field">CPU（核）<t-input-number v-model="row.cpu" :min="1" theme="column" /></label>
              <label class="tpl-field"
                >内存（GB）<t-input-number v-model="row.memoryGb" :min="1" theme="column"
              /></label>
              <label class="tpl-field">系统盘（GB）<t-input-number v-model="row.disk" :min="1" theme="column" /></label>
              <label class="tpl-field"
                >带宽（Mbps）<t-input-number v-model="row.bandwidth" :min="0" theme="column"
              /></label>
              <label class="tpl-field"
                >销售价（元）<t-input-number v-model="row.price" :min="0" :precision="2" theme="column"
              /></label>
            </div>

            <div class="tpl-row__fields">
              <label class="tpl-field"
                >区域 area<t-select v-model="row.area" clearable placeholder="沿用商品级默认" :options="areaOptions"
              /></label>
              <label class="tpl-field"
                >节点 node<t-select
                  v-model="row.node"
                  clearable
                  placeholder="沿用商品级默认"
                  :options="nodeOptionsFor(row.area)"
              /></label>
              <label class="tpl-field"
                >存储 store<t-select
                  v-model="row.store"
                  clearable
                  placeholder="沿用商品级默认"
                  :options="storeOptionsFor(row.area)"
              /></label>
              <label class="tpl-field">
                镜像 os（可多选，开放给客户自选）
                <!--
                  多选：平台镜像按系统家族分组（Ubuntu/Windows/CentOS），一个规格通常要开放
                  多个版本供客户选。单选只能开一个版本，客户侧看不到其他系统。
                  多选时第一个作为默认开通镜像（客户未选时用它）。
                -->
                <t-select
                  v-model="row.osList"
                  multiple
                  clearable
                  filterable
                  placeholder="沿用商品级默认 / 可多选"
                  :options="imageOptions"
                  :tag-props="{ maxWidth: '160px' }"
                  @change="onRowOsChange(row)"
                />
                <span v-if="row.osList.length > 1" class="tpl-field__hint">
                  默认开通用「{{ imageLabelOf(row.osList[0]) }}」，客户可选其余 {{ row.osList.length - 1 }} 个
                </span>
              </label>
            </div>

            <!--
              客户可选配置项：来源是档位模板的 option_selections，**模板里配了几个取值就开放几个**。

              为什么把原来的单选圆点换掉：圆点渲染出来像"客户只能选一个"的单选组，
              运营以为模板里配的 4核/8核 被砍成了一个，值也没法逐个定价。
              这里改成"取值平铺 + 逐值加价 + 基线值标记"：开放是常态（模板配置即开放），
              圆点改成的「基线值」只表示客户不选时开通哪一个，与"开放几个取值"无关。
            -->
            <div v-if="row.options.length" class="tpl-options">
              <div class="tpl-options__head">
                <span>
                  客户可选配置项
                  <em class="tpl-options__count">{{ row.options.length }} 项</em>
                </span>
                <!--
                  参数一多（模板 17 有 12 项）全部平铺展开会占满好几屏，逐项找要改的那个很费劲。
                  默认只显示一行摘要（名称 + 当前配置 + 是否加价），点「展开」才出现输入控件。
                -->
                <t-space size="small">
                  <t-link theme="primary" hover="color" @click="toggleAllOptions(row, true)">全部展开</t-link>
                  <t-link theme="default" hover="color" @click="toggleAllOptions(row, false)">全部收起</t-link>
                </t-space>
              </div>
              <div v-for="opt in row.options" :key="opt.key" class="tpl-opt" :class="{ 'tpl-opt--open': opt.open }">
                <div class="tpl-opt__head">
                  <t-checkbox v-model="opt.enabled" @change="onOptionToggle(opt)">
                    <span class="tpl-opt__name">{{ platformFieldLabel(opt.key) }}</span>
                    <code class="tpl-opt__key">{{ opt.key }}</code>
                  </t-checkbox>
                  <!-- 一行摘要：不展开也能看清这一项当前是什么状态 -->
                  <span class="tpl-opt__summary" :class="{ 'is-off': !opt.enabled }">{{ optionSummary(opt) }}</span>
                  <t-link
                    v-if="opt.enabled"
                    theme="default"
                    hover="color"
                    class="tpl-opt__toggle"
                    @click="opt.open = !opt.open"
                  >
                    {{ opt.open ? '收起' : '展开' }}
                  </t-link>
                </div>
                <span v-if="!opt.enabled" class="tpl-opt__closed">已关闭：客户不可改，固定按基线值开通</span>
                <template v-else-if="opt.open">
                  <template v-if="opt.isRange">
                    <div class="tpl-opt__value-block">
                      <div class="tpl-opt__value">
                        <label class="tpl-field tpl-field--inline">
                          范围
                          <t-input-number v-model="opt.rangeMin" :min="0" theme="column" />
                          ~
                          <t-input-number v-model="opt.rangeMax" :min="0" theme="column" />
                        </label>
                        <label class="tpl-field tpl-field--inline">
                          基线值
                          <t-input-number
                            v-model="opt.defaultNumber"
                            :min="opt.rangeMin ?? 0"
                            :max="opt.rangeMax ?? undefined"
                            theme="column"
                          />
                        </label>
                        <label class="tpl-field tpl-field--inline">
                          超出单价（元/{{ opt.unit || '单位' }}）
                          <t-input-number v-model="opt.unitPrice" :min="0" :precision="2" theme="column" />
                        </label>
                        <t-link theme="default" hover="color" @click="opt.showCycles = !opt.showCycles">
                          {{ opt.showCycles ? '收起长周期' : '长周期单价' }}
                        </t-link>
                      </div>
                      <!--
                        数量型（带宽/数据盘/系统盘/IP）的价格挂在"每单位"上，不是一个总价：
                        客户填 50G、超出基线的部分按这个单价加。留空的长周期档位
                        由后端按「月单价 × 周期月数」折算（与取值加价同一口径）。
                      -->
                      <div v-if="opt.showCycles" class="tpl-opt__cycles">
                        <label class="tpl-field tpl-field--inline">
                          季付单价
                          <t-input-number v-model="opt.unitPriceQuarterly" :min="0" :precision="2" theme="column" />
                        </label>
                        <label class="tpl-field tpl-field--inline">
                          年付单价
                          <t-input-number v-model="opt.unitPriceAnnually" :min="0" :precision="2" theme="column" />
                        </label>
                        <label class="tpl-field tpl-field--inline">
                          一次性单价
                          <t-input-number v-model="opt.unitPriceOnetime" :min="0" :precision="2" theme="column" />
                        </label>
                        <span class="tpl-field__hint">留空 = 月单价 × 周期月数（季 ×3、年 ×12）；一次性不折算</span>
                      </div>
                    </div>
                    <span v-if="optRangeError(opt)" class="tpl-opt__error">{{ optRangeError(opt) }}</span>
                  </template>
                  <div v-else class="tpl-opt__values">
                    <!-- 每个取值一块（取值行 + 可展开的长周期加价）：
                         长周期输入必须与它所属的取值同块，否则 v-for 的作用域取不到 v。 -->
                    <div v-for="v in opt.values" :key="v.value" class="tpl-opt__value-block">
                      <div class="tpl-opt__value">
                        <t-tag v-if="v.isDefault" theme="primary" variant="light" size="small" shape="round">
                          基线值
                        </t-tag>
                        <t-link v-else theme="default" hover="color" @click="setDefaultValue(opt, v.value)"
                          >设为基线</t-link
                        >
                        <span class="tpl-opt__label"
                          >{{ v.group ? v.group + ' / ' : '' }}{{ catalogValueLabel(opt.key, v.value) }}</span
                        >
                        <t-input-number
                          v-model="v.price"
                          :min="0"
                          :precision="2"
                          theme="column"
                          class="tpl-opt__price"
                        />
                        <span class="tpl-opt__unit">元/月</span>
                        <!--
                          长周期加价：留空即按「月价 × 周期月数」自动折算（后端 deriveMissingCyclePrices）。
                          默认收起（用户反馈"拥挤"）：多数商品只填月价就够，
                          三列铺开会让每行很长。
                        -->
                        <t-link theme="default" hover="color" @click="v.showCycles = !v.showCycles">
                          {{ v.showCycles ? '收起长周期' : '长周期加价' }}
                        </t-link>
                      </div>
                      <div v-if="v.showCycles" class="tpl-opt__cycles">
                        <label class="tpl-field tpl-field--inline">
                          季付
                          <t-input-number v-model="v.priceQuarterly" :min="0" :precision="2" theme="column" />
                        </label>
                        <label class="tpl-field tpl-field--inline">
                          年付
                          <t-input-number v-model="v.priceAnnually" :min="0" :precision="2" theme="column" />
                        </label>
                        <label class="tpl-field tpl-field--inline">
                          一次性
                          <t-input-number v-model="v.priceOnetime" :min="0" :precision="2" theme="column" />
                        </label>
                        <span class="tpl-field__hint">
                          留空 = 月价 × 周期月数（季付 ×3、年付 ×12）；一次性不折算，留空即按月度
                        </span>
                      </div>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div>
        </div>

        <!-- ══════════ 步骤 2：基础信息 ══════════ -->
        <div class="form-step">
          <div class="form-step__head">
            <span class="form-step__no">{{ mode === 'create' ? 3 : 1 }}</span>
            <div class="form-step__text">
              <div class="form-step__title">基础信息</div>
              <div class="form-step__desc">
                <template v-if="mode === 'create'">
                  选好档位后名称与售价会自动带入，可改。SKU 编码、分类、平台渠道、封面与描述需自己填。
                </template>
                <template v-else>商品本体信息；规格在详情页维护。</template>
              </div>
            </div>
          </div>
          <div class="form-grid">
            <t-form-item label="产品名称" name="name" :rules="[{ required: true, message: '产品名称不能为空' }]">
              <t-input v-model="form.name" placeholder="请输入产品名称" clearable />
            </t-form-item>
            <t-form-item
              label="SKU 编码"
              name="code"
              :rules="[{ required: mode === 'create', message: 'SKU 编码不能为空' }]"
            >
              <t-input
                v-model="form.code"
                placeholder="留空将按产品名自动生成"
                :disabled="mode === 'edit'"
                clearable
                @change="codeTouched = true"
              >
                <template v-if="mode === 'create'" #suffix>
                  <t-link theme="primary" hover="color" @click="regenerateCode">重新生成</t-link>
                </template>
              </t-input>
              <span class="form-hint">系统与上游用的标识，按产品名自动生成，可手工改写；创建后不可更改。</span>
            </t-form-item>
            <t-form-item
              label="分类"
              name="category_id"
              :rules="[{ required: true, message: '请选择商品分类' }]"
            >
              <!-- 分类必填：留空会让商品落进「未分类」——列表显示不出归属、导航也统计不到。 -->
              <t-select v-model="form.category_id" placeholder="请选择分类" :options="categoryOptions" />
            </t-form-item>
            <t-form-item label="产品类型" name="product_type">
              <t-select v-model="form.product_type" clearable placeholder="请选择类型" :options="productTypeOptions" />
            </t-form-item>
            <t-form-item label="平台渠道" name="source_provider_id" :rules="platformRules">
              <t-select
                v-model="form.source_provider_id"
                placeholder="请选择自营平台渠道（魔方云等）"
                :options="platformOptions"
                :loading="platformLoading"
                clearable
                @change="onPlatformChange"
              />
              <span class="form-hint">云主机必须绑定一个「算力平台」渠道，开通/暂停/销毁将下发到该平台。</span>
            </t-form-item>
            <!--
            可售周期（建品）：价格模型的替代。

            用户要求：「价格模型」只有固定价/按月两个选项，与真正的计费周期（月/小时/半年/年）
            对不上；应该直接选周期。这里改成多选，勾选的周期在创建后写入价格矩阵
            （product_prices.status=1），客户下单时看到的周期选项就是这几个。

            为什么不再让运营单填「价格模型」：它是 products.price_model 这个存量兼容列，
            只认 fixed/hourly/monthly，真正决定客户能买什么周期的权威数据是价格矩阵。
            继续让运营在两个字段里各说一半，正是"周期与价格模型对不上"的来源。
            提交时由 derivePriceModel 从勾选周期反推一个合理的兼容值（见脚本注释）。
          -->
            <t-form-item v-if="mode === 'create'" label="可售周期" class="form-grid__full">
              <div class="cycle-pick">
                <t-checkbox-group v-model="sellableCycles">
                  <t-checkbox v-for="c in sellableCycleOptions" :key="c.value" :value="c.value">{{
                    c.label
                  }}</t-checkbox>
                </t-checkbox-group>
                <span class="form-hint">
                  勾选的周期会写入价格矩阵并开放给客户下单，至少选一个。按小时/按天适合做试用型商品；
                  月付及以上的周期用下方「周期价格」按优惠月数一键算出。
                </span>
              </div>
            </t-form-item>
            <!-- 编辑既有商品时保留价格模型：它只是无周期订单的兜底值，周期价在商品详情页的价格矩阵里改。 -->
            <t-form-item v-else label="价格模型" name="price_model">
              <t-select v-model="form.price_model" placeholder="请选择价格模型" :options="priceModelOptions" />
              <span class="form-hint">无显式周期时的兜底计费方式；可售周期与各周期价格在商品详情页维护。</span>
            </t-form-item>
            <t-form-item label="库存" name="stock">
              <t-input-number v-model="form.stock" :min="-1" theme="column" placeholder="-1 表示不限" />
            </t-form-item>
            <t-form-item label="销售价（元）" name="price">
              <t-input-number v-model="form.price" :min="0" :precision="2" theme="column" placeholder="请输入销售价" />
              <span class="form-hint">{{ priceHint }}</span>
            </t-form-item>
            <t-form-item label="成本价（元）" name="cost_price">
              <t-input-number
                v-model="form.cost_price"
                :min="0"
                :precision="2"
                theme="column"
                placeholder="请输入成本价"
              />
            </t-form-item>

            <!--
            周期价格：只对「可售周期」里勾选的周期生成，逐周期可勾选是否参与一键计算。

            用户要求：有的周期要一起算（月/季/半年/年按优惠月数推导），有的要单独设
            （按小时常用于试用商品，没有"月数×价"的口径）。原来四个优惠月数输入框
            对所有周期一律生效，按小时也只能跟着月付价走，算不出试用价。
            现在：每个勾选的周期一行，勾上「一键计算」的参与推导，没勾的自己填价，
            「一键生成」不会覆盖手动定价的行。

            口径：周期价 = 月付价 × 月数 − 优惠月数 × 月付价。优惠月数上限为该周期
            月数 − 1，至少留一个月不优惠，避免生成 0 元周期价（0 元价上架后客户真能 0 元买）。
          -->
            <t-form-item v-if="mode === 'create'" label="周期价格" class="form-grid__full">
              <div class="price-gen">
                <p v-if="!cyclePlans.length" class="price-gen__empty">请先在上方勾选可售周期，这里再逐周期定价。</p>
                <template v-else>
                  <div class="price-gen__row">
                    <span class="price-gen__tip">
                      「月付价」是基准价；勾了「一键计算」的周期按它推导，未勾的由你手填（试用价这类）。
                    </span>
                    <t-button size="small" theme="primary" variant="outline" @click="generateCyclePrices">
                      一键生成
                    </t-button>
                    <t-link theme="default" hover="color" @click="resetCyclePlans">重置</t-link>
                  </div>
                  <div class="price-gen__plans">
                    <div v-for="plan in cyclePlans" :key="plan.cycle" class="price-gen__plan">
                      <label class="price-gen__plan-head">
                        <t-checkbox
                          :model-value="plan.auto"
                          :disabled="!isAutoEligible(plan)"
                          @change="(v: boolean) => setPlanAuto(plan, v)"
                        />
                        <span class="price-gen__plan-name">{{ plan.label }}</span>
                      </label>
                      <label v-if="plan.auto && plan.monthCount > 1" class="price-gen__field price-gen__field--sm">
                        优惠（月）
                        <t-input-number
                          :model-value="plan.offMonths"
                          :min="0"
                          :max="Math.max(0, plan.monthCount - 1)"
                          theme="column"
                          @change="(v: number | string | undefined) => setPlanOff(plan, Number(v ?? 0))"
                        />
                      </label>
                      <label class="price-gen__field price-gen__field--sm">
                        {{ plan.cycle === 'monthly' ? '月付价' : plan.auto ? '生成价' : '自定价' }}（元）
                        <t-input-number
                          :model-value="plan.amount"
                          :min="0"
                          :precision="2"
                          theme="column"
                          @change="(v: number | string | undefined) => setPlanAmount(plan, Number(v ?? 0))"
                        />
                      </label>
                      <span class="price-gen__plan-hint">{{ cyclePlanHint(plan) }}</span>
                    </div>
                  </div>
                  <p class="price-gen__hint">
                    周期价只作用于该商品的默认规格；要按 SKU 分别定价，请在商品详情页的「周期价格」里维护。
                    未勾选「一键计算」的周期由你手工定价，生成时不会被覆盖。
                  </p>
                </template>
              </div>
            </t-form-item>
            <t-form-item label="排序" name="sort_order">
              <t-input-number v-model="form.sort_order" :min="0" theme="column" placeholder="数值越小越靠前" />
            </t-form-item>
            <t-form-item label="状态" name="status">
              <t-select v-model="form.status" placeholder="请选择状态" :options="productStatusOptions" />
            </t-form-item>
          </div>

          <t-form-item label="封面图 URL" name="cover_image">
            <t-input
              v-model="form.cover_image"
              placeholder="选填，官网产品卡与详情页展示用，如 /branding/logo.svg"
              clearable
            />
          </t-form-item>

          <div v-if="form.cover_image" class="cover-preview">
            <span class="cover-preview__label">封面预览</span>
            <img class="cover-preview__img" :src="form.cover_image" alt="封面预览" />
          </div>

          <t-form-item label="产品描述" name="description">
            <t-textarea
              v-model="form.description"
              :autosize="{ minRows: 2, maxRows: 5 }"
              placeholder="选填，产品简介"
            />
          </t-form-item>
        </div>
        <!-- ══════════ 步骤 4：平台默认参数（可折叠） ══════════
             商品级默认值：档位没显式指定时用它。多数情况下档位模板已带全参数，
             这里保持折叠，避免与档位配置重复造成"到底填哪个"的困惑。 -->
        <div v-if="!isUpstream" class="form-step">
          <div class="form-step__head form-step__head--toggle" @click="platformSectionOpen = !platformSectionOpen">
            <span class="form-step__no">{{ mode === 'create' ? 4 : 2 }}</span>
            <div class="form-step__text">
              <div class="form-step__title">
                平台默认参数
                <t-tag theme="default" variant="light" size="small" shape="round">选填</t-tag>
              </div>
              <div class="form-step__desc">
                商品级默认值，仅当档位未指定对应项时生效（档位自身的配置优先级更高）。
                <template v-if="!platformSectionOpen">需要时展开。</template>
              </div>
            </div>
            <t-link theme="primary" hover="color">{{ platformSectionOpen ? '收起' : '展开' }}</t-link>
          </div>

          <template v-if="platformSectionOpen">
            <t-alert
              theme="info"
              message="此处是商品级默认参数；「规格明细」里每个档位的绑定优先级更高。选好平台渠道后可下拉选取平台上真实存在的区域/节点/存储/镜像。"
            />
            <div class="form-grid">
              <t-form-item label="区域 area">
                <t-select
                  v-model="platformParams.area"
                  clearable
                  placeholder="平台区域"
                  :options="areaOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
              <t-form-item label="节点 node">
                <t-select
                  v-model="platformParams.node"
                  clearable
                  placeholder="平台节点"
                  :options="nodeOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
              <t-form-item label="存储 store">
                <t-select
                  v-model="platformParams.store"
                  clearable
                  placeholder="系统盘所在存储（可选）"
                  :options="storeOptions"
                  :disabled="!form.source_provider_id"
                />
              </t-form-item>
              <t-form-item label="镜像 os">
                <t-select
                  v-model="platformParams.os"
                  clearable
                  filterable
                  placeholder="平台镜像（默认开通用）"
                  :options="imageOptions"
                  :disabled="!form.source_provider_id"
                />
                <span class="form-hint"
                  >此处是默认镜像；要让客户在多个镜像间自选，在下面规格行的「镜像 os」里多选。</span
                >
              </t-form-item>
            </div>

            <!--
          平台配置项默认值：以前这些默认值只存在于平台文档里，运营看不到就容易忽略，
          或重复手填与平台默认冲突。显式列出来，"不填会是什么"一眼可见。
        -->
            <template v-if="catalogDefaults.length">
              <div class="defaults-hint">平台默认取值（未显式设置时按这些值开通）</div>
              <div class="defaults-list">
                <div v-for="d in catalogDefaults" :key="d.key" class="defaults-item">
                  <span class="defaults-item__label">{{ d.label }}</span>
                  <code class="defaults-item__value">{{ d.value }}</code>
                  <span v-if="d.help" class="defaults-item__help">{{ d.help }}</span>
                </div>
              </div>
            </template>
            <t-form-item label="其他平台参数（高级，JSON）">
              <t-textarea
                v-model="extraParamsText"
                :autosize="{ minRows: 2, maxRows: 6 }"
                placeholder='上面四个下拉之外的键，如 {"network_type":"normal","ip_num":1,"traffic_quota":0}'
              />
            </t-form-item>
          </template>
        </div>

        <div v-if="!embedded" class="form-footer">
          <t-button variant="outline" @click="handleCancel">取消</t-button>
          <t-button theme="primary" :loading="submitting" @click="handleSubmit">{{
            mode === 'create' ? '创建' : '保存'
          }}</t-button>
        </div>
      </t-form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'

import { AddIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getProviderList, getProviderPlatformResources } from '@/api/admin'
import { getSpecTemplateList, getOptionCatalog, getProductList } from '@/api/product'
import {
  billingCycleLabel,
  platformFieldLabel,
  priceModelOptions,
  productStatusOptions,
  productTypeOptions,
} from '@/pages/product/constants'
import { useCategoryOptions } from '@/composables/useCategoryOptions'
import type {
  PlatformResourceItem,
  ProviderInfo,
  SaleProductCreateRequest,
  SaleProductUpdateRequest,
  SaleProductInfo,
  SaleProductOptionOverride,
  SaleProductSpecTemplateSelection,
  SpecTemplateInfo,
} from '@/types/interface'

const props = defineProps<{
  mode: 'create' | 'edit'
  initial?: SaleProductInfo | null
  /** 提交中由父页（真正发请求的一侧）持有，避免子组件 emit 后立即复位 loading。 */
  submitting?: boolean
  /**
   * 内嵌到商品详情页（合并后的单页形态）：
   *   - 隐藏独立页的表单外壳（页头/卡片底色/底部按钮）——详情页有自己的页头与
   *     页级「保存」，再套一层外壳会出现"卡片里套卡片、两个保存按钮"；
   *   - 本组件此时只渲染「基础信息」这一段：价格由详情页的「价格」子页承载
   *     （见 up 链路的 UpstreamPriceEditor / 自营的周期价格矩阵）。
   *
   * 为什么价格不留在本组件里：合并后「价格」是详情页的一级子页，与基础信息**平级**；
   * 若仍由本组件内的页签托管，就会出现「一级页签 → 二级页签」两层导航，
   * 运营分不清自己停在那一层。拆开后每段只有一个归属。
   */
  embedded?: boolean
}>()

const emit = defineEmits<{
  // cyclePrices：周期价格矩阵（月付/季付/半年付/年付 → 金额）。创建接口不收，
  // 由父页在拿到商品 id 后写价格矩阵接口。
  (e: 'submit', payload: SaleProductCreateRequest, cyclePrices?: Record<string, number>): void
  (e: 'cancel'): void
}>()

/** 代理（上游转售）商品：链路由导入决定，表单不提供链路选择。 */
const isUpstream = computed(() => props.initial?.source_mode === 'upstream')

/**
 * 上游提供商展示名：详情里只有 source_provider_id 时界面只显示一个裸 ID，
 * 运营无法判断这条链路接到哪家渠道。这里用提供商列表把它解析成「名称（类型） · #ID」。
 * 解析不到时回落为 ID 文本，不让界面空着。
 */
const providerNames = ref<Record<number, string>>({})

const providerDisplay = computed(() => {
  const id = props.initial?.source_provider_id
  if (!id) return '—'
  const name = providerNames.value[id]
  return name ? `${name} · #${id}` : `#${id}`
})

/**
 * 上游资源商品展示：只有 source_product_id 时同样是个裸 ID。
 * 商品名在我们自己库里没有别名（上游商品镜像不通过这个组件加载），
 * 因此展示为「#ID」，并在提示里说明去「上游商品目录」核对。
 */
const upstreamProductDisplay = computed(() => {
  const id = props.initial?.source_product_id
  return id ? `#${id}` : '—'
})

/** 库存提示：上游转售商品的库存是上游限量余量，随同步刷新；本地改动会被同步覆盖。 */
const stockHint = computed(() => {
  if (!isUpstream.value) return '-1 表示不限量；填 0 表示暂不可售。'
  return '上游限量商品的可售余量为权威值，会随商品目录同步刷新覆盖本地填写；-1 表示不限量。'
})

const pageTitle = computed(() => (props.mode === 'create' ? '新建自营产品' : '编辑产品'))
const headerDesc = computed(() =>
  props.mode === 'create'
    ? '自营商品：选平台渠道 + 勾规格模板，创建后即为可上架商品（规格与平台映射一步到位）。'
    : '自营商品：链路固定为自营，平台参数可在此调整，规格在详情页维护。',
)

const { categoryOptions, loadCategories } = useCategoryOptions()

// ===== 平台渠道与平台资源目录 =====
const platformOptions = ref<{ label: string; value: number }[]>([])
const platformLoading = ref(false)

async function loadPlatformOptions() {
  platformLoading.value = true
  try {
    const data = await getProviderList({ page_size: 100 })
    platformOptions.value = data.items
      .filter((item: ProviderInfo) => item.kind === 'compute')
      .map((item: ProviderInfo) => ({ label: `${item.name}（${item.provider_type}）`, value: item.id }))
  } catch {
    platformOptions.value = []
  } finally {
    platformLoading.value = false
  }
}

const resources = ref<{
  areas: PlatformResourceItem[]
  nodes: PlatformResourceItem[]
  stores: PlatformResourceItem[]
  images: PlatformResourceItem[]
}>({ areas: [], nodes: [], stores: [], images: [] })

/** 只保留启用项：平台返回 offline 的取值不该再被选进新规格。 */
function toOptions(items: PlatformResourceItem[]): { label: string; value: string }[] {
  return items
    .filter((item) => !item.status || item.status === 'active')
    .map((item) => ({ label: `${item.label}（${item.value}）`, value: item.value }))
}

const areaOptions = computed(() => toOptions(resources.value.areas))
const imageOptions = computed(() => toOptions(resources.value.images))
// 节点/存储挂在区域下：给行级过滤用，行未选区域时用商品级区域的过滤结果。
function filterByArea(items: PlatformResourceItem[], area?: string) {
  if (!area) return items
  return items.filter((item) => !item.parent_id || item.parent_id === area)
}
function nodeOptionsFor(area?: string) {
  return toOptions(filterByArea(resources.value.nodes, area || platformParams.area))
}
function storeOptionsFor(area?: string) {
  return toOptions(filterByArea(resources.value.stores, area || platformParams.area))
}
const nodeOptions = computed(() => nodeOptionsFor())
const storeOptions = computed(() => storeOptionsFor())

async function loadPlatformResources(providerId?: number) {
  const id = Number(providerId)
  resources.value = { areas: [], nodes: [], stores: [], images: [] }
  if (!id) return
  try {
    resources.value = await getProviderPlatformResources(id)
  } catch (error) {
    MessagePlugin.warning((error as Error).message || '该渠道未提供平台资源目录，可用「其他平台参数」手工填写')
  }
}

function onPlatformChange(value?: number | string) {
  const id = Number(value)
  // 换渠道后旧参数多半对新平台无效：清空下拉选择，避免跨平台脏值。
  Object.assign(platformParams, { area: undefined, node: undefined, store: undefined, os: undefined })
  void loadPlatformResources(id)
  // 拉该渠道的配置项目录，把静态默认值列出来（"不填会是什么"一眼可见，只读不写入）。
  const hit = platformOptions.value.find((o) => o.value === id)
  const type = hit?.label.match(/（([^）]+)）/)?.[1] || ''
  void loadCatalogDefaults(type)
  // 先勾档位、后选渠道时补上平台前缀：命名规则不该取决于两个动作的先后。
  // 只补"自动生成且还没有平台前缀"的名字，手填过的名字不动。
  if (props.mode === 'create') {
    const matched = form.name.trim().match(/^(\d+)核(\d+)G$/)
    if (matched) {
      const platformName = platformDisplayName(id)
      if (platformName) form.name = `${platformName} ${matched[0]}`
    }
  }
}

// ===== 商品级平台参数（后端字段 config_options，这里用下拉 + 高级 JSON 组合编辑）=====
const platformParams = reactive<{ area?: string; node?: string; store?: string; os?: string }>({
  area: undefined,
  node: undefined,
  store: undefined,
  os: undefined,
})
const extraParamsText = ref('')
const PLATFORM_KEYS = ['area', 'node', 'store', 'os'] as const

/**
 * 平台配置项默认值（选渠道后从平台配置项目录读回）。
 *
 * 为什么要把默认值显式列出来：默认值以前只存在于平台文档里，运营看不到，
 * 于是要么忽略（以为没配）、要么手工再填一遍（可能与平台默认冲突）。
 *
 * 只读展示、不自动写入 config_options —— 平台默认值里有 system_disk_size=50
 * 这类会与 SKU 自身规格冲突的项（档位是 40G 系统盘时，写成 50 就把档位覆盖了）。
 * 默认值的意义是"不填就是它"，本来就该由平台在未传参时生效，我方不该主动替它写死。
 *
 * 只列「静态默认」的项：资源类（区域/节点/存储/镜像）默认值为空，列出来没意义。
 */
const catalogDefaults = ref<{ key: string; label: string; value: string; help?: string }[]>([])

/**
 * 平台取值的展示名（值 → 中文/友好名），如 cpu 的 "8" → "8核"、memory 的 "8192" → "8G"。
 *
 * 为什么需要：建品页的客户可选配置项直接渲染 option_selections 里的裸取值，
 * 运营看到的是 "6144"、"8192" 这样的数字，既认不出是内存的 6G/8G，
 * 也无法与模板里勾选的取值对照。取值名是平台的权威口径，从目录读一次即可。
 */
const catalogValueLabels = ref<Record<string, Record<string, string>>>({})

/** 目录里的单位（option_key → GB/Mbps/个）：范围型标注"元/单位"要用它。 */
const catalogUnits = ref<Record<string, string>>({})

/** 展示名；目录没有该项时回落取值本身（界面永远有字可显示）。 */
function catalogValueLabel(optionKey: string, value: string): string {
  return catalogValueLabels.value[optionKey]?.[value] || value
}

/**
 * 目录里的展示名，**查不到返回空串**（不回落取值）。
 *
 * 与 catalogValueLabel 的区别是"查不到"的处理：界面要回落取值免得空着，
 * 但提交时必须留空 —— 后端只在 label 非空时才覆盖目录名，
 * 传裸值（"62"）反而会把后端的「CentOS / CentOS-7.9」盖掉。
 */
function catalogLabelOrEmpty(optionKey: string, value: string): string {
  return catalogValueLabels.value[optionKey]?.[value] || ''
}

/** 目录里的单位（GB / Mbps / 个）；查不到返回空串，由界面回落"单位"二字。 */
function catalogUnit(optionKey: string): string {
  return catalogUnits.value[optionKey] || ''
}

async function loadCatalogDefaults(providerType: string) {
  catalogDefaults.value = []
  catalogValueLabels.value = {}
  if (!providerType) return
  try {
    const list = await getOptionCatalog({ provider_type: providerType })
    catalogDefaults.value = list
      .filter((item) => {
        if (!item.default_value) return false
        // 资源类默认值是"未配置"的占位，不该当默认值展示。
        return !['areas', 'nodes', 'stores', 'images'].includes(item.value_source || '')
      })
      .map((item) => ({
        key: item.option_key,
        label: item.label || platformFieldLabel(item.option_key),
        value: item.default_value,
        help: item.options_help?.[item.default_value] || '',
      }))
    const labels: Record<string, Record<string, string>> = {}
    for (const item of list) {
      for (const v of item.values || []) {
        if (!v?.value) continue
        if (!labels[item.option_key]) labels[item.option_key] = {}
        // 目录标签优先；平台没给标签时保持原值（不隐藏信息）。
        if (v.label) labels[item.option_key][v.value] = v.label
      }
    }
    catalogValueLabels.value = labels
    const units: Record<string, string> = {}
    for (const item of list) {
      if (item.option_key && item.unit) units[item.option_key] = item.unit
    }
    catalogUnits.value = units
  } catch {
    // 目录读不到不影响建品：默认值为辅助信息，非必需。
    catalogDefaults.value = []
    catalogValueLabels.value = {}
    catalogUnits.value = {}
  }
}

/** 把 config_options JSON 拆成「四个下拉 + 高级 JSON」；未知键留在高级里不丢。 */
function splitConfigOptions(raw: string) {
  let parsed: Record<string, unknown> = {}
  if (raw && raw.trim()) {
    try {
      const obj = JSON.parse(raw)
      if (obj && typeof obj === 'object' && !Array.isArray(obj)) parsed = obj as Record<string, unknown>
    } catch {
      // 非法 JSON 原样留在高级文本框里，保存时由后端校验报错，不静默丢弃运营输入
      extraParamsText.value = raw
      Object.assign(platformParams, { area: undefined, node: undefined, store: undefined, os: undefined })
      return
    }
  }
  const rest: Record<string, unknown> = { ...parsed }
  for (const key of PLATFORM_KEYS) {
    const v = parsed[key]
    platformParams[key] = v == null || v === '' ? undefined : String(v)
    delete rest[key]
  }
  extraParamsText.value = Object.keys(rest).length ? JSON.stringify(rest, null, 2) : ''
}

/** 把下拉与高级 JSON 合并回 config_options 字符串。 */
function composeConfigOptions(): string | null {
  let extra: Record<string, unknown> = {}
  if (extraParamsText.value.trim()) {
    try {
      const obj = JSON.parse(extraParamsText.value)
      if (!obj || typeof obj !== 'object' || Array.isArray(obj)) {
        MessagePlugin.warning('「其他平台参数」必须是 JSON 对象')
        return null
      }
      extra = obj as Record<string, unknown>
    } catch {
      MessagePlugin.warning('「其他平台参数」JSON 格式非法')
      return null
    }
  }
  const merged: Record<string, unknown> = { ...extra }
  for (const key of PLATFORM_KEYS) {
    if (platformParams[key]) merged[key] = platformParams[key]
  }
  return Object.keys(merged).length ? JSON.stringify(merged) : ''
}

// ===== 规格模板选择（建品即生成 SKU + 客户可选配置项）=====
/** 客户可选配置项：档位勾选的某个参数，及其取值与加价。 */
type OptionRow = {
  key: string
  /** 是否开放给客户选；不开放则只用基线值（默认取第一个取值）。 */
  enabled: boolean
  /** 数量型（带宽/盘/IP 数）用区间表达，没有离散取值。 */
  isRange: boolean
  rangeMin: number
  rangeMax: number
  defaultNumber: number | null
  /**
   * 数量型的「超出单价（元/单位）」四档周期价。
   *
   * 价格挂在每单位上而不是总价上：客户填 50G，超出基线的部分按这个单价加。
   * 留空（null）的长周期档位由后端按「月单价 × 周期月数」折算。
   */
  unitPrice: number
  unitPriceQuarterly: number | null
  unitPriceAnnually: number | null
  unitPriceOnetime: number | null
  /** 单位（GB / Mbps / 个），来自平台目录；用于"元/单位"的文案与提示。 */
  unit: string
  /** 纯界面状态：是否展开该项的编辑控件（参数多时默认收起，只显示摘要）。 */
  open?: boolean
  /** 纯界面状态：是否展开长周期单价（不参与提交）。 */
  showCycles?: boolean
  /**
   * 取值列表。四档加价分开存：
   * 上游/平台按周期各给一个价（内存 8G 月付 13、年付 156），
   * 只填"元/月"一个数会让年付订单只加一个月的钱（用户反馈的实测故障：
   * 换周期后加价不变，年付的 8G 内存仍只加 13 元）。
   */
  values: {
    value: string
    label: string
    group: string
    /** 月度加价（元）；下面三档留空表示"按月度 × 周期月数"自动折算。 */
    price: number
    priceQuarterly: number | null
    priceAnnually: number | null
    priceOnetime: number | null
    isDefault: boolean
    /** 纯界面状态：是否展开长周期加价输入（不参与提交）。 */
    showCycles?: boolean
  }[]
}

type TemplateRow = {
  tpl: SpecTemplateInfo
  selected: boolean
  /**
   * 机型（doc122）：'vps' 独立 IP / 'nat' NAT 机型 / '' 未标注（存量档位）。
   * 决定这台机器客户怎么访问：独立 IP 直连，NAT 经节点转发地址。
   */
  machineKind: string
  cpu: number
  memoryGb: number
  disk: number
  bandwidth: number
  price: number
  costPrice: number
  area?: string
  node?: string
  store?: string
  os?: string
  /**
   * osList 该规格开放给客户的镜像（多选）：第一个是默认开通镜像，
   * 其余作为客户可选值下发。单元素时等同于普通单值。
   */
  osList: string[]
  /** 客户可选配置项（由档位 option_selections 展开，可改加价与开放与否）。 */
  options: OptionRow[]
  /** 模板里 CPU/内存/磁盘/带宽之外的原子取值（如网络流量），原样保留不丢 */
  extraSpecValues: Record<string, unknown>
  /** 模板里 area/node/store/os 之外的平台参数（含 ip_num/nat_*，即机型参数），原样保留不丢 */
  extraPlatformParams: Record<string, unknown>
}

const specMode = ref<'template' | 'later'>('template')
const templateRows = ref<TemplateRow[]>([])
const selectedTemplateCount = computed(() => templateRows.value.filter((r) => r.selected).length)

/** 已选档位：步骤 3「规格明细」只渲染这些，未勾选的不占版面。 */
const selectedRows = computed(() => templateRows.value.filter((r) => r.selected))

/**
 * axisCombinationCount 单个档位按「SKU 维度」展开会生成几个规格。
 *
 * 口径与后端 buildAxisPlan 一致：只数标记了 sku_axis 的离散参数，取它们取值数的乘积。
 * 轴参数不在这里的 options 里（buildOptionRows 已把它们排除），所以直接读模板声明。
 */
function axisCombinationCount(row: TemplateRow): number {
  const sel = (row.tpl.option_selections || {}) as Record<
    string,
    { values?: string[]; range?: number[]; sku_axis?: boolean }
  >
  let count = 1
  let hasAxis = false
  for (const cfg of Object.values(sel)) {
    if (!cfg?.sku_axis || Array.isArray(cfg.range)) continue
    const n = (cfg.values || []).length
    if (n <= 0) continue
    hasAxis = true
    count *= n
  }
  return hasAxis ? count : 1
}

/** 本次建品将生成的 SKU 总数：各已选档位的正交组合数之和。 */
const totalSkuCount = computed(() =>
  selectedRows.value.reduce((acc, row) => acc + axisCombinationCount(row), 0),
)

/**
 * 平台默认参数区默认折叠：多数档位模板已带全参数，展开反而造成"填哪个"的困惑。 */
const platformSectionOpen = ref(false)

/**
 * 销售价提示：随当前档位选择变化，讲清"这个价到底管不管用"。
 *
 * 原提示恒为「规格模板自带参考售价时以模板为准」，没勾档位时读起来莫名其妙。
 * 按当前状态分情况说明，运营才知道要不要在这里填价。
 */
const priceHint = computed(() => {
  if (props.mode !== 'create') return '规格在详情页维护；此价用于「仅本地/虚拟商品」。'
  if (specMode.value !== 'template') return '未按档位配置时，商品售价以此为准。'
  const count = selectedTemplateCount.value
  if (count === 0) return '尚未勾选档位：勾选后会把档位参考价自动带入，也可现在手填。'
  if (count === 1) return '该价作为商品展示价；单个档位的 SKU 售价在「规格明细」里可微调。'
  return '已选多个档位：此价是商品展示价（通常取最低档），各档位实际售价在「规格明细」分别设置。'
})

/** 档位机型短标签（步骤 3 的档位标题用）。 */
function machineKindLabel(row: TemplateRow): string {
  const kind = kindOfRow(row)
  if (kind === 'nat') return 'NAT'
  if (kind === 'vps') return '独立 IP'
  return '未标注'
}

/** 机型筛选（doc122）：两边档位访问方式不同，混在一起选容易配错，默认看全部。 */
const machineKindFilter = ref<'' | 'vps' | 'nat' | 'unset'>('')
const machineKindFilterOptions = [
  { label: '全部机型', value: '' },
  { label: '独立 IP', value: 'vps' },
  { label: 'NAT 机型', value: 'nat' },
  { label: '未标注', value: 'unset' },
]

/** kindOfRow 归一档位机型：未标注归到 'unset' 便于筛选与分组。 */
function kindOfRow(row: TemplateRow): 'vps' | 'nat' | 'unset' {
  if (row.machineKind === 'vps') return 'vps'
  if (row.machineKind === 'nat') return 'nat'
  return 'unset'
}

const visibleTemplateRows = computed(() =>
  machineKindFilter.value
    ? templateRows.value.filter((r) => kindOfRow(r) === machineKindFilter.value)
    : templateRows.value,
)

/**
 * machineKindGroups 按机型分组渲染模板列表。
 * 分组是本次改造的主要目的：NAT 档位与独立 IP 档位的访问方式完全不同，
 * 混在一个平铺列表里运营容易把 NAT 参数配进 VPS 定位的商品。
 */
const machineKindGroups = computed(() => {
  const order: { kind: 'vps' | 'nat' | 'unset'; label: string; rows: TemplateRow[] }[] = [
    { kind: 'vps', label: '独立 IP 机型', rows: [] },
    { kind: 'nat', label: 'NAT 机型', rows: [] },
    { kind: 'unset', label: '未标注机型', rows: [] },
  ]
  for (const row of visibleTemplateRows.value) {
    const g = order.find((x) => x.kind === kindOfRow(row))
    if (g) g.rows.push(row)
  }
  return order.filter((g) => g.rows.length)
})

/** 已选档位里出现的机型集合：混选时给出提示（不阻断）。 */
const selectedKinds = computed(() => {
  const kinds = new Set<string>()
  for (const row of templateRows.value) {
    if (row.selected) kinds.add(kindOfRow(row))
  }
  return kinds
})
const hasMixedKinds = computed(() => selectedKinds.value.has('vps') && selectedKinds.value.has('nat'))

/** 分组标题下的说明，讲清这一组档位的机器客户怎么访问。 */
function machineKindGroupDesc(kind: string): string {
  if (kind === 'nat') return '不分配独立 IP，客户经节点转发地址（IP:端口）访问'
  if (kind === 'vps') return '分配独立公网 IP，客户直连'
  return '存量档位未标注机型，按平台参数取值决定开通方式'
}

/** 档位是否走 NAT 链路：ip_num 固定 0，开放为选配项会覆盖基线。 */
function isNatRow(row: TemplateRow): boolean {
  return kindOfRow(row) === 'nat'
}

/**
 * machineKindError 机型与平台参数的互斥校验，规则与后端 normalizeAndValidateMachineKind 一致。
 * 未标注机型（''）跳过：存量档位没有这个维度，不能因为新增校验就让它们改不动。
 */
function machineKindError(row: TemplateRow, params: Record<string, unknown>): string {
  const kind = row.machineKind
  if (!kind) return ''
  const numOrNull = (v: unknown): number | null => {
    if (v == null || String(v).trim() === '') return null
    const n = Number(v)
    return Number.isFinite(n) ? n : null
  }
  const ipNum = numOrNull(params.ip_num)
  const acl = numOrNull(params.nat_acl_limit)
  const web = numOrNull(params.nat_web_limit)

  if (kind === 'nat') {
    if (ipNum == null) return 'NAT 机型需在模板里设置 IP 数量 ip_num = 0（不分配独立 IP）'
    if (ipNum !== 0) return 'NAT 机型的 IP 数量 ip_num 必须为 0，请到规格模板里修正'
    if ((acl == null || acl < 0) && (web == null || web < 0)) {
      return 'NAT 机型至少需开通一种访问入口（NAT 端口转发或共享建站 >= 0），否则客户将无法连接'
    }
    return ''
  }
  if (ipNum != null && ipNum < 1) return '独立 IP 机型的 IP 数量 ip_num 必须 >= 1'
  if (acl != null && acl >= 0) return '独立 IP 机型的 nat_acl_limit 必须为 -1（与独立 IP 互斥）'
  if (web != null && web >= 0) return '独立 IP 机型的 nat_web_limit 必须为 -1（与独立 IP 互斥）'
  return ''
}

/**
 * 由档位 option_selections 展开客户可选配置项。
 *
 * 展开语义：模板里为一个参数配了几个取值，就开放几个给客户（这正是"多选"的含义 ——
 * 客户从这组取值里挑一个）。原先界面上用单选圆点表示"默认取值"，
 * 视觉上像把模板配的 4核/8核 砍成了单选，且取值没有各自的加价输入；
 * 现在所有取值平铺展示并可逐值定价，圆点换成显式的「基线值」标记。
 *
 * 基线值保证：模板没声明 default 时，把第一个取值标为基线。
 * 没有基线时建品会拿不到默认下发值（后端 pickEnumValue 依赖 IsDefault），
 * 客户下单界面的预选也会落到空。
 */
function buildOptionRows(tpl: SpecTemplateInfo): OptionRow[] {
  const sel = (tpl.option_selections || {}) as Record<
    string,
    { values?: string[]; range?: number[]; default?: string; group_label?: string; sku_axis?: boolean }
  >
  const out: OptionRow[] = []
  for (const [key, cfg] of Object.entries(sel)) {
    // 参与 SKU 正交的参数不进客户选配项（T4.7）：它的取值已经落成独立的规格变体，
    // 客户通过"选规格"来选它。若这里也开放一件同名配置项，客户在配置项里选的取值
    // 会覆盖 SKU 基线 —— 那正是「商品名 6G、实际开通 4G」的成因。
    if (cfg?.sku_axis) continue
    if (Array.isArray(cfg?.range) && cfg.range.length === 2) {
      const defStr = cfg.default ?? ''
      const defNum = Number(defStr)
      out.push({
        key,
        enabled: true,
        isRange: true,
        rangeMin: cfg.range[0],
        rangeMax: cfg.range[1],
        defaultNumber: defStr !== '' && Number.isFinite(defNum) ? defNum : null,
        unitPrice: 0,
        unitPriceQuarterly: null,
        unitPriceAnnually: null,
        unitPriceOnetime: null,
        unit: catalogUnit(key) || '',
        open: false,
        values: [],
      })
      continue
    }
    const rawValues = cfg?.values || []
    // 模板没声明默认（或声明的值不在取值列表里）时，第一个取值顶上基线位。
    const hasDeclaredDefault = !!cfg?.default && rawValues.includes(cfg.default)
    const values = rawValues.map((v, idx) => ({
      value: v,
      // label 留空：展示名由 catalogValueLabel 在渲染/提交时按目录**实时**解析。
      //
      // 为什么不在这里烘焙（实测故障）：buildRow 在页面挂载时就跑（loadTemplates），
      // 而平台目录要等运营选了渠道才加载（loadCatalogDefaults）。烘焙会把还没加载到的
      // 展示名固化成裸值（os 存成 "62"、area 存成 "1"），且提交时用这个裸值当 label
      // 覆盖掉后端的目录中文名 —— 下单界面于是只显示 ID。
      label: '',
      group: cfg?.group_label || '',
      price: 0,
      priceQuarterly: null,
      priceAnnually: null,
      priceOnetime: null,
      isDefault: hasDeclaredDefault ? cfg.default === v : idx === 0,
    }))
    if (values.length) {
      out.push({
        key,
        enabled: true,
        isRange: false,
        rangeMin: 0,
        rangeMax: 0,
        defaultNumber: null,
        unitPrice: 0,
        unitPriceQuarterly: null,
        unitPriceAnnually: null,
        unitPriceOnetime: null,
        unit: catalogUnit(key) || '',
        open: false,
        values,
      })
    }
  }
  return out
}

/**
 * buildOsList 组装该规格开放给客户的镜像集合。
 *
 * 两个来源：模板 platform_params.os（默认开通镜像）+ option_selections.os.values（客户可选取值）。
 * 默认值排第一 —— 客户未选时用它开通。
 */
function buildOsList(tpl: SpecTemplateInfo, platformParamsOfTpl: Record<string, unknown>): string[] {
  const out: string[] = []
  const push = (v: unknown) => {
    const s = v == null ? '' : String(v).trim()
    if (s && !out.includes(s)) out.push(s)
  }
  push(platformParamsOfTpl.os)
  const sel = (tpl.option_selections || {}) as Record<string, { values?: string[] }>
  for (const v of sel.os?.values || []) push(v)
  return out
}

/** 镜像取值 → 展示名（取不到时回落原值，不隐藏信息）。 */
function imageLabelOf(value: string): string {
  const hit = imageOptions.value.find((o) => o.value === value)
  return hit ? hit.label : value
}

/**
 * onRowOsChange 镜像选择变化：保证第一个值存在（它承担默认开通镜像的角色）。
 * 清空时回落模板默认，避免生成出「没有镜像」的 SKU。
 */
function onRowOsChange(row: TemplateRow) {
  if (!row.osList.length) {
    const fallback = (row.tpl.platform_params as Record<string, unknown> | null)?.os
    if (fallback != null) row.osList = [String(fallback)]
  }
  row.os = row.osList[0]
}

function buildRow(tpl: SpecTemplateInfo): TemplateRow {
  const specValues = (tpl.spec_values || {}) as Record<string, unknown>
  const platformParamsOfTpl = (tpl.platform_params || {}) as Record<string, unknown>

  const pickNumber = (keys: string[], fallback: number): number => {
    for (const key of keys) {
      const v = specValues[key]
      if (v != null && v !== '' && Number(v) > 0) return Number(v)
    }
    return fallback
  }
  // 原子字典里内存单位是 MB，模板结构化字段是 GB —— 两边都归一到 GB 展示。
  const memoryMb = pickNumber(['compute.memory'], 0)
  const row: TemplateRow = {
    tpl,
    selected: false,
    machineKind: (tpl.machine_kind as string) || '',
    cpu: pickNumber(['compute.cpu'], tpl.cpu || 1),
    memoryGb: memoryMb > 0 ? Math.round(memoryMb / 1024) : Math.round(tpl.memory || 1),
    disk: pickNumber(['storage.system.size'], tpl.disk || 40),
    bandwidth: pickNumber(['network.bandwidth'], tpl.bandwidth || 0),
    price: tpl.price || 0,
    costPrice: 0,
    area: platformParamsOfTpl.area != null ? String(platformParamsOfTpl.area) : undefined,
    node: platformParamsOfTpl.node != null ? String(platformParamsOfTpl.node) : undefined,
    store: platformParamsOfTpl.store != null ? String(platformParamsOfTpl.store) : undefined,
    os: platformParamsOfTpl.os != null ? String(platformParamsOfTpl.os) : undefined,
    // 模板的 option_selections.os.values 是「开放给客户的镜像集合」；
    // 模板 platform_params.os 是默认开通镜像。两者合并成 osList（默认值排第一）。
    osList: buildOsList(tpl, platformParamsOfTpl),
    options: buildOptionRows(tpl),
    extraSpecValues: {},
    extraPlatformParams: {},
  }
  const consumedSpecKeys = new Set(['compute.cpu', 'compute.memory', 'storage.system.size', 'network.bandwidth'])
  for (const [k, v] of Object.entries(specValues)) {
    if (!consumedSpecKeys.has(k) && v != null && v !== '') row.extraSpecValues[k] = v
  }
  for (const [k, v] of Object.entries(platformParamsOfTpl)) {
    if (!(PLATFORM_KEYS as readonly string[]).includes(k) && v != null && v !== '') row.extraPlatformParams[k] = v
  }
  return row
}

/** 组装请求里的 option_overrides（只带开放给客户的参数与其取值加价）。 */
function buildOptionOverrides(row: TemplateRow): SaleProductOptionOverride[] | undefined {
  const out: SaleProductOptionOverride[] = []
  for (const opt of row.options) {
    if (!opt.enabled) continue
    // NAT 机型的 ip_num 固定为 0：客户选配优先级高于档位基线，开放出去客户选了非 0
    // 就把 0 覆盖掉，开出来的机器会退回分配独立 IP。
    if (isNatRow(row) && opt.key === 'ip_num') continue
    if (opt.isRange) {
      out.push({
        option_key: opt.key,
        min_value: opt.rangeMin,
        max_value: opt.rangeMax,
        default: opt.defaultNumber != null ? String(opt.defaultNumber) : undefined,
        // 单位加价（元/单位）：不带上就没法给带宽/数据盘定价，只能建完去详情页补。
        // 留空的档位传 undefined（不是 0），后端据此按月数折算；显式 0 表示不加价。
        unit_price_monthly: opt.unitPrice,
        unit_price_quarterly: opt.unitPriceQuarterly ?? undefined,
        unit_price_annually: opt.unitPriceAnnually ?? undefined,
        unit_price_onetime: opt.unitPriceOnetime ?? undefined,
        values: [],
      })
      continue
    }
    const values = opt.values.map((v) => ({
      value: v.value,
      // 展示名按目录实时解析（6144 → 「6G」）。
      // 目录缺该项时返回 undefined（不是裸值）—— 后端 applyOverride 只在 label 非空时
      // 才覆盖，留空即保留目录里的中文名；传裸值反而会把好名字盖掉（实测故障）。
      label: catalogLabelOrEmpty(opt.key, v.value) || undefined,
      group_label: v.group || undefined,
      is_default: v.isDefault,
      price_monthly: v.price,
      // 留空的档位传 undefined（不是 0）：后端据此按「月价 × 周期月数」折算，
      // 与运营"只填月价即可"的直觉一致；显式填 0 则表示该周期不加价。
      price_quarterly: v.priceQuarterly ?? undefined,
      price_annually: v.priceAnnually ?? undefined,
      price_onetime: v.priceOnetime ?? undefined,
    }))
    out.push({
      option_key: opt.key,
      default: (opt.values.find((v) => v.isDefault) || opt.values[0])?.value,
      values,
    })
  }
  // 多选镜像 → 客户可选 os 配置项（第一个为默认开通镜像）。
  // 只选了一个镜像时不生成该配置项：单选没有"选"的意义，只会多一个必选下拉。
  if (row.osList.length > 1 && !out.some((o) => o.option_key === 'os')) {
    out.push({
      option_key: 'os',
      label: '操作系统',
      widget: 'select',
      default: row.osList[0],
      values: row.osList.map((v, i) => ({
        value: v,
        label: imageLabelOf(v),
        is_default: i === 0,
        // os 的展示按镜像家族分组（Ubuntu/Windows/CentOS），与平台目录口径一致。
        group_label: imageGroupOf(v),
        price_monthly: 0,
      })),
    })
  }
  return out.length ? out : undefined
}

/** 镜像取值 → 所属系统家族（用于客户侧下拉分组）。 */
function imageGroupOf(value: string): string | undefined {
  const hit = resources.value.images.find((i) => i.value === value)
  return hit?.group || undefined
}

/** 单选默认值：同一参数只能有一个默认（TDesign 没有原生 radio，用点击切换）。 */
function setDefaultValue(opt: OptionRow, value: string) {
  for (const v of opt.values) v.isDefault = v.value === value
}

/**
 * optRangeError 数量型选配项的区间自检。
 *
 * 与模板编辑器同一类校验（配置项默认值越界是实测故障的来源：客户下单预填出 0）。
 * 这里拦在提交前，省得建出的商品再走一遍迁移修数据。
 */
function optRangeError(opt: OptionRow): string {
  const min = opt.rangeMin
  const max = opt.rangeMax
  if (min == null || max == null) return ''
  if (Number(min) > Number(max)) return '最小值不能大于最大值'
  const def = opt.defaultNumber
  if (def != null && (Number(def) < Number(min) || Number(def) > Number(max))) {
    return `基线值 ${def} 不在区间 [${min}, ${max}] 内，客户下单会预填出无效值`
  }
  return ''
}

/**
 * optionSummary 未展开时的一行摘要：讲清这一项当前配成什么样。
 *
 * 参数多的档位（模板 17 有 12 项）全部展开会占满几屏，逐项找要改的很费劲。
 * 收起时至少要能看出"范围多少、基线多少、加不加价"，否则得挨个点开才知道。
 */
function optionSummary(opt: OptionRow): string {
  if (opt.isRange) {
    const range = `${opt.rangeMin ?? 0}~${opt.rangeMax ?? '—'}${opt.unit ? ' ' + opt.unit : ''}`
    const base = opt.defaultNumber != null ? `，基线 ${opt.defaultNumber}` : ''
    const price = opt.unitPrice > 0 ? `，超出 ¥${opt.unitPrice}/${opt.unit || '单位'}` : '，不加价'
    return `${range}${base}${price}`
  }
  const base = opt.values.find((v) => v.isDefault)
  const baseText = base ? `，基线 ${catalogValueLabel(opt.key, base.value)}` : ''
  const priced = opt.values.filter((v) => v.price > 0).length
  return `${opt.values.length} 个取值${baseText}，${priced > 0 ? `${priced} 个带加价` : '均不加价'}`
}

/** 勾选/取消某项：勾上时自动展开（运营勾选多半就是要改它）。 */
function onOptionToggle(opt: OptionRow) {
  if (opt.enabled) opt.open = true
}

/** 整档位批量展开/收起。 */
function toggleAllOptions(row: TemplateRow, open: boolean) {
  for (const opt of row.options) {
    if (opt.enabled) opt.open = open
  }
}

/** 收集全部档位的选配项区间错误（提交前统一拦一次）。 */
function firstOptionRangeError(): { spec: string; name: string; message: string } | null {
  for (const row of selectedRows.value) {
    for (const opt of row.options) {
      if (!opt.enabled || !opt.isRange) continue
      const err = optRangeError(opt)
      if (err) return { spec: row.tpl.name, name: platformFieldLabel(opt.key), message: err }
    }
  }
  return null
}

// ===== 可售周期与周期价格 =====
//
// 口径：周期价 = 月付价 × 月数 − 优惠月数 × 月付价（即"优惠 N 个月"）。
// 优惠月数按「周期月数 − 1」收敛，至少留一个月不优惠，避免算出 0 元或倒挂价
// （0 元周期价上架后，客户真能 0 元买）。

/** 可售周期候选：小时/天用于试用型商品，月付及以上是常规周期。 */
const sellableCycleOptions = [
  { label: '按小时', value: 'hourly' },
  { label: '按天', value: 'daily' },
  { label: '月付', value: 'monthly' },
  { label: '季付', value: 'quarterly' },
  { label: '半年付', value: 'semiannually' },
  { label: '年付', value: 'annually' },
]

/** 已勾选的可售周期；默认月付，运营按需增删。 */
const sellableCycles = ref<string[]>(['monthly'])

const priceGen = reactive({ monthly: 0 })

/** 一个可售周期的定价方案：勾选「一键计算」的参与推导，其余手工定价。 */
type CyclePlan = {
  cycle: string
  label: string
  /** 是否参与一键计算（月付恒为 true：它就是基准价本身）。 */
  auto: boolean
  /** 优惠月数（仅按月历周期有意义）。 */
  offMonths: number
  amount: number
  /** 周期月数；0 表示非月历周期（小时/天），无"优惠月数"口径。 */
  monthCount: number
}

/** 各周期的月数与默认优惠档：季付省 0、半年省 1、年付省 2（与旧口径一致）。 */
const cycleMeta: Record<string, { months: number; defaultOff: number }> = {
  hourly: { months: 0, defaultOff: 0 },
  daily: { months: 0, defaultOff: 0 },
  monthly: { months: 1, defaultOff: 0 },
  quarterly: { months: 3, defaultOff: 0 },
  semiannually: { months: 6, defaultOff: 1 },
  annually: { months: 12, defaultOff: 2 },
}

/**
 * cyclePlanState 逐周期的定价状态（价与优惠月数）。
 *
 * 用普通对象而不是 ref 的数组：cyclePlans 是 computed，模板里直接改 plan.amount
 * 需要落到这里才不会被下一次重算丢掉。勾选/取消周期时这些值保留，
 * 运营来回勾选不会把已经填好的价清掉。
 */
const cyclePlanState = reactive<Record<string, { auto: boolean; offMonths: number; amount: number }>>({})

/**
 * cyclePlans 由勾选的可售周期派生，勾选变化时保留已填的价与优惠月数。
 *
 * 为什么按勾选派生而不是固定四行：运营只卖月付时不该看到季付/年付的输入框
 * （填了也不生效，只会让人以为没保存住）；按小时这类非月历周期更是没有
 * "优惠月数"的概念，得单独给一行自己填价。
 */
const cyclePlans = computed<CyclePlan[]>(() => {
  return sellableCycles.value.map((cycle) => {
    const meta = cycleMeta[cycle] || { months: 0, defaultOff: 0 }
    const existing = cyclePlanState[cycle]
    // 月付即基准价本身：它的价直接取 priceGen.monthly，不再另存一份，
    // 否则"基准月付价"与"月付生成价"两个输入框会互相打架。
    const isMonthly = cycle === 'monthly'
    return {
      cycle,
      label: billingCycleLabel(cycle),
      auto: isMonthly ? true : (existing?.auto ?? meta.months > 0),
      offMonths: existing?.offMonths ?? meta.defaultOff,
      amount: isMonthly ? Math.round(Number(priceGen.monthly || 0) * 100) / 100 : (existing?.amount ?? 0),
      monthCount: meta.months,
    }
  })
})

/** 勾选变化时初始化新周期的状态（computed 里直接写 state 会在渲染期赋值，故用 watch）。 */
watch(
  sellableCycles,
  (cycles) => {
    for (const c of cycles) {
      if (!cyclePlanState[c]) {
        const meta = cycleMeta[c] || { months: 0, defaultOff: 0 }
        cyclePlanState[c] = { auto: c === 'monthly' || meta.months > 0, offMonths: meta.defaultOff, amount: 0 }
      }
    }
  },
  { immediate: true },
)

// 注意：form.price 的同步 watcher 放在 form 声明之后（见「销售价变化时同步月付基准价」）——
// watch 会在 setup 期立即求值一次 getter，声明顺序颠倒会命中 const 的暂时性死区（TDZ），
// setup 抛错、整个页面白屏（实测故障，vue-tsc 无法发现：访问在闭包里）。

/**
 * 逐周期状态的写入口。
 *
 * cyclePlans 是 computed（由勾选的周期派生），模板里不能直接 v-model 它的字段 ——
 * 写进去会被下一次重算丢掉。所有改动都经这三个函数落到 cyclePlanState，
 * 再由图上的 computed 反映出来。
 */

/** 勾/取消「一键计算」。取消时保留已生成的价，运营可在此基础上手改。 */
function setPlanAuto(plan: CyclePlan, auto: boolean) {
  const prev = cyclePlanState[plan.cycle] || { auto: plan.auto, offMonths: plan.offMonths, amount: plan.amount }
  cyclePlanState[plan.cycle] = { ...prev, auto, amount: plan.amount }
}

/** 改优惠月数：参与一键计算的周期立刻重算价，手工定价的不动。 */
function setPlanOff(plan: CyclePlan, offMonths: number) {
  const prev = cyclePlanState[plan.cycle] || { auto: plan.auto, offMonths: plan.offMonths, amount: plan.amount }
  const next = { ...prev, offMonths }
  if (plan.auto && plan.cycle !== 'monthly') next.amount = computeAmount(plan.cycle, offMonths)
  cyclePlanState[plan.cycle] = next
}

/** 直接改价：月付即基准价（写进 priceGen），其余视作手工定价，不再被一键计算覆盖。 */
function setPlanAmount(plan: CyclePlan, amount: number) {
  const value = Number.isFinite(amount) ? amount : 0
  if (plan.cycle === 'monthly') {
    priceGen.monthly = value
    return
  }
  const prev = cyclePlanState[plan.cycle] || { auto: plan.auto, offMonths: plan.offMonths, amount: plan.amount }
  cyclePlanState[plan.cycle] = { ...prev, amount: value }
}

/** 某周期的推导价：月付价 × (月数 − 优惠月数)；非月历周期返回 0（需手工定价）。 */
function computeAmount(cycle: string, offMonths: number): number {
  const meta = cycleMeta[cycle]
  const m = Number(priceGen.monthly)
  if (!meta || meta.months <= 0 || !Number.isFinite(m) || m <= 0) return 0
  const off = Math.max(0, Math.min(Number(offMonths) || 0, meta.months - 1))
  return Math.round(m * (meta.months - off) * 100) / 100
}

function generateCyclePrices() {
  const m = Number(priceGen.monthly)
  if (!Number.isFinite(m) || m <= 0) {
    MessagePlugin.warning('请先填入大于 0 的基准月付价')
    return
  }
  let touched = 0
  for (const plan of cyclePlans.value) {
    if (!plan.auto) continue // 手工定价的周期不覆盖
    cyclePlanState[plan.cycle] = {
      auto: plan.auto,
      offMonths: plan.offMonths,
      amount: computeAmount(plan.cycle, plan.offMonths),
    }
    touched++
  }
  if (!touched) {
    MessagePlugin.info('当前没有勾选「一键计算」的周期；请勾选后再生成，或直接逐行填价')
    return
  }
  MessagePlugin.success(`已按基准月付价生成 ${touched} 个周期价，可逐行微调`)
}

/** 重置：清掉所有派生价与优惠月数，月付价也归零，回到勾选后的初始态。 */
function resetCyclePlans() {
  for (const key of Object.keys(cyclePlanState)) delete cyclePlanState[key]
  priceGen.monthly = 0
  for (const c of sellableCycles.value) {
    const meta = cycleMeta[c] || { months: 0, defaultOff: 0 }
    cyclePlanState[c] = { auto: c === 'monthly' || meta.months > 0, offMonths: meta.defaultOff, amount: 0 }
  }
}

/** 每行的说明文字：讲清这一行的价是怎么来的（或为什么需要手填）。 */
function cyclePlanHint(plan: CyclePlan): string {
  if (plan.cycle === 'monthly') return '基准周期，其他周期按它推导'
  if (!plan.auto) return plan.monthCount > 0 ? '手工定价，不参与一键计算' : '非月历周期，只能手工定价'
  if (plan.offMonths > 0) return `${plan.monthCount} 个月 · 省 ${plan.offMonths} 个月`
  return `${plan.monthCount} 个月`
}

/**
 * 是否允许勾选「一键计算」。
 *
 * 月付是基准价本身（不允许取消）；小时/天没有"月数×价"的口径，
 * 勾了也只会算出 0 —— 不允许勾，逼运营自己填试用价，
 * 否则会生成一个 0 元价（客户真能 0 元买）。
 */
function isAutoEligible(plan: CyclePlan): boolean {
  return plan.cycle !== 'monthly' && plan.monthCount > 0
}

/**
 * 周期价 → 提交用的「周期 → 金额」映射（未定价的周期不带）。
 * 仅建品时产出：编辑页的周期价在商品详情页的价格矩阵里维护，
 * 这里带出去只会被父页丢弃（edit.vue 不接第二个参数）。
 */
function cyclePriceMap(): Record<string, number> {
  if (props.mode !== 'create') return {}
  const out: Record<string, number> = {}
  for (const plan of cyclePlans.value) {
    if (plan.amount > 0) out[plan.cycle] = plan.amount
  }
  return out
}

/**
 * 月付价兜底：运营在「销售价」填了数、却没在周期价格里填月付价时，用销售价顶上。
 * 商品展示价与实际月付价不一致是常见的困惑来源，这里让两者默认一致
 * （运营在周期价格里显式填过就尊重他填的）。
 */
function seedMonthlyFromProductPrice() {
  if (Number(priceGen.monthly) > 0) return
  const p = Number(form.price)
  if (Number.isFinite(p) && p > 0) priceGen.monthly = Math.round(p * 100) / 100
}

/** 已勾选但没有价的周期（提交前提示，避免建出客户买不了的周期）。 */
function cyclesWithoutPrice(): string[] {
  return cyclePlans.value.filter((p) => !(p.amount > 0)).map((p) => p.label)
}

/**
 * derivePriceModel 由勾选的可售周期反推 products.price_model（存量兼容列）。
 *
 * 该列只认 fixed/hourly/monthly，且是"订单没带周期时"的兜底口径
 * （见 billingcycle.NormalizeOrDefault）。勾了按小时 → hourly；
 * 勾了任何月历周期 → monthly；都没有（理论上被提交校验挡住）→ fixed。
 */
function derivePriceModel(): string {
  const cycles = sellableCycles.value
  if (cycles.includes('hourly')) return 'hourly'
  if (cycles.some((c) => (cycleMeta[c]?.months ?? 0) > 0)) return 'monthly'
  return 'fixed'
}

async function loadTemplates() {
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100, status: 1 })
    templateRows.value = data.items.map(buildRow)
  } catch {
    templateRows.value = []
  }
}

/**
 * 勾选/取消档位时的联动。
 *
 * 用户要求（本轮）：选档位后把"属于规格"的字段自动填进表单，不属于规格的不填。
 *   - 属于规格 → 自动带入：产品名称、售价（单档位时）；
 *   - 不属于规格 → 保持空/原值：SKU 编码、分类、平台渠道、封面、描述、成本价。
 *     这些是商品的经营属性，不能由档位推导（例如分类与封面是运营决策，
 *     SKU 编码要唯一且创建后不可改）。
 *
 * 多档位时名称与售价不自动填：一个商品挂多个档位时，名称应是"这组规格的统称"，
 * 取任一档位的名字或价都会误导（售价更是多价，填谁都不对）。此时只提示去步骤 3 逐档定价。
 *
 * 只在字段仍为空时回填：运营已经手填过的值不能被勾选动作悄悄覆盖。
 */
function onRowToggle(row: TemplateRow) {
  // 档位没配区域时补商品级区域，省得逐档再选一遍。
  if (row.selected && !row.area && platformParams.area) row.area = platformParams.area
  if (props.mode !== 'create') return
  applyRowDefaultsToForm()
}

function applyRowDefaultsToForm() {
  const rows = selectedRows.value
  if (rows.length !== 1) {
    if (rows.length > 1) MessagePlugin.info('已选多个档位：名称与售价请统一填写，各档位售价在「规格明细」里分别设置')
    return
  }
  const row = rows[0]
  if (!form.name.trim()) {
    // 单档位时用「平台 + 核/内存」拼出可读名称，运营可改；改名会自动带动编码。
    const spec = `${row.cpu || 1}核${row.memoryGb || 1}G`
    const platformName = platformDisplayName(form.source_provider_id)
    form.name = platformName ? `${platformName} ${spec}` : spec
  }
  if (form.price <= 0 && row.price > 0) form.price = row.price
}

/** 平台渠道显示名（自动命名用；未选渠道时留空由运营自己起名）。 */
function platformDisplayName(providerId?: number): string {
  const hit = platformOptions.value.find((o) => o.value === providerId)
  return hit ? hit.label.replace(/（.*）$/, '') : ''
}

function selectAllTemplates() {
  // 只选当前筛选出的行：装了 NAT 筛选时点「全选」不应把 VPS 档位也勾上。
  for (const row of visibleTemplateRows.value) {
    row.selected = true
    onRowToggle(row)
  }
}
function clearAllTemplates() {
  for (const row of visibleTemplateRows.value) row.selected = false
}

/** 由行内参数组装原子取值 JSON（内存 GB→MB 与字典口径对齐）。 */
function buildSpecValues(row: TemplateRow): Record<string, unknown> {
  const out: Record<string, unknown> = { ...row.extraSpecValues }
  if (row.cpu > 0) out['compute.cpu'] = row.cpu
  if (row.memoryGb > 0) out['compute.memory'] = row.memoryGb * 1024
  if (row.disk > 0) out['storage.system.size'] = row.disk
  if (row.bandwidth > 0) out['network.bandwidth'] = row.bandwidth
  if (row.area) out['placement.region'] = row.area
  return out
}

function buildPlatformParams(row: TemplateRow): Record<string, unknown> {
  const out: Record<string, unknown> = { ...row.extraPlatformParams }
  if (row.area) out.area = row.area
  if (row.node) out.node = row.node
  if (row.store) out.store = row.store
  if (row.os) out.os = row.os
  return out
}

function previewSpecCode(row: TemplateRow): string {
  // 编码不再带「规格族」前缀（用户要求去掉 通用型/计算型 这类分组）：
  // 用平台类型 + 核/内存/盘 表达，与档位实际取值一致。
  const parts: string[] = []
  if (row.tpl.provider_type) parts.push(row.tpl.provider_type)
  parts.push(`${row.cpu || 1}c${row.memoryGb || 1}g`)
  if (row.disk > 0) parts.push(`${row.disk}g`)
  return parts.join('-')
}

const form = reactive({
  code: '',
  name: '',
  category_id: undefined as number | undefined,
  product_type: 'cloud_host',
  description: '',
  cover_image: '',
  price_model: 'fixed',
  price: 0,
  cost_price: 0,
  source_provider_id: undefined as number | undefined,
  stock: -1,
  sort_order: 0,
  status: 0,
  // 上游转售商品的上架门禁放行项（见模板说明：仅透传 = 参数原样转发，不做映射）。
  spec_passthrough: false,
})

/**
 * 销售价变化时同步月付基准价（仅当运营还没在周期价格里填过）。
 *
 * 两个字段表达同一件事（默认按月的商品价）：让后填的那个跟上先填的，
 * 避免出现"销售价 20、月付价 0"，建出的商品展示价与下单价不一致。
 *
 * 必须放在 form 声明之后：watch 在 setup 期会立即求值一次 getter，
 * 声明顺序颠倒会命中 const 的暂时性死区，setup 抛错导致整页白屏。
 */
watch(
  () => form.price,
  (value) => {
    if (props.mode !== 'create') return
    if (Number(priceGen.monthly) <= 0 && Number(value) > 0) {
      priceGen.monthly = Math.round(Number(value) * 100) / 100
    }
  },
)

// ===== SKU 编码自动生成 =====
// 编码是给系统与上游看的标识（不是展示名），手工想一个既费神又容易重名。
// 这里按「产品类型 + 中文名转写」自动生成，运营可随时手改；
// 一旦手改过就不再覆盖（见 codeTouched），否则改个名字把人工编码冲掉更糟。
const codeTouched = ref(false)

/** 中文/符号 → 拉丁化的简易转写表：覆盖常见商品名字词，转不出来就跳过。 */
const codeWordMap: Record<string, string> = {
  香港: 'hk',
  新加坡: 'sg',
  日本: 'jp',
  美国: 'us',
  韩国: 'kr',
  台湾: 'tw',
  华东: 'east',
  华南: 'south',
  华北: 'north',
  华中: 'central',
  西部: 'west',
  云服务器: 'cloud',
  云主机: 'cloud',
  高防: 'ddos',
  独立: 'dedicated',
  站群: 'seo',
  虚拟主机: 'vhost',
  数据库: 'db',
  物理机: 'bare',
  入门: 'basic',
  基础: 'basic',
  标准: 'std',
  进阶: 'pro',
  高级: 'adv',
  旗舰: 'ultra',
  豪华: 'premium',
  定制: 'custom',
  试用: 'trial',
  核: 'c',
  内存: 'mem',
  带宽: 'bw',
  系统盘: 'disk',
  年付: 'y',
  月付: 'm',
}

/**
 * generateProductCode 由产品名生成候选 SKU 编码。
 * 规则：产品类型前缀 + 名称转写；无可用转写时退回带时间戳的前缀码，保证非空且大概率不重名。
 */
/** 已占用的商品编码（小写），用于自动生成时避重。 */
const takenCodes = ref<Set<string>>(new Set())

/**
 * loadTakenCodes 拉一份现有商品编码，供自动生成避重。
 *
 * 为什么需要：编码是「按名称转写」生成的，而名称经常重复
 * （实测故障：第二个「魔方云 4核6G」又生成 ch-4-c-6g，提交时撞唯一索引，
 * 界面弹出一行 SQL 报错）。这里在生成前先知道哪些码被占了，直接生成不冲突的码。
 *
 * 只读一页 100 条：这是"尽力避开"的优化，不是唯一性保证 ——
 * 真正的唯一性仍由后端唯一索引 + 提交时的查重兜底（见后端 ErrProductCodeExists）。
 * 分页读全量在商品多时成本过高，不值得。
 */
async function loadTakenCodes() {
  try {
    const data = await getProductList({ page: 1, page_size: 100 })
    takenCodes.value = new Set((data?.items || []).map((p) => String(p.code || '').toLowerCase()))
  } catch {
    // 拉不到不影响建品：仅在提交时由后端兜底提示重名。
    takenCodes.value = new Set()
  }
}

function generateProductCode(): string {
  const prefix = form.product_type === 'cloud_host' ? 'ch' : form.product_type || 'prod'
  let name = form.name.trim().toLowerCase()
  for (const [zh, en] of Object.entries(codeWordMap)) {
    name = name.split(zh).join(`-${en}-`)
  }
  // 保留拉丁字母与数字，其余作分隔；连续分隔符合并，去掉首尾。
  const slug = name
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 40)
  if (!slug) {
    // 名称全是中文且转写表没覆盖：用类型 + 时间戳，避免生成空编码或一堆同码。
    return `${prefix}-${Date.now().toString(36).slice(-6)}`
  }
  // 撞码时加数字后缀（-2/-3…），与「同一个名字建第二次」的直觉一致，
  // 比直接拼时间戳更好读，也让运营看得出是同一系列。
  const base = `${prefix}-${slug}`
  if (!takenCodes.value.has(base.toLowerCase())) return base
  for (let n = 2; n <= 99; n++) {
    const candidate = `${base}-${n}`
    if (!takenCodes.value.has(candidate.toLowerCase())) return candidate
  }
  return `${base}-${Date.now().toString(36).slice(-4)}`
}

/** 名称变化时自动更新编码（用户没手动改过才更新）。 */
watch(
  () => form.name,
  () => {
    if (props.mode !== 'create' || codeTouched.value) return
    form.code = generateProductCode()
  },
)

/** 平台渠道变化后重算编码前缀意义不大，但选完渠道再生成一次更贴合（此时类型通常已定）。 */
function regenerateCode() {
  codeTouched.value = false
  form.code = generateProductCode()
}

// 平台渠道仅在「云主机」类商品上必填：它决定开通/暂停/销毁下发到哪个平台；
// 虚拟主机、数据库等纯本地商品不强制绑定渠道（存量演示商品即属此类）。
const platformRules = computed(() =>
  form.product_type === 'cloud_host' ? [{ required: true, message: '请选择开通该商品的平台渠道' }] : [],
)

watch(
  () => props.initial,
  async (initial) => {
    if (!initial) return
    form.code = initial.code
    form.name = initial.name
    form.category_id = initial.category_id || undefined
    form.product_type = initial.product_type
    form.description = initial.description || ''
    form.cover_image = initial.cover_image || ''
    form.spec_passthrough = !!initial.spec_passthrough
    form.price_model = initial.price_model
    form.price = initial.price
    form.cost_price = initial.cost_price
    form.source_provider_id = initial.source_provider_id || undefined
    form.stock = initial.stock
    form.sort_order = initial.sort_order
    form.status = initial.status
    splitConfigOptions(initial.config_options || '')
    // 编辑既有商品时把平台资源目录也拉起来，让下拉能回显与改选。
    //
    // 只对自营商品做：上游转售商品这一侧表单里根本没有平台参数控件（区域/节点/存储/系统盘
    // 都来自上游配置组），而财务型上游渠道压根不实现平台资源目录读取能力 ——
    // 照旧无条件调用会稳定弹「该渠道未提供平台资源目录读取能力」，运营每次进编辑页都被
    // 误导一次（实测反馈）。上游商品不需要这份数据，直接跳过。
    if (form.source_provider_id && !isUpstream.value) {
      await loadPlatformResources(form.source_provider_id)
    }
  },
  { immediate: true },
)

function handleCancel() {
  emit('cancel')
}

function buildSpecTemplatePayload(): SaleProductSpecTemplateSelection[] | null {
  if (props.mode !== 'create' || specMode.value !== 'template') return []
  const rows = templateRows.value.filter((r) => r.selected)
  if (!rows.length) {
    MessagePlugin.warning('请勾选至少一个规格模板，或改选「稍后在详情页配置」')
    return null
  }
  // 数量型选配项的区间自检（与模板编辑器同一类校验）：越界的基线值
  // 会让客户下单时预填出无效数字，正是「预填 0」那类故障的成因。
  const rangeErr = firstOptionRangeError()
  if (rangeErr) {
    MessagePlugin.warning(`规格「${rangeErr.spec}」的「${rangeErr.name}」：${rangeErr.message}`)
    return null
  }
  const payload: SaleProductSpecTemplateSelection[] = []
  for (const row of rows) {
    const platformParamsOfRow = buildPlatformParams(row)
    if (!Object.keys(platformParamsOfRow).length) {
      MessagePlugin.warning(`规格「${row.tpl.name}」没有平台参数（模板未配映射且未就地选择），生成后无法上架`)
      return null
    }
    const kindError = machineKindError(row, platformParamsOfRow)
    if (kindError) {
      MessagePlugin.warning(`规格「${row.tpl.name}」：${kindError}`)
      return null
    }
    payload.push({
      spec_template_id: row.tpl.id,
      // 带上预览编码：后端在"请求未给编码"时会按模板结构派生，就地改过的配置会生成
      // 名不副实的编码（如模板 2C4G 改成 4C8G 却叫 general-2c4g-30g）。
      spec_code: previewSpecCode(row),
      name: `${row.tpl.name}（${row.cpu || 1}核${row.memoryGb || 1}G）`,
      // 机型不进 payload：它由 platform_params 的取值唯一决定（ip_num/nat_* 原样
      // 复制进 SKU 绑定），再存一列就有了两个可能不一致的真源。
      spec_values: buildSpecValues(row),
      platform_params: platformParamsOfRow,
      // 客户可选配置项：只带开放给客户的参数（含取值加价），后端按档位目录补齐控件元数据。
      option_overrides: buildOptionOverrides(row),
      price: row.price,
      cost_price: row.costPrice,
      stock: form.stock,
    })
  }
  return payload
}

function buildPayload(): SaleProductCreateRequest | null {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return null
  }
  if (props.mode === 'create' && !form.code.trim()) {
    MessagePlugin.warning('请输入 SKU 编码')
    return null
  }
  // 云主机必须绑定平台渠道才有履约通道；虚拟/服务类商品（如演示虚拟主机）不强制。
  const isCloud = form.product_type === 'cloud_host'
  if (isCloud && !form.source_provider_id) {
    MessagePlugin.warning('请选择自营平台渠道')
    return null
  }
  const configOptions = composeConfigOptions()
  if (configOptions === null) return null
  const specTemplates = buildSpecTemplatePayload()
  if (specTemplates === null) return null
  // 可售周期至少要勾一个：没勾就没有任何周期价能落进矩阵，商品上架后客户无周期可选
  // （价格模型那一列只是无周期订单的兜底，替代不了矩阵）。
  if (props.mode === 'create' && !sellableCycles.value.length) {
    MessagePlugin.warning('请至少勾选一个可售周期')
    return null
  }
  // 勾了却没定价的周期：落矩阵时会被跳过（price=0 不算可售），客户看不到它。
  // 直接拦住并说清是哪几个，比建完商品再回详情页补价省事。
  if (props.mode === 'create') {
    seedMonthlyFromProductPrice()
    const missing = cyclesWithoutPrice()
    if (missing.length) {
      MessagePlugin.warning(`以下周期还没定价：${missing.join('、')}。请在「周期价格」里填价或取消勾选`)
      return null
    }
  }
  // 自营专属 payload：链路恒为 self，不携带任何上游转售字段（source_product_id / 加价 / 仅透传）。
  return {
    code: props.mode === 'create' ? form.code.trim() : form.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    // 建品时由勾选的可售周期反推（该列只认 fixed/hourly/monthly，见 derivePriceModel）；
    // 编辑既有商品时沿用原值。
    price_model: props.mode === 'create' ? derivePriceModel() : form.price_model,
    price: form.price,
    cost_price: form.cost_price,
    source_mode: 'self',
    source_product_id: 0,
    source_provider_id: form.source_provider_id,
    config_options: configOptions,
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
    spec_templates: specTemplates.length ? specTemplates : undefined,
    upstream_markup_type: '',
    upstream_markup_value: 0,
    spec_passthrough: false,
  }
}

function handleSubmit() {
  const payload = buildPayload()
  if (!payload) return
  // 周期价随 payload 一起带出：创建接口不收周期价（它走价格矩阵接口），
  // 父页在拿到商品 id 后再落矩阵。用第二个参数而不是塞进 payload，
  // 避免多出一个后端不认识的字段。
  emit('submit', payload, cyclePriceMap())
}

/**
 * 代理商品保存基础信息。
 *
 * 刻意**不回传** source_mode / source_provider_id / source_product_id：
 * 这三项由导入流程决定，后端只接受"未传=保持原值"。此前把读到的原值原样回传，
 * 后端对上游商品无条件拒绝渠道字段，导致改名/改描述都保存不了（实测故障）；
 * 即使后端放行，回传本身也没有任何意义。
 *
 * specs / config_options 必须回传：后端 Update 对这两个字段是非指针类型，
 * 漏传等于写空 —— 会把导入时带过来的规格 JSON 与配置项静默清掉。
 * （配置项现在由详情页的独立编辑器维护并单独保存，这里仍回传当前值以免被清空。）
 */
function handleSubmitUpstream() {
  const payload = buildUpstreamPayload()
  if (!payload) return
  emit('submit', payload)
}

/** 上游商品的基础信息 payload（校验失败返回 null 并已提示）。 */
function buildUpstreamPayload(): SaleProductCreateRequest | null {
  if (!props.initial) return null
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return null
  }
  const configOptions = composeConfigOptions()
  if (configOptions === null) return null
  return {
    code: props.initial.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    price_model: form.price_model,
    price: form.price,
    cost_price: form.cost_price,
    // 规格快照与配置项回传原值（见上方注释：漏传会被写成空）。
    specs: props.initial.specs || '',
    config_options: configOptions,
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
    // 仅透传是上架门禁开关：表单里可改（此前的实现恒取初值，改不动）
    spec_passthrough: !!form.spec_passthrough,
  }
}

/**
 * 独立编辑页的统一保存（运营要求：两个保存按钮合并成一个）。
 *
 * 合并到商品详情页后，价格与配置项各自是独立子页、由详情页统一调度（见 detail.vue），
 * 这里只剩"基础信息 + 提交"这一件事 —— 保留独立形态是为了 create.vue 之外的
 * 旧入口与书签仍可用（/products/:id/edit 会重定向到详情页，见路由注释）。
 */
function handleSaveAll() {
  if (props.submitting) return
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return
  }
  const configOptions = composeConfigOptions()
  if (configOptions === null) return
  // 自营编辑走 buildPayload（含周期价校验）；上游走 buildUpstreamPayload。
  if (isUpstream.value) {
    const payload = buildUpstreamPayload()
    if (!payload) return
    emit('submit', payload)
    return
  }
  // 自营商品的编辑在合并后的详情页里由基础信息段 + 价格子页分别提交，
  // 这里只在独立页（书签直达前的兜底）使用：走 buildPayload 的完整校验。
  const payload = buildPayload()
  if (!payload) return
  emit('submit', payload)
}

/**
 * 内嵌到详情页时由父页调用的**校验 + 取 payload** 入口（不自行提交）。
 *
 * 为什么把提交留给父页：合并后一次「保存」要按顺序落多块（配置项 → 基础信息 → 价格），
 * 每块各有自己的接口与成败，父页才能决定"中途失败就停在这里、不要跳走"。
 * 本方法只保证：基础信息合法，并给出该链路的更新 payload。
 *
 * 返回 `SaleProductUpdateRequest`（**不含 code**）：更新接口不接受 SKU 编码
 * （它是系统与上游的标识，改动会让历史订单与绑定对不上），类型上就不带这个字段，
 * 免得每个调用点各写一次解构。
 */
function prepareBasicPayload(): SaleProductUpdateRequest | null {
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return null
  }
  if (composeConfigOptions() === null) return null
  const payload = isUpstream.value ? buildUpstreamPayload() : buildSelfPayload()
  if (!payload) return null
  const { code: _code, ...rest } = payload
  void _code
  return rest
}

/** 自营商品的本体 payload（不含周期价——它在「价格」子页单独提交）。 */
function buildSelfPayload(): SaleProductCreateRequest {
  return {
    code: form.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    price_model: form.price_model,
    price: form.price,
    cost_price: form.cost_price,
    // specs 必须回传：后端 Update 对它是非指针类型，漏传会清空规格快照。
    specs: props.initial?.specs || '',
    source_mode: 'self',
    source_provider_id: form.source_provider_id,
    config_options: composeConfigOptions() || '',
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
  }
}

defineExpose({ prepareBasicPayload })

onMounted(() => {
  loadCategories()
  loadPlatformOptions()
  if (props.mode === 'create') {
    void loadTemplates()
    void loadTakenCodes()
  }
  // 提供商名称解析：详情页合并后本组件仍要显示上游渠道名（失败回落 #ID）。
  void loadProviderNames()
})

/** 拉取提供商列表建 ID→名称映射（上游提供商展示名解析用；失败不影响表单）。 */
async function loadProviderNames() {
  try {
    const data = await getProviderList({ page_size: 100 })
    const map: Record<number, string> = {}
    for (const item of data.items || []) map[item.id] = item.name
    providerNames.value = map
  } catch {
    // 解析失败只影响展示名，回落为 #ID，不打断表单。
  }
}
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
/* ===== 步骤区块（新建/编辑自营表单的流程骨架）=====
   建品是从"选规格"到"补信息"的顺序过程，用编号分区让流程可见；
   每块之间留出明显间距与分隔线，避免原来看起来是一整片连续表单。 */
.form-step + .form-step {
  margin-top: var(--space-lg);
  padding-top: var(--space-lg);
  border-top: 1px solid var(--color-border);
}

.form-step__head {
  display: flex;
  align-items: flex-start;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

/* 可折叠区的标题行：整行可点，右侧给展开/收起提示 */
.form-step__head--toggle {
  cursor: pointer;
  user-select: none;
  margin-bottom: var(--space-sm);
}

.form-step__head--toggle:hover .form-step__title {
  color: var(--color-primary, #0052d9);
}

.form-step__no {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  margin-top: 1px;
  border-radius: 50%;
  background: var(--color-primary, #0052d9);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
}

.form-step__text {
  flex: 1;
  min-width: 0;
}

.form-step__title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-foreground);
}

.form-step__desc {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.form-step__meta {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* ===== 档位选择器（步骤 1）===== */
.tpl-picker__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  flex-wrap: wrap;
  margin-bottom: var(--space-sm);
}

/* 紧凑档位卡片：一屏看多个档位，勾选即选中；细节不在此展开 */
.tpl-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: var(--space-sm);
}

.tpl-chip {
  display: flex;
  align-items: flex-start;
  gap: var(--space-sm);
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md, 6px);
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background-color 0.15s ease;
}

.tpl-chip:hover {
  border-color: var(--color-primary, #0052d9);
}

.tpl-chip--on {
  border-color: var(--color-primary, #0052d9);
  background: var(--color-primary-light, rgba(0, 82, 217, 0.04));
}

.tpl-chip__body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tpl-chip__name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tpl-chip__spec {
  font-size: 12px;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.tpl-chip__meta {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.tpl-chip__price {
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-primary, #0052d9);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* ===== 规格明细（步骤 3）：一个已选档位一块 ===== */
.spec-detail {
  padding: var(--space-md);
  margin-bottom: var(--space-md);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md, 6px);
}

.spec-detail__head {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
  margin-bottom: var(--space-sm);
}

.spec-detail__name {
  font-size: 13px;
  font-weight: 600;
}

.spec-detail__code {
  font-size: 11px;
  color: var(--color-muted-foreground);
}

.spec-detail__code code {
  padding: 1px 4px;
  border-radius: 3px;
  background: var(--color-muted, rgba(0, 0, 0, 0.04));
}

/* 内嵌到商品详情页：详情页已有卡片外壳，这里不再套一层（避免卡片里套卡片）。
   页面级「保存」也由详情页渲染，本组件此时不带任何按钮。 */
.form-plain {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.upstream-readonly {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.upstream-readonly p {
  margin: 0;
  line-height: 1.8;
}

.upstream-readonly__meta {
  font-size: 12px;
  color: var(--td-text-color-secondary, #6b7280);
}

.upstream-readonly__hint {
  margin: 0;
  color: var(--td-text-color-secondary, #999);
  font-size: 13px;
}

/* 已选计数（步骤 1 顶栏；样式沿用，名字保留以少改模板） */
.tpl-list__count {
  font-weight: 600;
  color: var(--td-text-color-primary, #333);
}

.tpl-mix-warn {
  margin-top: 2px;
}

/* 多选档位的说明：讲清"勾 N 个 = N 个 SKU、选配项按参数名合并"，
   避免运营以为参数逐档位隔离。 */
.tpl-multi-tip {
  margin-top: 2px;
}

.tpl-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tpl-group__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding-top: 4px;
}

.tpl-group__desc {
  font-size: 12.5px;
  color: var(--td-text-color-secondary, #888);
}

/* 选中 NAT 档位时的访问方式提示，紧跟行头，不与下面的字段混在一起。 */
.tpl-row__notice {
  margin-bottom: 8px;
}

.tpl-row {
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 6px;
  padding: 10px 12px;
  background: var(--td-bg-color-container, #fff);
}

.tpl-row--on {
  border-color: var(--td-brand-color, #0052d9);
  box-shadow: 0 0 0 1px rgba(0, 82, 217, 0.12);
}

.tpl-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.tpl-row__name {
  font-weight: 600;
}

.tpl-row__meta {
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
}

.tpl-row__warn {
  color: var(--td-warning-color, #e37318);
  font-style: normal;
}

.tpl-row__body {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tpl-row__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px 12px;
}

.tpl-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #666);
}

.tpl-row__gen {
  font-size: 12px;
  color: var(--td-text-color-secondary, #999);
  word-break: break-all;
}

.tpl-row__gen code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  color: var(--td-text-color-primary, #333);
}

.tpl-options {
  border-top: 1px dashed var(--td-component-stroke, #e7e7e7);
  padding-top: 8px;
}

.tpl-options__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
  margin-bottom: 8px;
}

/* 项数徽标：让"这个档位有多少可选参数"一眼可见 */
.tpl-options__count {
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 8px;
  background: var(--td-bg-color-container-hover, #f0f1f3);
  font-style: normal;
  font-size: 11px;
  color: var(--td-text-color-secondary, #777);
}

/* 周期价格一键生成：一行输入 + 生成结果网格。 */
.price-gen {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.price-gen__row {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  flex-wrap: wrap;
}

.price-gen__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12.5px;
  color: var(--td-text-color-secondary, #666);
  min-width: 120px;
}

/* 可售周期多选：勾选项平铺，提示文字另起一行不与勾选框抢宽度 */
.cycle-pick {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* 逐周期定价：一行一个周期，勾选框/名称/优惠/价格/说明横向对齐 */
.price-gen__plans {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.price-gen__plan {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  flex-wrap: wrap;
  padding: 6px 10px;
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 6px;
}

.price-gen__plan-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 116px;
  padding-bottom: 6px;
}

.price-gen__plan-name {
  font-size: 13px;
  font-weight: 600;
}

.price-gen__field--sm {
  min-width: 132px;
}

.price-gen__plan-hint {
  padding-bottom: 6px;
  font-size: 11.5px;
  color: var(--td-text-color-secondary, #888);
}

.price-gen__tip {
  padding-bottom: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.price-gen__empty {
  margin: 0;
  font-size: 12.5px;
  color: var(--td-text-color-secondary, #888);
}

.price-gen__hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
  line-height: 1.6;
}

/* 平台配置项默认值清单：让"不填会是什么"一眼可见。 */
.defaults-hint {
  margin: 4px 0 6px;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--td-text-color-secondary, #666);
}

.defaults-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.defaults-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 999px;
  font-size: 12px;
  background: var(--td-bg-color-container-hover, #fafafa);
}

.defaults-item__label {
  color: var(--td-text-color-secondary, #666);
}

.defaults-item__value {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-weight: 600;
}

.defaults-item__help {
  color: var(--td-text-color-placeholder, #999);
}

.tpl-opt__name {
  margin-right: 6px;
}

.tpl-opt__key {
  font-size: 11.5px;
  opacity: 0.65;
}

.tpl-field__hint {
  font-size: 11.5px;
  color: var(--td-text-color-secondary, #888);
  line-height: 1.5;
}

.tpl-opt {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 7px 10px;
  border: 1px solid transparent;
  border-radius: 6px;
}

/* 展开态给个底色与描边：一眼看出哪几项正在编辑 */
.tpl-opt--open {
  border-color: var(--td-component-stroke, #e7e7e7);
  background: var(--td-bg-color-container-hover, #fafbfc);
}

.tpl-opt__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

/* 摘要：收起时的唯一信息来源，用弱化色但不省略 */
.tpl-opt__summary {
  margin-left: auto;
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
  font-variant-numeric: tabular-nums;
}

.tpl-opt__summary.is-off {
  color: var(--td-text-color-placeholder, #bbb);
}

.tpl-opt__toggle {
  flex-shrink: 0;
  font-size: 12px;
}

.tpl-opt code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.tpl-opt__values {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 一个取值及其长周期加价：整体成块，展开的长周期输入跟随所属取值 */
.tpl-opt__value-block {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tpl-opt__value {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

/* 「设为基线」链接与「基线值」标签同宽占位，切换时整行不跳动 */
.tpl-opt__value > .t-link,
.tpl-opt__value > .t-tag {
  min-width: 58px;
  justify-content: center;
}

.tpl-opt__label {
  min-width: 52px;
}

.tpl-opt__price {
  width: 110px;
}

.tpl-opt__unit {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.tpl-opt__closed {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
}

.tpl-opt__error {
  font-size: 12px;
  color: var(--td-error-color-6, #d54941);
}

/* 长周期加价：展开后另起一行，与取值行左对齐缩进 */
.tpl-opt__cycles {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 16px;
  margin: 2px 0 4px 64px;
  padding: 8px 12px;
  border-radius: 6px;
  background: var(--td-bg-color-container-hover, #f7f8fa);
}

.tpl-field--inline {
  flex-direction: row;
  align-items: center;
  gap: 6px;
}
.form-grid__full {
  grid-column: 1 / -1;
}
</style>
