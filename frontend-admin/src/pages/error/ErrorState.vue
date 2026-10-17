<template>
  <div class="error-state">
    <div class="error-state__inner">
      <div class="error-state__code" aria-hidden="true">{{ code }}</div>
      <h2 class="error-state__title">{{ title }}</h2>
      <p class="error-state__desc">{{ description }}</p>

      <div class="error-state__actions">
        <t-button theme="primary" size="medium" @click="onPrimary">
          {{ primaryText }}
        </t-button>
        <t-button variant="outline" size="medium" @click="onSecondary">
          {{ secondaryText }}
        </t-button>
      </div>

      <p v-if="hint" class="error-state__hint">{{ hint }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

/**
 * 错误页统一外观。
 *
 * 三种错误共用一套结构（状态码 / 标题 / 说明 / 两个动作 / 可选提示），
 * 只换文案与动作 —— 这样 404、500、chunk 失效在视觉上是同一族，
 * 用户不会因为「错误长什么样」而额外困惑。
 *
 * 动作默认覆盖后台最常见的两个目的地：回上一页、回仪表盘。
 * 调用方可用 primaryAction / secondaryTo 换成更贴切的行为。
 */
const props = withDefaults(
  defineProps<{
    code: string | number
    title: string
    description: string
    primaryText?: string
    secondaryText?: string
    /** 主动作：'back' 回上一页（无历史时退到 secondaryTo）、'reload' 整页刷新、其余按路由跳转。 */
    primaryAction?: 'back' | 'reload' | string
    /** 次动作目标路由。 */
    secondaryTo?: string
    hint?: string
  }>(),
  {
    primaryText: '返回上一页',
    secondaryText: '回到仪表盘',
    primaryAction: 'back',
    secondaryTo: '/dashboard/base',
    hint: '',
  },
)

const router = useRouter()

function hasHistoryBack() {
  // 与会话内返回按钮同一套判定：history.state.back 为空说明这是会话第一跳
  // （或整页刷新后重建的初始态），此时 router.back() 会把用户带出管理端。
  const back = (window.history.state as { back?: string | null } | null)?.back
  return typeof back === 'string' && back.length > 0
}

function onPrimary() {
  if (props.primaryAction === 'reload') {
    window.location.reload()
    return
  }
  if (props.primaryAction === 'back') {
    if (hasHistoryBack()) {
      router.back()
      return
    }
    // 没有可回退的历史：退到次动作，避免按钮点了没反应。
  }
  void router.push(props.secondaryTo)
}

function onSecondary() {
  void router.push(props.secondaryTo)
}
</script>

<style scoped>
.error-state {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: min(62vh, 520px);
  padding: 24px 16px;
}

.error-state__inner {
  width: 100%;
  max-width: 480px;
  text-align: center;
}

.error-state__code {
  font-size: 64px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.02em;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.error-state__title {
  margin-top: 16px;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-foreground);
}

.error-state__desc {
  margin-top: 10px;
  font-size: 13.5px;
  line-height: 1.75;
  color: var(--color-muted-foreground);
}

.error-state__actions {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 24px;
}

.error-state__hint {
  margin-top: 18px;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
  opacity: 0.85;
}

/* 深色下只需给状态码补一点提亮：标题/说明已由 html.dark 的
   --color-foreground / --color-muted-foreground 令牌自动翻转。 */
html.dark .error-state__code {
  color: #f3f4f6;
}

@media (max-width: 560px) {
  .error-state {
    min-height: min(52vh, 380px);
  }

  .error-state__code {
    font-size: 48px;
  }

  .error-state__title {
    font-size: 18px;
  }
}
</style>
