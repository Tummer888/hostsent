<template>
  <div v-if="instance" class="detail-page">
    <div class="detail-header">
      <div class="detail-title-wrap">
        <t-button theme="default" variant="text" @click="$router.back()">
          <template #icon><t-icon name="arrow-left" /></template>
          返回
        </t-button>
        <h2 class="detail-title">{{ instance.name }}</h2>
        <t-tag :theme="statusTheme" variant="light" shape="round">{{ statusText }}</t-tag>
      </div>
      <div class="power-bar">
        <t-button theme="success" :disabled="instance.power_status === 'on'" @click="onPower('on')">
          <template #icon><t-icon name="poweroff" /></template>
          开机
        </t-button>
        <t-button theme="danger" :disabled="instance.power_status === 'off'" @click="onPower('off')">
          <template #icon><t-icon name="poweroff" /></template>
          关机
        </t-button>
        <t-button theme="warning" :disabled="instance.power_status !== 'on'" @click="onPower('reboot')">
          <template #icon><t-icon name="refresh" /></template>
          重启
        </t-button>
        <t-button theme="primary" :disabled="instance.status !== 'running'" @click="onVNC">
          <template #icon><t-icon name="screen-zoom" /></template>
          VNC 控制台
        </t-button>
      </div>
    </div>

    <div class="detail-grid">
      <div class="card">
        <h3 class="card-title">基础信息</h3>
        <div class="kv-list">
          <div class="kv"><span class="kv-key">实例 ID</span><span class="kv-val">{{ instance.instance_id }}</span></div>
          <div class="kv"><span class="kv-key">公网 IP</span><span class="kv-val">{{ instance.public_ip || '—' }}</span></div>
          <div class="kv"><span class="kv-key">私有 IP</span><span class="kv-val">{{ instance.private_ip || '—' }}</span></div>
          <div class="kv"><span class="kv-key">操作系统</span><span class="kv-val">{{ instance.os || '—' }}</span></div>
          <div class="kv"><span class="kv-key">区域</span><span class="kv-val">{{ instance.region || instance.zone || '—' }}</span></div>
          <div class="kv"><span class="kv-key">计费方式</span><span class="kv-val">{{ billingText }}</span></div>
        </div>
      </div>

      <div class="card">
        <h3 class="card-title">配置规格</h3>
        <div class="kv-list">
          <div class="kv"><span class="kv-key">CPU</span><span class="kv-val">{{ instance.cpu }} 核</span></div>
          <div class="kv"><span class="kv-key">内存</span><span class="kv-val">{{ instance.memory }} MB</span></div>
          <div class="kv"><span class="kv-key">系统盘</span><span class="kv-val">{{ instance.disk }} GB</span></div>
          <div class="kv"><span class="kv-key">带宽</span><span class="kv-val">{{ instance.bandwidth }} Mbps</span></div>
          <div class="kv"><span class="kv-key">到期时间</span><span class="kv-val">{{ instance.expire_at ? instance.expire_at.replace('T', ' ').slice(0, 16) : '—' }}</span></div>
          <div class="kv"><span class="kv-key">创建时间</span><span class="kv-val">{{ instance.created_at?.replace('T', ' ').slice(0, 16) || '—' }}</span></div>
        </div>
      </div>
    </div>

    <t-dialog v-model:visible="vnc.visible" :header="`远程控制台 · ${instance.name}`" width="80%" :footer="false" destroy-on-close>
      <div class="vnc-wrap">
        <iframe v-if="vnc.url && isHttp(vnc.url)" :src="vnc.url" class="vnc-frame" frameborder="0" />
        <t-empty v-else :description="'控制台地址无法内嵌显示，请点击打开'">
          <template #action>
            <t-button theme="primary" @click="openVnc">打开控制台</t-button>
          </template>
        </t-empty>
      </div>
    </t-dialog>
  </div>
  <div v-else class="loading-wrap"><t-loading size="large" text="加载中..." /></div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'

import { getInstance, powerInstance, vncInstance, type InstanceInfo } from '@/api/cloud'

defineOptions({ name: 'InstanceDetail' })

const route = useRoute()
const instance = ref<InstanceInfo | null>(null)

const statusText = computed(() => {
  const s = instance.value?.status
  if (s === 'running') return '运行中'
  if (s === 'stopped') return '已关机'
  if (s === 'creating') return '创建中'
  return s || '未知'
})
const statusTheme = computed<'success' | 'warning' | 'default' | 'danger'>(() => {
  const s = instance.value?.status
  if (s === 'running') return 'success'
  if (s === 'creating') return 'warning'
  return 'default'
})
const billingText = computed(() => {
  const b = instance.value?.billing_mode
  return { monthly: '按月付费', quarterly: '按季付费', annually: '按年付费', hourly: '按小时付费' }[b || ''] || b || '—'
})

const vnc = ref<{ visible: boolean; url: string }>({ visible: false, url: '' })

async function load(live = false) {
  const id = Number(route.params.id)
  const res = await getInstance(id, live)
  instance.value = res.data
}

async function onPower(action: 'on' | 'off' | 'reboot') {
  const label = { on: '开机', off: '关机', reboot: '重启' }[action]
  try {
    await powerInstance(Number(route.params.id), action)
    MessagePlugin.success(`${label}指令已发送`)
    await load(true)
  } catch {
    /* 错误已由拦截器提示 */
  }
}

async function onVNC() {
  try {
    const res = await vncInstance(Number(route.params.id))
    vnc.value = { visible: true, url: res.data?.url || '' }
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function isHttp(url: string): boolean {
  return /^https?:/i.test(url)
}

function openVnc() {
  window.open(vnc.value.url, '_blank')
}

onMounted(() => load(true))
</script>

<style scoped>
.detail-page {
  padding: 4px 0;
}
.loading-wrap {
  display: flex;
  justify-content: center;
  padding: 60px;
}
.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
}
.detail-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}
.detail-title {
  font-size: 22px;
  font-weight: 600;
}
.power-bar {
  display: flex;
  gap: 8px;
}
.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  padding: 18px 20px;
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 14px;
}
.kv-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.kv {
  display: flex;
  justify-content: space-between;
}
.kv-key {
  color: var(--td-text-color-secondary);
}
.kv-val {
  font-weight: 500;
}
.vnc-wrap {
  height: 62vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
.vnc-frame {
  width: 100%;
  height: 100%;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  background: #000;
}
@media (max-width: 768px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
