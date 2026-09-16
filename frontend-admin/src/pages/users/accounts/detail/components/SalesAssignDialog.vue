<template>
  <t-dialog
    v-model:visible="visible"
    header="归属销售"
    width="560px"
    :confirm-btn="{ content: '保存归属', theme: 'primary', loading: submitting }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
      <t-form-item label="客户">
        <t-input :model-value="username || '—'" disabled />
      </t-form-item>
      <t-form-item label="当前归属">
        <t-input :model-value="currentOwnerText" disabled />
        <div v-if="relations.length" class="relation-history">
          <div v-for="item in relations" :key="item.id" class="relation-item">
            <t-tag :theme="item.status === 'active' ? 'success' : 'default'" variant="light" size="small" shape="round">
              {{ item.status === 'active' ? '生效中' : '已解除' }}
            </t-tag>
            <span class="relation-text">
              {{ item.admin_real_name || item.admin_name || item.admin_id }}
              <template v-if="item.department_name"> · {{ item.department_name }}</template>
              · {{ formatDateTime(item.effective_at) }}
            </span>
          </div>
        </div>
      </t-form-item>
      <t-form-item label="目标销售" name="admin_id">
        <t-select
          v-model="form.admin_id"
          :options="candidateOptions"
          :loading="candidatesLoading"
          filterable
          clearable
          placeholder="留空表示解除归属"
        />
      </t-form-item>
      <t-form-item label="变更原因" name="reason">
        <t-textarea
          v-model="form.reason"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="必填且不少于 5 个字，将记入归属变更记录"
        />
      </t-form-item>
      <p class="form-hint">
        归属变更只影响<strong>之后</strong>产生的订单；已成交订单的归属快照不变。
      </p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import {
  assignCustomer,
  getCustomerRelations,
  getSalesCandidates,
  releaseCustomer,
} from '@/api/sales'
import { formatDateTime } from '@/pages/users/constants'
import type { SalesRelationInfo } from '@/types/interface'

const props = defineProps<{
  modelValue: boolean
  userId: number
  username: string
  salesAdminId: number
  salesAdminName: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = ref(props.modelValue)
const formRef = ref<FormInstanceFunctions>()
const submitting = ref(false)
const candidatesLoading = ref(false)
const candidateOptions = ref<{ label: string; value: number }[]>([])
const relations = ref<SalesRelationInfo[]>([])
const form = reactive<{ admin_id: number | undefined; reason: string }>({ admin_id: undefined, reason: '' })

const rules: Record<string, FormRule[]> = {
  reason: [
    { required: true, message: '请填写变更原因', type: 'error' },
    { min: 5, message: '变更原因不少于 5 个字', type: 'error' },
  ],
}

const currentOwnerText = computed(() =>
  props.salesAdminId ? props.salesAdminName || `ID ${props.salesAdminId}` : '未归属',
)

watch(() => props.modelValue, (value) => {
  visible.value = value
  if (value) {
    form.admin_id = props.salesAdminId > 0 ? props.salesAdminId : undefined
    form.reason = ''
    relations.value = []
    void loadCandidates()
    void loadRelations()
  }
})
watch(visible, (value) => emit('update:modelValue', value))

async function loadCandidates() {
  candidatesLoading.value = true
  try {
    const data = await getSalesCandidates()
    candidateOptions.value = (data.items || []).map((item) => ({
      label: `${item.real_name || item.username}${item.department_name ? `（${item.department_name}）` : ''} · 在管 ${item.active_customers}`,
      value: item.admin_id,
    }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载销售候选失败')
  } finally {
    candidatesLoading.value = false
  }
}

async function loadRelations() {
  try {
    const data = await getCustomerRelations(props.userId)
    relations.value = (data.items || []).slice(0, 5)
  } catch {
    relations.value = []
  }
}

function close() {
  visible.value = false
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  submitting.value = true
  try {
    if (form.admin_id) {
      await assignCustomer({ user_id: props.userId, admin_id: form.admin_id, reason: form.reason })
      MessagePlugin.success('归属已更新')
    } else {
      await releaseCustomer({ user_id: props.userId, reason: form.reason })
      MessagePlugin.success('已解除销售归属')
    }
    visible.value = false
    emit('saved')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.form-hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.relation-history {
  margin-top: var(--space-sm);
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.relation-item {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.relation-text {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>