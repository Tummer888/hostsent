<template>
  <div class="oauth-callback">
    <div class="oauth-callback__card">
      <t-loading v-if="phase === 'loading'" size="large" text="正在完成登录…" />
      <template v-else-if="phase === 'need_bind'">
        <h2 class="oauth-callback__title">该{{ providerLabel }}账号尚未注册</h2>
        <p class="oauth-callback__desc">
          平台未开启第三方自动注册，请先用账号密码登录，再到「个人中心 → 第三方登录」绑定该{{ providerLabel }}账号。
        </p>
        <t-button theme="primary" @click="goLogin">返回登录</t-button>
      </template>
      <template v-else>
        <h2 class="oauth-callback__title">登录未完成</h2>
        <p class="oauth-callback__desc">{{ errorMessage }}</p>
        <t-button theme="primary" @click="goLogin">返回登录</t-button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
// 第三方登录回调页（doc104 §6.4）。
//
// 这个页面只做一件事：把 URL 上的一次性 ticket 换成正式令牌。
// 令牌不走 URL —— 会被浏览器历史、Referer 与中间层日志记下来，
// 因此后端回调时只带 ticket（60 秒一次性），换票这一步必须由前端发起。
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { exchangeOAuthTicket } from '@/api/oauth'
import { useUserStore } from '@/store'

defineOptions({ name: 'OAuthCallback' })

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

type Phase = 'loading' | 'need_bind' | 'error'
const phase = ref<Phase>('loading')
const errorMessage = ref('')

// 后端在回跳时同时带上 provider，失败场景下也带 error 码；
// 这两个参数只用于展示，不能作为身份依据。
const provider = computed(() => String(route.query.provider || ''))
const providerLabel = computed(() => {
  const map: Record<string, string> = { wechat: '微信', qq: 'QQ', alipay: '支付宝' }
  return map[provider.value] || '第三方'
})

// 后端错误码 → 人话。直接展示英文码对用户没有意义。
const errorTextMap: Record<string, string> = {
  state_invalid: '授权已过期，请重新发起登录。',
  state_mismatch: '授权信息与当前渠道不匹配，请重新发起登录。',
  provider_unavailable: '该登录方式当前不可用，请改用其他方式登录。',
  credential_error: '该登录方式配置有误，请联系管理员。',
  exchange_failed: '获取授权信息失败，请重新发起登录。',
  userinfo_failed: '获取账号信息失败，请稍后重试。',
  openid_missing: '未取得该平台的账号标识，请重新发起登录。',
  not_logged_in: '请先登录后再绑定第三方账号。',
  user_not_found: '未找到对应的平台账号，请先用账号密码登录。',
  openid_taken: '该第三方账号已绑定其他平台账号，请先解绑后再试。',
  bind_failed: '绑定失败，请稍后重试。',
}

function goLogin() {
  router.replace({ path: '/login', query: { redirect: String(route.query.redirect || '/') } })
}

onMounted(async () => {
  const error = String(route.query.error || '')
  if (error) {
    phase.value = 'error'
    errorMessage.value = errorTextMap[error] || '登录未完成，请重新发起。'
    return
  }
  const ticket = String(route.query.ticket || '')
  if (!ticket) {
    phase.value = 'error'
    errorMessage.value = '缺少授权票据，请重新发起登录。'
    return
  }
  try {
    const { data } = await exchangeOAuthTicket(ticket)
    if (data.need_bind || !data.token) {
      phase.value = 'need_bind'
      return
    }
    // 写入会话后走统一收尾：replace 而不是 push，避免用户回退又落到回调页
    // 触发一次必然失败的二次换票（ticket 是一次性的）。
    userStore.applyOAuthSession({ token: data.token, user: data.user as never })
    const redirect = String(route.query.redirect || '/')
    router.replace(redirect)
  } catch (e) {
    phase.value = 'error'
    errorMessage.value = (e as Error)?.message || '登录未完成，请重新发起。'
  }
})
</script>

<style scoped>
.oauth-callback {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-page, #f3f3f3);
  padding: 24px;
}

.oauth-callback__card {
  width: 100%;
  max-width: 420px;
  padding: 32px 28px;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 8px 24px rgb(0 0 0 / 8%);
  text-align: center;
}

.oauth-callback__title {
  margin: 0 0 12px;
  font-size: 18px;
  font-weight: 600;
}

.oauth-callback__desc {
  margin: 0 0 20px;
  font-size: 14px;
  line-height: 1.7;
  color: var(--td-text-color-secondary, #666);
}
</style>
