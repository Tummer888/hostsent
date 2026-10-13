<template>
  <div class="provider-cell">
    <span class="provider-cell__name">{{ name }}</span>
    <t-tag v-if="kind" :theme="chainKindTheme(kind)" variant="light" size="small" shape="round">
      {{ chainKindLabel(kind) }}
    </t-tag>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import { chainKindLabel, chainKindTheme } from '../chain-kind'

/**
 * 渠道单元格：渠道名 + 链路标签。
 *
 * 单独成组件是因为它出现在控制台的每一张表里（调度 / 入站同步 / 差异 / 调价 / 出站动作），
 * 而「显示渠道名而不是裸 ID」+「两条链路要能一眼区分」是这次重构的核心诉求之一。
 * 后端已把 provider_name / provider_kind 带在列表行上，这里只在缺失时回退成 `渠道 #id`。
 */
const props = defineProps<{
  providerId: number
  /** 服务端反查的渠道名（旧数据可能为空）。 */
  providerName?: string
  /** 渠道链路（upstream/compute）。 */
  providerKind?: string
}>()

const name = computed(() => props.providerName || `渠道 #${props.providerId}`)
// 名称兜底成 `渠道 #id` 时不再显示链路标签：那时链路也多半缺失，硬显示会是空标签。
const kind = computed(() => (props.providerName ? props.providerKind || '' : ''))
</script>

<style scoped lang="css">
.provider-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.provider-cell__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
  color: var(--color-foreground);
}
</style>
