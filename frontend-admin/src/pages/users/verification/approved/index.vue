<template>
  <VerificationListPage
    title="实名认证审核通过"
    subtitle="查看已完成审核并通过的实名认证主体，便于后续等级、权限和配额联动。"
    table-title="审核通过记录"
    table-desc="展示已审核通过的实名认证记录。"
    empty-text="暂无审核通过记录"
    :fetcher="getApprovedVerificationList"
  />
</template>

<script setup lang="ts">
import { getApprovedVerificationList } from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationApproved' })
</script>
