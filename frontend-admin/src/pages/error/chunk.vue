<template>
  <ErrorState
    code="!"
    title="检测到新版本"
    description="页面资源在后台更新过，当前标签页仍是旧版本，所以打不开这个页面。刷新一次即可继续使用。"
    primary-text="刷新页面"
    primary-action="reload"
    secondary-text="回到仪表盘"
    hint="刷新会重新加载最新版本，不会退出登录。"
  />
</template>

<script setup lang="ts">
import ErrorState from './ErrorState.vue'

/**
 * 发版后的 chunk 失效页。
 *
 * admin 是内部工具，运营人员常常一挂一整天不刷新；每次发版后旧 chunk 的哈希
 * 就失效了，点任何菜单都会抛 "Failed to fetch dynamically imported module"。
 * 之前这种情况是纯白屏，用户只能自己猜「是不是网断了」。
 */
defineOptions({ name: 'ChunkLoadError' })
</script>
