<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded"
      class="degraded-tip"
      theme="warning"
      message="权限或角色数据采集失败，以下内容可能不完整。"
    />

    <section class="section-block">
      <div class="section-label">客户侧权限</div>
      <p class="section-hint">
        客户侧权限是固定枚举，落 sub_account_permissions；主账号天然拥有全部权限，子账号按勾选授予。
      </p>
      <div v-if="profile?.is_sub_account">
        <div v-if="permissions.length" class="tag-group">
          <t-tag v-for="code in permissions" :key="code" theme="success" variant="light" shape="round">
            {{ customerPermissionLabel(code) }}
          </t-tag>
        </div>
        <div v-else class="empty-state empty-state--compact">该子账号未授予任何客户侧权限</div>
      </div>
      <div v-else class="tag-group">
        <t-tag v-for="opt in customerPermissionOptions" :key="opt.value" theme="primary" variant="light" shape="round">
          {{ opt.label }}
        </t-tag>
        <t-tag theme="warning" variant="light" shape="round">成员管理</t-tag>
      </div>
    </section>

    <section class="section-block">
      <div class="section-label">后台角色绑定</div>
      <p class="section-hint">
        这里是后台 RBAC 角色（roles.scope=admin），只读展示；客户账号不应持有任何后台权限码。
      </p>
      <div v-if="rbacRoles.length" class="tag-group">
        <t-tag v-for="role in rbacRoles" :key="role.id" theme="primary" variant="light" shape="round">
          {{ role.name }}（{{ role.code }}）· {{ roleScopeLabel(role.scope) }}
        </t-tag>
      </div>
      <div v-else class="empty-state empty-state--compact">该账号未绑定任何后台角色</div>
    </section>

    <section v-if="!profile?.is_sub_account" class="section-block">
      <div class="table-card__head">
        <h3 class="card-title">成员（子账号）</h3>
        <span class="table-card__meta">共 {{ members.length }} 个成员</span>
      </div>
      <t-table
        row-key="id"
        :data="members"
        :columns="memberColumns"
        :loading="membersLoading"
        size="small"
        hover
        table-layout="fixed"
        cell-empty-content="—"
      >
        <template #username="{ row }">
          <div>
            <span class="cell-strong">{{ row.username }}</span>
            <div class="cell-sub">{{ row.name || '未实名' }}</div>
          </div>
        </template>
        <template #contact="{ row }">
          <div>{{ row.email || '—' }}</div>
          <div class="cell-sub">{{ row.phone || '未填写' }}</div>
        </template>
        <template #permissions="{ row }">
          <div v-if="row.permissions?.length" class="tag-group">
            <t-tag v-for="code in row.permissions" :key="code" size="small" variant="outline" shape="round">
              {{ customerPermissionLabel(code) }}
            </t-tag>
          </div>
          <span v-else class="cell-sub">未授予权限</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="userStatusTheme(row.status)" variant="light" size="small" shape="round">
            {{ userStatusLabel(row.status) }}
          </t-tag>
        </template>
        <template #last_login_at="{ row }">
          <span class="time-text">{{ formatDateTime(row.last_login_at) }}</span>
        </template>
        <template #empty>
          <t-empty description="该主账号下暂无成员" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getUserMembers } from '@/api/user'
import {
  customerPermissionLabel,
  customerPermissionOptions,
  formatDateTime,
  roleScopeLabel,
  userStatusLabel,
  userStatusTheme,
} from '@/pages/users/constants'
import type { SubAccountMemberInfo, UserInfo, UserRoleBrief } from '@/api/user'

const props = defineProps<{
  userId: number
  profile: UserInfo | null
  rbacRoles: UserRoleBrief[]
  permissions: string[]
  degraded?: boolean
}>()

const members = ref<SubAccountMemberInfo[]>([])
const membersLoading = ref(false)

const memberColumns: PrimaryTableCol<SubAccountMemberInfo>[] = [
  { colKey: 'username', title: '成员', minWidth: 180 },
  { colKey: 'contact', title: '联系方式', minWidth: 180 },
  { colKey: 'permissions', title: '已授权限', minWidth: 240 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'last_login_at', title: '最近登录', width: 150 },
]

// 只有主账号才有成员列表；子账号不请求该接口（后端返回空 items 但没必要发请求）。
const canListMembers = computed(() => !!props.profile && !props.profile.is_sub_account)

async function loadMembers() {
  if (!props.userId || !canListMembers.value) {
    members.value = []
    return
  }
  membersLoading.value = true
  try {
    const data = await getUserMembers(props.userId)
    members.value = data.items || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载成员列表失败')
  } finally {
    membersLoading.value = false
  }
}

watch(
  () => [props.userId, props.profile?.is_sub_account] as const,
  () => {
    void loadMembers()
  },
  { immediate: true },
)

defineExpose({ reload: loadMembers })
</script>

<style scoped>
.section-hint {
  margin: 0 0 var(--space-md);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}
</style>