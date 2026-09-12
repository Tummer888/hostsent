/** 促销活动通用 CRUD 的文案配置：折扣活动与套餐组合仅在此差异。 */
export interface PromotionLabels {
  pageTitle: string
  tableTitle: string
  createText: string
  nameLabel: string
  namePlaceholder: string
  ruleLabel: string
  rulePlaceholder: string
  ruleEditor: 'input' | 'textarea'
  /** 量词：活动 / 套餐 */
  unit: string
  emptyText: string
}
