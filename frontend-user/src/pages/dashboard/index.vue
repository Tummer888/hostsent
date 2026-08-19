<template>
  <div class="dashboard-page">
    <!-- 欢迎区 -->
    <section class="welcome-card">
      <div class="welcome-content">
        <div class="welcome-greeting">
          <span class="greeting-text">{{ greeting }}，</span>
          <span class="user-name">{{ displayName }}</span>
        </div>
        <p class="welcome-desc">
          今天是 {{ today }}，欢迎回到宿派云控控制台
        </p>
        <div class="welcome-actions">
          <t-button theme="primary" size="large">
            <template #icon><AddIcon /></template>
            创建云主机
          </t-button>
          <t-button variant="outline" size="large">
            <template #icon><ArrowUpIcon /></template>
            升级套餐
          </t-button>
        </div>
      </div>
      <div class="welcome-stats">
        <div class="stat-item">
          <span class="stat-value">{{ stats.running }}</span>
          <span class="stat-label">运行中</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.total }}</span>
          <span class="stat-label">总实例</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.balance }}</span>
          <span class="stat-label">账户余额</span>
        </div>
      </div>
    </section>

    <!-- 快速操作 -->
    <section class="quick-actions">
      <h3 class="section-title">快速操作</h3>
      <div class="action-grid">
        <div class="action-card" v-for="action in quickActions" :key="action.title">
          <div class="action-icon" :style="{ background: action.color }">
            <component :is="action.icon" />
          </div>
          <div class="action-info">
            <h4>{{ action.title }}</h4>
            <p>{{ action.desc }}</p>
          </div>
          <ChevronRightIcon class="action-arrow" />
        </div>
      </div>
    </section>

    <!-- 最近活动 -->
    <section class="recent-section">
      <div class="section-header">
        <h3 class="section-title">最近活动</h3>
        <a href="#" class="view-all">查看全部</a>
      </div>
      <t-table
        :data="recentActivities"
        :columns="activityColumns"
        size="medium"
        :pagination="false"
        :bordered="false"
        row-key="id"
      >
        <template #type="{ row }">
          <span class="type-tag" :class="`type-${row.type}`">{{ row.typeLabel }}</span>
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, h } from 'vue'
import {
  AddIcon,
  ArrowUpIcon,
  ChevronRightIcon,
  CloudIcon,
  WalletIcon,
  OrderIcon,
  TicketIcon,
  LayersIcon,
  ServerIcon,
} from 'tdesign-icons-vue-next'

import { useUserStore } from '@/store'

defineOptions({ name: 'UserDashboard' })

const userStore = useUserStore()

const displayName = computed(() => userStore.displayName || '用户')

const today = new Date().toLocaleDateString('zh-CN', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  weekday: 'long',
})

const hour = new Date().getHours()
const greeting = computed(() => {
  if (hour < 6) return '夜深了'
  if (hour < 12) return '早上好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  return '晚上好'
})

const stats = {
  running: 5,
  total: 8,
  balance: '¥ 2,580.00',
}

const quickActions = [
  { title: '创建云主机', desc: '快速创建新的云主机实例', icon: CloudIcon, color: '#2563EB' },
  { title: '充值账户', desc: '为账户充值，继续使用服务', icon: WalletIcon, color: '#10B981' },
  { title: '我的订单', desc: '查看订单记录和状态', icon: OrderIcon, color: '#F59E0B' },
  { title: '提交工单', desc: '联系技术支持团队', icon: TicketIcon, color: '#8B5CF6' },
  { title: '镜像管理', desc: '管理系统和自定义镜像', icon: LayersIcon, color: '#EC4899' },
  { title: '实例快照', desc: '为云主机创建快照备份', icon: ServerIcon, color: '#06B6D4' },
]

interface ActivityRow {
  id: number
  type: string
  typeLabel: string
  title: string
  resource: string
  time: string
}

const recentActivities: ActivityRow[] = [
  { id: 1, type: 'create', typeLabel: '创建', title: '创建云主机', resource: 'host-user-001', time: '2026-08-19 14:30' },
  { id: 2, type: 'start', typeLabel: '启动', title: '启动实例', resource: 'host-user-002', time: '2026-08-19 13:15' },
  { id: 3, type: 'payment', typeLabel: '支付', title: '订单支付成功', resource: 'ORD20260819001', time: '2026-08-19 10:45' },
  { id: 4, type: 'stop', typeLabel: '停止', title: '停止实例', resource: 'host-user-003', time: '2026-08-18 18:20' },
  { id: 5, type: 'snapshot', typeLabel: '快照', title: '创建快照', resource: 'snap-20260818', time: '2026-08-18 15:00' },
]

const activityColumns = [
  { colKey: 'type', title: '类型', width: 80 },
  { colKey: 'title', title: '操作', ellipsis: true },
  { colKey: 'resource', title: '资源', width: 150 },
  { colKey: 'time', title: '时间', width: 160 },
]
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* 欢迎卡片 */
.welcome-card {
  background: linear-gradient(135deg, #2563EB 0%, #3B82F6 100%);
  border-radius: 16px;
  padding: 32px;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 32px;
}

.welcome-content {
  flex: 1;
}

.welcome-greeting {
  font-size: 24px;
  font-weight: 600;
  margin-bottom: 8px;
}

.greeting-text {
  opacity: 0.9;
}

.user-name {
  font-weight: 700;
}

.welcome-desc {
  font-size: 14px;
  opacity: 0.8;
  margin-bottom: 20px;
}

.welcome-actions {
  display: flex;
  gap: 12px;
}

.welcome-actions .t-button {
  height: 40px;
  font-weight: 500;
}

.welcome-actions .t-button:first-child {
  background: #fff;
  color: #2563EB;
}

.welcome-stats {
  display: flex;
  align-items: center;
  gap: 24px;
  padding-left: 32px;
  border-left: 1px solid rgba(255, 255, 255, 0.2);
}

.stat-item {
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 28px;
  font-weight: 700;
  margin-bottom: 4px;
}

.stat-label {
  font-size: 13px;
  opacity: 0.8;
}

.stat-divider {
  width: 1px;
  height: 40px;
  background: rgba(255, 255, 255, 0.2);
}

/* 区域标题 */
.section-title {
  font-size: 18px;
  font-weight: 600;
  color: #1E293B;
  margin-bottom: 16px;
}

/* 快速操作 */
.quick-actions {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.action-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.action-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px;
  border: 1px solid #E2E8F0;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
}

.action-card:hover {
  border-color: #2563EB;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.1);
  transform: translateY(-2px);
}

.action-icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.action-icon :deep(svg) {
  font-size: 22px;
  color: #fff;
}

.action-info {
  flex: 1;
}

.action-info h4 {
  font-size: 15px;
  font-weight: 600;
  color: #1E293B;
  margin-bottom: 4px;
}

.action-info p {
  font-size: 13px;
  color: #64748B;
}

.action-arrow {
  color: #94A3B8;
}

/* 最近活动 */
.recent-section {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-header .section-title {
  margin-bottom: 0;
}

.view-all {
  color: #2563EB;
  font-size: 14px;
  text-decoration: none;
}

.view-all:hover {
  text-decoration: underline;
}

.type-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.type-create {
  background: #DBEAFE;
  color: #1D4ED8;
}

.type-start {
  background: #D1FAE5;
  color: #059669;
}

.type-payment {
  background: #FEF3C7;
  color: #D97706;
}

.type-stop {
  background: #FEE2E2;
  color: #DC2626;
}

.type-snapshot {
  background: #EDE9FE;
  color: #7C3AED;
}

/* 响应式 */
@media (max-width: 768px) {
  .dashboard-page {
    gap: 16px;
  }

  .welcome-card {
    flex-direction: column;
    align-items: flex-start;
    padding: 24px;
    gap: 24px;
  }
  
  .welcome-greeting {
    font-size: 20px;
  }

  .welcome-actions {
    flex-wrap: wrap;
  }

  .welcome-actions .t-button {
    flex: 1;
    min-width: 120px;
  }

  .welcome-stats {
    padding-left: 0;
    border-left: none;
    padding-top: 20px;
    border-top: 1px solid rgba(255, 255, 255, 0.2);
    width: 100%;
    justify-content: space-around;
  }

  .stat-value {
    font-size: 24px;
  }
  
  .action-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
  }

  .quick-actions,
  .recent-section {
    padding: 16px;
  }

  .action-card {
    padding: 12px;
    gap: 12px;
  }

  .action-info h4 {
    font-size: 14px;
  }

  .action-info p {
    display: none;
  }

  .section-title {
    font-size: 16px;
    margin-bottom: 12px;
  }
}

@media (max-width: 480px) {
  .dashboard-page {
    gap: 12px;
  }

  .welcome-card {
    padding: 20px;
    border-radius: 12px;
  }

  .welcome-greeting {
    font-size: 18px;
  }

  .welcome-desc {
    font-size: 13px;
    margin-bottom: 16px;
  }

  .welcome-actions {
    gap: 8px;
  }

  .welcome-actions .t-button {
    flex: 1;
    min-width: 100px;
    height: 36px;
    font-size: 13px;
  }

  .welcome-stats {
    gap: 12px;
  }

  .stat-value {
    font-size: 20px;
  }

  .stat-label {
    font-size: 12px;
  }

  .stat-divider {
    height: 30px;
  }

  .action-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .action-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
    padding: 12px;
  }

  .action-icon {
    width: 36px;
    height: 36px;
  }

  .action-icon :deep(svg) {
    font-size: 18px;
  }

  .action-info h4 {
    font-size: 13px;
  }

  .action-arrow {
    display: none;
  }

  .quick-actions,
  .recent-section {
    padding: 16px;
    border-radius: 10px;
  }

  .section-header {
    margin-bottom: 12px;
  }

  .view-all {
    font-size: 13px;
  }
}
</style>
