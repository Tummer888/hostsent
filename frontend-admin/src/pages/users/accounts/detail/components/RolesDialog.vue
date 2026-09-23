<template>
  <t-dialog
    v-model:visible="visible"
    header="分配客户角色"
    width="480px"
    :confirm-btn="{ content: '保存角色', theme: 'primary', loading: submitting }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSubmit"
    @close="close"
  >
    <t-form label-align="top" @submit.prevent>
      <t-form-item label="用户">
        <t-input :model-value="username || '—'" disabled />
      </t-form-item>
      <t-form-item label="角色">
        <t-select
          v-model="selected"
          :options="roleOptions"
          :loading="loading"
          multiple
          clearable
          filterable
          placeholder="不选择表示解除全部客户角色"
        />
      </t-form-item>
      <p class="form-hint">
        这里分配的是<strong>客户</strong>角色（roles.scope=user，如「普通用户」）。
        员工角色（超级管理员/运维等）由员工管理维护，绑到客户账号上不会生效，
        后端也会直接拒绝。客户侧权限（查看实例/下单等）是固定枚举，在「角色与权限」页只读展示，不在此处调整。
      </p>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import { assignUserRoles, getRoleList, type RoleInfo } from '@/api/user'

const props = defineProps<{
  modelValue: boolean
  userId: number
  username: string
  currentRoleIds: number[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const visible = ref(props.modelValue)
const selected = ref<number[]>([])
const loading = ref(false)
const submitting = ref(false)
const roleOptions = ref<{ label: string; value: number }[]>([])

watch(() => props.modelValue, (value) => {
  visible.value = value
  if (value) {
    selected.value = [...props.currentRoleIds]
    void loadRoles()
  }
})
watch(visible, (value) => emit('update:modelValue', value))

async function loadRoles() {
  loading.value = true
  try {
    // 只列客户角色（roles.scope=user）：本弹窗给客户账号分配角色，
    // 员工角色（scope=admin）出现在这里会让人误以为客户号能拿到后台权限。
    const roles: RoleInfo[] = await getRoleList('user')
    roleOptions.value = (roles || []).map((role) => ({
      label: `${role.name}（${role.code}）`,
      value: role.id,
    }))
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载角色列表失败')
  } finally {
    loading.value = false
  }
}

function close() {
  visible.value = false
}

async function handleSubmit() {
  submitting.value = true
  try {
    await assignUserRoles(props.userId, { role_ids: selected.value })
    MessagePlugin.success('角色已更新')
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
</style>