<template>
  <div class="agent-page">
    <AgentNav />

    <div class="agent-header">
      <h2 class="agent-title">推广素材</h2>
    </div>

    <section class="agent-panel">
      <div class="panel-head">
        <span class="section-title">推广链接与邀请码</span>
      </div>
      <div class="promo">
        <div class="promo__row">
          <span class="promo__label">邀请码</span>
          <span class="promo__value promo__value--code">{{ profile?.invite_code || '—' }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copyText(profile?.invite_code || '', '邀请码已复制')">
            复制
          </t-button>
        </div>
        <div class="promo__row">
          <span class="promo__label">推广链接</span>
          <span class="promo__value">{{ inviteUrl }}</span>
          <t-button size="small" variant="outline" :disabled="!profile?.invite_code" @click="copyText(inviteUrl, '推广链接已复制')">
            复制
          </t-button>
        </div>
        <p class="promo__tip">链接已内置你的邀请码，客户注册后自动归属为你的下级。</p>
      </div>
    </section>

    <section class="agent-panel">
      <div class="panel-head">
        <span class="section-title">推广文案模板</span>
      </div>
      <div class="templates">
        <div v-for="tpl in templates" :key="tpl.label" class="template">
          <div class="template__head">
            <span class="template__label">{{ tpl.label }}</span>
            <t-button size="small" variant="text" theme="primary" @click="copyText(tpl.render(inviteUrl, profile?.invite_code), '文案已复制')">
              复制文案
            </t-button>
          </div>
          <p class="template__body">{{ tpl.render(inviteUrl, profile?.invite_code) }}</p>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import { getAgentProfile, type AgentProfile } from '@/api/agent'
import AgentNav from '@/components/agent-nav/index.vue'

defineOptions({ name: 'AgentMaterials' })

const profile = ref<AgentProfile | null>(null)

const inviteUrl = computed(() => {
  if (!profile.value?.invite_code) return '—'
  const path = profile.value.invite_link || `/register?invite_code=${profile.value.invite_code}`
  return `${window.location.origin}${path.startsWith('/') ? path : `/${path}`}`
})

const templates = [
  {
    label: '社群短文案',
    render: (url: string) => `宿派云控新用户福利，通过专属链接注册享优惠：${url}`,
  },
  {
    label: '邀请码文案',
    render: (_url: string, code?: string) => `注册宿派云控时填写邀请码 ${code || '—'}，即可绑定专属服务与优惠。`,
  },
  {
    label: '长文案',
    render: (url: string) =>
      `宿派云控提供云主机、对象存储与数据库等云产品，专业团队 7×24 支持。通过我的专属链接注册，可享受专属折扣与一对一服务：${url}`,
  },
]

async function copyText(value: string, successMessage: string) {
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
  try {
    const { data } = await getAgentProfile()
    profile.value = data
  } catch (e: any) {
    MessagePlugin.error(e?.message || '加载代理信息失败')
  }
}

onMounted(load)
</script>

<style scoped>
.agent-page { padding: 16px 24px; display: flex; flex-direction: column; gap: 16px; }
.agent-header { display: flex; align-items: center; justify-content: space-between; }
.agent-title { font-size: 20px; font-weight: 700; margin: 0; }
.agent-panel {
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
.templates { display: flex; flex-direction: column; gap: 12px; }
.template {
  border: 1px dashed var(--td-border-level-1-color, #e6e6e6);
  border-radius: 10px; padding: 12px 14px;
}
.template__head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
.template__label { font-size: 13px; font-weight: 600; color: #444; }
.template__body { margin: 0; color: #666; font-size: 13px; line-height: 1.7; word-break: break-all; }
</style>
