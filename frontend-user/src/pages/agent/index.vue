<template>
  <div class="agent-page">
    <AgentNav />

    <section class="agent-hero">
      <div class="hero-left">
        <span class="hero-chip"><UserCircleIcon size="22" /></span>
        <div class="hero-info">
          <span class="hero-label">代理中心</span>
          <span class="hero-desc">
            团队业绩、佣金与结算一览。代理价在购买时自动生效，无需手动改价。
          </span>
        </div>
      </div>
      <t-tag v-if="profile" theme="warning" variant="light" size="large">
        {{ profile.level_name || '代理' }}
      </t-tag>
    </section>

    <div v-loading="loading" class="agent-stats">
      <t-card class="stat-card" :bordered="true">
        <span class="stat-label">直属下级</span>
        <span class="stat-value">{{ stats?.direct_sub_count ?? profile?.direct_sub_count ?? 0 }}</span>
        <span class="stat-sub">团队共 {{ stats?.team_sub_count ?? profile?.team_sub_count ?? 0 }} 人</span>
      </t-card>
      <t-card class="stat-card" :bordered="true">
        <span class="stat-label">待结算佣金</span>
        <span class="stat-value stat-value--money">¥{{ money(stats?.pending_commission) }}</span>
        <span class="stat-sub">已结算 ¥{{ money(stats?.settled_commission) }}</span>
      </t-card>
      <t-card class="stat-card" :bordered="true">
        <span class="stat-label">累计佣金</span>
        <span class="stat-value stat-value--money">¥{{ money(profile?.total_commission) }}</span>
        <span class="stat-sub">可结算余额 ¥{{ money(profile?.available_balance) }}</span>
      </t-card>
      <t-card class="stat-card" :bordered="true">
        <span class="stat-label">结算进度</span>
        <span class="stat-value stat-value--money">¥{{ money(stats?.paid_settlement) }}</span>
        <span class="stat-sub">待发放 ¥{{ money(stats?.pending_settlement) }}</span>
      </t-card>
    </div>

    <section class="agent-panel">
      <div class="panel-head">
        <span class="section-title">我的推广</span>
        <t-button size="small" variant="text" theme="primary" @click="router.push('/agent/materials')">
          更多推广素材
        </t-button>
      </div>
      <div class="promo">
        <div class="promo__row">
          <span class="promo__label">邀请码</span>
          <span class="promo__value promo__value--code">{{ profile?.invite_code || '—' }}</span>
        </div>
        <div class="promo__row">
          <span class="promo__label">推广链接</span>
          <span class="promo__value">{{ inviteUrl }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copyInvite">
            复制
          </t-button>
        </div>
        <p class="promo__tip">客户通过该链接注册后即成为你的下级，其后续下单按你的代理等级佣金率计佣。</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { UserCircleIcon } from 'tdesign-icons-vue-next'

import { getAgentProfile, getAgentStats, type AgentProfile, type AgentStats } from '@/api/agent'
import AgentNav from '@/components/agent-nav/index.vue'

defineOptions({ name: 'AgentOverview' })

const router = useRouter()
const profile = ref<AgentProfile | null>(null)
const stats = ref<AgentStats | null>(null)
const loading = ref(false)

const inviteUrl = computed(() => {
  if (!profile.value?.invite_code) return '—'
  const path = profile.value.invite_link || `/register?invite_code=${profile.value.invite_code}`
  return `${window.location.origin}${path.startsWith('/') ? path : `/${path}`}`
})

function money(value?: number): string {
  return Number(value || 0).toFixed(2)
}

async function load() {
  loading.value = true
  try {
    const [{ data: profileData }, { data: statsData }] = await Promise.all([getAgentProfile(), getAgentStats()])
    profile.value = profileData
    stats.value = statsData
  } catch (e: any) {
    MessagePlugin.error(e?.message || '加载代理信息失败')
  } finally {
    loading.value = false
  }
}

async function copyInvite() {
  try {
    await navigator.clipboard.writeText(inviteUrl.value)
    MessagePlugin.success('推广链接已复制')
  } catch {
    MessagePlugin.warning('复制失败，请手动选择链接')
  }
}

onMounted(load)
</script>

<style scoped>
.agent-page { padding: 16px 24px; display: flex; flex-direction: column; gap: 16px; }
.agent-hero {
  display: flex; align-items: center; justify-content: space-between;
  background: linear-gradient(120deg, #fff7e8, #ffffff);
  border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px; padding: 18px 20px;
}
.hero-left { display: flex; align-items: center; gap: 14px; }
.hero-chip {
  display: inline-flex; align-items: center; justify-content: center;
  width: 42px; height: 42px; border-radius: 12px;
  background: #ffe6bf; color: #b76a00;
}
.hero-info { display: flex; flex-direction: column; gap: 4px; }
.hero-label { font-size: 17px; font-weight: 700; }
.hero-desc { color: #888; font-size: 13px; }
.agent-stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 14px; }
.stat-card { display: flex; flex-direction: column; gap: 6px; }
.stat-label { color: #888; font-size: 13px; }
.stat-value { font-size: 22px; font-weight: 700; }
.stat-value--money { color: #e37318; }
.stat-sub { color: #b0b0b0; font-size: 12px; }
.agent-panel {
  background: #fff; border: 1px solid var(--td-border-level-1-color, #f0f0f0);
  border-radius: 12px; padding: 16px 20px;
}
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.section-title { font-size: 15px; font-weight: 700; }
.promo { display: flex; flex-direction: column; gap: 10px; }
.promo__row { display: flex; align-items: center; gap: 12px; font-size: 14px; }
.promo__label { width: 80px; color: #888; }
.promo__value { color: #333; word-break: break-all; }
.promo__value--code { font-family: monospace; font-weight: 700; color: #b76a00; }
.promo__tip { color: #999; font-size: 12px; margin: 0; }
</style>
