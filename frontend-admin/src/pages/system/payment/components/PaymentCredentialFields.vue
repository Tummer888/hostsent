<template>
  <!--
    兼容门面：动态凭证表单已于 doc91 C8 抽到 components/credential-fields/index.vue，
    供支付渠道 / 通知渠道 / 验证码服务商三处共用（同一份描述符渲染逻辑）。
    本文件保留旧名与旧默认标题，避免改动既有支付页调用方。
  -->
  <CredentialFields
    :fields="fields"
    :model-value="modelValue"
    :empty-hint="emptyHint"
    :title="title"
    @update:model-value="(v) => emit('update:modelValue', v)"
  />
</template>

<script setup lang="ts">
import CredentialFields from '@/components/credential-fields/index.vue'
import type { PaymentField } from '@/types/interface'

withDefaults(
  defineProps<{
    /** 渠道能力描述符中的凭证字段声明 */
    fields?: PaymentField[] | null
    /** 凭证键值（编辑态为脱敏回显；新增态为空） */
    modelValue?: Record<string, string>
    /** 无字段声明时的提示语（假渠道等） */
    emptyHint?: string
    /** 分组标题 */
    title?: string
  }>(),
  {
    fields: null,
    modelValue: () => ({}),
    emptyHint: '该渠道类型未声明凭证字段，可先创建后补充。',
    title: '凭证配置',
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, string>): void
}>()
</script>
