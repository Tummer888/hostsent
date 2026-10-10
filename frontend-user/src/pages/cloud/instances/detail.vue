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
        <!-- 维护类自助（重装/重置密码/救援/快照）：收纳在下拉里，操作栏不再膨胀 -->
        <t-dropdown :options="maintenanceOptions" trigger="click" @click="onMaintenancePick">
          <t-button theme="default" variant="outline">
            更多维护
            <template #suffix><t-icon name="chevron-down" /></template>
          </t-button>
        </t-dropdown>
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

    <!-- 重装系统（危险：清系统盘）。与销毁同款交互：手输实例 ID 二次确认。 -->
    <t-dialog
      v-model:visible="reinstall.visible"
      header="重装系统"
      :confirm-btn="{ content: '发起重装', theme: 'danger', loading: reinstall.submitting }"
      :close-on-overlay-click="false"
      width="520px"
      @confirm="submitReinstall"
    >
      <div class="destroy-body">
        <t-alert theme="error" class="destroy-alert">
          重装会<strong>清空系统盘</strong>（可选同时格式化数据盘），数据不可恢复。
        </t-alert>
        <t-form label-align="top">
          <t-form-item label="目标系统（平台镜像 ID）">
            <t-input v-model="reinstall.os" placeholder="如 62；不确定请联系客服" />
          </t-form-item>
          <t-form-item label="自定义端口（选填，SSH/RDP）">
            <t-input-number v-model="reinstall.port" :min="0" :max="65535" theme="column" />
          </t-form-item>
          <t-form-item label="同时格式化数据盘（数据将丢失）">
            <t-switch v-model="reinstall.format_data_disk" />
          </t-form-item>
        </t-form>
        <p class="destroy-tip">
          请输入实例 ID <code>{{ instance.instance_id }}</code> 以确认重装：
        </p>
        <t-input v-model="reinstall.confirmMark" placeholder="请输入实例 ID" size="large" />
        <div v-if="reinstall.needVerify" class="destroy-verify">
          <div class="destroy-code-row">
            <t-input v-model="reinstall.code" placeholder="请输入验证码" size="large" maxlength="6" />
            <t-button variant="outline" theme="primary" :disabled="reinstall.countdown > 0" @click="sendCode('instance_reinstall', reinstall)">
              {{ reinstall.countdown > 0 ? `${reinstall.countdown}s` : '获取验证码' }}
            </t-button>
          </div>
        </div>
      </div>
    </t-dialog>

    <!-- 重置密码 -->
    <t-dialog
      v-model:visible="resetPwd.visible"
      header="重置登录密码"
      :confirm-btn="{ content: '重置密码', loading: resetPwd.submitting }"
      width="460px"
      @confirm="submitResetPassword"
    >
      <t-alert theme="info" class="destroy-alert">系统与数据保留，仅更换登录密码。</t-alert>
      <t-input
        v-model="resetPwd.password"
        type="password"
        size="large"
        class="destroy-reason"
        placeholder="新密码（建议 12 位以上，含大小写与符号）"
      />
    </t-dialog>

    <!-- 救援系统 -->
    <t-dialog
      v-model:visible="rescue.visible"
      header="进入救援系统"
      :confirm-btn="{ content: '进入救援', loading: rescue.submitting }"
      width="480px"
      @confirm="submitRescue"
    >
      <t-alert theme="warning" class="destroy-alert">
        进入救援后系统变成临时系统，处理完请务必点「退出救援」回到原系统。
      </t-alert>
      <t-radio-group v-model="rescue.system" variant="default-filled" class="destroy-reason">
        <t-radio-button :value="1">类型 1</t-radio-button>
        <t-radio-button :value="2">类型 2</t-radio-button>
      </t-radio-group>
      <t-input v-model="rescue.temp_password" type="password" size="large" placeholder="临时密码" class="destroy-reason" />
    </t-dialog>

    <!-- 快照与备份 -->
    <t-dialog
      v-model:visible="snap.visible"
      header="快照与备份"
      width="680px"
      :footer="false"
      destroy-on-close
      @close="snap.visible = false"
    >
      <t-space class="snap-toolbar">
        <t-input v-model="snap.name" placeholder="快照名称（留空自动生成）" class="snap-grow" />
        <t-select v-model="snap.type" :options="snapTypeOptions" class="snap-select" />
        <t-button theme="primary" :loading="snap.loading" @click="submitCreateSnapshot">创建</t-button>
        <t-button variant="outline" :loading="snap.loading" @click="loadSnapshots()">刷新</t-button>
      </t-space>
      <t-table row-key="id" :data="snap.rows" :columns="snapColumns" :loading="snap.loading" size="small" cell-empty-content="暂无快照">
        <template #snap_action="{ row }">
          <t-space size="small">
            <t-link theme="warning" hover="color" @click="openRestoreSnapshot(row)">恢复</t-link>
            <t-link theme="danger" hover="color" @click="submitDeleteSnapshot(row)">删除</t-link>
          </t-space>
        </template>
      </t-table>
      <div class="destroy-verify">
        <p class="destroy-tip">用快照恢复会覆盖当前系统盘，请输入实例 ID <code>{{ instance.instance_id }}</code> 确认：</p>
        <t-input v-model="snap.restoreMark" placeholder="实例 ID" />
        <div v-if="snap.needVerify" class="destroy-code-row">
          <t-input v-model="snap.code" placeholder="请输入验证码" maxlength="6" />
          <t-button variant="outline" theme="primary" :disabled="snap.countdown > 0" @click="sendCode('instance_snapshot_restore', snap)">
            {{ snap.countdown > 0 ? `${snap.countdown}s` : '获取验证码' }}
          </t-button>
        </div>
        <div v-if="snap.restoreId" class="destroy-code-row">
          <t-button theme="danger" variant="outline" :disabled="snap.restoreMark.trim() !== instance.instance_id" @click="submitRestoreSnapshot">
            恢复所选快照
          </t-button>
        </div>
      </div>
    </t-dialog>

    <!-- 重装后平台签发的新凭据：只显示一次 -->
    <t-dialog
      v-model:visible="credential.visible"
      header="重装已受理 · 请立即保存新凭据"
      :confirm-btn="{ content: '我已保存' }"
      :close-btn="false"
      width="460px"
      @confirm="credential.visible = false"
    >
      <t-alert theme="warning" class="destroy-alert">
        平台新签发的登录凭据只显示这一次，本系统不保存。
      </t-alert>
      <div class="destroy-body">
        <p class="destroy-tip">用户名：<code>{{ credential.username || '—' }}</code></p>
        <p class="destroy-tip">新密码：<code>{{ credential.password || '（平台未返回，沿用原密码）' }}</code></p>
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
import {
  createInstanceSnapshot,
  deleteInstanceSnapshot,
  exitRescueInstance,
  listInstanceSnapshots,
  reinstallInstance,
  resetInstancePassword,
  rescueInstance,
  restoreInstanceSnapshot,
  type SnapshotInfo,
} from '@/api/cloud'
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

// ===== 维护类自助操作 =====
//
// 全部走「更多维护 ▾」下拉：电源栏已经有 5 个按钮，再加 6 个会挤成一团。
// 每个动作都在服务端复用运维台的能力分派 + 操作流水 + 能力位校验，
// 前端只负责收集参数、做二次确认与展示一次性凭据。

const maintenanceOptions = computed(() => [
  { content: '重装系统', value: 'reinstall' },
  { content: '重置密码', value: 'resetpwd' },
  { content: '进入救援系统', value: 'rescue' },
  { content: '退出救援系统', value: 'exitrescue' },
  { content: '快照与备份', value: 'snapshot' },
])

function onMaintenancePick(data: { value: string }) {
  switch (data.value) {
    case 'reinstall':
      Object.assign(reinstall, {
        os: '', port: 0, format_data_disk: false, confirmMark: '', code: '',
        needVerify: false, visible: true,
      })
      void prepareVerify('instance_reinstall', reinstall)
      break
    case 'resetpwd':
      resetPwd.password = ''
      resetPwd.visible = true
      break
    case 'rescue':
      rescue.system = 1
      rescue.temp_password = ''
      rescue.visible = true
      break
    case 'exitrescue':
      void submitExitRescue()
      break
    case 'snapshot':
      snap.name = ''
      snap.type = 'snap'
      snap.restoreId = ''
      snap.restoreMark = ''
      snap.visible = true
      loadSnapshots()
      break
  }
}

/** 需要二次验证的维护动作（重装/快照恢复）：先问一次策略，填完不必被打回。 */
async function prepareVerify(scene: string, target: { needVerify: boolean }) {
  try {
    const { data } = await getVerificationRequirement(scene)
    target.needVerify = !!data.need_verification
  } catch {
    // 查询失败不阻断：真需要验证时后端返回 20017，提交阶段再补。
  }
}

/** 发验证码并倒计时（重装与快照恢复共用）；通道由场景策略决定。 */
async function sendCode(scene: string, target: { countdown: number; sendingCode: boolean }) {
  target.sendingCode = true
  try {
    let channel: 'sms' | 'email' = 'sms'
    try {
      const { data } = await getVerificationRequirement(scene)
      channel = data.channel === 'email' ? 'email' : 'sms'
    } catch {
      // 取不到策略就按短信通道试一次，失败时用户会看到明确原因。
    }
    await sendSecurityVerification({ scene, channel })
    MessagePlugin.success('验证码已发送，请查收')
    target.countdown = 60
    const timer = setInterval(() => {
      target.countdown -= 1
      if (target.countdown <= 0) clearInterval(timer)
    }, 1000)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '验证码发送失败')
  } finally {
    target.sendingCode = false
  }
}

/** 取二次验证票据；不需要验证时返回空串。 */
async function acquireTicket(scene: string, needVerify: boolean, code: string): Promise<string> {
  if (!needVerify) return ''
  if (!code.trim()) throw new Error('请输入验证码')
  const { data } = await verifySecurityCode({ scene, code: code.trim() })
  return data.verify_ticket
}

/** 二次验证缺失（403 / 20017）时把验证码区展开。 */
function isVerifyRequired(error: unknown): boolean {
  const e = error as { response?: { status?: number; data?: { code?: number } } }
  return e?.response?.status === 403 || e?.response?.data?.code === 20017
}

const reinstall = reactive({
  visible: false, os: '', port: 0, format_data_disk: false,
  confirmMark: '', code: '', countdown: 0, sendingCode: false,
  needVerify: false, submitting: false,
})

const credential = reactive({ visible: false, username: '', password: '' })

async function submitReinstall() {
  if (!reinstall.os.trim()) {
    MessagePlugin.warning('请填写目标系统（镜像 ID）')
    return
  }
  if (reinstall.confirmMark.trim() !== (instance.value?.instance_id || '')) {
    MessagePlugin.warning('实例 ID 输入不一致，请准确输入后再试')
    return
  }
  reinstall.submitting = true
  try {
    const ticket = await acquireTicket('instance_reinstall', reinstall.needVerify, reinstall.code)
    const res = await reinstallInstance(
      Number(route.params.id),
      {
        os: reinstall.os.trim(),
        port: reinstall.port || undefined,
        format_data_disk: reinstall.format_data_disk,
      },
      ticket || undefined,
    )
    reinstall.visible = false
    const out = res.data || {}
    if (out.password || out.username) {
      credential.username = out.username || ''
      credential.password = out.password || ''
      credential.visible = true
    } else {
      MessagePlugin.success('重装指令已提交')
    }
    await load(true)
  } catch (error) {
    if (isVerifyRequired(error)) {
      reinstall.needVerify = true
      MessagePlugin.warning('该操作需要二次验证，请输入验证码后重试')
    } else {
      const e = error as { response?: { data?: { message?: string } }; message?: string }
      MessagePlugin.error(e?.response?.data?.message || e?.message || '重装失败')
    }
  } finally {
    reinstall.submitting = false
  }
}

const resetPwd = reactive({ visible: false, password: '', submitting: false })

async function submitResetPassword() {
  if (!resetPwd.password) {
    MessagePlugin.warning('请输入新密码')
    return
  }
  resetPwd.submitting = true
  try {
    await resetInstancePassword(Number(route.params.id), resetPwd.password)
    MessagePlugin.success('密码已重置')
    resetPwd.visible = false
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '重置密码失败')
  } finally {
    resetPwd.submitting = false
  }
}

const rescue = reactive({ visible: false, system: 1, temp_password: '', submitting: false })

async function submitRescue() {
  if (!rescue.temp_password) {
    MessagePlugin.warning('请输入临时密码')
    return
  }
  rescue.submitting = true
  try {
    await rescueInstance(Number(route.params.id), rescue.system, rescue.temp_password)
    MessagePlugin.success('已发起进入救援系统')
    rescue.visible = false
    await load(true)
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '进入救援系统失败')
  } finally {
    rescue.submitting = false
  }
}

async function submitExitRescue() {
  try {
    await exitRescueInstance(Number(route.params.id))
    MessagePlugin.success('已发起退出救援系统')
    await load(true)
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '退出救援系统失败')
  }
}

const snapTypeOptions = [
  { label: '快照', value: 'snap' },
  { label: '备份', value: 'backup' },
]

const snapColumns = [
  { colKey: 'name', title: '名称', minWidth: 180 },
  { colKey: 'type', title: '类型', width: 80 },
  { colKey: 'size', title: '大小', width: 90 },
  { colKey: 'create_time', title: '创建时间', width: 170 },
  { colKey: 'snap_action', title: '操作', width: 130, align: 'center' as const },
]

const snap = reactive({
  visible: false,
  loading: false,
  rows: [] as SnapshotInfo[],
  name: '',
  type: 'snap',
  restoreId: '',
  restoreMark: '',
  code: '',
  needVerify: false,
  countdown: 0,
  sendingCode: false,
})

async function loadSnapshots() {
  snap.loading = true
  try {
    const res = await listInstanceSnapshots(Number(route.params.id))
    snap.rows = res.data || []
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '加载快照失败')
  } finally {
    snap.loading = false
  }
}

async function submitCreateSnapshot() {
  snap.loading = true
  try {
    await createInstanceSnapshot(Number(route.params.id), { type: snap.type, name: snap.name || undefined })
    MessagePlugin.success('快照创建指令已提交（异步执行，稍后刷新查看）')
    snap.name = ''
    setTimeout(() => loadSnapshots(), 5000)
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '创建快照失败')
  } finally {
    snap.loading = false
  }
}

function openRestoreSnapshot(row: SnapshotInfo) {
  snap.restoreId = row.id
  snap.restoreMark = ''
  snap.code = ''
  snap.needVerify = false
  void prepareVerify('instance_snapshot_restore', snap)
}

async function submitRestoreSnapshot() {
  if (snap.restoreMark.trim() !== (instance.value?.instance_id || '')) {
    MessagePlugin.warning('实例 ID 输入不一致')
    return
  }
  try {
    const ticket = await acquireTicket('instance_snapshot_restore', snap.needVerify, snap.code)
    await restoreInstanceSnapshot(
      Number(route.params.id),
      snap.restoreId,
      snap.restoreMark.trim(),
      ticket || undefined,
    )
    MessagePlugin.success('恢复指令已提交')
    snap.restoreId = ''
    snap.restoreMark = ''
    await load(true)
  } catch (error) {
    if (isVerifyRequired(error)) {
      snap.needVerify = true
      MessagePlugin.warning('该操作需要二次验证，请输入验证码后重试')
      return
    }
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '恢复失败')
  }
}

async function submitDeleteSnapshot(row: SnapshotInfo) {
  try {
    await deleteInstanceSnapshot(Number(route.params.id), row.id)
    MessagePlugin.success('快照已删除')
    loadSnapshots()
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '删除失败')
  }
}
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
/* 快照弹窗工具栏 */
.snap-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  width: 100%;
}
.snap-grow {
  flex: 1;
}
.snap-select {
  width: 110px;
}
@media (max-width: 768px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
