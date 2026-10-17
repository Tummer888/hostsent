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
 * 错误页统一外观（与管理端同一套结构：状态码 / 标题 / 说明 / 两个动作 / 可选提示）。
 *
 * 三端不共用组件 —— 技术栈与目的地不同：这里跑在用户中心的 layout 里，
 * 主动作按用户习惯给「返回上一页」，次动作回控制台。
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
    secondaryTo?: string
    hint?: string
  }>(),
  {
    primaryText: '返回上一页',
    secondaryText: '回到控制台',
    primaryAction: 'back',
    secondaryTo: '/dashboard',
    hint: '',
  },
)

const router = useRouter()

function hasHistoryBack() {
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
  min-height: min(60vh, 480px);
  padding: 24px 16px;
}

.error-state__inner {
  width: 100%;
  max-width: 460px;
  text-align: center;
}

.error-state__code {
  font-size: 60px;
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.02em;
  color: var(--color-foreground);
  font-variant-numeric: tabular-nums;
}

.error-state__title {
  margin-top: 16px;
  font-size: 19px;
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

html.dark .error-state__code {
  color: #f3f4f6;
}

@media (max-width: 560px) {
  .error-state {
    min-height: min(50vh, 360px);
  }

  .error-state__code {
    font-size: 46px;
  }

  .error-state__title {
    font-size: 17px;
  }
}
</style>
