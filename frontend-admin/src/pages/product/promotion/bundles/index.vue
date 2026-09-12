<template>
  <PromotionCrud promotion-type="bundle" :labels="labels" />
</template>

<script setup lang="ts">
import PromotionCrud from '../shared/PromotionCrud.vue'
import type { PromotionLabels } from '../shared/types'

defineOptions({ name: 'ProductPromotionBundles' })

const labels: PromotionLabels = {
  pageTitle: '套餐组合',
  tableTitle: '套餐组合',
  createText: '新建套餐',
  nameLabel: '套餐名称',
  namePlaceholder: '套餐名称',
  ruleLabel: '组合规则',
  rulePlaceholder: '填写组合的商品与规则，JSON 格式',
  ruleEditor: 'textarea',
  unit: '套餐',
  emptyText: '暂无套餐组合，请先新建',
}
</script>
