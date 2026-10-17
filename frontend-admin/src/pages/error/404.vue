<template>
  <ErrorState
    code="404"
    title="页面不存在"
    description="这个地址没有对应的页面，可能是链接已失效，或菜单里的入口已经被调整。"
    primary-text="返回上一页"
    secondary-text="回到仪表盘"
  />
</template>

<script setup lang="ts">
import ErrorState from './ErrorState.vue'

// 路由 catch-all 的落点。挂在 layouts/index.vue 之下，
// 所以侧边栏、顶栏、返回按钮都还在，用户不至于困在死页面里。
defineOptions({ name: 'NotFound' })
</script>
