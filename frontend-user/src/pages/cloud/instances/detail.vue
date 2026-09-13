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
        <!-- 销毁入口（doc91 §6.4）：危险操作，二次确认需手输实例标识 -->
        <t-button theme="danger" variant="outline" @click="openDestroy">
          <template #icon><t-icon name="delete" /></template>
          销毁实例
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

    <!--
      销毁确认（doc91 §6.4）：与 admin 侧交互一致 —— 必须手输实例标识。
      文案明确「不可逆」与「不退还剩余费用」，避免用户误以为会退款。
    -->
    <t-dialog
      v-model:visible="destroy.visible"
      header="销毁实例"
      :confirm-btn="{ content: '确认销毁', theme: 'danger', loading: destroy.submitting }"
      :close-on-overlay-click="false"
      width="480px"
      @confirm="submitDestroy"
    >
      <div class="destroy-body">
        <t-alert theme="error" class="destroy-alert">
          销毁操作<strong>不可逆</strong>，实例数据将无法恢复，且<strong>不退还剩余费用</strong>。
        </t-alert>
        <p class="destroy-tip">
          请输入实例 ID <code>{{ instance.instance_id }}</code> 以确认销毁：
        </p>
        <t-input v-model="destroy.confirmMark" placeholder="请输入实例 ID" size="large" />
        <t-input
          v-model="destroy.reason"
          placeholder="销毁原因（选填）"
          size="large"
          class="destroy-reason"
        />
        <!-- 二次验证：策略开启后需要验证码，后端返回 20017 时展开此区域 -->
        <div v-if="destroy.needVerify" class="destroy-verify">
          <p class="destroy-tip">
            该操作需要二次验证，验证码将发送至账号绑定的{{ destroy.channelLabel }}。
          </p>
          <div class="destroy-code-row">
            <t-input v-model="destroy.code" placeholder="请输入验证码" size="large" maxlength="6" />
            <t-button
              variant="outline"
              theme="primary"
              :disabled="destroy.countdown > 0"
              :loading="destroy.sendingCode"
              @click="sendDestroyCode"
            >
              {{ destroy.countdown > 0 ? `${destroy.countdown}s` : '获取验证码' }}
            </t-button>
          </div>
        </div>
      </div>
    </t-dialog>
  </div>
  <div v-else class="loading-wrap"><t-loading size="large" text="加载中..." /></div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'

import { destroyInstance, getInstance, powerInstance, vncInstance, type InstanceInfo } from '@/api/cloud'
import { getVerificationRequirement, sendSecurityVerification, verifySecurityCode } from '@/api/security'

defineOptions({ name: 'InstanceDetail' })

const route = useRoute()
const router = useRouter()
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

// —— 销毁实例（doc91 §6.4）——
const destroy = reactive({
  visible: false,
  confirmMark: '',
  reason: '',
  submitting: false,
  // 二次验证：策略要求时才展开验证码区（后端 403 + 20017 会告知）
  needVerify: false,
  channel: 'sms',
  channelLabel: '手机',
  code: '',
  sendingCode: false,
  countdown: 0,
})

/** 打开销毁弹窗：先查一次该场景是否需要二次验证，避免用户填完才被打回。 */
async function openDestroy() {
  destroy.confirmMark = ''
  destroy.reason = ''
  destroy.code = ''
  destroy.needVerify = false
  destroy.visible = true
  try {
    const { data } = await getVerificationRequirement('instance_destroy')
    if (data.need_verification) {
      destroy.needVerify = true
      destroy.channel = data.channel === 'email' ? 'email' : 'sms'
      destroy.channelLabel = destroy.channel === 'sms' ? '手机' : '邮箱'
    }
  } catch {
    // 查询失败不阻断：真需要验证时后端会在提交阶段返回 20017，再走一次补验。
  }
}

async function sendDestroyCode() {
  destroy.sendingCode = true
  try {
    await sendSecurityVerification({ scene: 'instance_destroy', channel: destroy.channel as 'sms' | 'email' })
    MessagePlugin.success('验证码已发送，请查收')
    destroy.countdown = 60
    const timer = setInterval(() => {
      destroy.countdown -= 1
      if (destroy.countdown <= 0) clearInterval(timer)
    }, 1000)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败')
  } finally {
    destroy.sendingCode = false
  }
}

async function submitDestroy() {
  const id = Number(route.params.id)
  const expected = instance.value?.instance_id || ''
  if (destroy.confirmMark.trim() !== expected) {
    MessagePlugin.warning('实例 ID 输入不一致，请准确输入后再试')
    return
  }
  destroy.submitting = true
  try {
    let ticket = ''
    if (destroy.needVerify) {
      if (!destroy.code.trim()) {
        MessagePlugin.warning('请输入验证码')
        return
      }
      const { data } = await verifySecurityCode({ scene: 'instance_destroy', code: destroy.code.trim() })
      ticket = data.verify_ticket
    }
    await destroyInstance(id, destroy.confirmMark.trim(), destroy.reason.trim() || undefined, ticket || undefined)
    destroy.visible = false
    MessagePlugin.success('实例已销毁')
    router.replace('/cloud/instances')
  } catch (error) {
    // 403 且业务码 20017 = 缺二次验证票据：展开验证码区让用户补验后重试。
    // 响应拦截器抛的是 axios 原始 error（非 2xx 走 error 分支），因此状态码与
    // 业务码要从 response 上读，不能只看 error.code。
    const axiosErr = error as {
      response?: { status?: number; data?: { code?: number; message?: string } }
      message?: string
    }
    const bizCode = axiosErr?.response?.data?.code
    if (axiosErr?.response?.status === 403 || bizCode === 20017) {
      destroy.needVerify = true
      MessagePlugin.warning('该操作需要二次验证，请输入验证码后重试')
    } else {
      MessagePlugin.error(axiosErr?.response?.data?.message || axiosErr?.message || '销毁失败，请稍后重试')
    }
  } finally {
    destroy.submitting = false
  }
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
/* 销毁确认弹窗 */
.destroy-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.destroy-alert {
  border-radius: 8px;
}
.destroy-tip {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-secondary);
  line-height: 1.7;
}
.destroy-tip code {
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-primary);
  font-family: 'Courier New', Consolas, monospace;
}
.destroy-reason {
  margin-top: 2px;
}
.destroy-verify {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.destroy-code-row {
  display: flex;
  gap: 10px;
}
.destroy-code-row > :first-child {
  flex: 1;
  min-width: 0;
}
@media (max-width: 768px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
