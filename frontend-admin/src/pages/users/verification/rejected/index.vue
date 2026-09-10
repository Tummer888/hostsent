<template>
  <VerificationListPage
    title="实名认证审核拒绝"
    table-title="审核拒绝记录"
    empty-text="暂无审核拒绝记录"
    :fetcher="getRejectedVerificationList"
  />
</template>

<script setup lang="ts">
import { getRejectedVerificationList } from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationRejected' })
</script>
