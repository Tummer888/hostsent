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
      <!-- 注销信息（doc104 §4）：只在已注销时出现。
           它是 profile 的一部分而不是独立数据段，因此沿用「只有 profile 失败才整页
           报错」的容错策略 —— 注销态是用户身份的一部分，不该降级成「数据暂不可用」。 -->
      <section v-if="profile.deleted_at" class="info-card info-card--danger">
        <div class="info-card__head">
          <span class="info-card__icon">
            <DeleteIcon size="16" aria-hidden="true" />
          </span>
          <h3 class="info-card__title">注销信息</h3>
        </div>
        <t-alert
          class="deleted-tip"
          theme="warning"
          :message="`该账号已于 ${formatDateTime(profile.deleted_at)} 注销，留存期内可恢复；到期后由清理任务硬删除。`"
        />
        <div class="info-rows">
          <div class="info-row">
            <span class="info-row__label">注销时间</span>
            <span class="info-row__value">{{ formatDateTime(profile.deleted_at) }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">注销前状态</span>
            <span class="info-row__value">{{ userStatusLabel(profile.status_before_delete || '') || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">操作人</span>
            <span class="info-row__value">
              {{ profile.deleted_by_name || (profile.deleted_by ? `管理员 #${profile.deleted_by}` : '系统/用户自助') }}
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">注销原因</span>
            <span class="info-row__value">{{ profile.delete_reason || '—' }}</span>
          </div>
        </div>
      </section>

      <section class="info-card info-card--account">
        <div class="info-card__head">
          <span class="info-card__icon">
            <UserIcon size="16" aria-hidden="true" />
          </span>
          <h3 class="info-card__title">账号信息</h3>
          <span class="info-card__meta">ID {{ profile.id }}</span>
        </div>
        <div class="info-rows">
          <div class="info-row">
            <span class="info-row__label">用户名</span>
            <span class="info-row__value"
              ><span class="cell-strong">{{ profile.username || '—' }}</span></span
            >
          </div>
          <div class="info-row">
            <span class="info-row__label">实名姓名</span>
            <span class="info-row__value">{{ profile.real_name || '未实名' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">账号状态</span>
            <span class="info-row__value">
              <t-tag :theme="userStatusTheme(profile.status)" variant="light" size="small" shape="round">
                {{ userStatusLabel(profile.status) }}
              </t-tag>
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">账号类型</span>
            <span class="info-row__value">
              <t-tag v-if="profile.is_sub_account" theme="warning" variant="light" size="small" shape="round">
                子账号
              </t-tag>
              <t-tag v-else theme="primary" variant="light" size="small" shape="round">主账号</t-tag>
            </span>
          </div>
          <div v-if="profile.is_sub_account" class="info-row">
            <span class="info-row__label">归属主账号</span>
            <span class="info-row__value">{{ profile.owner_name || profile.owner_user_id || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">备注</span>
            <span class="info-row__value">{{ profile.sub_account_remark || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">邀请码</span>
            <span class="info-row__value"
              ><span class="cell-mono">{{ profile.invite_code || '—' }}</span></span
            >
          </div>
          <div class="info-row">
            <span class="info-row__label">邀请人</span>
            <span class="info-row__value">{{ profile.inviter_name || profile.inviter_user_id || '无' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">绑定邀请时间</span>
            <span class="info-row__value">{{ formatDateTime(profile.invited_at) }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">注册时间</span>
            <span class="info-row__value">{{ formatDateTime(profile.created_at) }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">最近更新</span>
            <span class="info-row__value">{{ formatDateTime(profile.updated_at) }}</span>
          </div>
        </div>
      </section>

      <section class="info-card info-card--contact">
        <div class="info-card__head">
          <span class="info-card__icon">
            <MailIcon size="16" aria-hidden="true" />
          </span>
          <h3 class="info-card__title">联系方式与认证</h3>
        </div>
        <div class="info-rows">
          <div class="info-row">
            <span class="info-row__label">邮箱</span>
            <span class="info-row__value">
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
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">手机号</span>
            <span class="info-row__value">
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
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">地域</span>
            <span class="info-row__value">{{ profile.region || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">第三方登录</span>
            <span class="info-row__value">{{ oauthProviderLabel(profile.oauth_provider || '') }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">OpenID</span>
            <span class="info-row__value"
              ><span class="cell-mono">{{ profile.oauth_openid || '—' }}</span></span
            >
          </div>
        </div>
      </section>

      <section class="info-card info-card--group">
        <div class="info-card__head">
          <span class="info-card__icon">
            <UsergroupIcon size="16" aria-hidden="true" />
          </span>
          <h3 class="info-card__title">归属与分组</h3>
        </div>
        <div class="info-rows">
          <div class="info-row">
            <span class="info-row__label">用户组</span>
            <span class="info-row__value">
              {{ profile.user_group_name || '未分组' }}
              <span v-if="profile.user_group_id" class="cell-sub">ID {{ profile.user_group_id }}</span>
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">用户等级</span>
            <span class="info-row__value">
              <LevelBadge
                :name="profile.user_level_name"
                :icon="profile.user_level_icon"
                :color="profile.user_level_color"
                :weight="profile.user_level_weight"
              />
              <span v-if="profile.user_level_code" class="cell-sub">（{{ profile.user_level_code }}）</span>
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">代理等级</span>
            <span class="info-row__value">
              <t-tag v-if="profile.agent_level_id" theme="warning" variant="light" size="small" shape="round">
                {{ profile.agent_level_name || `#${profile.agent_level_id}` }}
              </t-tag>
              <span v-else>非代理</span>
              <span class="cell-sub">{{ profile.agent_level_id ? '享受代理折扣' : '不享受折扣' }}</span>
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">累计消费</span>
            <span class="info-row__value">
              <span class="cell-strong">¥{{ formatAmount(profile.total_consume_amount || 0) }}</span>
              <span class="cell-sub">等级按累计消费自动升级（只升不降）</span>
            </span>
          </div>
          <div class="info-row">
            <span class="info-row__label">归属销售</span>
            <span class="info-row__value">
              <span v-if="profile.sales_admin_id">{{ profile.sales_admin_name || profile.sales_admin_id }}</span>
              <t-tag v-else theme="warning" variant="light" size="small" shape="round">未归属</t-tag>
            </span>
          </div>
        </div>
      </section>

      <section class="info-card info-card--login">
        <div class="info-card__head">
          <span class="info-card__icon">
            <TimeIcon size="16" aria-hidden="true" />
          </span>
          <h3 class="info-card__title">最近登录</h3>
        </div>
        <div class="info-rows">
          <div class="info-row">
            <span class="info-row__label">登录时间</span>
            <span class="info-row__value">{{ formatDateTime(profile.last_login_at) }}</span>
          </div>
          <div class="info-row">
            <span class="info-row__label">登录 IP</span>
            <span class="info-row__value">{{ profile.last_login_ip || '—' }}</span>
          </div>
          <!-- 角色标签数量不定，独占整行，避免与上排字段挤在同一格。 -->
          <div class="info-row info-row--wide">
            <span class="info-row__label">客户角色</span>
            <span class="info-row__value">
              <div v-if="rbacRoles.length" class="tag-group">
                <t-tag
                  v-for="role in rbacRoles"
                  :key="role.id"
                  theme="primary"
                  variant="light"
                  size="small"
                  shape="round"
                >
                  {{ role.name }}（{{ role.code }}）
                </t-tag>
              </div>
              <span v-else>—</span>
            </span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { DeleteIcon, MailIcon, TimeIcon, UsergroupIcon, UserIcon } from 'tdesign-icons-vue-next'

import {
  formatAmount,
  formatDateTime,
  oauthProviderLabel,
  userStatusLabel,
  userStatusTheme,
} from '@/pages/users/constants'
import type { UserInfo, UserRoleBrief } from '@/api/user'
import LevelBadge from '@/components/level-badge/index.vue'

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
</script>

<style scoped>
/* 卡片之间留出大于卡片内边距的间距，让分组成立。 */
.info-card + .info-card {
  margin-top: var(--space-lg);
}

.verify-badge {
  margin-top: 4px;
}

.deleted-tip {
  margin-bottom: 12px;
}

/* .info-rows 的窄屏单列规则在 pages/users/shared.css 的响应式段落里。
   写在 scoped 里会失效：.info-rows[data-v-x] 与 .users-module .info-rows
   特异性同为 0-2-0，同分时按源码顺序落败。 */
</style>
