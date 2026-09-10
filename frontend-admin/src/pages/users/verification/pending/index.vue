<template>
  <VerificationListPage
    title="实名认证待审核"
    table-title="待审核申请"
    empty-text="暂无待审核实名认证"
    :fetcher="getPendingVerificationList"
  />
</template>

<script setup lang="ts">
import { getPendingVerificationList } from '@/api/verification'
import VerificationListPage from '../VerificationListPage.vue'

defineOptions({ name: 'UserVerificationPending' })
</script>
