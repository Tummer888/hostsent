<template>
  <div class="page-body instances-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <ServerIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <div class="page-header__title-row">
            <h2 class="page-header__title">{{ instanceData?.name || '实例详情' }}</h2>
            <t-tag
              v-if="instanceData"
              :theme="instanceStatusTheme(instanceData.status)"
              variant="light"
              size="small"
              shape="round"
            >
              {{ instanceStatusLabel(instanceData.status) }}
            </t-tag>
          </div>
          <p class="page-header__desc">
            实例标识 {{ instanceData?.instance_id || '—' }} · 公网 IP {{ instanceData?.public_ip || '—' }}
          </p>
        </div>
      </div>
      <div class="detail-actions">
        <t-button variant="outline" @click="goBack">返回</t-button>
        <t-button variant="outline" :loading="loading" @click="reloadAll">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          :disabled="powerDisabled('on')"
          :title="powerTitle('on')"
          @click="handlePower('on')"
        >
          开机
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          :disabled="powerDisabled('off')"
          :title="powerTitle('off')"
          @click="handlePower('off')"
        >
          关机
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          :disabled="powerDisabled('reboot')"
          :title="powerTitle('reboot')"
          @click="handlePower('reboot')"
        >
          重启
        </t-button>
        <!-- 硬电源（断电级）：独立指令下发，不是"软关再开"。危险动作故用 danger 且需二次确认 -->
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          theme="danger"
          :disabled="powerDisabled('hard_off')"
          :title="powerTitle('hard_off')"
          @click="handlePower('hard_off')"
        >
          强制关机
        </t-button>
        <t-button
          v-permission="'instance:action'"
          variant="outline"
          theme="danger"
          :disabled="powerDisabled('hard_reboot')"
          :title="powerTitle('hard_reboot')"
          @click="handlePower('hard_reboot')"
        >
          强制重启
        </t-button>
        <t-button
          v-permission="'instance:console'"
          variant="outline"
          :disabled="consoleDisabled"
          :title="consoleDisabled ? consoleTitle : ''"
          @click="handleVnc"
        >
          控制台
        </t-button>
        <!-- 暂停/恢复（T5.5）：按当前生命周期阶段二选一展示 -->
        <t-button
          v-if="instanceData?.lifecycle_stage === 'suspended'"
          v-permission="'instance:action'"
          variant="outline"
          :loading="actioning"
          @click="handleUnsuspend"
        >
          恢复服务
        </t-button>
        <t-button
          v-else
          v-permission="'instance:action'"
          variant="outline"
          :disabled="!instanceData?.capabilities?.suspend"
          :title="capabilityTitle(instanceData?.capabilities?.suspend)"
          :loading="actioning"
          @click="handleSuspend"
        >
          暂停服务
        </t-button>
        <t-button v-permission="'instance:action'" variant="outline" :loading="syncing" @click="handleSync">
          同步刷新
        </t-button>
        <t-button v-permission="'instance:action'" variant="outline" @click="openRemarkDialog">备注</t-button>
        <t-button
          v-permission="'instance:resize'"
          variant="outline"
          :disabled="!instanceData?.capabilities?.resize"
          :title="capabilityTitle(instanceData?.capabilities?.resize)"
          @click="openResizeDialog"
        >
          变配
        </t-button>
        <!-- 维护类（平台接口实测存在）：重装 / 重置密码 / 救援 / 快照 / 硬件。
             用下拉收纳，避免操作栏被十几个按钮撑爆；每一项仍按自己的权限码与能力位置灰。 -->
        <t-dropdown
          :options="maintenanceOptions"
          trigger="click"
          @click="onMaintenancePick"
        >
          <t-button variant="outline" :disabled="maintenanceDisabled">维护 ▾</t-button>
        </t-dropdown>
        <t-button v-permission="'lifecycle:renew'" variant="outline" @click="openRenewDialog">续费</t-button>
        <t-button
          v-permission="'instance:destroy'"
          theme="danger"
          :disabled="!instanceData?.capabilities?.destroy"
          :title="capabilityTitle(instanceData?.capabilities?.destroy)"
          @click="openDestroyDialog"
        >
          销毁
        </t-button>
      </div>
    </header>

    <t-alert
      v-if="instanceData?.capability_error"
      theme="warning"
      :message="`部分操作不可用：${instanceData.capability_error}`"
    />

    <section class="table-card surface-card">
      <t-tabs v-model="activeTab" :default-value="activeTab" theme="normal">
        <t-tab-panel value="overview" label="概览">
          <div class="tabs-section">
            <t-descriptions v-if="instanceData" :column="2" bordered size="medium" class="detail-desc">
              <t-descriptions-item label="实例标识">{{ instanceData.instance_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="归属用户">
                <span class="cell-strong">{{ instanceData.username || '—' }}</span>
                <div class="cell-sub">
                  {{ instanceData.user_email || '—' }} · {{ instanceData.user_phone || '—' }}
                </div>
              </t-descriptions-item>
              <t-descriptions-item label="用户 ID">{{ instanceData.user_id }}</t-descriptions-item>
              <t-descriptions-item label="来源订单">
                {{ instanceData.order_id > 0 ? instanceData.order_id : '—' }}
              </t-descriptions-item>
              <t-descriptions-item label="服务商">
                {{ instanceData.provider_name || '—' }}
                <span v-if="instanceData.provider_type" class="cell-sub">（{{ instanceData.provider_type }}）</span>
              </t-descriptions-item>
              <t-descriptions-item label="售出商品 ID">{{ instanceData.sell_product_id || instanceData.upstream_product_id || '—' }}</t-descriptions-item>
              <t-descriptions-item label="规格">
                {{ instanceData.cpu }} 核 / {{ instanceData.memory }} MB / {{ instanceData.disk }} GB
              </t-descriptions-item>
              <t-descriptions-item label="磁盘类型">{{ instanceData.disk_type || '—' }}</t-descriptions-item>
              <t-descriptions-item label="带宽">
                {{ instanceData.bandwidth ? `${instanceData.bandwidth} Mbps` : '—' }}
              </t-descriptions-item>
              <t-descriptions-item label="系统">{{ instanceData.os || '—' }}</t-descriptions-item>
              <t-descriptions-item label="区域 / 可用区">
                {{ instanceData.region || '—' }} / {{ instanceData.zone || '—' }}
              </t-descriptions-item>
              <t-descriptions-item label="计费模式">{{ instanceData.billing_mode || '—' }}</t-descriptions-item>
              <t-descriptions-item label="内网 IP">{{ instanceData.private_ip || '—' }}</t-descriptions-item>
              <t-descriptions-item label="公网 IP">{{ instanceData.public_ip || '—' }}</t-descriptions-item>
              <t-descriptions-item label="操作人">{{ instanceData.actor_name || '主账号' }}</t-descriptions-item>
              <t-descriptions-item label="服务状态">
                <t-tag :theme="instanceStatusTheme(instanceData.status)" variant="light" size="small" shape="round">
                  {{ instanceStatusLabel(instanceData.status) }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="生命周期阶段">
                <t-tag :theme="lifecycleStageTheme(instanceData.lifecycle_stage)" variant="light" size="small" shape="round">
                  {{ lifecycleStageLabel(instanceData.lifecycle_stage) }}
                </t-tag>
              </t-descriptions-item>
              <t-descriptions-item label="电源状态">{{ powerStatusText(instanceData.power_status) }}</t-descriptions-item>
              <t-descriptions-item label="到期时间">
                <span :class="expireClass(instanceData)">
                  {{ instanceData.expire_at ? formatTime(instanceData.expire_at) : '—' }}
                  <template v-if="instanceData.expire_state !== 'none' && instanceData.expire_at">
                    （{{ expireStateLabel(instanceData.expire_state) }}，剩余 {{ instanceData.days_left }} 天）
                  </template>
                </span>
              </t-descriptions-item>
              <t-descriptions-item label="最近回源时间">{{ formatTime(instanceData.last_synced_at) }}</t-descriptions-item>
              <t-descriptions-item v-if="instanceData.enforce_attempts > 0 || instanceData.last_enforce_error" label="到期处置">
                <span class="expire-danger">连续失败 {{ instanceData.enforce_attempts }} 次</span>
                <div class="cell-sub" v-if="instanceData.enforce_next_at">
                  下次重试：{{ formatTime(instanceData.enforce_next_at) }}
                </div>
                <div class="cell-sub op-error" v-if="instanceData.last_enforce_error">
                  {{ instanceData.last_enforce_error }}
                </div>
              </t-descriptions-item>
              <t-descriptions-item label="创建时间">{{ formatTime(instanceData.created_at) }}</t-descriptions-item>
              <t-descriptions-item label="更新时间">{{ formatTime(instanceData.updated_at) }}</t-descriptions-item>
              <t-descriptions-item label="管理员备注">{{ instanceData.remark || '—' }}</t-descriptions-item>
            </t-descriptions>
            <t-empty v-else description="暂无数据" />
          </div>
        </t-tab-panel>

        <t-tab-panel value="operations" label="操作记录">
          <div class="tabs-section">
            <div class="table-card__head">
              <h3 class="card-title">操作记录</h3>
              <span class="table-card__meta">共 {{ opPagination.total }} 条</span>
            </div>
            <t-table
              row-key="id"
              :data="operations"
              :columns="operationColumns"
              :loading="opLoading"
              size="small"
              hover
              table-layout="fixed"
              cell-empty-content="—"
              :pagination="opPagination"
              @page-change="handleOpPageChange"
            >
              <template #action="{ row }">
                <span>{{ operationActionLabel(row.action) }}</span>
              </template>
              <template #result="{ row }">
                <t-tag
                  :theme="row.result === 'success' ? 'success' : 'danger'"
                  variant="light"
                  size="small"
                  shape="round"
                >
                  {{ row.result === 'success' ? '成功' : '失败' }}
                </t-tag>
              </template>
              <template #operator="{ row }">
                <div>
                  <span class="cell-strong">{{ operatorTypeLabel(row.operator_type) }}</span>
                  <div class="cell-sub">{{ row.operator_name || '—' }}</div>
                </div>
              </template>
              <template #status_change="{ row }">
                <span class="cell-muted">{{ row.before_status || '—' }}</span>
                <span class="status-arrow">→</span>
                <span class="cell-muted">{{ row.after_status || '—' }}</span>
              </template>
              <template #detail="{ row }">
                <t-tooltip v-if="row.result === 'failed' && row.error_message" :content="row.error_message">
                  <span class="op-error">{{ row.error_message }}</span>
                </t-tooltip>
                <span v-else>—</span>
              </template>
              <template #created_at="{ row }">
                <span class="time-text">{{ formatTime(row.created_at) }}</span>
              </template>
              <template #empty>
                <t-empty description="暂无操作记录" />
              </template>
            </t-table>
          </div>
        </t-tab-panel>

        <t-tab-panel value="related" label="关联记录">
          <div class="tabs-section">
            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">关联订单</h3>
                <span class="table-card__meta">共 {{ related.orders.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.orders"
                :columns="relatedOrderColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #order_no="{ row }">
                  <t-link theme="primary" hover="color" @click="openOrder(row)">{{ row.order_no }}</t-link>
                </template>
                <template #total_amount="{ row }">
                  <span>¥{{ formatPrice(row.total_amount) }}</span>
                </template>
                <template #paid_amount="{ row }">
                  <span>¥{{ formatPrice(row.paid_amount) }}</span>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无关联订单" />
                </template>
              </t-table>
            </div>

            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">续费记录</h3>
                <span class="table-card__meta">共 {{ related.renewals.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.renewals"
                :columns="relatedRenewalColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #amount="{ row }">
                  <span>¥{{ formatPrice(row.amount) }}</span>
                </template>
                <template #expire_before="{ row }">
                  <span class="time-text">{{ formatTime(row.expire_before) }}</span>
                </template>
                <template #expire_after="{ row }">
                  <span class="time-text">{{ formatTime(row.expire_after) }}</span>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无续费记录" />
                </template>
              </t-table>
            </div>

            <div class="related-block">
              <div class="related-block__head">
                <h3 class="card-title">关联工单</h3>
                <span class="table-card__meta">共 {{ related.tickets.length }} 条</span>
              </div>
              <t-table
                row-key="id"
                :data="related.tickets"
                :columns="relatedTicketColumns"
                :loading="relatedLoading"
                size="small"
                hover
                table-layout="fixed"
                cell-empty-content="—"
              >
                <template #ticket_no="{ row }">
                  <t-link theme="primary" hover="color" @click="openTicket(row)">{{ row.ticket_no }}</t-link>
                </template>
                <template #created_at="{ row }">
                  <span class="time-text">{{ formatTime(row.created_at) }}</span>
                </template>
                <template #empty>
                  <t-empty description="暂无关联工单" />
                </template>
              </t-table>
            </div>
          </div>
        </t-tab-panel>
      </t-tabs>
    </section>

    <t-dialog
      v-model:visible="remarkVisible"
      header="实例备注"
      width="480px"
      :confirm-btn="{ content: '保存', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleSaveRemark"
      @close="remarkVisible = false"
    >
      <t-form label-align="top" :data="remarkForm" @submit.prevent>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="remarkForm.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="请输入实例备注" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="resizeVisible"
      header="变配"
      width="520px"
      :confirm-btn="{ content: '提交变配', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleResize"
      @close="resizeVisible = false"
    >
      <t-form label-align="top" :data="resizeForm" @submit.prevent>
        <t-form-item label="CPU（核）" name="cpu">
          <t-input-number v-model="resizeForm.cpu" :min="1" :max="256" theme="column" />
        </t-form-item>
        <t-form-item label="内存（MB）" name="memory">
          <t-input-number v-model="resizeForm.memory" :min="128" :step="128" theme="column" />
        </t-form-item>
        <t-form-item label="磁盘（GB）" name="disk">
          <t-input-number v-model="resizeForm.disk" :min="1" :step="10" theme="column" />
        </t-form-item>
        <t-form-item label="磁盘类型" name="disk_type">
          <t-input v-model="resizeForm.disk_type" placeholder="如 cloud_ssd" />
        </t-form-item>
        <t-form-item label="变配原因" name="reason">
          <t-textarea v-model="resizeForm.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写变配原因" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="destroyVisible"
      header="销毁实例"
      width="520px"
      :confirm-btn="{
        content: '确认销毁',
        theme: 'danger',
        disabled: destroyConfirmDisabled,
      }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleDestroy"
      @close="destroyVisible = false"
    >
      <t-form label-align="top" :data="destroyForm" @submit.prevent>
        <t-form-item>
          <t-alert
            theme="error"
            :message="`销毁后实例与数据不可恢复。请输入实例标识「${instanceData?.instance_id || ''}」以确认。`"
          />
        </t-form-item>
        <t-form-item label="实例标识确认" name="confirm_mark">
          <t-input v-model="destroyForm.confirm_mark" :placeholder="instanceData?.instance_id || '请输入实例标识'" />
        </t-form-item>
        <t-form-item label="销毁原因" name="reason">
          <t-textarea v-model="destroyForm.reason" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写销毁原因" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="renewVisible"
      header="续费"
      width="520px"
      :confirm-btn="{ content: '提交续费', theme: 'primary' }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRenew"
      @close="renewVisible = false"
    >
      <t-form label-align="top" :data="renewForm" @submit.prevent>
        <t-form-item label="续费时长（周期）" name="period_count">
          <t-input-number v-model="renewForm.period_count" :min="1" :max="120" theme="column" />
        </t-form-item>
        <t-form-item label="金额（元）" name="amount">
          <t-input-number v-model="renewForm.amount" :min="0" :precision="2" theme="column" placeholder="留空按默认价格" />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea v-model="renewForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填，请填写代续费备注" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="vnc.visible"
      :header="`远程控制台 · ${instanceData?.name || vnc.name}`"
      width="80%"
      :footer="false"
      destroy-on-close
      @close="vnc.visible = false"
    >
      <div class="vnc-wrap">
        <div v-if="vnc.password" class="vnc-meta">
          <span class="vnc-meta__label">控制台密码</span>
          <code class="vnc-meta__value">{{ vnc.password }}</code>
          <t-button size="small" variant="text" theme="primary" @click="copyVncPassword">复制</t-button>
        </div>
        <iframe v-if="vnc.url && isHttp(vnc.url)" :src="vnc.url" class="vnc-frame" frameborder="0" />
        <t-empty v-else description="控制台地址无法内嵌显示，请点击打开">
          <template #action>
            <t-button theme="primary" @click="openVnc">打开控制台</t-button>
          </template>
        </t-empty>
      </div>
    </t-dialog>

    <!-- 维护类操作弹窗：重装 / 重置密码 / 救援 / 快照 / 硬件 -->
    <t-dialog
      v-model:visible="reinstall.visible"
      header="重装系统"
      :confirm-btn="{ content: '发起重装', theme: 'danger' }"
      :confirm-disabled="!reinstall.os"
      destroy-on-close
      @confirm="handleReinstall"
    >
      <t-alert theme="warning" class="dlg-alert" message="重装会清空系统盘（可选同时格式化数据盘），操作不可撤销。" />
      <t-form label-align="top">
        <t-form-item label="目标镜像 ID">
          <t-input v-model="reinstall.os" placeholder="平台镜像 ID；财务型上游填配置子项 ID（商品配置里的 os 选项 upstream_id）" />
        </t-form-item>
        <t-form-item label="自定义端口（可选，SSH/RDP）">
          <t-input-number v-model="reinstall.port" :min="0" :max="65535" theme="column" />
        </t-form-item>
        <t-form-item label="系统盘大小 GB（可选）">
          <t-input-number v-model="reinstall.system_disk_size" :min="0" theme="column" />
        </t-form-item>
        <t-form-item label="同时格式化数据盘（数据将丢失）">
          <t-switch v-model="reinstall.format_data_disk" />
        </t-form-item>
        <t-form-item label="原因（可选）">
          <t-input v-model="reinstall.reason" placeholder="写入操作流水" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="resetPwd.visible"
      header="重置实例登录密码"
      :confirm-btn="{ content: '重置密码', theme: 'danger' }"
      :confirm-disabled="!resetPwd.password"
      destroy-on-close
      @confirm="handleResetPassword"
    >
      <t-alert theme="info" class="dlg-alert" message="系统与数据保留，仅更换登录密码。运行中的实例可能被强制重启。" />
      <t-form label-align="top">
        <t-form-item label="新密码">
          <t-input v-model="resetPwd.password" type="password" placeholder="建议 12 位以上，含大小写与符号" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="rescue.visible"
      header="进入救援系统"
      :confirm-btn="{ content: '进入救援' }"
      :confirm-disabled="!rescue.temp_password"
      destroy-on-close
      @confirm="handleRescue"
    >
      <t-alert
        theme="warning"
        class="dlg-alert"
        message="进入救援后系统变为临时系统，处理完请务必点「退出救援」回到原系统。"
      />
      <t-form label-align="top">
        <t-form-item label="救援系统类型">
          <t-radio-group v-model="rescue.system" variant="default-filled">
            <t-radio-button :value="1">类型 1</t-radio-button>
            <t-radio-button :value="2">类型 2</t-radio-button>
          </t-radio-group>
        </t-form-item>
        <t-form-item label="临时密码">
          <t-input v-model="rescue.temp_password" type="password" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="snap.visible"
      header="快照与备份"
      width="680px"
      :footer="false"
      destroy-on-close
      @close="snap.visible = false"
    >
      <t-space class="dlg-toolbar">
        <t-input v-model="snap.name" placeholder="快照名称（留空自动生成）" class="dlg-grow" />
        <t-select v-model="snap.type" :options="snapTypeOptions" class="dlg-select" />
        <t-button theme="primary" :loading="snap.loading" @click="handleCreateSnapshot">创建</t-button>
        <t-button variant="outline" :loading="snap.loading" @click="loadSnapshots(false)">刷新</t-button>
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
            <t-link theme="danger" hover="color" @click="handleDeleteSnapshot(row)">删除</t-link>
          </t-space>
        </template>
      </t-table>
    </t-dialog>

    <t-dialog
      v-model:visible="snap.restoreVisible"
      header="用快照恢复实例"
      :confirm-btn="{ content: '确认恢复', theme: 'danger' }"
      :confirm-disabled="snap.restoreMark.trim() !== (instanceData?.instance_id || '')"
      destroy-on-close
      @confirm="handleRestoreSnapshot"
    >
      <t-alert theme="warning" class="dlg-alert" message="恢复会覆盖当前系统盘，盘上数据将回到快照时刻。请输入实例标识确认。" />
      <p class="dlg-confirm-tip">
        请输入实例标识 <code>{{ instanceData?.instance_id }}</code>
      </p>
      <t-input v-model="snap.restoreMark" placeholder="实例标识" />
    </t-dialog>

    <t-dialog
      v-model:visible="hw.visible"
      header="硬件变更"
      :footer="false"
      destroy-on-close
      @close="hw.visible = false"
    >
      <t-form label-align="top">
        <t-form-item label="带宽（Mbps · 0 表示不改该方向）">
          <t-space>
            <t-input-number v-model="hw.in_bw" :min="0" theme="column" />
            <span class="dlg-unit">流入</span>
            <t-input-number v-model="hw.out_bw" :min="0" theme="column" />
            <span class="dlg-unit">流出</span>
            <t-button size="small" theme="primary" :disabled="!hw.in_bw && !hw.out_bw" @click="handleSetBandwidth">
              提交
            </t-button>
          </t-space>
        </t-form-item>
        <t-form-item label="增加 IP">
          <t-space>
            <t-radio-group v-model="hw.ip_version" variant="default-filled">
              <t-radio-button :value="4">IPv4</t-radio-button>
              <t-radio-button :value="6">IPv6</t-radio-button>
            </t-radio-group>
            <t-input-number v-model="hw.ip_num" :min="1" theme="column" />
            <t-input v-model="hw.ip_group" placeholder="IP 分组 ID（可选）" />
            <t-button size="small" theme="primary" :disabled="!hw.ip_num" @click="handleAddIP">提交</t-button>
          </t-space>
        </t-form-item>
        <t-form-item label="挂载数据盘（GB）">
          <t-space>
            <t-input-number v-model="hw.disk_size" :min="1" theme="column" />
            <t-input v-model="hw.store" placeholder="存储 ID（可选）" />
            <t-button size="small" theme="primary" :disabled="!hw.disk_size" @click="handleAttachDisk">提交</t-button>
          </t-space>
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 重装后平台签发的新凭据：只显示这一次，必须提示立即保存 -->
    <t-dialog
      v-model:visible="credential.visible"
      header="重装已受理 · 请立即保存新凭据"
      :confirm-btn="{ content: '我已保存' }"
      :close-btn="false"
      destroy-on-close
      @confirm="credential.visible = false"
    >
      <t-alert theme="warning" class="dlg-alert" message="平台新签发的登录凭据只显示这一次，本系统不保存；关闭后只能去平台面板重置。" />
      <t-descriptions :column="1" bordered size="small">
        <t-descriptions-item label="用户名">{{ credential.username || '—' }}</t-descriptions-item>
        <t-descriptions-item label="新密码">
          <code class="cell-strong">{{ credential.password || '（平台未返回，沿用原密码）' }}</code>
        </t-descriptions-item>
      </t-descriptions>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { RefreshIcon, ServerIcon } from 'tdesign-icons-vue-next'
import { DialogPlugin, MessagePlugin, type PageInfo, type PrimaryTableCol } from 'tdesign-vue-next'

import {
  addInstanceIP,
  attachInstanceDisk,
  createInstanceSnapshot,
  deleteInstanceSnapshot,
  destroyInstance,
  exitRescueInstance,
  getInstanceDetail,
  getInstanceOperations,
  getInstanceRelated,
  getInstanceSnapshots,
  getInstanceVNC,
  powerInstance,
  reinstallInstance,
  resetInstancePassword,
  resizeInstance,
  rescueInstance,
  restoreInstanceSnapshot,
  suspendInstance,
  syncInstance,
  unsuspendInstance,
  updateInstanceBandwidth,
  updateInstanceRemark,
} from '@/api/instance'
import { renewInstance } from '@/api/lifecycle'
import {
  expireStateLabel,
  formatPrice,
  formatTime,
  instanceStatusLabel,
  instanceStatusTheme,
  lifecycleStageLabel,
  lifecycleStageTheme,
  operationActionLabel,
  operatorTypeLabel,
  powerActionLabel,
  powerActionTips,
} from '@/pages/instances/constants'
import type {
  InstanceDetail,
  InstanceRelatedResponse,
  InstanceSnapshotInfo,
  OperationItem,
  RelatedOrder,
  RelatedRenewal,
  RelatedTicket,
} from '@/types/interface'
import { usePermission } from '@/composables/usePermission'

defineOptions({ name: 'InstanceDetail' })

const route = useRoute()
const router = useRouter()

const instanceId = Number(route.params.id)
const instanceData = ref<InstanceDetail | null>(null)
const loading = ref(false)
const syncing = ref(false)
const actioning = ref(false)
const activeTab = ref('overview')

const operations = ref<OperationItem[]>([])
const opLoading = ref(false)
const opPagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showJumper: true,
})

const related = reactive<InstanceRelatedResponse>({ orders: [], renewals: [], tickets: [] })
const relatedLoading = ref(false)

const operationColumns: PrimaryTableCol<OperationItem>[] = [
  { colKey: 'created_at', title: '时间', width: 170 },
  { colKey: 'action', title: '动作', width: 110 },
  { colKey: 'result', title: '结果', width: 90 },
  { colKey: 'operator', title: '操作人', width: 140 },
  { colKey: 'status_change', title: '状态变化', minWidth: 200 },
  { colKey: 'detail', title: '详情', minWidth: 180 },
]

const relatedOrderColumns: PrimaryTableCol<RelatedOrder>[] = [
  { colKey: 'order_no', title: '订单号', minWidth: 180 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'product_name', title: '产品名称', minWidth: 160 },
  { colKey: 'total_amount', title: '应付', width: 110 },
  { colKey: 'paid_amount', title: '实付', width: 110 },
  { colKey: 'pay_method', title: '支付方式', width: 110 },
  { colKey: 'created_at', title: '下单时间', width: 170 },
]

const relatedRenewalColumns: PrimaryTableCol<RelatedRenewal>[] = [
  { colKey: 'renewal_no', title: '续费单号', minWidth: 180 },
  { colKey: 'source', title: '来源', width: 100 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'period_count', title: '周期', width: 80, align: 'center' as const },
  { colKey: 'amount', title: '金额', width: 110 },
  { colKey: 'expire_before', title: '原到期', width: 170 },
  { colKey: 'expire_after', title: '新到期', width: 170 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

const relatedTicketColumns: PrimaryTableCol<RelatedTicket>[] = [
  { colKey: 'ticket_no', title: '工单号', minWidth: 170 },
  { colKey: 'title', title: '标题', minWidth: 200 },
  { colKey: 'priority', title: '优先级', width: 90 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'assigned_name', title: '处理人', width: 110 },
  { colKey: 'created_at', title: '创建时间', width: 170 },
]

function powerStatusText(status: string): string {
  if (status === 'on') return '开机'
  if (status === 'off') return '关机'
  return '未知'
}

function expireClass(row: InstanceDetail): string {
  if (row.expire_state === 'expired') return 'expire-danger'
  if (row.expire_state === 'expiring') return 'expire-warning'
  return ''
}

function capabilityTitle(supported: boolean | undefined): string {
  return supported ? '' : '该服务商不支持此操作'
}

const consoleDisabled = computed(
  () => !instanceData.value?.capabilities?.console || instanceData.value?.status !== 'running',
)

const consoleTitle = computed(() => {
  if (!instanceData.value?.capabilities?.console) return '该服务商不支持此操作'
  if (instanceData.value?.status !== 'running') return '仅运行中的实例可打开控制台'
  return ''
})

function powerDisabled(action: string): boolean {
  if (!instanceData.value?.capabilities?.power) return true
  const ps = instanceData.value?.power_status
  if (action === 'on') return ps === 'on'
  if (action === 'off') return ps === 'off'
  return ps !== 'on'
}

function powerTitle(action: string): string {
  if (!instanceData.value?.capabilities?.power) return '该服务商不支持此操作'
  const ps = instanceData.value?.power_status
  if (action === 'on') return ps === 'on' ? '实例已处于运行状态' : ''
  if (action === 'off') return ps === 'off' ? '实例已处于关机状态' : ''
  return ps !== 'on' ? '仅运行中的实例可重启' : ''
}

async function loadDetail() {
  loading.value = true
  try {
    instanceData.value = await getInstanceDetail(instanceId, false)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载实例详情失败')
  } finally {
    loading.value = false
  }
}

async function loadOperations() {
  opLoading.value = true
  try {
    const data = await getInstanceOperations(instanceId, {
      page: opPagination.current,
      page_size: opPagination.pageSize,
    })
    operations.value = data.items
    opPagination.total = data.meta.total
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载操作记录失败')
  } finally {
    opLoading.value = false
  }
}

async function loadRelated() {
  relatedLoading.value = true
  try {
    const data = await getInstanceRelated(instanceId)
    related.orders = data.orders
    related.renewals = data.renewals
    related.tickets = data.tickets
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载关联记录失败')
  } finally {
    relatedLoading.value = false
  }
}

function reloadAll() {
  loadDetail()
  loadOperations()
}

function handleOpPageChange(pageInfo: PageInfo) {
  opPagination.current = pageInfo.current
  opPagination.pageSize = pageInfo.pageSize
  loadOperations()
}

function goBack() {
  router.back()
}

function openOrder(row: RelatedOrder) {
  router.push(`/orders/detail/${row.id}`)
}

function openTicket(row: RelatedTicket) {
  router.push(`/tickets/detail/${row.id}`)
}

function handlePower(action: string) {
  if (action === 'on') {
    doPower(action)
    return
  }
  const name = instanceData.value?.name || instanceData.value?.instance_id || ''
  const label = powerActionLabel(action)
  const dialog = DialogPlugin.confirm({
    header: `${label}实例`,
    body: `确认对实例「${name}」执行${label}？${powerActionTips[action] || ''}`,
    confirmBtn: { content: `确认${label}`, theme: action === 'reboot' ? 'warning' : 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      await doPower(action)
    },
    onClose: () => dialog.destroy(),
  })
}

async function doPower(action: string) {
  try {
    await powerInstance(instanceId, action)
    MessagePlugin.success(`${powerActionLabel(action)}指令已发送`)
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || `${powerActionLabel(action)}失败`)
  }
}

async function handleSync() {
  syncing.value = true
  try {
    await syncInstance(instanceId)
    MessagePlugin.success('实例状态已同步')
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '同步失败')
  } finally {
    syncing.value = false
  }
}

// —— 暂停/恢复（T5.5）——
function handleSuspend() {
  const name = instanceData.value?.name || instanceData.value?.instance_id || ''
  const dialog = DialogPlugin.confirm({
    header: '暂停实例服务',
    body: `确认暂停实例「${name}」？暂停后平台侧实例将停止服务，用户需续费或联系客服才能恢复。`,
    confirmBtn: { content: '确认暂停', theme: 'danger' },
    cancelBtn: { content: '再想想' },
    onConfirm: async () => {
      dialog.destroy()
      actioning.value = true
      try {
        await suspendInstance(instanceId)
        MessagePlugin.success('实例已暂停')
        reloadAll()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '暂停失败')
      } finally {
        actioning.value = false
      }
    },
    onClose: () => dialog.destroy(),
  })
}

async function handleUnsuspend() {
  actioning.value = true
  try {
    await unsuspendInstance(instanceId)
    MessagePlugin.success('实例已恢复')
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复失败')
  } finally {
    actioning.value = false
  }
}

const remarkVisible = ref(false)
const remarkForm = reactive<{ remark: string }>({ remark: '' })

function openRemarkDialog() {
  remarkForm.remark = instanceData.value?.remark ?? ''
  remarkVisible.value = true
}

async function handleSaveRemark() {
  try {
    await updateInstanceRemark(instanceId, { remark: remarkForm.remark })
    MessagePlugin.success('备注已更新')
    remarkVisible.value = false
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '更新备注失败')
  }
}

const resizeVisible = ref(false)
const resizeForm = reactive<{ cpu: number; memory: number; disk: number; disk_type: string; reason: string }>({
  cpu: 0,
  memory: 0,
  disk: 0,
  disk_type: '',
  reason: '',
})

function openResizeDialog() {
  const data = instanceData.value
  if (!data) return
  resizeForm.cpu = data.cpu
  resizeForm.memory = data.memory
  resizeForm.disk = data.disk
  resizeForm.disk_type = data.disk_type
  resizeForm.reason = ''
  resizeVisible.value = true
}

async function handleResize() {
  try {
    await resizeInstance(instanceId, {
      cpu: resizeForm.cpu,
      memory: resizeForm.memory,
      disk: resizeForm.disk,
      disk_type: resizeForm.disk_type,
      reason: resizeForm.reason || undefined,
    })
    MessagePlugin.success('变配指令已提交')
    resizeVisible.value = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '变配失败')
  }
}

const destroyVisible = ref(false)
const destroyForm = reactive<{ confirm_mark: string; reason: string }>({ confirm_mark: '', reason: '' })

const destroyConfirmDisabled = computed(
  () => !instanceData.value?.instance_id || destroyForm.confirm_mark.trim() !== instanceData.value.instance_id,
)

function openDestroyDialog() {
  destroyForm.confirm_mark = ''
  destroyForm.reason = ''
  destroyVisible.value = true
}

async function handleDestroy() {
  try {
    await destroyInstance(instanceId, {
      confirm_mark: destroyForm.confirm_mark.trim(),
      reason: destroyForm.reason || undefined,
    })
    MessagePlugin.success('实例已销毁')
    destroyVisible.value = false
    router.push('/instances/list')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '销毁失败')
  }
}

const renewVisible = ref(false)
const renewForm = reactive<{ period_count: number; amount: number; remark: string }>({
  period_count: 1,
  amount: 0,
  remark: '',
})

function openRenewDialog() {
  renewForm.period_count = 1
  renewForm.amount = 0
  renewForm.remark = ''
  renewVisible.value = true
}

async function handleRenew() {
  try {
    await renewInstance(instanceId, {
      period_count: renewForm.period_count,
      amount: renewForm.amount || undefined,
      remark: renewForm.remark || undefined,
    })
    MessagePlugin.success('代续费成功，已延长实例到期时间')
    renewVisible.value = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '代续费失败')
  }
}

const vnc = reactive({ visible: false, name: '', url: '', password: '' })

async function handleVnc() {
  try {
    const data = await getInstanceVNC(instanceId)
    vnc.name = instanceData.value?.name || instanceData.value?.instance_id || ''
    vnc.url = data.url
    vnc.password = data.password
    vnc.visible = true
  } catch (error) {
    MessagePlugin.error((error as Error).message || '获取控制台地址失败')
  }
}

function openVnc() {
  window.open(vnc.url, '_blank')
}

function copyVncPassword() {
  if (!vnc.password) return
  void navigator.clipboard?.writeText(vnc.password)
  MessagePlugin.success('控制台密码已复制')
}

function isHttp(url: string): boolean {
  return /^https?:/i.test(url)
}

// ===== 维护类操作（平台接口实测存在）=====
//
// 统一入口是一枚「维护 ▾」下拉：操作栏已经有 9 个按钮，再加 6 个会挤成一团。
// 每个子项各自带权限码（用 v-permission 不适用于下拉项，故在 options 里显式过滤）
// 与能力位（平台不支持时置灰并说明原因）。

const { has: hasPerm } = usePermission()

const maintenanceOptions = computed(() => {
  const caps = instanceData.value?.capabilities
  const opts: { content: string; value: string; disabled?: boolean }[] = []
  if (hasPerm('instance:reinstall')) {
    opts.push({ content: '重装系统', value: 'reinstall', disabled: !caps?.reinstall })
  }
  if (hasPerm('instance:password')) {
    opts.push({ content: '重置密码', value: 'resetpwd', disabled: !caps?.reset_password })
  }
  if (hasPerm('instance:rescue')) {
    opts.push({ content: '进入救援系统', value: 'rescue', disabled: !caps?.rescue })
    opts.push({ content: '退出救援系统', value: 'exitrescue', disabled: !caps?.rescue })
  }
  if (hasPerm('instance:snapshot')) {
    opts.push({ content: '快照与备份', value: 'snapshot', disabled: !caps?.snapshot })
  }
  if (hasPerm('instance:hardware')) {
    opts.push({ content: '硬件变更', value: 'hardware', disabled: !caps?.bandwidth && !caps?.add_ip && !caps?.attach_disk })
  }
  return opts
})

const maintenanceDisabled = computed(() => maintenanceOptions.value.length === 0)

function onMaintenancePick(data: { value: string; disabled?: boolean }) {
  if (data.disabled) {
    MessagePlugin.warning('当前服务商不支持该操作')
    return
  }
  switch (data.value) {
    case 'reinstall':
      reinstall.os = ''
      reinstall.port = 0
      reinstall.system_disk_size = 0
      reinstall.format_data_disk = false
      reinstall.reason = ''
      reinstall.visible = true
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
      handleExitRescue()
      break
    case 'snapshot':
      snap.name = ''
      snap.type = 'snap'
      snap.visible = true
      loadSnapshots(true)
      break
    case 'hardware':
      hw.visible = true
      break
  }
}

// —— 重装 ——
const reinstall = reactive<{
  visible: boolean
  os: string
  port: number
  system_disk_size: number
  format_data_disk: boolean
  reason: string
  loading: boolean
}>({ visible: false, os: '', port: 0, system_disk_size: 0, format_data_disk: false, reason: '', loading: false })

// 重装后平台签发的新凭据（只此一次展示）
const credential = reactive<{ visible: boolean; username: string; password: string }>({
  visible: false,
  username: '',
  password: '',
})

async function handleReinstall() {
  if (!reinstall.os) {
    MessagePlugin.warning('请填写目标镜像 ID')
    return
  }
  reinstall.loading = true
  try {
    const res = await reinstallInstance(instanceId, {
      os: reinstall.os.trim(),
      port: reinstall.port || undefined,
      system_disk_size: reinstall.system_disk_size || undefined,
      format_data_disk: reinstall.format_data_disk,
      reason: reinstall.reason || undefined,
    })
    reinstall.visible = false
    if (res?.password || res?.username) {
      credential.username = res.username || ''
      credential.password = res.password || ''
      credential.visible = true
    } else {
      MessagePlugin.success('重装指令已提交')
    }
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重装失败')
  } finally {
    reinstall.loading = false
  }
}

// —— 重置密码 ——
const resetPwd = reactive<{ visible: boolean; password: string; loading: boolean }>({
  visible: false,
  password: '',
  loading: false,
})

async function handleResetPassword() {
  if (!resetPwd.password) {
    MessagePlugin.warning('请输入新密码')
    return
  }
  resetPwd.loading = true
  try {
    await resetInstancePassword(instanceId, resetPwd.password)
    MessagePlugin.success('密码已重置')
    resetPwd.visible = false
  } catch (error) {
    MessagePlugin.error((error as Error).message || '重置密码失败')
  } finally {
    resetPwd.loading = false
  }
}

// —— 救援系统 ——
const rescue = reactive<{ visible: boolean; system: number; temp_password: string; loading: boolean }>({
  visible: false,
  system: 1,
  temp_password: '',
  loading: false,
})

async function handleRescue() {
  if (!rescue.temp_password) {
    MessagePlugin.warning('请输入临时密码')
    return
  }
  rescue.loading = true
  try {
    await rescueInstance(instanceId, { system: rescue.system, temp_password: rescue.temp_password })
    MessagePlugin.success('已发起进入救援系统')
    rescue.visible = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '进入救援系统失败')
  } finally {
    rescue.loading = false
  }
}

async function handleExitRescue() {
  const dialog = DialogPlugin.confirm({
    header: '退出救援系统',
    body: '确认退出救援系统？实例将回到原系统。',
    theme: 'warning',
    onConfirm: async () => {
      try {
        await exitRescueInstance(instanceId)
        MessagePlugin.success('已发起退出救援系统')
        reloadAll()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '退出救援系统失败')
      } finally {
        dialog.hide()
      }
    },
  })
}

// —— 快照与备份 ——
const snapTypeOptions = [
  { label: '快照', value: 'snap' },
  { label: '备份', value: 'backup' },
]

const snapColumns: PrimaryTableCol[] = [
  { colKey: 'name', title: '名称', minWidth: 180 },
  { colKey: 'type', title: '类型', width: 80 },
  { colKey: 'size', title: '大小', width: 90 },
  { colKey: 'create_time', title: '创建时间', width: 170 },
  { colKey: 'snap_action', title: '操作', width: 130, align: 'center' as const },
]

const snap = reactive<{
  visible: boolean
  loading: boolean
  rows: InstanceSnapshotInfo[]
  name: string
  type: string
  restoreVisible: boolean
  restoreMark: string
  restoreId: string
}>({
  visible: false,
  loading: false,
  rows: [],
  name: '',
  type: 'snap',
  restoreVisible: false,
  restoreMark: '',
  restoreId: '',
})

async function loadSnapshots(withLoading = true) {
  if (withLoading) snap.loading = true
  try {
    snap.rows = (await getInstanceSnapshots(instanceId)) || []
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载快照失败')
  } finally {
    snap.loading = false
  }
}

async function handleCreateSnapshot() {
  snap.loading = true
  try {
    await createInstanceSnapshot(instanceId, { type: snap.type, name: snap.name || undefined })
    MessagePlugin.success('快照创建指令已提交（面板异步执行，稍后刷新查看）')
    snap.name = ''
    setTimeout(() => loadSnapshots(false), 5000)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '创建快照失败')
  } finally {
    snap.loading = false
  }
}

function openRestoreSnapshot(row: InstanceSnapshotInfo) {
  snap.restoreId = row.id
  snap.restoreMark = ''
  snap.restoreVisible = true
}

async function handleRestoreSnapshot() {
  try {
    await restoreInstanceSnapshot(instanceId, { snapshot_id: snap.restoreId, confirm_mark: snap.restoreMark.trim() })
    MessagePlugin.success('恢复指令已提交')
    snap.restoreVisible = false
    reloadAll()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '恢复失败')
  }
}

async function handleDeleteSnapshot(row: InstanceSnapshotInfo) {
  const dialog = DialogPlugin.confirm({
    header: '删除快照',
    body: `确认删除「${row.name}」？删除后无法用它恢复。`,
    theme: 'danger',
    onConfirm: async () => {
      try {
        await deleteInstanceSnapshot(instanceId, row.id)
        MessagePlugin.success('快照已删除')
        loadSnapshots()
      } catch (error) {
        MessagePlugin.error((error as Error).message || '删除失败')
      } finally {
        dialog.hide()
      }
    },
  })
}

// —— 硬件变更 ——
const hw = reactive<{
  visible: boolean
  in_bw: number
  out_bw: number
  ip_version: number
  ip_num: number
  ip_group: string
  disk_size: number
  store: string
  loading: boolean
}>({
  visible: false,
  in_bw: 0,
  out_bw: 0,
  ip_version: 4,
  ip_num: 0,
  ip_group: '',
  disk_size: 0,
  store: '',
  loading: false,
})

async function handleSetBandwidth() {
  if (!hw.in_bw && !hw.out_bw) {
    MessagePlugin.warning('至少填一个方向的带宽')
    return
  }
  hw.loading = true
  try {
    await updateInstanceBandwidth(instanceId, { in_bw: hw.in_bw, out_bw: hw.out_bw })
    MessagePlugin.success('带宽已修改')
    hw.in_bw = 0
    hw.out_bw = 0
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '修改带宽失败')
  } finally {
    hw.loading = false
  }
}

async function handleAddIP() {
  if (!hw.ip_num) {
    MessagePlugin.warning('请输入数量')
    return
  }
  hw.loading = true
  try {
    await addInstanceIP(instanceId, { version: hw.ip_version, num: hw.ip_num, ip_group: hw.ip_group || undefined })
    MessagePlugin.success('IP 增加指令已提交')
    hw.ip_num = 0
    hw.ip_group = ''
  } catch (error) {
    MessagePlugin.error((error as Error).message || '增加 IP 失败')
  } finally {
    hw.loading = false
  }
}

async function handleAttachDisk() {
  if (!hw.disk_size) {
    MessagePlugin.warning('请输入数据盘容量')
    return
  }
  hw.loading = true
  try {
    await attachInstanceDisk(instanceId, { size_gb: hw.disk_size, store: hw.store || undefined })
    MessagePlugin.success('数据盘挂载指令已提交')
    hw.disk_size = 0
    hw.store = ''
    loadDetail()
  } catch (error) {
    MessagePlugin.error((error as Error).message || '挂载数据盘失败')
  } finally {
    hw.loading = false
  }
}

onMounted(() => {
  loadDetail()
  loadOperations()
  loadRelated()
})
</script>

<style lang="css">
@import './shared.css';
</style>
