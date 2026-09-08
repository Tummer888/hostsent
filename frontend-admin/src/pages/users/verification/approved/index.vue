<template>
  <VerificationListPage
    title="实名认证审核通过"
    table-title="审核通过记录"
    empty-text="暂无审核通过记录"
    :fetcher="getApprovedVerificationList"
  />
</template>

<script setup lang="ts">
import { getApprovedVerificationList } from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationApproved' })
</script>
