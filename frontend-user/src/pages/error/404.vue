<template>
  <ErrorState
    code="404"
    title="页面不存在"
    description="这个地址没有对应的页面，可能是链接已失效，或入口位置有调整。"
    primary-text="返回上一页"
    secondary-text="回到控制台"
  />
</template>

<script setup lang="ts">
import ErrorState from './ErrorState.vue'

/**
 * 404 兜底。
 *
 * 之前这个页面挂在路由最外层，渲染时整个 layout 都不在 —— 用户看到 404
 * 之后没有侧边栏可点、也没有页面可退，只能按浏览器后退键。现在随 layout
 * 一起渲染（见 router/index.ts 的 catch-all），导航能力保留。
 */
defineOptions({ name: 'NotFound' })
</script>
