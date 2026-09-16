<template>
  <t-dialog
    v-model:visible="visible"
    header="重置登录密码"
    width="440px"
    :confirm-btn="{ content: '确认重置', theme: 'primary', loading: submitting }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
      <t-form-item label="用户">
        <t-input :model-value="username || '—'" disabled />
      </t-form-item>
      <t-form-item label="新密码" name="password">
        <t-input v-model="form.password" type="password" placeholder="至少 8 位，建议含字母与数字" />
      </t-form-item>
      <t-form-item label="确认密码" name="confirm">
        <t-input v-model="form.confirm" type="password" placeholder="再次输入新密码" />
      </t-form-item>
      <p class="form-hint">重置后该用户的现有会话不会被强制下线，如需立即生效请到「安全与登录」中强制下线会话。</p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import { resetUserPassword } from '@/api/user'

const props = defineProps<{
  modelValue: boolean
  userId: number
  username: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const visible = ref(props.modelValue)
watch(() => props.modelValue, (value) => {
  visible.value = value
  if (value) {
    form.password = ''
    form.confirm = ''
  }
})
watch(visible, (value) => emit('update:modelValue', value))

const formRef = ref<FormInstanceFunctions>()
const submitting = ref(false)
const form = reactive({ password: '', confirm: '' })

const rules: Record<string, FormRule[]> = {
  password: [
    { required: true, message: '请输入新密码', type: 'error' },
    { min: 8, message: '密码至少 8 位', type: 'error' },
  ],
  confirm: [
    { required: true, message: '请再次输入新密码', type: 'error' },
    {
      validator: (value: unknown) => String(value) === form.password,
      message: '两次输入的密码不一致',
      type: 'error',
    },
  ],
}

function close() {
  visible.value = false
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  submitting.value = true
  try {
    await resetUserPassword(props.userId, { password: form.password })
    MessagePlugin.success('密码已重置')
    visible.value = false
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重置失败')
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