<template>
  <div class="referral-page">
    <ReferralNav />

    <div class="referral-header">
      <h2 class="referral-title">推广概览</h2>
      <t-button variant="outline" size="small" :loading="loading" @click="load">
        <template #icon><RefreshIcon /></template>
        刷新
      </t-button>
    </div>

    <t-alert v-if="profile && !profile.enabled" theme="warning" class="referral-alert">
      推广返现当前未开放，请联系管理员。
    </t-alert>

    <section class="stat-grid">
      <div class="stat-card stat-card--primary">
        <span class="stat-card__label">可用返现余额</span>
        <span class="stat-card__value">¥{{ money(profile?.balance) }}</span>
        <span class="stat-card__hint">
          冻结 ¥{{ money(profile?.frozen) }}
          <template v-if="(profile?.balance || 0) < 0"> · 已形成欠款，请补足后方可提现</template>
        </span>
      </div>
      <div class="stat-card">
        <span class="stat-card__label">累计返现</span>
        <span class="stat-card__value">¥{{ money(profile?.total_income) }}</span>
        <span class="stat-card__hint">退款会按比例冲减</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__label">已邀请人数</span>
        <span class="stat-card__value">{{ profile?.invitee_count ?? 0 }}</span>
        <span class="stat-card__hint">单级邀请，仅统计直接邀请</span>
      </div>
      <div class="stat-card">
        <span class="stat-card__label">待审核提现</span>
        <span class="stat-card__value">¥{{ money(profile?.pending_withdraw) }}</span>
        <span class="stat-card__hint">最低提现 ¥{{ money(profile?.min_withdraw_amount) }}</span>
      </div>
    </section>

    <section class="referral-panel">
      <div class="panel-head">
        <span class="section-title">我的邀请码与推广链接</span>
      </div>
      <div class="promo">
        <div class="promo__row">
          <span class="promo__label">邀请码</span>
          <span class="promo__value promo__value--code">{{ profile?.invite_code || '—' }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copy(inviteCode, '邀请码已复制')">
            复制
          </t-button>
        </div>
        <div class="promo__row">
          <span class="promo__label">推广链接</span>
          <span class="promo__value">{{ inviteUrl }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copy(inviteUrl, '推广链接已复制')">
            复制
          </t-button>
        </div>
        <p class="promo__tip">好友通过该链接注册后，其订单完成即按比率返现到你的返现余额。</p>
      </div>
    </section>

    <section class="referral-panel">
      <div class="panel-head">
        <span class="section-title">当前返现比率</span>
      </div>
      <div class="rate-grid">
        <div class="rate-item">
          <span class="rate-item__label">首单</span>
          <span class="rate-item__value">{{ percent(profile?.first_order_rate) }}</span>
        </div>
        <div class="rate-item">
          <span class="rate-item__label">后续下单</span>
          <span class="rate-item__value">{{ percent(profile?.subsequent_rate) }}</span>
        </div>
        <div class="rate-item">
          <span class="rate-item__label">续费</span>
          <span class="rate-item__value">{{ percent(profile?.renewal_rate) }}</span>
        </div>
      </div>
      <p class="rate-tip">返现基数按被邀请人订单实付金额计算；退款会按退款额占实付比例冲减对应返现。</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { RefreshIcon } from 'tdesign-icons-vue-next'

import { getReferralProfile, type ReferralProfile } from '@/api/referral'
import ReferralNav from '@/components/referral-nav/index.vue'

defineOptions({ name: 'ReferralOverview' })

const loading = ref(false)
const profile = ref<ReferralProfile | null>(null)

const inviteCode = computed(() => profile.value?.invite_code || '')
const inviteUrl = computed(() => {
  if (!inviteCode.value) return '—'
  return `${window.location.origin}/register?invite_code=${inviteCode.value}`
})

function money(v?: number): string {
  return Number(v || 0).toFixed(2)
}

function percent(v?: number): string {
  return `${(Number(v || 0) * 100).toFixed(2)}%`
}

async function copy(value: string, successMessage: string) {
  if (!value || value === '—') {
    MessagePlugin.warning('暂无可复制内容')
    return
  }
  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(successMessage)
  } catch {
    MessagePlugin.warning('复制失败，请手动选择内容')
  }
}

async function load() {
  loading.value = true
  try {
    const { data } = await getReferralProfile()
    profile.value = data
  } catch (e: any) {
    MessagePlugin.error(e?.message || '加载推广概览失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.referral-page { padding: 16px 24px; display: flex; flex-direction: column; gap: 16px; }
.referral-header { display: flex; align-items: center; justify-content: space-between; }
.referral-title { font-size: 20px; font-weight: 700; margin: 0; }
.referral-alert { border-radius: 10px; }
.stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; }
.stat-card {
  background: #fff; border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px; padding: 16px 18px; display: flex; flex-direction: column; gap: 6px;
}
.stat-card--primary { border-color: #f6d9a8; background: linear-gradient(180deg, #fffaf1 0%, #fff 100%); }
.stat-card__label { font-size: 13px; color: #888; }
.stat-card__value { font-size: 24px; font-weight: 700; color: #1f2937; }
.stat-card--primary .stat-card__value { color: #b76a00; }
.stat-card__hint { font-size: 12px; color: #999; }
.referral-panel {
  background: #fff; border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px; padding: 16px 20px;
}
.panel-head { margin-bottom: 12px; }
.section-title { font-size: 15px; font-weight: 700; }
.promo { display: flex; flex-direction: column; gap: 10px; }
.promo__row { display: flex; align-items: center; gap: 12px; font-size: 14px; }
.promo__label { width: 80px; color: #888; }
.promo__value { color: #333; word-break: break-all; }
.promo__value--code { font-family: monospace; font-weight: 700; color: #b76a00; }
.promo__tip { color: #999; font-size: 12px; margin: 0; }
.rate-grid { display: flex; gap: 12px; flex-wrap: wrap; }
.rate-item {
  flex: 1; min-width: 120px; border: 1px dashed var(--td-border-level-1-color, #e6e6e6);
  border-radius: 10px; padding: 12px 14px; display: flex; flex-direction: column; gap: 4px;
}
.rate-item__label { font-size: 13px; color: #888; }
.rate-item__value { font-size: 20px; font-weight: 700; color: #2563eb; }
.rate-tip { color: #999; font-size: 12px; margin: 12px 0 0; line-height: 1.6; }
</style>
