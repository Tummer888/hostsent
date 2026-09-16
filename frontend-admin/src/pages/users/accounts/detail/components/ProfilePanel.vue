<template>
  <div class="tabs-section">
    <t-alert
      v-if="degraded.length"
      class="degraded-tip"
      theme="warning"
      :message="`以下数据段采集失败，展示可能不完整：${degraded.join('、')}`"
    />

    <div v-if="!profile" class="empty-state empty-state--error">未获取到用户信息</div>

    <template v-else>
      <section class="section-block">
        <div class="section-label">账号信息</div>
        <t-descriptions :column="descColumn" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="用户 ID">{{ profile.id }}</t-descriptions-item>
          <t-descriptions-item label="用户名">
            <span class="cell-strong">{{ profile.username || '—' }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="实名姓名">{{ profile.real_name || '未实名' }}</t-descriptions-item>
          <t-descriptions-item label="账号状态">
            <t-tag :theme="userStatusTheme(profile.status)" variant="light" size="small" shape="round">
              {{ userStatusLabel(profile.status) }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="账号类型">
            <t-tag v-if="profile.is_sub_account" theme="warning" variant="light" size="small" shape="round">
              子账号
            </t-tag>
            <t-tag v-else theme="primary" variant="light" size="small" shape="round">主账号</t-tag>
          </t-descriptions-item>
          <t-descriptions-item v-if="profile.is_sub_account" label="归属主账号">
            {{ profile.owner_name || profile.owner_user_id || '—' }}
          </t-descriptions-item>
          <t-descriptions-item label="用户分层">
            <t-tag :theme="userTierTheme(profile.tier || '')" variant="light" size="small" shape="round">
              {{ userTierLabel(profile.tier || '') }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="备注">{{ profile.sub_account_remark || '—' }}</t-descriptions-item>
          <t-descriptions-item label="邀请码">
            <span class="cell-mono">{{ profile.invite_code || '—' }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="邀请人">
            {{ profile.inviter_name || profile.inviter_user_id || '无' }}
          </t-descriptions-item>
          <t-descriptions-item label="绑定邀请时间">{{ formatDateTime(profile.invited_at) }}</t-descriptions-item>
          <t-descriptions-item label="注册时间">{{ formatDateTime(profile.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="最近更新">{{ formatDateTime(profile.updated_at) }}</t-descriptions-item>
        </t-descriptions>
      </section>

      <section class="section-block">
        <div class="section-label">联系方式与认证</div>
        <t-descriptions :column="descColumn" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="邮箱">
            <div>{{ profile.email || '—' }}</div>
            <t-tag
              class="verify-badge"
              :theme="profile.email_verified_at ? 'success' : 'default'"
              variant="light"
              size="small"
              shape="round"
            >
              {{ profile.email_verified_at ? '已验证' : '未验证' }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="手机号">
            <div>{{ profile.phone || '未填写' }}</div>
            <t-tag
              class="verify-badge"
              :theme="profile.phone_verified_at ? 'success' : 'default'"
              variant="light"
              size="small"
              shape="round"
            >
              {{ profile.phone_verified_at ? '已验证' : '未验证' }}
            </t-tag>
          </t-descriptions-item>
          <t-descriptions-item label="地域">{{ profile.region || '—' }}</t-descriptions-item>
          <t-descriptions-item label="第三方登录">{{ oauthProviderLabel(profile.oauth_provider || '') }}</t-descriptions-item>
          <t-descriptions-item label="OpenID">
            <span class="cell-mono">{{ profile.oauth_openid || '—' }}</span>
          </t-descriptions-item>
        </t-descriptions>
      </section>

      <section class="section-block">
        <div class="section-label">归属与分组</div>
        <t-descriptions :column="descColumn" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="用户组">
            {{ profile.user_group_name || '未分组' }}
            <span v-if="profile.user_group_id" class="cell-sub">ID {{ profile.user_group_id }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="用户等级">
            {{ profile.user_level_name || '—' }}
            <span v-if="profile.user_level_code" class="cell-sub">（{{ profile.user_level_code }}）</span>
          </t-descriptions-item>
          <t-descriptions-item label="归属销售">
            <span v-if="profile.sales_admin_id">
              {{ profile.sales_admin_name || profile.sales_admin_id }}
            </span>
            <t-tag v-else theme="warning" variant="light" size="small" shape="round">未归属</t-tag>
          </t-descriptions-item>
        </t-descriptions>
      </section>

      <section class="section-block">
        <div class="section-label">最近登录</div>
        <t-descriptions :column="descColumn" bordered size="medium" class="detail-desc">
          <t-descriptions-item label="登录时间">{{ formatDateTime(profile.last_login_at) }}</t-descriptions-item>
          <t-descriptions-item label="登录 IP">{{ profile.last_login_ip || '—' }}</t-descriptions-item>
          <t-descriptions-item label="IP 归属地">{{ profile.last_login_ip_region || '—' }}</t-descriptions-item>
          <t-descriptions-item label="后台角色">
            <div v-if="rbacRoles.length" class="tag-group">
              <t-tag v-for="role in rbacRoles" :key="role.id" theme="primary" variant="light" size="small" shape="round">
                {{ role.name }}（{{ role.code }}）
              </t-tag>
            </div>
            <span v-else>—</span>
          </t-descriptions-item>
        </t-descriptions>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import {
  formatDateTime,
  oauthProviderLabel,
  userStatusLabel,
  userStatusTheme,
  userTierLabel,
  userTierTheme,
} from '@/pages/users/constants'
import type { UserInfo, UserRoleBrief } from '@/api/user'

const props = withDefaults(
  defineProps<{
    profile: UserInfo | null
    rbacRoles?: UserRoleBrief[]
    degraded?: string[]
    isMobile?: boolean
  }>(),
  {
    rbacRoles: () => [],
    degraded: () => [],
    isMobile: false,
  },
)

const rbacRoles = computed(() => props.rbacRoles)
const degraded = computed(() => props.degraded)
// 窄屏单列，宽屏两列 —— 与实例详情页的描述块一致。
const descColumn = computed(() => (props.isMobile ? 1 : 2))
</script>

<style scoped>
.verify-badge {
  margin-top: 4px;
}
</style>