<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SwapIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">人工调账</h2>
        </div>
      </div>
    </header>

    <div class="surface-card recon-result" style="padding: var(--space-lg) 20px">
      <h3 class="card-title" style="margin-bottom: var(--space-md)">调账信息</h3>
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="filter-card__grid">
          <div class="field">
            <span class="field__label">用户 ID（必填）</span>
            <t-input v-model="form.user_id" placeholder="请输入用户 ID" clearable />
          </div>
          <div class="field">
            <span class="field__label">收支方向（必填）</span>
            <t-select v-model="form.direction" placeholder="请选择方向" :options="directionOptions" />
          </div>
          <div class="field">
            <span class="field__label">金额（元，必填）</span>
            <t-input-number v-model="form.amount" :min="0" :precision="2" theme="column" placeholder="请输入调账金额" />
          </div>
          <div class="field">
            <span class="field__label">流水类型</span>
            <t-select v-model="form.type" placeholder="默认调账" clearable :options="txTypeOptions" />
          </div>
          <div class="field">
            <span class="field__label">幂等键（必填）</span>
            <t-input v-model="form.biz_key" placeholder="同一键只记一次账，如 ADJ-xxx" clearable />
          </div>
          <div class="field">
            <span class="field__label">备注</span>
            <t-input v-model="form.remark" placeholder="选填，调账说明" clearable />
          </div>
        </div>
        <div style="margin-top: var(--space-md)">
          <t-space size="small">
            <t-button theme="primary" :loading="loading" @click="handleSubmit">
              <template #icon>
                <CheckCircleIcon aria-hidden="true" />
              </template>
              提交调账
            </t-button>
            <t-button variant="outline" @click="handleReset">重置</t-button>
          </t-space>
        </div>
      </t-form>

      <t-alert
        v-if="result"
        style="margin-top: var(--space-lg)"
        :theme="result.direction > 0 ? 'success' : 'warning'"
        :message="`调账成功，流水号 ${result.tx_no}，用户 ${result.user_id} 余额变为 ¥${formatPrice(result.balance_after)}`"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { CheckCircleIcon, SwapIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { adjustBalance } from '@/api/finance'
import { directionOptions, formatPrice, txTypeOptions } from '@/pages/finance/constants'
import type { TransactionInfo } from '@/types/interface'

defineOptions({ name: 'FinanceAdjust' })

const loading = ref(false)
const result = ref<TransactionInfo | null>(null)

const form = reactive<{
  user_id: string | undefined
  direction: number | undefined
  amount: number | undefined
  type: string | undefined
  biz_key: string | undefined
  remark: string | undefined
}>({
  user_id: undefined,
  direction: undefined,
  amount: undefined,
  type: undefined,
  biz_key: undefined,
  remark: undefined,
})

async function handleSubmit() {
  const userId = Number(form.user_id)
  if (!userId || userId <= 0) {
    MessagePlugin.warning('请输入有效的用户 ID')
    return
  }
  if (!form.direction) {
    MessagePlugin.warning('请选择收支方向')
    return
  }
  if (!form.amount || form.amount <= 0) {
    MessagePlugin.warning('请输入大于 0 的金额')
    return
  }
  if (!form.biz_key) {
    MessagePlugin.warning('请输入幂等键')
    return
  }
  loading.value = true
  try {
    result.value = await adjustBalance({
      user_id: userId,
      direction: form.direction,
      amount: form.amount,
      type: form.type,
      biz_key: form.biz_key,
      remark: form.remark,
    })
    MessagePlugin.success('调账成功')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '调账失败')
  } finally {
    loading.value = false
  }
}

function handleReset() {
  form.user_id = undefined
  form.direction = undefined
  form.amount = undefined
  form.type = undefined
  form.remark = undefined
  result.value = null
  genBizKey()
}

function genBizKey() {
  const ts = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  const stamp = `${ts.getFullYear()}${pad(ts.getMonth() + 1)}${pad(ts.getDate())}${pad(ts.getHours())}${pad(ts.getMinutes())}${pad(ts.getSeconds())}`
  form.biz_key = `ADJ-${stamp}-${Math.floor(Math.random() * 1000)}`
}

onMounted(genBizKey)
</script>

<style lang="css">
@import '../shared.css';
</style>
