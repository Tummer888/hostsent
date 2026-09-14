<template>
  <div class="page-body console-module">
    <section class="surface-card not-open">
      <span class="not-open__icon"><LayersIcon size="26" /></span>
      <h2 class="not-open__title">镜像管理尚未开放</h2>
      <p class="not-open__desc">
        平台还未上线镜像主数据与镜像库，因此这一页暂时没有可展示的内容。当前开通实例时，操作系统随所选规格自带，
        无需在此选择镜像。镜像库上线后，本页会出现在左侧「云产品」目录下。
      </p>
      <t-button theme="primary" @click="go('/cloud/instances')">返回我的云主机</t-button>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { LayersIcon } from 'tdesign-icons-vue-next'

defineOptions({ name: 'ImageList' })

const router = useRouter()

function go(path: string) {
  void router.push(path)
}
</script>

<style scoped>
.not-open {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 56px 32px;
  text-align: center;
}

.not-open__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  border-radius: 14px;
  background: var(--color-primary-light, #eff6ff);
  color: var(--color-primary, #2563eb);
}

.not-open__title {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--color-foreground);
}

.not-open__desc {
  max-width: 520px;
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.8;
  color: var(--color-muted-foreground);
}
</style>
