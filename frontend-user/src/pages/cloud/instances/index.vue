<template>
  <div class="instance-page">
    <div class="instance-header">
      <h2 class="instance-title">我的云主机</h2>
      <div class="header-actions">
        <t-button theme="default" variant="outline" shape="round" :loading="loading" @click="load(true)">
          <template #icon><t-icon name="refresh" /></template>
          刷新状态
        </t-button>
      </div>
    </div>

    <t-table :data="instances" :columns="columns" row-key="id" :loading="loading" :bordered="true" hover :pagination="false">
      <template #name="{ row }">
        <div class="cell-strong">{{ row.name }}</div>
        <div class="cell-sub">{{ row.instance_id }}</div>
      </template>
      <template #spec="{ row }">
        <div class="cell-sub">{{ row.cpu }} 核 / {{ row.memory }}MB / {{ row.disk }}GB</div>
        <div class="cell-sub">{{ row.os || '—' }}</div>
      </template>
      <template #ip="{ row }">
        <span class="cell-strong">{{ row.public_ip || '—' }}</span>
      </template>
      <template #status="{ row }">
        <t-tag :theme="statusTheme(row)" variant="light" size="small" shape="round">{{ statusText(row) }}</t-tag>
      </template>
      <template #expire_at="{ row }">{{ row.expire_at ? row.expire_at.replace('T', ' ').slice(0, 16) : '—' }}</template>
      <template #op="{ row }">
        <t-button theme="primary" variant="text" size="small" @click="goDetail(row)">详情</t-button>
        <t-button theme="success" variant="text" size="small" :disabled="row.power_status === 'on'" @click="onPower(row, 'on')">开机</t-button>
        <t-button theme="danger" variant="text" size="small" :disabled="row.power_status === 'off'" @click="onPower(row, 'off')">关机</t-button>
        <t-button theme="warning" variant="text" size="small" :disabled="row.power_status !== 'on'" @click="onPower(row, 'reboot')">重启</t-button>
        <t-button theme="primary" variant="text" size="small" :disabled="row.status !== 'running'" @click="onVNC(row)">VNC</t-button>
      </template>
    </t-table>

    <t-empty v-if="!loading && !instances.length" description="暂无云主机，去购买一台吧">
      <template #action>
        <t-button theme="primary" @click="$router.push('/shop')">去购买</t-button>
      </template>
    </t-empty>

    <t-dialog v-model:visible="vnc.visible" :header="`远程控制台 · ${vnc.name}`" width="80%" :footer="false" destroy-on-close>
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
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { listInstances, powerInstance, vncInstance, type InstanceInfo } from '@/api/cloud'

defineOptions({ name: 'InstanceList' })

const router = useRouter()
const instances = ref<InstanceInfo[]>([])
const loading = ref(false)
const vnc = reactive<{ visible: boolean; name: string; url: string }>({ visible: false, name: '', url: '' })

const columns: PrimaryTableCol<InstanceInfo>[] = [
  { colKey: 'name', title: '主机', minWidth: 200 },
  { colKey: 'spec', title: '配置', minWidth: 150 },
  { colKey: 'ip', title: '公网 IP', minWidth: 130 },
  { colKey: 'status', title: '状态', width: 110 },
  { colKey: 'expire_at', title: '到期时间', minWidth: 150 },
  { colKey: 'op', title: '操作', width: 260, fixed: 'right' },
]

function statusText(row: InstanceInfo): string {
  if (row.status === 'running') return '运行中'
  if (row.status === 'stopped') return '已关机'
  if (row.status === 'creating') return '创建中'
  return row.status || '未知'
}
function statusTheme(row: InstanceInfo): 'success' | 'warning' | 'default' | 'danger' {
  if (row.status === 'running') return 'success'
  if (row.status === 'creating') return 'warning'
  if (row.status === 'stopped') return 'default'
  return 'default'
}

async function load(live = false) {
  loading.value = true
  try {
    const res = await listInstances({ live })
    instances.value = res.data.items || []
  } finally {
    loading.value = false
  }
}

async function onPower(row: InstanceInfo, action: 'on' | 'off' | 'reboot') {
  const label = { on: '开机', off: '关机', reboot: '重启' }[action]
  try {
    await powerInstance(row.id, action)
    MessagePlugin.success(`${label}指令已发送`)
    load(true)
  } catch {
    /* 错误已由请求拦截器提示 */
  }
}

async function onVNC(row: InstanceInfo) {
  try {
    const res = await vncInstance(row.id)
    vnc.name = row.name
    vnc.url = res.data?.url || ''
    vnc.visible = true
  } catch {
    /* 错误已由拦截器提示 */
  }
}

function openVnc() {
  window.open(vnc.url, '_blank')
}
function isHttp(url: string): boolean {
  return /^https?:/i.test(url)
}

function goDetail(row: InstanceInfo) {
  router.push(`/cloud/instances/${row.id}`)
}

onMounted(() => load(true))
</script>

<style scoped>
.instance-page {
  padding: 4px 0;
}
.instance-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.instance-title {
  font-size: 20px;
  font-weight: 600;
}
.cell-strong {
  font-weight: 600;
}
.cell-sub {
  color: var(--td-text-color-secondary);
  font-size: 12px;
  margin-top: 2px;
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
</style>
