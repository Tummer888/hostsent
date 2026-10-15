<template>
  <div class="page-body order-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="20" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">退款策略</h2>
          <p class="page-header__desc">按「商品来源 + 分类」配置可退时限与免审核自动退款</p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button theme="primary" @click="openCreate">
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新建策略
        </t-button>
      </t-space>
    </header>

    <!-- 规则说明：匹配顺序与默认口径写清楚，避免运营误以为"没配就是不限" -->
    <t-alert theme="info" class="policy-alert">
      <template #title>匹配规则</template>
      <template #message>
        <p class="policy-alert__line">
          ① 先按「商品来源 + 商品所属分类」逐级向上找，**分类越具体优先级越高**（例如「云主机/香港」优先于「云主机」）；
        </p>
        <p class="policy-alert__line">② 未命中则回落「同来源 + 全部分类」；</p>
        <p class="policy-alert__line">
          ③ **都没有命中时，该商品不支持客户自助申请退款**（运营仍可在订单列表手动发起）。
        </p>
        <p class="policy-alert__line">
          ④ 可退时限自「订单开通成功时间」起算；未开通成功的订单不受时限约束，但一律走人工审核。
        </p>
        <p class="policy-alert__line">
          ⑤ 原路退回（有渠道支付记录时可选）目前无自动退款接口，**一律人工审核**并需上传凭据。
        </p>
        <p class="policy-alert__line">
          ⑥ 退款生效后是否暂停/销毁实例由**生命周期策略**决定（实例管理 → 生命周期策略 → 退款处置）。
        </p>
      </template>
    </t-alert>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">策略列表</h3>
        <span class="table-card__meta">共 {{ policies.length }} 条</span>
      </div>
      <t-table
        row-key="id"
        :data="policies"
        :columns="columns"
        :loading="loading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #source_mode="{ row }">
          <t-tag
            v-if="row.source_mode"
            :theme="sourceModeTag(row.source_mode).theme"
            variant="light"
            size="small"
            shape="round"
          >
            {{ sourceModeTag(row.source_mode).text }}
          </t-tag>
          <t-tag v-else theme="default" variant="light" size="small" shape="round">不限来源</t-tag>
        </template>

        <template #category_id="{ row }">
          <span v-if="row.category_id === 0" class="cell-muted">全部商品</span>
          <span v-else class="cell-strong">{{ row.category_name || `#${row.category_id}` }}（含子分类）</span>
        </template>

        <template #refundable="{ row }">
          <t-tag :theme="row.refundable ? 'success' : 'danger'" variant="light" size="small" shape="round">
            {{ row.refundable ? '可自助申请' : '不可退' }}
          </t-tag>
        </template>

        <template #window_hours="{ row }">
          <span v-if="row.refundable">
            {{ row.window_hours > 0 ? `${row.window_hours} 小时内` : '不限时' }}
          </span>
          <span v-else class="cell-muted">—</span>
        </template>

        <template #auto_approve="{ row }">
          <t-tag v-if="row.auto_approve" theme="primary" variant="light" size="small" shape="round">免审核自动退</t-tag>
          <span v-else class="cell-muted">人工审核</span>
        </template>

        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '停用' }}
          </t-tag>
        </template>

        <template #action="{ row }">
          <div class="action-cell">
            <t-link theme="primary" hover="color" @click="openEdit(row)">编辑</t-link>
            <t-link theme="danger" hover="color" @click="handleDelete(row)">删除</t-link>
          </div>
        </template>

        <template #empty>
          <t-empty description="暂无退款策略（未配置时客户无法自助申请退款）" />
        </template>
      </t-table>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      :header="editing ? '编辑退款策略' : '新建退款策略'"
      width="560px"
      :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSave"
      @close="dialogVisible = false"
    >
      <t-form label-align="top" :data="form" @submit.prevent>
        <t-form-item label="商品来源" name="source_mode">
          <t-select v-model="form.source_mode" :options="sourceOptions" placeholder="不限来源（兜底）" />
        </t-form-item>
        <t-form-item label="适用分类" name="category_id">
          <t-select
            v-model="form.category_id"
            :options="categorySelectOptions"
            filterable
            placeholder="全部分类（该来源下所有商品）"
          />
          <p class="form-hint">选具体分类时对**该分类及其子分类**生效；分类越具体优先级越高。</p>
        </t-form-item>
        <t-form-item label="是否允许客户自助申请退款" name="refundable">
          <t-switch v-model="form.refundable" />
          <p class="form-hint">关闭表示该范围内商品「买了不退」，客户侧不展示申请入口。</p>
        </t-form-item>
        <t-form-item v-if="form.refundable" label="可退时限（小时）" name="window_hours">
          <t-input-number v-model="form.window_hours" :min="0" :max="8760" theme="column" />
          <p class="form-hint">自「订单开通成功」起算；填 0 表示不限时。默认 24 小时。</p>
        </t-form-item>
        <t-form-item
          v-if="form.refundable && form.source_mode !== 'upstream'"
          label="免审核自动退款"
          name="auto_approve"
        >
          <t-switch v-model="form.auto_approve" />
          <p class="form-hint">
            开启后，命中策略且为「退回余额 + 全额 + 在时限内」的申请将自动退款，
            之后仍需财务在退款列表做一次事后复核。原路退回永远走人工审核。
          </p>
        </t-form-item>
        <t-form-item label="状态" name="status">
          <t-radio-group v-model="form.status">
            <t-radio-button value="active">启用</t-radio-button>
            <t-radio-button value="disabled">停用</t-radio-button>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea
            v-model="form.remark"
            :autosize="{ minRows: 2, maxRows: 3 }"
            placeholder="选填，例如：自营云主机默认 24 小时无理由"
          />
        </t-form-item>
      </t-form>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'

import { AddIcon, RefreshIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { createRefundPolicy, deleteRefundPolicy, getRefundPolicies, updateRefundPolicy } from '@/api/order'
import { sourceModeTag } from '@/pages/product/constants'
import type { RefundPolicyInfo, RefundPolicyRequest } from '@/types/interface'
import { useCategoryOptions } from '@/composables/useCategoryOptions'

defineOptions({ name: 'OrderRefundPolicy' })

const { categoryOptions, loadCategories } = useCategoryOptions()

const policies = ref<RefundPolicyInfo[]>([])
const loading = ref(false)

const columns: PrimaryTableCol<RefundPolicyInfo>[] = [
  { colKey: 'source_mode', title: '商品来源', width: 120 },
  { colKey: 'category_id', title: '适用分类', minWidth: 180 },
  { colKey: 'refundable', title: '可自助申请', width: 120 },
  { colKey: 'window_hours', title: '可退时限', width: 120 },
  { colKey: 'auto_approve', title: '审核方式', width: 140 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'remark', title: '备注', minWidth: 160 },
  { colKey: 'updated_at', title: '更新时间', width: 170 },
  { colKey: 'action', title: '操作', width: 110, fixed: 'right' as const, align: 'center' as const },
]

// 来源下拉：不选 = 不限（兜底行）。
const sourceOptions = [
  { label: '不限来源（兜底）', value: '' },
  { label: '自营', value: 'self' },
  { label: '上游转售', value: 'upstream' },
]

// 分类下拉：0 = 全部商品（该来源下）。
const categorySelectOptions = computed(() => [
  { label: '全部分类（该来源下所有商品）', value: 0 },
  ...categoryOptions.value,
])

async function load() {
  loading.value = true
  try {
    const data = await getRefundPolicies()
    policies.value = data.items || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载退款策略失败')
  } finally {
    loading.value = false
  }
}

const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref<RefundPolicyInfo | null>(null)
const form = reactive<RefundPolicyRequest>({
  source_mode: '',
  category_id: 0,
  refundable: true,
  window_hours: 24,
  auto_approve: false,
  status: 'active',
  sort_order: 0,
  remark: '',
})

function openCreate() {
  editing.value = null
  Object.assign(form, {
    source_mode: '',
    category_id: 0,
    refundable: true,
    window_hours: 24,
    auto_approve: false,
    status: 'active',
    sort_order: 0,
    remark: '',
  })
  dialogVisible.value = true
}

function openEdit(row: RefundPolicyInfo) {
  editing.value = row
  Object.assign(form, {
    source_mode: row.source_mode || '',
    category_id: row.category_id,
    refundable: row.refundable,
    window_hours: row.window_hours,
    auto_approve: row.auto_approve,
    status: row.status,
    sort_order: row.sort_order,
    remark: row.remark || '',
  })
  dialogVisible.value = true
}

async function handleSave() {
  saving.value = true
  try {
    // 不可退时清掉时限与免审开关：留着会在列表上显示成误导性的"24 小时内"。
    const payload: RefundPolicyRequest = { ...form }
    if (!payload.refundable) {
      payload.window_hours = 0
      payload.auto_approve = false
    }
    // 上游转售不允许配免审（一律人工审核）。
    if (payload.source_mode === 'upstream') {
      payload.auto_approve = false
    }
    if (editing.value) {
      await updateRefundPolicy(editing.value.id, payload)
    } else {
      await createRefundPolicy(payload)
    }
    MessagePlugin.success(editing.value ? '策略已更新' : '策略已创建')
    dialogVisible.value = false
    load()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

function handleDelete(row: RefundPolicyInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除退款策略',
    body: `确认删除该策略（${row.source_mode ? sourceModeTag(row.source_mode).text : '不限来源'} / ${row.category_id === 0 ? '全部商品' : row.category_name || `#${row.category_id}`}）？删除后该范围将回到「不可自助申请」。`,
    confirmBtn: { content: '删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      try {
        await deleteRefundPolicy(row.id)
        MessagePlugin.success('策略已删除')
        dialog.destroy()
        load()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      }
    },
    onClose: () => dialog.destroy(),
  })
}

onMounted(() => {
  void loadCategories()
  void load()
})
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped>
.policy-alert {
  margin-bottom: var(--space-lg);
}

.policy-alert__line {
  margin: 2px 0;
  font-size: 12px;
  line-height: 1.7;
}
</style>
