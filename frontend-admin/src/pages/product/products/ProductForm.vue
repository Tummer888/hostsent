<template>
  <div class="page-body product-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AddIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">{{ pageTitle }}</h2>
          <p class="page-header__desc">{{ headerDesc }}</p>
        </div>
      </div>
    </header>

    <!-- 代理（上游转售）商品：链路与上游绑定由导入决定（只读），基础信息（名称/分类/描述等）可改。 -->
    <section v-if="isUpstream" class="form-card surface-card">
      <t-alert theme="info">
        <template #message>
          <div class="upstream-readonly">
            <p>
              这是<b>上游转售商品</b>：它的规格与开通参数由上游决定，
              <b>链路与上游绑定不可改</b>；名称、分类、描述、封面、库存、排序这些基础信息可以直接改。
            </p>
            <t-descriptions :column="2" size="small" bordered>
              <t-descriptions-item label="链路">
                <t-tag theme="warning" variant="light" size="small" shape="round">上游转售</t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="上游提供商 ID">{{ initial?.source_provider_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="上游资源商品 ID">{{ initial?.source_product_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="上游加价规则">
                {{ markupLabel(initial?.upstream_markup_type || '', initial?.upstream_markup_value || 0) }}
              </t-descriptions-item>
            </t-descriptions>
            <p class="upstream-readonly__hint">
              售价在「商品调价」或周期价格里操作；开通规格由上游决定，无需在此维护 SKU。
            </p>
          </div>
        </template>
      </t-alert>

      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="产品名称" name="name" :rules="[{ required: true, message: '产品名称不能为空' }]">
            <t-input v-model="form.name" placeholder="请输入产品名称" clearable />
          </t-form-item>
          <t-form-item label="分类" name="category_id">
            <t-select v-model="form.category_id" clearable placeholder="请选择分类" :options="categoryOptions" />
          </t-form-item>
          <t-form-item label="已售库存">
            <t-input-number v-model="form.stock" :min="-1" theme="column" placeholder="-1 表示不限" />
          </t-form-item>
          <t-form-item label="排序">
            <t-input-number v-model="form.sort_order" :min="0" theme="column" placeholder="数值越小越靠前" />
          </t-form-item>
          <t-form-item label="封面图" class="form-grid__full">
            <t-input v-model="form.cover_image" placeholder="选填，官网产品卡与详情页展示用" clearable />
          </t-form-item>
          <t-form-item label="产品描述" class="form-grid__full">
            <t-textarea v-model="form.description" :autosize="{ minRows: 3, maxRows: 8 }" placeholder="选填；导入时已自动带入上游描述，可在此改写" />
          </t-form-item>
        </div>

        <!-- 仅透传：上架门禁的显式放行项，语义务必写清楚 -->
        <t-form-item label="仅透传上游参数（上架门禁）">
          <t-switch v-model="form.spec_passthrough" />
          <span class="clone-tip" style="display: block; margin-top: 6px; line-height: 1.7">
            <b>这是什么：</b>客户下单时要把参数发给上游开通。开启「仅透传」= 我们<b>不解释、不改写</b>，
            直接把上游商品自己的配置项与取值原样转发过去（适合上游商品是整机固定配置、没有可选规格的情况）。<br>
            <b>不开启时：</b>上架前必须先确认「本地规格 → 上游开通参数」的对应关系（规格页签里完成平台绑定确认）。<br>
            <b>什么时候可以用：</b>上游商品规格固定、下单参数与上游商品一一对应、不会出现"本地改了参数但上游不认"的情况。
          </span>
        </t-form-item>
      </t-form>

      <div class="form-footer">
        <t-button variant="outline" @click="handleCancel">取消</t-button>
        <t-button theme="primary" :loading="submitting" @click="handleSubmitUpstream">保存基础信息</t-button>
      </div>
    </section>

    <section v-else class="form-card surface-card">
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="产品名称" name="name" :rules="[{ required: true, message: '产品名称不能为空' }]">
            <t-input v-model="form.name" placeholder="请输入产品名称" clearable />
          </t-form-item>
          <t-form-item label="SKU 编码" name="code" :rules="[{ required: mode === 'create', message: 'SKU 编码不能为空' }]">
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
          <t-form-item label="分类" name="category_id">
            <t-select v-model="form.category_id" clearable placeholder="请选择分类" :options="categoryOptions" />
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
          </t-form-item>
          <t-form-item label="库存" name="stock">
            <t-input-number v-model="form.stock" :min="-1" theme="column" placeholder="-1 表示不限" />
          </t-form-item>
          <t-form-item label="销售价（元）" name="price">
            <t-input-number v-model="form.price" :min="0" :precision="2" theme="column" placeholder="请输入销售价" />
            <span class="form-hint">规格模板自带参考售价时以模板为准，此价用于「仅本地/虚拟商品」。</span>
          </t-form-item>
          <t-form-item label="成本价（元）" name="cost_price">
            <t-input-number v-model="form.cost_price" :min="0" :precision="2" theme="column" placeholder="请输入成本价" />
          </t-form-item>

          <!--
            周期价格一键生成：只填「月付价 + 各周期优惠月数」，其余周期价按
            月价 × 月数 − 优惠月数 算出。手工逐周期算错价是常见事故（多一位少一位），
            且多周期商品一个一个填很费事。生成后可逐格微调。
          -->
          <t-form-item label="周期价格" class="form-grid__full">
            <div class="price-gen">
              <div class="price-gen__row">
                <label class="price-gen__field">
                  月付价（元）
                  <t-input-number v-model="priceGen.monthly" :min="0" :precision="2" theme="column" />
                </label>
                <label class="price-gen__field">
                  季付优惠（月）
                  <t-input-number v-model="priceGen.quarterlyOff" :min="0" :max="3" theme="column" />
                </label>
                <label class="price-gen__field">
                  半年付优惠（月）
                  <t-input-number v-model="priceGen.semiannuallyOff" :min="0" :max="6" theme="column" />
                </label>
                <label class="price-gen__field">
                  年付优惠（月）
                  <t-input-number v-model="priceGen.annuallyOff" :min="0" :max="12" theme="column" />
                </label>
                <t-button size="small" theme="primary" variant="outline" @click="generateCyclePrices">
                  一键生成
                </t-button>
              </div>
              <div v-if="cyclePrices.length" class="price-gen__result">
                <div class="price-gen__result-head">
                  生成结果（可逐格微调）
                  <t-link theme="primary" hover="color" @click="clearCyclePrices">清空</t-link>
                </div>
                <div class="price-gen__grid">
                  <label v-for="p in cyclePrices" :key="p.cycle" class="price-gen__cell">
                    <span class="price-gen__cell-label">{{ p.label }}</span>
                    <t-input-number v-model="p.amount" :min="0" :precision="2" theme="column" />
                    <span v-if="p.monthCount" class="price-gen__cell-hint">
                      {{ p.monthCount }} 个月
                      <template v-if="p.offMonths > 0"> · 省 {{ p.offMonths }} 个月</template>
                    </span>
                  </label>
                </div>
                <p class="price-gen__hint">
                  周期价只作用于该商品的默认规格；要按 SKU 分别定价，请在商品详情页的「周期价格」里维护。
                </p>
              </div>
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
          <t-input v-model="form.cover_image" placeholder="选填，官网产品卡与详情页展示用，如 /branding/logo.svg" clearable />
        </t-form-item>

        <div v-if="form.cover_image" class="cover-preview">
          <span class="cover-preview__label">封面预览</span>
          <img class="cover-preview__img" :src="form.cover_image" alt="封面预览">
        </div>

        <t-form-item label="产品描述" name="description">
          <t-textarea v-model="form.description" :autosize="{ minRows: 2, maxRows: 5 }" placeholder="选填，产品简介" />
        </t-form-item>

        <!-- ===== 平台参数：下拉选真实取值，不再手写 JSON ===== -->
        <t-divider>平台参数（商品级开通默认值）</t-divider>
        <t-alert
          theme="info"
          message="此处是商品级默认参数；下面每个 SKU 的绑定优先级更高。选好平台渠道后可下拉选取平台上真实存在的区域/节点/存储/镜像。"
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
            <span class="form-hint">此处是默认镜像；要让客户在多个镜像间自选，在下面规格行的「镜像 os」里多选。</span>
          </t-form-item>
        </div>

        <!--
          平台配置项默认值：以前这些默认值只存在于平台文档里，运营看不到就容易忽略，
          或重复手填与平台默认冲突。显式列出来，"不填会是什么"一眼可见。
        -->
        <template v-if="catalogDefaults.length">
          <div class="defaults-hint">
            平台默认取值（未显式设置时按这些值开通）
          </div>
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

        <!-- ===== 规格配置：建品时直接选模板生成 SKU（自营链路） ===== -->
        <template v-if="mode === 'create'">
          <t-divider>规格配置</t-divider>
          <t-form-item label="配置方式">
            <t-radio-group v-model="specMode" variant="default-filled">
              <t-radio-button value="template">按规格模板生成 SKU（推荐）</t-radio-button>
              <t-radio-button value="later">稍后在详情页配置</t-radio-button>
            </t-radio-group>
          </t-form-item>

          <template v-if="specMode === 'template'">
            <t-alert v-if="!templateRows.length" theme="warning" message="还没有规格模板。请先到「产品管理 → 规格管理 → 规格模板」新建模板并配好平台映射。" />
            <div v-else class="tpl-list">
              <div class="tpl-list__head">
                <span>勾选要生成的规格；每行可直接改 CPU/内存/系统盘/带宽与区域/节点/存储/镜像。</span>
                <t-space size="small">
                  <span class="tpl-list__count">已选 {{ selectedTemplateCount }} / {{ templateRows.length }}</span>
                  <t-link theme="primary" hover="color" @click="selectAllTemplates">全选</t-link>
                  <t-link theme="primary" hover="color" @click="clearAllTemplates">清空</t-link>
                </t-space>
              </div>
              <!--
                多选的影响要说清楚：勾 N 个档位 = 生成 N 个 SKU，而「客户可选配置项」
                是按**参数名跨档位合并**的（各档位开放的同名参数取并集）。
                不说清楚的话，运营会以为参数是逐档位隔离的，看到合并后的取值列表就乱了。
              -->
              <t-alert v-if="selectedTemplateCount > 1" theme="info" class="tpl-multi-tip">
                <template #message>
                  已选 <b>{{ selectedTemplateCount }}</b> 个档位 → 将生成 <b>{{ selectedTemplateCount }}</b> 个 SKU（每个档位 1 个）。
                  客户可选配置项按<b>参数名跨档位合并</b>：如同一参数在多个档位都开放，客户可选取值取并集；
                  各档位的<b>基线值</b>（CPU/内存/盘）仍按各自档位下发，互不影响。
                </template>
              </t-alert>
              <!-- 机型筛选：独立 IP 与 NAT 的访问方式不同，分开看避免混选 -->
              <div class="tpl-filter">
                <span class="tpl-filter__label">机型</span>
                <t-radio-group v-model="machineKindFilter" variant="default-filled" size="small">
                  <t-radio-button
                    v-for="opt in machineKindFilterOptions"
                    :key="opt.value"
                    :value="opt.value"
                  >{{ opt.label }}</t-radio-button>
                </t-radio-group>
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
                  >{{ grp.label }}</t-tag>
                  <span class="tpl-group__desc">{{ machineKindGroupDesc(grp.kind) }}</span>
                </div>
                <div v-for="row in grp.rows" :key="row.tpl.id" class="tpl-row" :class="{ 'tpl-row--on': row.selected }">
                <div class="tpl-row__head">
                  <t-checkbox v-model="row.selected" @change="onRowToggle(row)">
                    <span class="tpl-row__name">{{ row.tpl.name }}</span>
                  </t-checkbox>
                  <span class="tpl-row__meta">
                    {{ row.tpl.provider_type || '未绑定平台' }}
                    <template v-if="row.tpl.platform_params"> · 已配平台映射</template>
                    <template v-else> · <em class="tpl-row__warn">模板缺少平台映射</em></template>
                    <template v-if="row.options.length"> · {{ row.options.length }} 个可选参数</template>
                  </span>
                </div>
                <div v-if="row.selected" class="tpl-row__body">
                  <t-alert
                    v-if="isNatRow(row)"
                    theme="info"
                    class="tpl-row__notice"
                    message="NAT 机型不分配独立 IP，客户通过节点转发地址（IP:端口）访问；如需客户自己配置端口映射，请确认平台侧已开启 NAT 转发。"
                  />
                  <div class="tpl-row__fields">
                    <label class="tpl-field">CPU（核）<t-input-number v-model="row.cpu" :min="1" theme="column" /></label>
                    <label class="tpl-field">内存（GB）<t-input-number v-model="row.memoryGb" :min="1" theme="column" /></label>
                    <label class="tpl-field">系统盘（GB）<t-input-number v-model="row.disk" :min="1" theme="column" /></label>
                    <label class="tpl-field">带宽（Mbps）<t-input-number v-model="row.bandwidth" :min="0" theme="column" /></label>
                    <label class="tpl-field">销售价（元）<t-input-number v-model="row.price" :min="0" :precision="2" theme="column" /></label>
                  </div>
                  <div class="tpl-row__fields">
                    <label class="tpl-field">区域 area<t-select v-model="row.area" clearable placeholder="沿用模板" :options="areaOptions" /></label>
                    <label class="tpl-field">节点 node<t-select v-model="row.node" clearable placeholder="沿用模板" :options="nodeOptionsFor(row.area)" /></label>
                    <label class="tpl-field">存储 store<t-select v-model="row.store" clearable placeholder="沿用模板" :options="storeOptionsFor(row.area)" /></label>
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
                        placeholder="沿用模板 / 可多选"
                        :options="imageOptions"
                        :tag-props="{ maxWidth: '160px' }"
                        @change="onRowOsChange(row)"
                      />
                      <span v-if="row.osList.length > 1" class="tpl-field__hint">
                        默认开通用「{{ imageLabelOf(row.osList[0]) }}」，客户可选其余 {{ row.osList.length - 1 }} 个
                      </span>
                    </label>
                  </div>
                  <!-- 客户可选配置项：勾选=开放给客户自选；不勾=只用基线值 -->
                  <div v-if="row.options.length" class="tpl-options">
                    <div class="tpl-options__head">
                      客户可选配置项（勾选即开放，可设加价；不勾选只按基线值开通）
                    </div>
                    <div v-for="opt in row.options" :key="opt.key" class="tpl-opt">
                      <t-checkbox v-model="opt.enabled">
                        <span class="tpl-opt__name">{{ platformFieldLabel(opt.key) }}</span>
                        <code class="tpl-opt__key">{{ opt.key }}</code>
                      </t-checkbox>
                      <template v-if="opt.enabled">
                        <template v-if="opt.isRange">
                          <label class="tpl-field tpl-field--inline">
                            范围
                            <t-input-number v-model="opt.rangeMin" :min="0" theme="column" />
                            ~
                            <t-input-number v-model="opt.rangeMax" :min="0" theme="column" />
                          </label>
                          <label class="tpl-field tpl-field--inline">
                            默认
                            <t-input-number v-model="opt.defaultNumber" theme="column" />
                          </label>
                        </template>
                        <div v-else class="tpl-opt__values">
                          <div v-for="v in opt.values" :key="v.value" class="tpl-opt__value">
                            <t-radio :checked="v.isDefault" @change="setDefaultValue(opt, v.value)" />
                            <span class="tpl-opt__label">{{ v.group ? v.group + ' / ' : '' }}{{ v.label }}</span>
                            <t-input-number v-model="v.price" :min="0" :precision="2" theme="column" class="tpl-opt__price" />
                            <span class="tpl-opt__unit">元/月</span>
                          </div>
                        </div>
                      </template>
                    </div>
                  </div>
                  <div class="tpl-row__gen">
                    SKU 编码 <code>{{ row.tpl.id ? previewSpecCode(row) : '—' }}</code> · 原子取值
                    <code>{{ JSON.stringify(buildSpecValues(row)) }}</code>
                  </div>
                </div>
                </div>
              </div>
            </div>
          </template>
          <t-alert v-else theme="info" message="商品建好后，到详情页「规格变体 → 按规格模板生成」逐个生成；未生成 SKU 前无法上架。" />
        </template>

        <t-alert v-else theme="info">
          <template #message>
            规格在商品详情页「规格变体」中维护；本页只改基础信息与平台参数。
          </template>
        </t-alert>

        <div class="form-footer">
          <t-button variant="outline" @click="handleCancel">取消</t-button>
          <t-button theme="primary" :loading="submitting" @click="handleSubmit">{{ mode === 'create' ? '创建' : '保存' }}</t-button>
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
import { getSpecTemplateList, getOptionCatalog } from '@/api/product'
import {
  billingCycleLabel,
  markupLabel,
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
}>()

const emit = defineEmits<{
  // cyclePrices：周期价格矩阵（月付/季付/半年付/年付 → 金额）。创建接口不收，
  // 由父页在拿到商品 id 后写价格矩阵接口。
  (e: 'submit', payload: SaleProductCreateRequest, cyclePrices?: Record<string, number>): void
  (e: 'cancel'): void
}>()

/** 代理（上游转售）商品：链路由导入决定，表单不提供链路选择。 */
const isUpstream = computed(() => props.initial?.source_mode === 'upstream')

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
}

// ===== 商品级平台参数（后端字段 config_options，这里用下拉 + 高级 JSON 组合编辑）=====
const platformParams = reactive<{ area?: string; node?: string; store?: string; os?: string }>({
  area: undefined, node: undefined, store: undefined, os: undefined,
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

async function loadCatalogDefaults(providerType: string) {
  catalogDefaults.value = []
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
  } catch {
    // 目录读不到不影响建品：默认值为辅助信息，非必需。
    catalogDefaults.value = []
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
  values: { value: string; label: string; group: string; price: number; isDefault: boolean }[]
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
const hasMixedKinds = computed(
  () => selectedKinds.value.has('vps') && selectedKinds.value.has('nat'),
)

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

/** 由档位 option_selections 展开客户可选配置项（基线取值落到 enabled=false 的行）。 */
function buildOptionRows(tpl: SpecTemplateInfo): OptionRow[] {
  const sel = (tpl.option_selections || {}) as Record<
    string,
    { values?: string[]; range?: number[]; default?: string; group_label?: string }
  >
  const out: OptionRow[] = []
  for (const [key, cfg] of Object.entries(sel)) {
    if (Array.isArray(cfg?.range) && cfg.range.length === 2) {
      const defStr = cfg.default ?? ''
      const defNum = Number(defStr)
      out.push({
        key, enabled: true, isRange: true,
        rangeMin: cfg.range[0], rangeMax: cfg.range[1],
        defaultNumber: defStr !== '' && Number.isFinite(defNum) ? defNum : null,
        values: [],
      })
      continue
    }
    const values = (cfg?.values || []).map((v) => ({
      value: v,
      label: v,
      group: cfg?.group_label || '',
      price: 0,
      isDefault: cfg?.default === v,
    }))
    if (values.length) out.push({ key, enabled: true, isRange: false, rangeMin: 0, rangeMax: 0, defaultNumber: null, values })
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
        values: [],
      })
      continue
    }
    const values = opt.values.map((v) => ({
      value: v.value,
      label: v.label,
      group_label: v.group || undefined,
      is_default: v.isDefault,
      price_monthly: v.price,
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

// ===== 周期价格一键生成 =====
// 规则：周期价 = 月付价 × 月数 − 优惠月数 × 月付价（即"优惠 N 个月"）。
// 优惠月数可超过可用区间时按区间上限收敛，避免算出负数或倒挂价。
const priceGen = reactive({
  monthly: 0,
  quarterlyOff: 0,
  semiannuallyOff: 1,
  annuallyOff: 2,
})

type CyclePrice = {
  cycle: string
  label: string
  amount: number
  monthCount: number
  offMonths: number
}

const cyclePrices = ref<CyclePrice[]>([])

function generateCyclePrices() {
  const m = Number(priceGen.monthly)
  if (!Number.isFinite(m) || m <= 0) {
    MessagePlugin.warning('请先填入大于 0 的月付价')
    return
  }
  // 优惠月数不能吃掉整个周期（否则价格为 0 或负），上限为该周期月数 − 1，
  // 留至少一个月不优惠，避免生成出 0 元周期价——0 元价上架后客户真能 0 元买。
  const clampOff = (off: number, months: number) =>
    Math.max(0, Math.min(Number(off) || 0, months - 1))
  const rows: { cycle: string; months: number; off: number }[] = [
    { cycle: 'monthly', months: 1, off: 0 },
    { cycle: 'quarterly', months: 3, off: clampOff(priceGen.quarterlyOff, 3) },
    { cycle: 'semiannually', months: 6, off: clampOff(priceGen.semiannuallyOff, 6) },
    { cycle: 'annually', months: 12, off: clampOff(priceGen.annuallyOff, 12) },
  ]
  cyclePrices.value = rows.map((r) => ({
    cycle: r.cycle,
    label: billingCycleLabel(r.cycle),
    amount: Math.round(m * (r.months - r.off) * 100) / 100,
    monthCount: r.months,
    offMonths: r.off,
  }))
}

function clearCyclePrices() {
  cyclePrices.value = []
}

/** 周期价 → 提交用的「周期 → 金额」映射（未生成的周期不带）。 */
function cyclePriceMap(): Record<string, number> {
  const out: Record<string, number> = {}
  for (const p of cyclePrices.value) {
    if (p.amount > 0) out[p.cycle] = p.amount
  }
  return out
}

async function loadTemplates() {
  try {
    const data = await getSpecTemplateList({ page: 1, page_size: 100, status: 1 })
    templateRows.value = data.items.map(buildRow)
  } catch {
    templateRows.value = []
  }
}

/** 勾选时把区域补成商品级区域，省得运营每行都选一遍。 */
function onRowToggle(row: TemplateRow) {
  if (row.selected && !row.area && platformParams.area) row.area = platformParams.area
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

// ===== SKU 编码自动生成 =====
// 编码是给系统与上游看的标识（不是展示名），手工想一个既费神又容易重名。
// 这里按「产品类型 + 中文名转写」自动生成，运营可随时手改；
// 一旦手改过就不再覆盖（见 codeTouched），否则改个名字把人工编码冲掉更糟。
const codeTouched = ref(false)

/** 中文/符号 → 拉丁化的简易转写表：覆盖常见商品名字词，转不出来就跳过。 */
const codeWordMap: Record<string, string> = {
  香港: 'hk', 新加坡: 'sg', 日本: 'jp', 美国: 'us', 韩国: 'kr', 台湾: 'tw',
  华东: 'east', 华南: 'south', 华北: 'north', 华中: 'central', 西部: 'west',
  云服务器: 'cloud', 云主机: 'cloud', 高防: 'ddos', 独立: 'dedicated',
  站群: 'seo', 虚拟主机: 'vhost', 数据库: 'db', 物理机: 'bare',
  入门: 'basic', 基础: 'basic', 标准: 'std', 进阶: 'pro', 高级: 'adv',
  旗舰: 'ultra', 豪华: 'premium', 定制: 'custom', 试用: 'trial',
  核: 'c', 内存: 'mem', 带宽: 'bw', 系统盘: 'disk', 年付: 'y', 月付: 'm',
}

/**
 * generateProductCode 由产品名生成候选 SKU 编码。
 * 规则：产品类型前缀 + 名称转写；无可用转写时退回带时间戳的前缀码，保证非空且大概率不重名。
 */
function generateProductCode(): string {
  const prefix = form.product_type === 'cloud_host' ? 'ch' : (form.product_type || 'prod')
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
  return `${prefix}-${slug}`
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
  form.product_type === 'cloud_host'
    ? [{ required: true, message: '请选择开通该商品的平台渠道' }]
    : [],
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
    if (form.source_provider_id) await loadPlatformResources(form.source_provider_id)
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
  // 自营专属 payload：链路恒为 self，不携带任何上游转售字段（source_product_id / 加价 / 仅透传）。
  return {
    code: props.mode === 'create' ? form.code.trim() : form.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    price_model: form.price_model,
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
 * 代理商品只允许改基础信息：链路与上游绑定字段原样回传（后端 Update 对未传字段保持原值，
 * 这里显式回传 source_mode 是幂等的，避免"编辑一次就被改成自营"的历史缺陷）。
 */
function handleSubmitUpstream() {
  if (!props.initial) return
  if (!form.name.trim()) {
    MessagePlugin.warning('请输入产品名称')
    return
  }
  const configOptions = composeConfigOptions()
  if (configOptions === null) return
  const payload: SaleProductCreateRequest = {
    code: props.initial.code,
    name: form.name.trim(),
    category_id: form.category_id || 0,
    product_type: form.product_type,
    description: form.description,
    cover_image: form.cover_image,
    price_model: form.price_model,
    price: form.price,
    cost_price: form.cost_price,
    source_mode: 'upstream',
    source_product_id: props.initial.source_product_id,
    source_provider_id: props.initial.source_provider_id,
    config_options: configOptions,
    stock: form.stock,
    sort_order: form.sort_order,
    status: form.status,
    upstream_markup_type: props.initial.upstream_markup_type || '',
    upstream_markup_value: props.initial.upstream_markup_value || 0,
    // 仅透传是上架门禁开关：表单里可改（此前的实现恒取初值，改不动）
    spec_passthrough: !!form.spec_passthrough,
  }
  emit('submit', payload)
}

onMounted(() => {
  loadCategories()
  loadPlatformOptions()
  if (props.mode === 'create') void loadTemplates()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.upstream-readonly {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.upstream-readonly__hint {
  margin: 0;
  color: var(--td-text-color-secondary, #999);
  font-size: 13px;
}

.tpl-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}

.tpl-list__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--td-text-color-secondary, #666);
}

.tpl-list__count {
  font-weight: 600;
  color: var(--td-text-color-primary, #333);
}

/* 机型筛选与分组标题：独立 IP 与 NAT 档位的访问方式不同，分开看避免混选。 */
.tpl-filter {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.tpl-filter__label {
  font-size: 13px;
  color: var(--td-text-color-secondary, #666);
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
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
  margin-bottom: 6px;
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

.price-gen__result {
  border: 1px solid var(--td-component-border, #e0e0e0);
  border-radius: 6px;
  padding: 10px;
  background: var(--td-bg-color-container, #fff);
}

.price-gen__result-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12.5px;
  font-weight: 600;
  margin-bottom: 8px;
}

.price-gen__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
}

.price-gen__cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.price-gen__cell-label {
  font-size: 12.5px;
  font-weight: 600;
}

.price-gen__cell-hint {
  font-size: 11.5px;
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
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 4px 0;
}

.tpl-opt code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}

.tpl-opt__values {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.tpl-opt__value {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.tpl-opt__price {
  width: 110px;
}

.tpl-opt__unit {
  font-size: 12px;
  color: var(--td-text-color-secondary, #888);
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
