<template>
  <div v-if="instance" class="page-body console-module detail-page">
    <!-- 页头：返回 + 主机标识 + 状态结论 + 到期信息 -->
    <header class="page-header surface-card">
      <div class="page-header__main">
        <button class="back-btn" type="button" @click="$router.push('/cloud/instances')">
          <ChevronLeftIcon size="18" aria-hidden="true" />
        </button>
        <span class="page-header__chip"><CloudIcon size="22" aria-hidden="true" /></span>
        <div class="page-header__text">
          <h2 class="page-header__title">
            {{ instance.name || instance.instance_id }}
            <t-tag :theme="state.theme" variant="light" size="small" shape="round">
              <template #icon>
                <component :is="stateIconComponent(state.label)" />
              </template>
              {{ state.label }}
            </t-tag>
          </h2>
          <p class="page-header__desc">
            实例 ID {{ instance.instance_id }}
            <span class="dot">·</span>
            {{ instance.os || '未知系统' }}
            <span class="dot">·</span>
            {{ isNatInstance ? natAddresses[0] || 'NAT 机型（暂无转发地址）' : instance.public_ip || '未分配公网 IP' }}
          </p>
        </div>
      </div>
      <div class="page-header__actions">
        <t-button variant="outline" :loading="loading" @click="load(true)">
          <template #icon><RefreshIcon /></template>
          刷新状态
        </t-button>
      </div>
    </header>

    <!-- 状态说明：把「为什么是这个状态」写清楚（待开通 / 到期 / 暂停） -->
    <t-alert
      v-if="isPending"
      theme="warning"
      class="detail-alert"
      :title="provisionAlertTitle"
      :message="provisionAlertMessage"
    />
    <t-alert v-else-if="stateAlert" :theme="stateAlert.theme" class="detail-alert" :message="stateAlert.message">
      <template v-if="stateAlert.action" #operation>
        <t-button theme="primary" size="small" variant="outline" @click="$router.push('/cloud/renewals')">
          {{ stateAlert.action }}
        </t-button>
      </template>
    </t-alert>

    <!-- 电源与维护操作条 -->
    <section class="surface-card action-bar">
      <template v-if="isPending">
        <t-button theme="primary" :loading="retrying" @click="onRetry">
          <template #icon><RefreshIcon /></template>
          重新开通
        </t-button>
        <span class="action-bar__hint">实例尚未开通完成，开通成功后才能进行电源与控制台操作</span>
      </template>
      <template v-else>
        <t-button theme="success" :disabled="!caps.power || instance.power_status === 'on'" @click="onPower('on')">
          <template #icon><PlayCircleIcon /></template>
          开机
        </t-button>
        <t-button theme="danger" :disabled="!caps.power || instance.power_status === 'off'" @click="onPower('off')">
          <template #icon><PoweroffIcon /></template>
          关机
        </t-button>
        <t-button theme="warning" :disabled="!caps.power || instance.power_status !== 'on'" @click="onPower('reboot')">
          <template #icon><RefreshIcon /></template>
          重启
        </t-button>
        <t-button
          theme="danger"
          variant="outline"
          :disabled="!caps.power || instance.power_status === 'off'"
          @click="onPower('hard_off', true)"
        >
          强制关机
        </t-button>
        <t-button
          theme="danger"
          variant="outline"
          :disabled="!caps.power || instance.power_status !== 'on'"
          @click="onPower('hard_reboot', true)"
        >
          强制重启
        </t-button>
        <t-button theme="primary" :disabled="!caps.console || instance.status !== 'running'" @click="onVNC">
          <template #icon><TerminalIcon /></template>
          VNC 控制台
        </t-button>
        <!-- 维护类自助（重装/重置密码/救援/快照）：收纳进下拉，操作条不再膨胀 -->
        <t-dropdown :options="maintenanceOptions" trigger="click" @click="onMaintenancePick">
          <t-button theme="default" variant="outline">
            更多维护
            <template #suffix><ChevronDownIcon /></template>
          </t-button>
        </t-dropdown>
        <t-button v-if="caps.destroy" theme="danger" variant="outline" @click="openDestroy">
          <template #icon><DeleteIcon /></template>
          销毁实例
        </t-button>
      </template>
    </section>

    <!-- 配置概览：四张统计卡片，与列表页视觉一致 -->
    <section class="spec-stat-grid">
      <div class="stat-card surface-card stat-card--blue">
        <span class="stat-card__icon"><CpuIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ instance.cpu }}</span>
          <span class="stat-card__hint">CPU 核心</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--purple">
        <span class="stat-card__icon"><DashboardIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ formatMemory(instance.memory) }}</span>
          <span class="stat-card__hint">内存</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--cyan">
        <span class="stat-card__icon"><HardDiskStorageIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ instance.disk }} GB</span>
          <span class="stat-card__hint">系统盘{{ instance.disk_type ? `（${instance.disk_type}）` : '' }}</span>
        </div>
      </div>
      <div class="stat-card surface-card stat-card--orange">
        <span class="stat-card__icon"><SwapIcon size="24" aria-hidden="true" /></span>
        <div class="stat-card__info">
          <span class="stat-card__value">{{ instance.bandwidth }}</span>
          <span class="stat-card__hint">带宽 Mbps</span>
        </div>
      </div>
    </section>

    <!-- 性能监控：CPU / 内存 / 磁盘 IO / 网卡流量。
         能力位已声明不支持时直接不挂载，省掉一次注定失败的目录请求。 -->
    <InstanceMetrics v-if="!isPending && caps.metrics" :instance-id="Number(route.params.id)" />

    <div class="detail-grid">
      <section class="surface-card info-card">
        <h3 class="card-title">基础信息</h3>
        <div class="kv-list">
          <div class="kv">
            <span class="kv-key">实例 ID</span><span class="kv-val mono">{{ instance.instance_id }}</span>
          </div>
          <!-- NAT 机型没有独立 IP：不能只写"—"，那会让客户以为机器没配好。
               直接把转发地址放在这一行，这是他们唯一能连上机器的方式。 -->
          <div v-if="isNatInstance" class="kv">
            <span class="kv-key">访问地址</span>
            <span class="kv-val mono">
              <template v-if="natAddresses.length">
                <span v-for="addr in natAddresses" :key="addr" class="nat-addr">
                  {{ addr }}
                  <t-link theme="primary" hover="color" size="small" @click="copyAddress(addr)">复制</t-link>
                </span>
              </template>
              <template v-else>暂无转发地址</template>
            </span>
          </div>
          <div class="kv">
            <span class="kv-key">公网 IP</span
            ><span class="kv-val mono">{{ isNatInstance ? '不适用（NAT 机型）' : instance.public_ip || '—' }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">内网 IP</span><span class="kv-val mono">{{ instance.private_ip || '—' }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">操作系统</span><span class="kv-val">{{ instance.os || '—' }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">区域</span>
            <span class="kv-val">{{ [instance.region, instance.zone].filter(Boolean).join(' / ') || '—' }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">开通操作人</span>
            <span class="kv-val">{{ instance.actor_name || '主账号' }}</span>
          </div>
        </div>
      </section>

      <section class="surface-card info-card">
        <h3 class="card-title">账期信息</h3>
        <div class="kv-list">
          <div class="kv">
            <span class="kv-key">计费方式</span>
            <span class="kv-val">{{ billingModeLabel(instance.billing_mode) }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">到期时间</span>
            <span class="kv-val">{{ formatTime(instance.expire_at) }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">剩余时间</span>
            <span class="kv-val" :class="`days--${daysLeftTheme(instance.days_left, instance.expire_state)}`">
              {{ daysLeftText(instance.days_left, instance.expire_state) }}
            </span>
          </div>
          <div class="kv">
            <span class="kv-key">创建时间</span>
            <span class="kv-val">{{ formatTime(instance.created_at) }}</span>
          </div>
          <div class="kv">
            <span class="kv-key">最近更新</span>
            <span class="kv-val">{{ formatTime(instance.updated_at) }}</span>
          </div>
          <div class="kv kv--action">
            <t-button theme="primary" variant="outline" size="small" @click="$router.push('/cloud/renewals')">
              去续费
            </t-button>
          </div>
        </div>
      </section>
    </div>

    <!-- 操作记录：只读审计，让用户能自己回答"不是我操作的？" -->
    <section class="surface-card info-card">
      <div class="card-head">
        <h3 class="card-title">操作记录</h3>
        <span class="card-head__meta">最近 {{ operations.length }} 条</span>
      </div>
      <t-loading :loading="operationsLoading" show-overlay>
        <t-table
          v-if="operations.length"
          row-key="id"
          :data="operations"
          :columns="operationColumns"
          size="small"
          :bordered="false"
          cell-empty-content="—"
        >
          <template #action="{ row }">
            <span class="op-action">{{ operationActionLabel(row.action) }}</span>
          </template>
          <template #operator_name="{ row }">
            <span>{{ operatorTypeLabel(row.operator_type) }}</span>
            <span v-if="row.operator_name" class="instance-sub"> · {{ row.operator_name }}</span>
          </template>
          <template #result="{ row }">
            <t-tag :theme="operationResultTheme(row.result)" variant="light" size="small" shape="round">
              {{ operationResultLabel(row.result) }}
            </t-tag>
            <div v-if="row.error_message" class="op-error">{{ row.error_message }}</div>
          </template>
          <template #created_at="{ row }">
            <span class="instance-sub">{{ formatTime(row.created_at) }}</span>
          </template>
        </t-table>
        <t-empty v-else description="暂无操作记录" />
      </t-loading>
    </section>

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
        <t-input v-model="destroy.reason" placeholder="销毁原因（选填）" size="large" class="destroy-reason" />
        <!-- 二次验证：策略开启后需要验证码，后端返回 20017 时展开此区域 -->
        <div v-if="destroy.needVerify" class="destroy-verify">
          <p class="destroy-tip">该操作需要二次验证，验证码将发送至账号绑定的{{ destroy.channelLabel }}。</p>
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
            <t-button
              variant="outline"
              theme="primary"
              :disabled="reinstall.countdown > 0"
              @click="sendCode('instance_reinstall', reinstall)"
            >
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
      :confirm-btn="{
        content: '重置密码',
        loading: resetPwd.submitting,
        disabled: !resetPwd.password || !resetPwd.agreed,
      }"
      width="460px"
      @confirm="submitResetPassword"
    >
      <t-alert theme="warning" class="destroy-alert">
        重置密码由平台执行，<b>多数平台会强制重启实例</b>，重启期间业务中断约 1–3
        分钟。系统盘与数据盘数据不受影响，仅更换登录密码。
      </t-alert>
      <t-input
        v-model="resetPwd.password"
        type="password"
        size="large"
        class="destroy-reason"
        placeholder="新密码（建议 12 位以上，含大小写与符号）"
      />
      <t-checkbox v-model="resetPwd.agreed" class="destroy-agree">
        已知悉：重置密码可能导致实例强制重启，业务会短暂中断
      </t-checkbox>
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
      <t-input
        v-model="rescue.temp_password"
        type="password"
        size="large"
        placeholder="临时密码"
        class="destroy-reason"
      />
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
      <t-table
        row-key="id"
        :data="snap.rows"
        :columns="snapColumns"
        :loading="snap.loading"
        size="small"
        cell-empty-content="暂无快照"
      >
        <template #snap_action="{ row }">
          <t-space size="small">
            <t-link theme="warning" hover="color" @click="openRestoreSnapshot(row)">恢复</t-link>
            <t-link theme="danger" hover="color" @click="submitDeleteSnapshot(row)">删除</t-link>
          </t-space>
        </template>
      </t-table>
      <div class="destroy-verify">
        <p class="destroy-tip">
          用快照恢复会覆盖当前系统盘，请输入实例 ID <code>{{ instance.instance_id }}</code> 确认：
        </p>
        <t-input v-model="snap.restoreMark" placeholder="实例 ID" />
        <div v-if="snap.needVerify" class="destroy-code-row">
          <t-input v-model="snap.code" placeholder="请输入验证码" maxlength="6" />
          <t-button
            variant="outline"
            theme="primary"
            :disabled="snap.countdown > 0"
            @click="sendCode('instance_snapshot_restore', snap)"
          >
            {{ snap.countdown > 0 ? `${snap.countdown}s` : '获取验证码' }}
          </t-button>
        </div>
        <div v-if="snap.restoreId" class="destroy-code-row">
          <t-button
            theme="danger"
            variant="outline"
            :disabled="snap.restoreMark.trim() !== instance.instance_id"
            @click="submitRestoreSnapshot"
          >
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
      <t-alert theme="warning" class="destroy-alert"> 平台新签发的登录凭据只显示这一次，本系统不保存。 </t-alert>
      <div class="destroy-body">
        <p class="destroy-tip">
          用户名：<code>{{ credential.username || '—' }}</code>
        </p>
        <p class="destroy-tip">
          新密码：<code>{{ credential.password || '（平台未返回，请改用「重置密码」或到平台面板查看）' }}</code>
        </p>
      </div>
    </t-dialog>
  </div>
  <div v-else-if="loadError" class="loading-wrap">
    <t-empty :description="loadError">
      <template #action>
        <t-button theme="primary" :loading="loading" @click="load(false)">重试</t-button>
      </template>
    </t-empty>
  </div>
  <div v-else class="loading-wrap"><t-loading size="large" text="加载中..." /></div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { TTable } from '@/plugins/tdesign-heavy'
import type { PrimaryTableCol } from 'tdesign-vue-next'
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  CloseCircleIcon,
  CloudIcon,
  CpuIcon,
  DashboardIcon,
  DeleteIcon,
  ErrorCircleIcon,
  HardDiskStorageIcon,
  InfoCircleIcon,
  PauseCircleIcon,
  PlayCircleIcon,
  PoweroffIcon,
  RefreshIcon,
  SwapIcon,
  TerminalIcon,
  TimeIcon,
} from 'tdesign-icons-vue-next'

import InstanceMetrics from '@/components/InstanceMetrics.vue'
import {
  destroyInstance,
  getInstance,
  listInstanceOperations,
  powerInstance,
  retryInstanceProvision,
  vncInstance,
  type InstanceInfo,
  type InstanceOperation,
  type PowerAction,
} from '@/api/cloud'
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
import {
  billingModeLabel,
  combineInstanceState,
  daysLeftText,
  daysLeftTheme,
  formatMemory,
  formatTime,
  operationActionLabel,
  operationResultLabel,
  operationResultTheme,
  operatorTypeLabel,
  stateIcon,
} from './constants'

defineOptions({ name: 'InstanceDetail' })

const route = useRoute()
const router = useRouter()
const instance = ref<InstanceInfo | null>(null)
const loading = ref(false)
/** 加载失败原因（空串=正常）：模板据此显示错误态 + 重试，而不是一直空着。 */
const loadError = ref('')

/**
 * 状态结论 → 图标组件（与列表页同一套映射，两处视觉一致）。
 * 图标让"运行中/已关机/已过期"能一眼扫出来，不必逐字读 tag。
 */
const STATE_ICONS: Record<string, Component> = {
  'play-circle': PlayCircleIcon,
  'pause-circle': PauseCircleIcon,
  time: TimeIcon,
  'error-circle': ErrorCircleIcon,
  delete: DeleteIcon,
  'close-circle': CloseCircleIcon,
  'info-circle': InfoCircleIcon,
}

function stateIconComponent(label: string): Component {
  return STATE_ICONS[stateIcon(label)] || InfoCircleIcon
}

const instanceId = computed(() => Number(route.params.id))

/** 状态结论与服务端列表页同源（combineInstanceState），避免同一台机器两处显示不一致。 */
const state = computed(() =>
  combineInstanceState({
    status: instance.value?.status || '',
    lifecycle_stage: instance.value?.lifecycle_stage,
    expire_state: instance.value?.expire_state,
    pending: instance.value?.pending,
  }),
)

/** 能力位：缺省（老数据）时按可操作处理，避免整页按钮被禁用。 */
const caps = computed(() => ({
  power: instance.value?.capabilities ? instance.value.capabilities.power : true,
  console: instance.value?.capabilities ? instance.value.capabilities.console : true,
  reinstall: instance.value?.capabilities ? instance.value.capabilities.reinstall : true,
  reset_password: instance.value?.capabilities ? instance.value.capabilities.reset_password : true,
  rescue: instance.value?.capabilities ? instance.value.capabilities.rescue : true,
  snapshot: instance.value?.capabilities ? instance.value.capabilities.snapshot : true,
  destroy: instance.value?.capabilities ? instance.value.capabilities.destroy : true,
  metrics: instance.value?.capabilities ? instance.value.capabilities.metrics : true,
}))

/** 到期相关提示（过期/宽限/临期），与状态徽标互补：徽标说结论，这里说该做什么。 */
const stateAlert = computed<{ theme: string; message: string; action?: string } | null>(() => {
  const it = instance.value
  if (!it || it.pending || it.lifecycle_stage === 'destroyed') return null
  if (it.lifecycle_stage === 'suspended') {
    return { theme: 'error', message: '实例已因到期暂停服务，续费后将自动恢复运行。', action: '去续费' }
  }
  if (it.expire_state === 'expired') {
    return {
      theme: 'warning',
      message: `实例已过期 ${Math.abs(it.days_left)} 天，处于宽限期仍可服务；请尽快续费，逾期将暂停并最终销毁。`,
      action: '去续费',
    }
  }
  if (it.expire_state === 'expiring') {
    return { theme: 'info', message: `实例将在 ${it.days_left} 天后到期，建议提前续费避免服务中断。`, action: '去续费' }
  }
  return null
})

// 待开通（尚未在上游开通）：只开放「重新开通」。
const isPending = computed(() => !!instance.value && !!instance.value.pending)
const retrying = ref(false)

// ===== NAT 机型访问地址（doc122）=====
// NAT 机型没有独立公网 IP，客户必须拿到节点转发地址才能连上机器；
// 只显示"未分配公网 IP"会让客户以为机器没配好。
const isNatInstance = computed(() => instance.value?.nat_access?.is_nat === true)

/** 全部可访问地址：端口转发优先，其后是建站域名（两者都能连上机器）。 */
const natAddresses = computed(() => {
  const nat = instance.value?.nat_access
  if (!nat) return []
  return [...(nat.forwards || []), ...(nat.domains || [])]
})

async function copyAddress(addr: string) {
  try {
    await navigator.clipboard.writeText(addr)
    MessagePlugin.success('已复制')
  } catch {
    MessagePlugin.warning('复制失败，请手动选择文本复制')
  }
}

const provisionAlertTitle = computed(() => {
  const st = instance.value?.provision_status
  if (st === 'manual') return '开通失败，需人工处理（可重新开通）'
  if (st === 'failed') return '开通失败，系统自动重试中'
  return '实例待开通'
})
const provisionAlertMessage = computed(() => {
  const reason = instance.value?.provision_error
  if (reason) {
    return `原因：${reason}。修复后可点「重新开通」重试，不会产生重复订单或重复扣费。`
  }
  return '订单未支付或正在开通中。未支付订单超出支付有效期会自动关单并删除本实例；已支付但开通失败的可在详情页「重新开通」。'
})

// 重新提交开通（复用既有开通单，不产生新订单、不重复扣费）。
async function onRetry() {
  if (!instance.value) return
  retrying.value = true
  try {
    await retryInstanceProvision(instance.value.id)
    MessagePlugin.success('已重新提交开通，请稍后刷新查看')
    load(false)
  } catch {
    /* 错误已由请求拦截器提示 */
  } finally {
    retrying.value = false
  }
}

async function load(live = false) {
  loading.value = true
  loadError.value = ''
  try {
    const res = await getInstance(instanceId.value, live)
    instance.value = res.data
  } catch (error) {
    // 与列表页同理：拦截器只弹提示并 reject，不 catch 会让详情页停在空白。
    // 已有数据时保留展示（刷新失败不该把已看到的内容擦掉），无数据才走错误态。
    loadError.value = (error as Error)?.message || '加载失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

async function onPower(action: PowerAction, needConfirm = false) {
  const label = { on: '开机', off: '关机', reboot: '重启', hard_off: '强制关机', hard_reboot: '强制重启' }[action]
  if (needConfirm) {
    // 硬电源 = 断电级动作，先确认再发指令；取消即什么都不做。
    const dialog = DialogPlugin.confirm({
      header: label,
      body: `${label}等同于断电，可能造成数据损坏，确认继续？`,
      confirmBtn: { content: `确认${label}`, theme: 'danger' },
      cancelBtn: { content: '取消' },
      onConfirm: () => {
        dialog.destroy()
        void doPower(action, label)
      },
      onClose: () => dialog.destroy(),
    })
    return
  }
  await doPower(action, label)
}

async function doPower(action: PowerAction, label: string) {
  try {
    await powerInstance(instanceId.value, action)
    MessagePlugin.success(`${label}指令已发送`)
    // 先本地乐观更新，让按钮/状态立刻有反馈（上游状态通常几秒后才变）。
    // 服务端已把预期状态落库，这里同步界面，用户不必盯着"没反应"的按钮。
    applyOptimisticPower(action)
    // 再拉一次真实状态与操作记录：服务端回源对齐后的值会覆盖乐观值。
    await refreshAll(true)
  } catch {
    /* 错误已由拦截器提示 */
  }
}

/**
 * 电源动作后的本地乐观状态。
 *
 * 只改"立刻能看到"的字段（status/power_status），不做整对象假设：
 * 上游延迟期间界面先反映预期，随后 refreshAll 用服务端真值覆盖。
 * 重启类不改状态——重启前后都是运行中，本地没有可写的中间态。
 */
function applyOptimisticPower(action: PowerAction) {
  if (!instance.value) return
  const target: Record<string, { status: string; power: string }> = {
    on: { status: 'running', power: 'on' },
    off: { status: 'stopped', power: 'off' },
    hard_off: { status: 'stopped', power: 'off' },
  }
  const next = target[action]
  if (!next) return
  instance.value.status = next.status
  instance.value.power_status = next.power
}

/**
 * 打开远程控制台：取到地址后直接开新浏览器标签页。
 *
 * 为什么不再用弹窗内嵌 iframe：控制台页面自带全屏/键鼠捕获，
 * 塞进 iframe 常被对方的 X-Frame-Options 拒掉（弹窗里就只能显示"无法内嵌"），
 * 且小窗口里操作体验很差。新标签页是这类控制台的标准用法。
 *
 * 注意 window.open 必须在点击的同步调用栈里发起，否则会被浏览器拦截；
 * 因此先同步开一个空白页，拿到地址后再改它的 location。
 */
async function onVNC() {
  const tab = window.open('', '_blank', 'noopener,noreferrer')
  if (!tab) {
    MessagePlugin.warning('浏览器拦截了新窗口，请允许本站弹出窗口后重试')
    return
  }
  try {
    const res = await vncInstance(instanceId.value)
    const url = res.data?.url || ''
    if (!url) {
      tab.close()
      MessagePlugin.warning('控制台地址暂不可用，请稍后重试')
      return
    }
    tab.location.href = url
  } catch (error) {
    tab.close()
    throw error
  }
}

// ===== 操作记录 =====

const operations = ref<InstanceOperation[]>([])
const operationsLoading = ref(false)

const operationColumns: PrimaryTableCol<InstanceOperation>[] = [
  { colKey: 'action', title: '操作', width: 130 },
  { colKey: 'operator_name', title: '操作人', width: 150 },
  { colKey: 'result', title: '结果', minWidth: 180 },
  { colKey: 'created_at', title: '时间', width: 150 },
]

async function loadOperations() {
  if (isPending.value) {
    operations.value = []
    return
  }
  operationsLoading.value = true
  try {
    const res = await listInstanceOperations(instanceId.value, 20)
    operations.value = res.data || []
  } catch {
    operations.value = []
  } finally {
    operationsLoading.value = false
  }
}

/**
 * 实例数据与操作记录一起刷新（电源动作后两者都会变）。
 *
 * live=true 会回源上游取实时状态，单次可能数秒：**不能让它阻塞首屏**。
 * 进页面先用本地数据把界面画出来，再在后台回源校正（refreshInBackground），
 * 用户感知到的就是"秒开 + 状态随后自动对齐"。
 */
async function refreshAll(live = true) {
  await load(live)
  await loadOperations()
}

/**
 * 后台静默刷新：不切 loading 态、不改动已渲染内容，仅在有变化时覆盖。
 * 首屏调用它做"进页面后自动对齐真实状态"，避免用户手动点刷新。
 */
async function refreshInBackground() {
  try {
    const res = await getInstance(instanceId.value, true)
    if (res.data) {
      instance.value = res.data
      void loadOperations()
    }
  } catch {
    // 静默失败：首屏本地数据已经展示，回源失败不该弹错误打扰用户。
  }
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
  const id = instanceId.value
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

onMounted(() => {
  // 首屏读本地（快），随后后台回源对齐（慢但不阻塞渲染）。
  void refreshAll(false)
  void refreshInBackground()
})

// ===== 维护类自助操作 =====
//
// 全部走「更多维护 ▾」下拉：电源栏已经有 5 个按钮，再加 6 个会挤成一团。
// 每个动作都在服务端复用运维台的能力分派 + 操作流水 + 能力位校验，
// 前端只负责收集参数、做二次确认与展示一次性凭据。

const maintenanceOptions = computed(() => [
  { content: '重装系统', value: 'reinstall', disabled: !caps.value.reinstall },
  { content: '重置密码', value: 'resetpwd', disabled: !caps.value.reset_password },
  { content: '进入救援系统', value: 'rescue', disabled: !caps.value.rescue },
  { content: '退出救援系统', value: 'exitrescue', disabled: !caps.value.rescue },
  { content: '快照与备份', value: 'snapshot', disabled: !caps.value.snapshot },
])

function onMaintenancePick(data: { value: string }) {
  switch (data.value) {
    case 'reinstall':
      Object.assign(reinstall, {
        os: '',
        port: 0,
        format_data_disk: false,
        confirmMark: '',
        code: '',
        needVerify: false,
        visible: true,
      })
      void prepareVerify('instance_reinstall', reinstall)
      break
    case 'resetpwd':
      resetPwd.password = ''
      // 同意勾选每次重开复位：重启风险不能靠上次点过而长期豁免。
      resetPwd.agreed = false
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
  visible: false,
  os: '',
  port: 0,
  format_data_disk: false,
  confirmMark: '',
  code: '',
  countdown: 0,
  sendingCode: false,
  needVerify: false,
  submitting: false,
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
      instanceId.value,
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
    await refreshAll()
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

const resetPwd = reactive({ visible: false, password: '', agreed: false, submitting: false })

async function submitResetPassword() {
  if (!resetPwd.password) {
    MessagePlugin.warning('请输入新密码')
    return
  }
  resetPwd.submitting = true
  try {
    await resetInstancePassword(instanceId.value, resetPwd.password)
    MessagePlugin.success('密码已重置')
    resetPwd.visible = false
    await loadOperations()
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
    await rescueInstance(instanceId.value, rescue.system, rescue.temp_password)
    MessagePlugin.success('已发起进入救援系统')
    rescue.visible = false
    await refreshAll()
  } catch (error) {
    const e = error as { response?: { data?: { message?: string } }; message?: string }
    MessagePlugin.error(e?.response?.data?.message || e?.message || '进入救援系统失败')
  } finally {
    rescue.submitting = false
  }
}

async function submitExitRescue() {
  try {
    await exitRescueInstance(instanceId.value)
    MessagePlugin.success('已发起退出救援系统')
    await refreshAll()
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
    const res = await listInstanceSnapshots(instanceId.value)
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
    await createInstanceSnapshot(instanceId.value, { type: snap.type, name: snap.name || undefined })
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
    await restoreInstanceSnapshot(instanceId.value, snap.restoreId, snap.restoreMark.trim(), ticket || undefined)
    MessagePlugin.success('恢复指令已提交')
    snap.restoreId = ''
    snap.restoreMark = ''
    await refreshAll()
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
    await deleteInstanceSnapshot(instanceId.value, row.id)
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
  gap: var(--space-lg);
}

.loading-wrap {
  display: flex;
  justify-content: center;
  padding: 60px;
}

/* ---------- 页头 ---------- */
.back-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  background: transparent;
  color: var(--color-muted-foreground);
  cursor: pointer;
  transition: all var(--hs-duration-fast) var(--hs-ease-out);
  flex-shrink: 0;
}

.back-btn:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
}

.page-header__title {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.dot {
  margin: 0 4px;
  color: var(--color-border);
}

/* ---------- 提示与操作条 ---------- */
.detail-alert {
  border-radius: var(--hs-radius-lg);
}

.action-bar {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
  padding: var(--space-md) var(--space-xl);
}

.action-bar__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* ---------- 配置概览 ---------- */
.spec-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--space-md);
}

/* ---------- 信息卡 ---------- */
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}

.info-card {
  padding: var(--space-lg) var(--space-xl);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

.card-head__meta {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.kv-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.kv {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-md);
}

.kv--action {
  justify-content: flex-end;
}

.kv-key {
  font-size: 13px;
  color: var(--color-muted-foreground);
  flex-shrink: 0;
}

.kv-val {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-foreground);
  text-align: right;
  word-break: break-all;
}

.mono {
  font-family: var(--hs-font-mono);
  font-size: 12.5px;
}

/* NAT 访问地址：可能有多条（SSH/网站/数据库各一条），逐条一行并各自带复制。 */
.nat-addr {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: 8px;
}

.nat-addr:first-child {
  margin-left: 0;
}

.days--danger {
  color: var(--color-error, #d54941);
  font-weight: 600;
}
.days--warning {
  color: var(--color-warning, #e37318);
  font-weight: 600;
}

/* ---------- 操作记录 ---------- */
.op-action {
  font-size: 13px;
  color: var(--color-foreground);
}

.op-error {
  margin-top: 2px;
  font-size: 11px;
  color: var(--color-error, #d54941);
  word-break: break-all;
}

.instance-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

/* ---------- 弹窗 ---------- */
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
.destroy-agree {
  display: block;
  margin-top: 10px;
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

@media (max-width: 1024px) {
  .spec-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
  .spec-stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .action-bar {
    padding: var(--space-md);
  }
  /* 窄屏：按钮等宽铺满，避免"强制关机"这类长标签被挤成两行 */
  .action-bar :deep(.t-button) {
    flex: 1 1 calc(50% - var(--space-sm));
    min-width: 0;
  }
}
</style>
