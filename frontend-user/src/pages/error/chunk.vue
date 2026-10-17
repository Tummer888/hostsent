<template>
  <ErrorState
    code="!"
    title="检测到新版本"
    description="页面资源在后台更新过，当前标签页仍是旧版本，所以打不开这个页面。刷新一次即可继续使用。"
    primary-text="刷新页面"
    primary-action="reload"
    secondary-text="回到控制台"
    hint="刷新会重新加载最新版本，不会退出登录。"
  />
</template>

<script setup lang="ts">
import ErrorState from './ErrorState.vue'

/**
 * 发版后的 chunk 失效页。
 *
 * 用户中心常被长时间挂着；发版后旧 chunk 的哈希失效，点菜单会抛
 * "Failed to fetch dynamically imported module"，此前同样是白屏。
 */
defineOptions({ name: 'ChunkLoadError' })
</script>
