<template>
  <div class="payment-credential-fields">
    <div class="payment-credential-fields__title">凭证配置</div>
    <t-alert
      v-if="!fields.length"
      theme="info"
      :message="emptyHint"
      class="payment-credential-fields__alert"
    />
    <div v-else class="form-grid">
      <t-form-item
        v-for="field in fields"
        :key="field.key"
        :label="field.secret ? `${field.label}（加密存储）` : field.label"
        :name="`credentials.${field.key}`"
        :class="{ 'form-item--full': field.type === 'textarea' }"
      >
        <t-textarea
          v-if="field.type === 'textarea'"
          v-model="model[field.key]"
          :placeholder="field.placeholder || `请输入${field.label}`"
          :autosize="{ minRows: 2, maxRows: 4 }"
        />
        <t-select
          v-else-if="field.type === 'select'"
          v-model="model[field.key]"
          :options="selectOptions(field)"
          :placeholder="field.placeholder || `请选择${field.label}`"
          clearable
        />
        <t-switch
          v-else-if="field.type === 'bool'"
          v-model="boolModel[field.key]"
        />
        <t-input-number
          v-else-if="field.type === 'number'"
          v-model="numberModel[field.key]"
          :placeholder="field.placeholder || `请输入${field.label}`"
        />
        <t-input
          v-else
          v-model="model[field.key]"
          :type="field.type === 'password' ? 'password' : 'text'"
          :placeholder="field.placeholder || `请输入${field.label}`"
        />
        <p v-if="field.help" class="field-help">{{ field.help }}</p>
      </t-form-item>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'

import type { PaymentField } from '@/types/interface'

const props = withDefaults(
  defineProps<{
    /** 渠道能力描述符中的凭证字段声明 */
    fields?: PaymentField[] | null
    /** 凭证键值（编辑态为脱敏回显；新增态为空） */
    modelValue?: Record<string, string>
    /** 无字段声明时的提示语（假渠道等） */
    emptyHint?: string
  }>(),
  {
    fields: null,
    modelValue: () => ({}),
    emptyHint: '该渠道类型未声明凭证字段，可先创建后补充。',
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, string>): void
}>()

const model = reactive<Record<string, string>>({})
const boolModel = reactive<Record<string, boolean>>({})
const numberModel = reactive<Record<string, number | undefined>>({})

function selectOptions(field: PaymentField) {
  return (field.options || []).map((opt) => ({ label: opt.label, value: opt.value }))
}

// 外部 modelValue 变化（打开编辑弹窗/切换渠道类型）时重建本地表单值。
watch(
  () => props.modelValue,
  (value) => {
    const next = value || {}
    for (const key of Object.keys(model)) {
      if (!(key in next)) delete model[key]
    }
    for (const field of props.fields || []) {
      const raw = next[field.key]
      if (field.type === 'bool') {
        boolModel[field.key] = raw === 'true' || raw === '1'
        model[field.key] = boolModel[field.key] ? 'true' : 'false'
        continue
      }
      if (field.type === 'number') {
        const parsed = raw === undefined || raw === '' ? undefined : Number(raw)
        numberModel[field.key] = Number.isNaN(parsed) ? undefined : parsed
        model[field.key] = numberModel[field.key] === undefined ? '' : String(numberModel[field.key])
        continue
      }
      model[field.key] = raw ?? field.default ?? ''
    }
  },
  { immediate: true, deep: true },
)

// 控件本地绑定回写到 model，并统一以字符串形式对外暴露（后端 credentials 为 map[string]string）。
watch(boolModel, () => {
  for (const field of props.fields || []) {
    if (field.type !== 'bool') continue
    model[field.key] = boolModel[field.key] ? 'true' : 'false'
  }
})

watch(numberModel, () => {
  for (const field of props.fields || []) {
    if (field.type !== 'number') continue
    const value = numberModel[field.key]
    model[field.key] = value === undefined || value === null ? '' : String(value)
  }
})

watch(
  model,
  () => {
    emit('update:modelValue', { ...model })
  },
  { deep: true },
)

const fields = computed(() => props.fields || [])
</script>

<style scoped lang="css">
.payment-credential-fields__title {
  margin: var(--space-md) 0 var(--space-sm);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-foreground);
}

.payment-credential-fields__alert {
  margin-bottom: var(--space-md);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.form-item--full {
  grid-column: 1 / -1;
}

.field-help {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-muted-foreground);
}

@media (max-width: 768px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
