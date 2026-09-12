<template>
  <PromotionCrud promotion-type="discount" :labels="labels" />
</template>

<script setup lang="ts">
import PromotionCrud from '../shared/PromotionCrud.vue'
import type { PromotionLabels } from '../shared/types'

defineOptions({ name: 'ProductPromotionActivities' })

const labels: PromotionLabels = {
  pageTitle: '折扣活动',
  tableTitle: '折扣活动',
  createText: '新建活动',
  nameLabel: '活动名称',
  namePlaceholder: '活动名称',
  ruleLabel: '活动规则（如 0.85 即 85 折）',
  rulePlaceholder: '填写折扣规则，JSON 或说明文本',
  ruleEditor: 'input',
  unit: '活动',
  emptyText: '暂无折扣活动，请先新建',
}
</script>
