<template>
  <VerificationListPage
    title="实名认证待审核"
    subtitle="集中处理新提交的实名认证申请，优先识别高风险资料与待补充项。"
    table-title="待审核申请"
    table-desc="展示当前处于待审核状态的实名认证记录。"
    empty-text="暂无待审核实名认证"
    :fetcher="getPendingVerificationList"
  />
</template>

<script setup lang="ts">
import { getPendingVerificationList } from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationPending' })
</script>
