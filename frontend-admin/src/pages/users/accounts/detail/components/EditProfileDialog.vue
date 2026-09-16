<template>
  <t-dialog
    v-model:visible="visible"
    header="编辑用户资料"
    width="560px"
    :confirm-btn="{ content: '保存', theme: 'primary', loading: submitting }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <t-form ref="formRef" label-align="top" :data="form" :rules="rules" @submit.prevent>
      <div class="form-grid">
        <t-form-item label="用户名" name="username">
          <t-input v-model="form.username" placeholder="登录账号，3–64 字符" />
        </t-form-item>
        <t-form-item label="实名姓名" name="real_name">
          <t-input v-model="form.real_name" placeholder="选填" />
        </t-form-item>
        <t-form-item label="邮箱" name="email">
          <t-input v-model="form.email" placeholder="选填，填写后需符合邮箱格式" />
        </t-form-item>
        <t-form-item label="手机号" name="phone">
          <t-input v-model="form.phone" placeholder="选填" />
        </t-form-item>
        <t-form-item label="地域" name="region">
          <t-input v-model="form.region" placeholder="选填，如 华东" />
        </t-form-item>
        <t-form-item label="账号状态" name="status">
          <t-select v-model="form.status" :options="statusOptions" placeholder="请选择账号状态" />
        </t-form-item>
      </div>
      <t-form-item label="用户组" name="user_group_id">
        <t-select
          v-model="form.user_group_id"
          :options="groupOptions"
          clearable
          filterable
          placeholder="不修改请保持原值；清空表示移出分组"
        />
      </t-form-item>
      <t-form-item label="备注" name="sub_account_remark">
        <t-input v-model="form.sub_account_remark" placeholder="选填，用于区分成员用途" />
      </t-form-item>
      <p class="form-hint">
        手机号与邮箱均为选填：留空即保存为空值（库中 14/40 个用户没有手机号，强制必填会让这些账号无法保存）。
      </p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import { getUserGroupList, updateUser, type UserInfo, type UserUpdateRequest } from '@/api/user'
import { userStatusOptions } from '@/pages/users/constants'

const props = defineProps<{
  modelValue: boolean
  profile: UserInfo | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = ref(props.modelValue)
watch(
  () => props.modelValue,
  (value) => {
    visible.value = value
    if (value) syncForm()
  },
)
watch(visible, (value) => emit('update:modelValue', value))

const formRef = ref<FormInstanceFunctions>()
const submitting = ref(false)
const groupOptions = ref<{ label: string; value: number }[]>([])
const statusOptions = userStatusOptions.map((item) => ({ label: item.label, value: item.value }))

const form = reactive<{
  username: string
  real_name: string
  email: string
  phone: string
  region: string
  status: string
  sub_account_remark: string
  user_group_id: number | undefined
}>({
  username: '',
  real_name: '',
  email: '',
  phone: '',
  region: '',
  status: 'active',
  sub_account_remark: '',
  user_group_id: undefined,
})

// 只有「填了什么才校验什么」：空字符串视为清空，不触发格式规则。
const rules: Record<string, FormRule[]> = {
  username: [
    { required: true, message: '请输入用户名', type: 'error' },
    { min: 3, max: 64, message: '用户名长度 3–64 字符', type: 'error' },
  ],
  email: [{ email: true, message: '邮箱格式不正确', trigger: 'blur' }],
}

function syncForm() {
  const p = props.profile
  form.username = p?.username || ''
  form.real_name = p?.real_name || ''
  form.email = p?.email || ''
  form.phone = p?.phone || ''
  form.region = p?.region || ''
  form.status = p?.status || 'active'
  form.sub_account_remark = p?.sub_account_remark || ''
  form.user_group_id = p?.user_group_id ?? undefined
  void loadGroups()
}

async function loadGroups() {
  if (groupOptions.value.length) return
  try {
    const data = await getUserGroupList({ page: 1, page_size: 200 })
    groupOptions.value = (data.items || []).map((item) => ({
      label: item.is_agent_group ? `${item.name}（代理组）` : item.name,
      value: item.id,
    }))
  } catch {
    MessagePlugin.error('加载用户组失败')
  }
}

function close() {
  visible.value = false
}

async function handleSubmit() {
  const valid = await formRef.value?.validate()
  if (valid !== true) return
  if (!props.profile) return

  // 部分更新语义：只发真正变化的字段；user_group_id 用 null 表示「移出分组」，
  // 但后端 DTO 是 *uint64，0 才代表移出 —— 这里把清空映射成 0。
  const payload: UserUpdateRequest = {}
  const p = props.profile
  if (form.username !== (p.username || '')) payload.username = form.username
  if (form.real_name !== (p.real_name || '')) payload.real_name = form.real_name
  if (form.email !== (p.email || '')) payload.email = form.email
  if (form.phone !== (p.phone || '')) payload.phone = form.phone
  if (form.region !== (p.region || '')) payload.region = form.region
  if (form.status !== (p.status || '')) payload.status = form.status
  if (form.sub_account_remark !== (p.sub_account_remark || '')) {
    payload.sub_account_remark = form.sub_account_remark
  }
  const currentGroup = p.user_group_id ?? 0
  const nextGroup = form.user_group_id ?? 0
  if (nextGroup !== currentGroup) payload.user_group_id = nextGroup

  if (!Object.keys(payload).length) {
    MessagePlugin.info('没有需要保存的修改')
    return
  }

  submitting.value = true
  try {
    await updateUser(p.id, payload)
    MessagePlugin.success('资料已保存')
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
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 var(--space-lg);
}

.form-hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>