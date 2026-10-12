<template>
  <router-view />

  <!--
    首次导航期间 router-view 是空的（守卫在等用户信息/菜单接口，或浏览器还在下载路由 chunk），
    页面就是一片纯白。这里盖一层最轻量的加载态，让等待可见而非「白屏」；
    首屏导航完成（router.isReady 兑现）后立即移除，不参与后续页内跳转。
  -->
  <div v-if="booting" class="app-boot">
    <span class="app-boot__spinner"></span>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { useBrandStore } from '@/store'

defineOptions({ name: 'App' })

const router = useRouter()
const booting = ref(true)

// 品牌信息（官网名称/Logo/版权）来自管理端配置，登录页与顶栏都要用，
// 因此在应用最外层拉起一次，不依赖具体页面。
const brandStore = useBrandStore()
onMounted(() => {
  void brandStore.load()
  void router.isReady().finally(() => {
    booting.value = false
  })
})
</script>

<style scoped>
.app-boot {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}

.app-boot__spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(0, 0, 0, 0.08);
  border-top-color: var(--td-brand-color, #0052d9);
  border-radius: 50%;
  animation: app-boot-spin 0.8s linear infinite;
}

@keyframes app-boot-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
