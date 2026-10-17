<template>
  <ErrorState
    code="500"
    title="页面加载失败"
    description="页面在渲染过程中出了问题。可以先刷新试试；如果反复出现，请联系客服说明访问的页面。"
    primary-text="重新加载"
    primary-action="reload"
    secondary-text="回到控制台"
    hint="已保留你的登录状态，重新加载不会退出登录。"
  />
</template>

<script setup lang="ts">
import ErrorState from './ErrorState.vue'

defineOptions({ name: 'ServerError' })
</script>
