<template>
  <router-view />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'

import { useBrandStore } from '@/store'

defineOptions({ name: 'App' })

// 品牌信息（官网名称/Logo/版权）来自管理端配置，登录页与顶栏都要用，
// 因此在应用最外层拉起一次，不依赖具体页面。
const brandStore = useBrandStore()
onMounted(() => {
  void brandStore.load()
})
</script>
