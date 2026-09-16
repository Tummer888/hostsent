<template>
  <t-dialog
    v-model:visible="visible"
    header="调整账户余额"
    width="480px"
    :confirm-btn="{ content: '确认提交', theme: 'primary', loading: submitting }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
      <t-form-item label="用户">
        <t-input :model-value="username || '—'" disabled />
      </t-form-item>
      <t-form-item label="当前余额">
        <t-input :model-value="`¥${formatAmount(currentBalance)}`" disabled />
      </t-form-item>
      <t-form-item label="调整方向" name="direction">
        <t-radio-group v-model="form.direction" variant="default-filled">
          <t-radio-button :value="1">增加余额（充值/补偿）</t-radio-button>
          <t-radio-button :value="-1">扣减余额（追回/纠正）</t-radio-button>
        </t-radio-group>
      </t-form-item>
      <t-form-item label="金额（元）" name="amount">
        <t-input-number v-model="form.amount" :min="0.01" :precision="2" theme="column" placeholder="请输入金额" />
      </t-form-item>
      <t-form-item label="备注" name="remark">
        <t-textarea
          v-model="form.remark"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="建议填写调账原因，将记入资金流水"
        />
      </t-form-item>
      <p class="form-hint">
        提交后不可撤销，将按「{{ form.direction === 1 ? '收入' : '支出' }}」写入 wallet_transactions 并同步更新用户余额。
      </p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import { rechargeUser } from '@/api/user'
import { formatAmount } from '@/pages/users/constants'

const props = defineProps<{
  modelValue: boolean
  userId: number
  username: string
  currentBalance: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = ref(props.modelValue)
watch(() => props.modelValue, (value) => {
  visible.value = value
  if (value) {
    form.direction = 1
    form.amount = 0
    form.remark = ''
  }
})
watch(visible, (value) => emit('update:modelValue', value))

const formRef = ref<FormInstanceFunctions>()
const submitting = ref(false)
const form = reactive<{ direction: number; amount: number; remark: string }>({
  direction: 1,
  amount: 0,
  remark: '',
})

const rules: Record<string, FormRule[]> = {
  amount: [{ required: true, message: '请输入调整金额', type: 'error' }],
}

// 说明文案随方向变化，避免出现「负数的充值」这种含糊表达。
const directionHint = computed(() => (form.direction === 1 ? '收入' : '支出'))

function close() {
  visible.value = false
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  if (!form.amount || form.amount <= 0) {
    MessagePlugin.warning('金额必须大于 0')
    return
  }
  submitting.value = true
  try {
    // 后端按符号判定方向：正数入账、负数扣减。
    await rechargeUser(props.userId, {
      amount: form.direction === 1 ? form.amount : -form.amount,
      remark: form.remark || `后台人工${directionHint.value}`,
    })
    MessagePlugin.success(`已${form.direction === 1 ? '增加' : '扣减'} ¥${formatAmount(form.amount)}`)
    visible.value = false
    emit('saved')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '调账失败')
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
</style>