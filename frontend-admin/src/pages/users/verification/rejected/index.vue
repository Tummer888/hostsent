<template>
  <VerificationListPage
    title="实名认证审核拒绝"
    subtitle="跟踪被驳回的实名认证申请，便于复核拒绝原因和补件情况。"
    table-title="审核拒绝记录"
    table-desc="展示已审核拒绝的实名认证记录。"
    empty-text="暂无审核拒绝记录"
    :fetcher="getRejectedVerificationList"
  />
</template>

<script setup lang="ts">
import { getRejectedVerificationList } from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationRejected' })
</script>
