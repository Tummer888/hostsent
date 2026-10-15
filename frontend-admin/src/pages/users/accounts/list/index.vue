<template>
  <div class="page-body users-module user-list-page">
    <header class="list-header surface-card">
      <div class="list-header__main">
        <div class="list-header__title-row">
          <div class="list-header__icon">
            <UserIcon size="22" aria-hidden="true" />
          </div>
          <h2 class="list-header__title">{{ isRecycleView ? '用户回收站' : '用户列表' }}</h2>
          <t-tag v-if="activeFilterLabel" class="page-chip" theme="primary" variant="light" shape="round">
            {{ activeFilterLabel }}
          </t-tag>
        </div>
      </div>
      <div class="list-header__actions">
        <!-- 用户列表 / 回收站（doc104 §4）：回收站不是一个新菜单，而是同一列表的
             filter=deleted 视图 —— 两边的列、筛选、分页语义完全一致，独立成页只会
             多出一份需要同步维护的表格。 -->
        <t-radio-group v-model="currentView" variant="default-filled" class="view-switch" @change="handleViewChange">
          <t-radio-button value="active">用户列表</t-radio-button>
          <t-radio-button value="recycle">回收站</t-radio-button>
        </t-radio-group>
        <t-button
          v-if="!isRecycleView && canCreate"
          class="page-btn page-btn--ghost"
          variant="outline"
          @click="openCreate"
        >
          <template #icon>
            <AddIcon aria-hidden="true" />
          </template>
          新增用户
        </t-button>
        <t-button
          v-if="isRecycleView && canPurge"
          class="page-btn page-btn--ghost"
          variant="outline"
          @click="openPurgeDialog"
        >
          <template #icon>
            <DeleteIcon aria-hidden="true" />
          </template>
          留存期清理
        </t-button>
        <t-button class="page-btn" variant="outline" :loading="loading" @click="reload">
          <template #icon>
            <RefreshIcon aria-hidden="true" />
          </template>
          刷新
        </t-button>
      </div>
    </header>

    <!--
      统一筛选卡：字段必须是 .filter-card__grid 的直接子元素，窄屏折叠规则
      （:nth-child(n+2) 隐藏）挂在直接子元素上。关键词是最常用的主筛选，
      必须排第一，窄屏才不会被折叠（primary-count=1 只保留它）。

      查询触发分两类：下拉框选完即查（@change，少点一次「查询」），文本框仍是
      手动点查询或回车 —— 文本要等输完，边打边查既费请求也闪。
    -->
    <FilterCard :primary-count="1">
      <div class="field" :class="{ 'field--active': !!filters.keyword }">
        <span class="field__label">关键词</span>
        <t-input
          v-model="filters.keyword"
          class="unified-control"
          clearable
          placeholder="搜索用户名 / 姓名 / 邮箱 / 手机号 / ID"
          @enter="handleSearch"
        >
          <template #prefix-icon>
            <SearchIcon />
          </template>
        </t-input>
      </div>

      <div v-if="!isRecycleView" class="field" :class="{ 'field--active': !!filters.status }">
        <span class="field__label">用户状态</span>
        <t-select
          v-model="filters.status"
          class="unified-control"
          clearable
          filterable
          placeholder="全部状态"
          :options="statusOptions"
          @change="handleSearch"
        />
      </div>

      <div v-if="!isRecycleView" class="field" :class="{ 'field--active': !!filters.filter }">
        <span class="field__label">快捷筛选</span>
        <t-select
          v-model="filters.filter"
          class="unified-control"
          clearable
          placeholder="快捷筛选"
          :options="quickFilterOptions"
          @change="handleSearch"
        />
      </div>

      <div class="field" :class="{ 'field--active': filters.user_level_id !== undefined }">
        <span class="field__label">用户等级</span>
        <t-select
          v-model="filters.user_level_id"
          class="unified-control"
          clearable
          filterable
          placeholder="全部等级"
          :options="levelOptions"
          @change="handleSearch"
        />
      </div>

      <div class="field" :class="{ 'field--active': filters.user_group_id !== undefined }">
        <span class="field__label">用户组</span>
        <t-select
          v-model="filters.user_group_id"
          class="unified-control"
          clearable
          filterable
          placeholder="全部用户组"
          :options="userGroupFilterOptions"
          @change="handleSearch"
        />
      </div>

      <div class="field" :class="{ 'field--active': !!filters.is_agent }">
        <span class="field__label">代理等级</span>
        <t-select
          v-model="filters.is_agent"
          class="unified-control"
          clearable
          placeholder="全部（含非代理）"
          :options="agentFilterOptions"
          @change="handleSearch"
        />
      </div>

      <div class="field" :class="{ 'field--active': filters.agent_level_id !== undefined }">
        <span class="field__label">代理等级归属</span>
        <t-select
          v-model="filters.agent_level_id"
          class="unified-control"
          clearable
          filterable
          placeholder="全部代理等级"
          :options="agentLevelFilterOptions"
          :disabled="filters.is_agent === 'false'"
          @change="handleSearch"
        />
      </div>

      <div class="field" :class="{ 'field--active': !!filters.is_sub_account }">
        <span class="field__label">账号类型</span>
        <t-select
          v-model="filters.is_sub_account"
          class="unified-control"
          clearable
          placeholder="全部账号"
          :options="accountTypeOptions"
          @change="handleSearch"
        />
      </div>

      <div class="field" :class="{ 'field--active': filters.sales_admin_id !== undefined }">
        <span class="field__label">归属销售</span>
        <t-select
          v-model="filters.sales_admin_id"
          class="unified-control"
          clearable
          filterable
          placeholder="全部销售"
          :options="salesFilterOptions"
          @change="handleSearch"
        />
      </div>

      <template #actions>
        <t-space>
          <t-button class="page-btn" theme="primary" @click="handleSearch">
            <template #icon>
              <SearchIcon aria-hidden="true" />
            </template>
            查询
          </t-button>
          <t-button class="page-btn page-btn--ghost" variant="outline" @click="handleReset">重置</t-button>
        </t-space>
      </template>
    </FilterCard>

    <section class="table-panel surface-card">
      <div class="table-panel__head">
        <h3 class="table-panel__title">{{ isRecycleView ? '已注销用户（留存期内可恢复）' : '用户数据' }}</h3>
        <!-- 移动端：批量模式开关。默认关闭 = 每行一个「操作」按钮；
             打开后才显示勾选列，把横向空间让给信息列，勾选不再遮挡账号/金额。 -->
        <t-button
          v-if="isMobile && canBatchSelect"
          class="batch-toggle"
          size="small"
          :variant="batchMode ? 'base' : 'outline'"
          :theme="batchMode ? 'primary' : 'default'"
          @click="toggleBatchMode"
        >
          {{ batchMode ? '退出批量' : '批量操作' }}
        </t-button>
        <t-space v-if="selectedIds.length" size="small">
          <span class="selection-hint">已选 {{ selectedIds.length }} 个</span>
          <template v-if="isRecycleView">
            <t-button
              v-if="canRestore"
              size="small"
              theme="primary"
              :loading="batchSubmitting"
              @click="handleBatchRestore"
            >
              批量恢复
            </t-button>
          </template>
          <template v-else>
            <t-button
              v-if="canUpdate"
              size="small"
              variant="outline"
              :loading="batchSubmitting"
              @click="handleBatchStatus('disabled')"
            >
              批量冻结
            </t-button>
            <t-button
              v-if="canUpdate"
              size="small"
              variant="outline"
              :loading="batchSubmitting"
              @click="handleBatchStatus('active')"
            >
              批量解冻
            </t-button>
            <t-button
              v-if="canDelete"
              size="small"
              theme="danger"
              :loading="batchSubmitting"
              @click="openBatchDeleteDialog"
            >
              批量注销
            </t-button>
          </template>
          <t-button size="small" variant="text" @click="clearSelection">取消选择</t-button>
        </t-space>
      </div>

      <div v-if="errorMessage" class="error-banner" role="alert">
        <ErrorCircleIcon size="16" aria-hidden="true" />
        <span>{{ errorMessage }}</span>
        <t-link class="page-link" theme="primary" hover="color" @click="reload">重试</t-link>
      </div>

      <div
        ref="tableDragRef"
        :class="['table-drag-scroll', { 'table-drag-scroll--dragging': isTableDragging }]"
        @mousedown="handleTableDragStart"
      >
        <t-table
          row-key="id"
          :data="sortedTableData"
          :columns="columns"
          :loading="loading"
          :pagination="isMobile ? undefined : pagination"
          size="small"
          hover
          table-layout="fixed"
          cell-empty-content="—"
          class="user-table"
          :selected-row-keys="selectedIds"
          @select-change="handleSelectChange"
          @page-change="handlePageChange"
        >
          <template #id="{ row }">
            <div class="id-cell">
              <span class="id-cell__value">{{ row.id }}</span>
            </div>
          </template>

          <template #username="{ row }">
            <div class="user-cell user-cell--primary">
              <div class="user-cell__head">
                <t-link class="user-link" theme="primary" hover="color" @click="goUserDetail(row)">
                  {{ row.username }}
                </t-link>
                <t-tag
                  v-if="row.is_sub_account"
                  class="account-type-tag account-type-tag--sub"
                  theme="warning"
                  variant="light"
                  size="small"
                  shape="round"
                >
                  子账号{{ row.owner_name ? ` · ${row.owner_name}` : '' }}
                </t-tag>
                <t-tag
                  v-else
                  class="account-type-tag"
                  theme="success"
                  variant="light-outline"
                  size="small"
                  shape="round"
                >
                  主账号
                </t-tag>
              </div>
              <div v-if="row.email" class="copy-row">
                <span class="copy-row__value copy-row__value--email">{{ row.email }}</span>
                <t-popup content="复制邮箱" placement="top">
                  <t-tag
                    class="copy-tag"
                    theme="primary"
                    variant="light"
                    size="small"
                    shape="round"
                    @click="copyText(row.email, '邮箱')"
                  >
                    复制
                  </t-tag>
                </t-popup>
              </div>
            </div>
          </template>

          <template #real_name="{ row }">
            <div class="user-cell">
              <span class="user-cell__name user-cell__name--secondary">{{ row.real_name || '待补充' }}</span>
              <div v-if="row.phone" class="copy-row">
                <span class="copy-row__value copy-row__value--phone">{{ row.phone }}</span>
                <t-popup content="复制手机号" placement="top">
                  <t-tag
                    class="copy-tag"
                    theme="primary"
                    variant="light"
                    size="small"
                    shape="round"
                    @click="copyText(row.phone, '手机号')"
                  >
                    复制
                  </t-tag>
                </t-popup>
              </div>
            </div>
          </template>

          <template #role="{ row }">
            <div class="role-tags">
              <t-tag
                v-for="role in resolveRoles(row)"
                :key="role"
                class="role-tag"
                :theme="roleTagTheme(role)"
                variant="light"
                size="small"
                shape="round"
              >
                {{ formatRoleLabel(role) }}
              </t-tag>
            </div>
          </template>

          <template #user_group_name="{ row }">
            <span class="text-muted">{{ row.user_group_name || '未分组' }}</span>
          </template>

          <template #sales_admin_name="{ row }">
            <div v-if="row.sales_admin_name" class="price-cell">
              <span class="user-cell__name user-cell__name--secondary">{{ row.sales_admin_name }}</span>
            </div>
            <!-- 这里只做状态展示：销售归属在用户详情页的「销售归属」面板里完成。
                 原先进位一个「分配」链接跳到销售客户页，那条路径并不真正执行分配
                 （只是把用户名当关键词带过去），点完看起来「没生效」，已移除。 -->
            <span v-else class="text-muted">未归属</span>
          </template>

          <template #user_level_name="{ row }">
            <LevelBadge
              :name="row.user_level_name"
              :icon="row.user_level_icon"
              :color="row.user_level_color"
              :weight="row.user_level_weight"
            />
          </template>

          <template #agent_level_name="{ row }">
            <t-tag v-if="row.agent_level_id" theme="warning" variant="light" size="small" shape="round">
              {{ row.agent_level_name || `#${row.agent_level_id}` }}
            </t-tag>
            <span v-else class="text-muted">非代理</span>
          </template>

          <template #last_login_ip="{ row }">
            <span class="ip-cell__value">{{ row.last_login_ip || '未记录' }}</span>
          </template>

          <template #oauth_provider="{ row }">
            <div class="oauth-cell">
              <t-popup v-for="provider in oauthProviders" :key="provider.key" :content="provider.label" placement="top">
                <span
                  :class="[
                    'oauth-provider-icon',
                    `oauth-provider-icon--${provider.key}`,
                    { 'oauth-provider-icon--inactive': !hasOauthProvider(row, provider.key) },
                  ]"
                >
                  <component :is="provider.icon" size="18" aria-hidden="true" />
                </span>
              </t-popup>
            </div>
          </template>

          <template #status="{ row }">
            <t-tag
              :class="['status-tag', `status-tag--${row.status || 'default'}`]"
              theme="default"
              variant="light"
              size="small"
              shape="round"
            >
              <span class="status-tag__dot" aria-hidden="true" />
              {{ statusLabelMap[row.status] || row.status || '未知' }}
            </t-tag>
          </template>

          <template #deleted_at="{ row }">
            <div class="user-cell">
              <span class="time-text">{{ row.deleted_at ? formatDateTime(row.deleted_at) : '—' }}</span>
              <span v-if="row.delete_reason" class="cell-sub" :title="row.delete_reason">{{ row.delete_reason }}</span>
              <span class="cell-sub">操作人：{{ row.deleted_by_name || '系统/自助' }}</span>
            </div>
          </template>

          <template #balance="{ row }">
            <div class="money-cell money-cell--balance">
              <span class="money" :class="{ 'money--zero': !Number(row.balance) }">
                <span class="money__symbol">¥</span>{{ formatMoney(row.balance) }}
              </span>
            </div>
          </template>

          <template #total_consume_amount="{ row }">
            <div class="money-cell money-cell--consume">
              <span class="money" :class="{ 'money--zero': !Number(row.total_consume_amount) }">
                <span class="money__symbol">¥</span>{{ formatMoney(row.total_consume_amount) }}
              </span>
            </div>
          </template>

          <template #last_login_at="{ row }">
            <span class="time-text" :class="{ 'time-text--muted': !row.last_login_at }">
              {{ row.last_login_at ? formatDateTime(row.last_login_at) : '未登录' }}
            </span>
          </template>

          <template #action="{ row }">
            <div class="action-cell">
              <t-dropdown
                v-if="isMobile"
                trigger="click"
                :options="buildMobileActionOptions(row)"
                @click="(value: string | number | Record<string, unknown>) => handleMobileActionClick(value, row)"
              >
                <t-button
                  theme="default"
                  variant="outline"
                  size="small"
                  shape="square"
                  class="mobile-action-button"
                  aria-label="操作"
                >
                  <template #icon><MoreIcon aria-hidden="true" /></template>
                </t-button>
              </t-dropdown>
              <t-space v-else size="small" class="action-links">
                <template v-if="isRecycleView">
                  <t-link class="action-link action-link--view" hover="color" @click="goUserDetail(row)">详情</t-link>
                  <t-popconfirm content="确认恢复该用户？状态将还原为注销前的状态。" @confirm="restoreUserRow(row)">
                    <t-link class="action-link action-link--restore" hover="color">恢复</t-link>
                  </t-popconfirm>
                </template>
                <template v-else>
                  <t-link class="action-link action-link--view" hover="color" @click="goUserDetail(row)">详情</t-link>
                  <t-link class="action-link action-link--recharge" hover="color" @click="handleRecharge(row)">
                    充值
                  </t-link>
                  <t-link class="action-link action-link--order" hover="color" @click="handleAddOrder(row)">订单</t-link>
                  <t-link class="action-link action-link--login" hover="color" @click="handleImpersonate(row)">
                    登录
                  </t-link>
                  <t-popconfirm
                    :content="row.status === 'active' ? '确认冻结该用户？' : '确认解冻该用户？'"
                    @confirm="toggleStatus(row)"
                  >
                    <t-link
                      :class="[
                        'action-link',
                        row.status === 'active' ? 'action-link--freeze' : 'action-link--restore',
                      ]"
                      hover="color"
                    >
                      {{ row.status === 'active' ? '冻结' : '解冻' }}
                    </t-link>
                  </t-popconfirm>
                  <t-link v-if="canDelete" class="action-link action-link--delete" hover="color" @click="openDeleteDialog(row)">
                    注销
                  </t-link>
                </template>
              </t-space>
            </div>
          </template>

          <template #empty>
            <t-empty :description="isRecycleView ? '回收站暂无已注销用户' : '当前筛选条件下暂无用户数据'" />
          </template>
        </t-table>

        <MobilePagination
          v-if="isMobile"
          :current="mobilePage.current"
          :page-size="mobilePage.pageSize"
          :total="mobilePage.total"
          @go="goMobilePage"
          @page-size="handleMobilePageSizeChange"
        />
      </div>
    </section>

    <t-dialog
      v-model:visible="dialogVisible"
      header="新增用户"
      width="620px"
      :confirm-btn="{ content: '创建用户', theme: 'success', loading: submitting }"
      :on-confirm="handleCreateUser"
      @close="handleDialogClose"
    >
      <t-form ref="formRef" :data="formData" :rules="rules" label-align="top" colonless>
        <div class="form-grid">
          <t-form-item label="用户 ID" name="id">
            <t-input v-model="formData.id" type="number" placeholder="可自定义用户 ID" />
          </t-form-item>

          <t-form-item label="用户名" name="username">
            <t-input v-model="formData.username" placeholder="登录账号，唯一不可重复" maxlength="50" />
          </t-form-item>

          <t-form-item label="邮箱" name="email">
            <t-input v-model="formData.email" placeholder="example@domain.com" />
          </t-form-item>
          <t-form-item label="手机号" name="phone">
            <t-input v-model="formData.phone" placeholder="11位手机号" maxlength="11" />
          </t-form-item>
          <t-form-item label="角色" name="role_ids">
            <t-select
              v-model="formData.role_ids"
              multiple
              clearable
              filterable
              placeholder="请选择角色"
              :options="roleSelectOptions"
            />
          </t-form-item>
          <t-form-item label="用户组" name="user_group_id">
            <t-select
              v-model="formData.user_group_id"
              clearable
              filterable
              placeholder="请选择用户组"
              :options="userGroupSelectOptions"
            />
          </t-form-item>
          <t-form-item label="用户等级" name="user_level_id">
            <t-select
              v-model="formData.user_level_id"
              clearable
              filterable
              placeholder="留空则用起始等级（白银会员）"
              :options="userLevelSelectOptions"
            />
          </t-form-item>
          <t-form-item label="代理等级" name="agent_level_id">
            <t-select
              v-model="formData.agent_level_id"
              clearable
              filterable
              placeholder="留空 = 非代理（无折扣）"
              :options="agentLevelSelectOptions"
            />
          </t-form-item>
          <t-form-item label="初始密码" name="password">
            <t-input v-model="formData.password" type="password" placeholder="建议包含字母与数字，至少8位" />
          </t-form-item>
          <t-form-item label="状态" name="status">
            <t-radio-group v-model="formData.status" variant="default-filled" class="status-radio-group">
              <t-radio-button value="active">正常</t-radio-button>
              <t-radio-button value="disabled">冻结</t-radio-button>
            </t-radio-group>
          </t-form-item>
        </div>
      </t-form>
    </t-dialog>

    <t-dialog
      v-model:visible="rechargeVisible"
      header="人工代充值"
      width="460px"
      :confirm-btn="{ content: '确认充值', theme: 'primary', loading: rechargeSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleRechargeConfirm"
      @close="rechargeVisible = false"
    >
      <t-form label-align="top" :data="rechargeForm" @submit.prevent>
        <t-form-item label="充值用户" name="username">
          <t-input :model-value="rechargeForm.username" disabled />
        </t-form-item>
        <t-form-item label="充值金额（元）" name="amount" :rules="[{ required: true, message: '请输入充值金额' }]">
          <t-input-number
            v-model="rechargeForm.amount"
            :min="0.01"
            :precision="2"
            theme="column"
            placeholder="请输入充值金额"
          />
        </t-form-item>
        <t-form-item label="备注" name="remark">
          <t-textarea
            v-model="rechargeForm.remark"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="选填，记录本次充值说明"
          />
        </t-form-item>
        <p class="form-hint">
          确认后立即到账：生成充值单（财务「充值管理」可查、用户端「我的充值单」可见），
          写入资金流水，并为本次充值单独开一张充值账单（每次充值一张，不并入当期消费账单）。 此操作不可撤销。
        </p>
      </t-form>
    </t-dialog>

    <!-- 注销确认（doc104 §4.4）：先拉 deletion-check 再弹窗。
         blockers 非空 = 硬阻断（在管实例，force 也绕不过），直接禁用确认按钮；
         warnings 非空 = 需勾选「强制注销」才放行。 -->
    <t-dialog
      v-model:visible="deleteVisible"
      header="注销用户"
      width="560px"
      :confirm-btn="{ content: '确认注销', theme: 'danger', loading: deleteSubmitting, disabled: !canConfirmDelete }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleDeleteConfirm"
      @close="deleteVisible = false"
    >
      <t-loading :loading="deleteCheckLoading" size="small">
        <div class="danger-note">
          注销为<b>软删除</b>：用户即刻无法登录，数据在留存期内保留，可由回收站恢复；
          留存期到期后由清理任务<b>彻底删除</b>，届时不可恢复。
        </div>

        <div class="detail-row">
          <span class="detail-row__label">目标用户</span>
          <span class="detail-row__value">{{ deleteTarget?.username }}（ID {{ deleteTarget?.id }}）</span>
        </div>

        <template v-if="deleteCheck">
          <div v-if="deleteCheck.blockers.length" class="check-block check-block--danger">
            <div class="check-block__title">存在阻断项，无法注销</div>
            <ul class="check-list">
              <li v-for="item in deleteCheck.blockers" :key="item.code">
                {{ item.label }}：<b>{{ item.count }}</b> 项 —— 请先释放后再注销
              </li>
            </ul>
          </div>

          <div v-if="deleteCheck.warnings.length" class="check-block check-block--warning">
            <div class="check-block__title">存在未结清事项，需强制注销</div>
            <ul class="check-list">
              <li v-for="item in deleteCheck.warnings" :key="item.code">
                {{ item.label }}：<b>{{ item.count }}</b> 项
              </li>
            </ul>
            <t-checkbox v-model="deleteForce">我已确认上述影响，执行强制注销</t-checkbox>
          </div>

          <div v-if="!deleteCheck.blockers.length && !deleteCheck.warnings.length" class="check-block check-block--ok">
            前置校验通过，无阻断项与未结清事项。
          </div>
        </template>

        <t-form label-align="top" :data="deleteForm" class="delete-form" @submit.prevent>
          <t-form-item
            label="注销原因（必填，写入留痕）"
            name="reason"
            :rules="[{ required: true, message: '请填写注销原因' }]"
          >
            <t-textarea
              v-model="deleteForm.reason"
              :autosize="{ minRows: 2, maxRows: 4 }"
              maxlength="255"
              placeholder="如：用户主动申请注销 / 违规账号处置"
            />
          </t-form-item>
        </t-form>
      </t-loading>
    </t-dialog>

    <!-- 批量注销：逐条走同一套校验，跳过项在结果里逐条回显。 -->
    <t-dialog
      v-model:visible="batchDeleteVisible"
      header="批量注销"
      width="520px"
      :confirm-btn="{ content: `确认注销 ${selectedIds.length} 个用户`, theme: 'danger', loading: batchSubmitting }"
      :cancel-btn="{ content: '取消' }"
      @confirm="handleBatchDeleteConfirm"
      @close="batchDeleteVisible = false"
    >
      <div class="danger-note">
        将逐个注销选中的 <b>{{ selectedIds.length }}</b> 个用户。存在在管实例的用户会被跳过并逐条返回原因；
        其余未结清事项需勾选强制。
      </div>
      <t-checkbox v-model="batchForce" class="batch-force">强制注销（忽略余额/账单/工单/订单警告）</t-checkbox>
      <t-form label-align="top" :data="deleteForm" @submit.prevent>
        <t-form-item label="注销原因（必填）" name="reason" :rules="[{ required: true, message: '请填写注销原因' }]">
          <t-textarea
            v-model="deleteForm.reason"
            :autosize="{ minRows: 2, maxRows: 4 }"
            maxlength="255"
            placeholder="批量注销的统一原因"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 留存期清理（doc104 §4.6）：先 dry_run 预览再执行，且执行按钮需二次确认。 -->
    <t-dialog
      v-model:visible="purgeVisible"
      header="留存期清理（硬删除）"
      width="680px"
      :footer="false"
      @close="purgeVisible = false"
    >
      <t-alert theme="error" class="purge-alert">
        本操作会<b>彻底删除</b>留存期已过的已注销用户及其全部个人数据，<b>不可恢复</b>。 默认先预览，确认无误后再执行。
      </t-alert>

      <div v-if="purgeResult" class="purge-meta">
        <span
          >留存期：<b>{{ purgeResult.retention_days }}</b> 天</span
        >
        <span>截止时刻：{{ formatDateTime(purgeResult.cutoff) }}</span>
        <span
          >候选：<b>{{ purgeResult.candidates.length }}</b> 个{{
            purgeResult.has_more ? '（本轮取满上限，仍有积压）' : ''
          }}</span
        >
      </div>

      <t-table
        v-if="purgeResult?.candidates.length"
        :data="purgeResult.candidates"
        :columns="purgeColumns"
        row-key="id"
        size="small"
        max-height="320"
      />
      <t-empty v-else-if="purgeResult" description="没有留存期已过的用户，无需清理" />

      <div class="purge-actions">
        <t-button variant="outline" :loading="purgeLoading" @click="runPurge(true)">重新预览</t-button>
        <t-button
          theme="danger"
          :loading="purgeLoading"
          :disabled="!purgeResult?.candidates.length"
          @click="handlePurgeExecute"
        >
          执行清理
        </t-button>
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import {
  AddIcon,
  ArrowDownIcon,
  ArrowUpIcon,
  DeleteIcon,
  ErrorCircleIcon,
  LogoAlipayFilledIcon,
  LogoQqIcon,
  LogoWechatStrokeIcon,
  MoreIcon,
  RefreshIcon,
  SearchIcon,
  UserIcon,
} from 'tdesign-icons-vue-next'
import {
  DialogPlugin,
  MessagePlugin,
  type FormInstanceFunctions,
  type FormRule,
  type PageInfo,
  type PrimaryTableCol,
} from 'tdesign-vue-next'

import {
  batchDeleteUsers,
  batchRestoreUsers,
  createUser,
  deleteUser,
  getRoleList,
  getUserDeletionCheck,
  getUserGroupList,
  getUserLevelList,
  getUserList,
  impersonateUser,
  purgeUsers,
  rechargeUser,
  restoreUser,
  updateUserStatus,
  type RoleInfo,
  type UserCreateRequest,
  type UserDeletionCheckResponse,
  type UserGroupInfo,
  type UserInfo,
  type UserLevelInfo,
  type UserListQuery,
  type UserPurgePreviewItem,
  type UserPurgeResponse,
} from '@/api/user'
import { getAgentLevelList, type AgentLevelInfo } from '@/api/agent-level'
import { getSalesCandidates } from '@/api/sales'
import FilterCard from '@/components/filter-card/index.vue'
import MobilePagination from '@/components/mobile-pagination/index.vue'
import LevelBadge from '@/components/level-badge/index.vue'
import { useUserStore } from '@/store'
import type { SalesCandidateInfo } from '@/types/interface'
import { copyText } from '@/utils/clipboard'
import { USER_CONSOLE_URL } from '@/utils/config'

defineOptions({ name: 'UserAccountsList' })

type SortOrder = 'asc' | 'desc'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

// 权限门控（doc104 §3.3 F18）：此前本页零门控，任何能进用户列表的角色都能看到
// 新增/注销等入口。v-permission 只做体验优化，真正的越权防护在后端。
const canCreate = computed(() => userStore.hasPermission('user:create'))
const canUpdate = computed(() => userStore.hasPermission('user:update_status'))
const canDelete = computed(() => userStore.hasPermission('user:delete'))
const canRestore = computed(() => userStore.hasPermission('user:restore'))
// 硬删除不可逆，后端用 superOnly，前端同样只对超管展示入口。
const canPurge = computed(() => userStore.isSuperAdmin)

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<UserInfo[]>([])
const roleOptions = ref<RoleInfo[]>([])
const userGroupOptions = ref<UserGroupInfo[]>([])
const userLevelOptions = ref<UserLevelInfo[]>([])
const agentLevelOptions = ref<AgentLevelInfo[]>([])
const salesOptions = ref<SalesCandidateInfo[]>([])
const sortOrder = ref<SortOrder>('desc')
const isMobile = ref(false)
// 移动端批量模式：默认 false = 每行一个「⋮」操作按钮，勾选列不渲染。
// 打开后勾选列接管操作列的位置（贴右固定），顶部出现批量动作条。
// 桌面端不使用该开关，勾选列始终可见（宽屏放得下）。
const batchMode = ref(false)
const tableDragRef = ref<HTMLElement | null>(null)
const activeTableScrollRef = ref<HTMLElement | null>(null)
const isTableDragging = ref(false)
const dragState = {
  startX: 0,
  startScrollLeft: 0,
}

// 视图模式（doc104 §4.3）：'active' = 常规列表（后端默认排除已注销）；
// 'recycle' = 回收站，落到 filter=deleted。用 URL 的 filter 作为唯一真相，
// 这样刷新、前进后退、分享链接都能还原同一个视图。
const currentView = ref<'active' | 'recycle'>('active')
const isRecycleView = computed(() => currentView.value === 'recycle')

// 行选择（批量操作，doc104 §3.3 F19）
const selectedIds = ref<number[]>([])
const batchSubmitting = ref(false)

// 注销（软删除）
const deleteVisible = ref(false)
const deleteCheckLoading = ref(false)
const deleteSubmitting = ref(false)
const deleteTarget = ref<UserInfo | null>(null)
const deleteCheck = ref<UserDeletionCheckResponse | null>(null)
const deleteForce = ref(false)
const deleteForm = reactive<{ reason: string }>({ reason: '' })

const batchDeleteVisible = ref(false)
const batchForce = ref(false)

// 留存期清理（硬删除）
const purgeVisible = ref(false)
const purgeLoading = ref(false)
const purgeResult = ref<UserPurgeResponse | null>(null)

const purgeColumns: PrimaryTableCol<UserPurgePreviewItem>[] = [
  { colKey: 'id', title: 'ID', width: 90 },
  { colKey: 'username', title: '用户名', minWidth: 160 },
  { colKey: 'deleted_at', title: '注销时间', width: 180 },
  { colKey: 'reason', title: '注销原因', minWidth: 200 },
]

// 阻断项存在时确认按钮必须禁用 —— 否则用户点下去只会收到一个 409。
const canConfirmDelete = computed(() => !!deleteCheck.value && deleteCheck.value.blockers.length === 0)

const filters = reactive<UserListQuery>({
  page: 1,
  page_size: 10,
  status: '',
  filter: '',
  keyword: '',
  user_level_id: undefined,
  user_group_id: undefined,
  agent_level_id: undefined,
  is_agent: '',
  is_sub_account: '',
  sales_admin_id: undefined,
  unassigned_sales: '',
  include_deleted: false,
})

const pagination = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
  showJumper: true,
  showPageSize: true,
  pageSizeOptions: [10, 20, 50, 100],
})

// 移动端分页状态：与桌面端 pagination 同步维护（见 loadUsers）
const mobilePage = reactive({
  current: 1,
  pageSize: 10,
  total: 0,
})

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstanceFunctions | null>(null)
const formData = reactive<UserCreateRequest>({
  id: undefined,
  username: '',
  email: '',
  phone: '',
  password: '',
  status: 'active',
  role_ids: [],
  user_group_id: undefined,
  user_level_id: undefined,
  agent_level_id: undefined,
})

const rules: Record<string, FormRule[]> = {
  id: [
    {
      validator: (value) => !value || (Number.isInteger(Number(value)) && Number(value) > 0),
      message: '用户ID需为正整数',
      type: 'error',
    },
  ],
  username: [
    { required: true, message: '请输入用户名', type: 'error', trigger: 'blur' },
    { min: 3, message: '用户名至少 3 个字符', type: 'error', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', type: 'error', trigger: 'blur' },
    { email: true, message: '请输入正确的邮箱格式', type: 'error', trigger: 'blur' },
  ],
  phone: [
    { required: true, message: '请输入手机号', type: 'error', trigger: 'blur' },
    { pattern: /^\d{11}$/, message: '手机号需为11位数字', type: 'error', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入初始密码', type: 'error', trigger: 'blur' },
    { min: 8, message: '密码至少需要 8 位', type: 'error', trigger: 'blur' },
  ],
  status: [{ required: true, message: '请选择状态', type: 'error', trigger: 'change' }],
}

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '正常', value: 'active' },
  { label: '冻结', value: 'disabled' },
  { label: '待审核', value: 'pending' },
  { label: '已注销', value: 'cancelled' },
]

const quickFilterOptions = [
  { label: '今日新增', value: 'today' },
  { label: '待实名', value: 'pending_real_name' },
  { label: '已购用户', value: 'purchased' },
  // 未归属销售（doc86 §4.1.10）：对应后端 unassigned_sales=1
  { label: '未归属销售', value: 'unassigned_sales' },
]

const statusLabelMap: Record<string, string> = {
  active: '正常',
  disabled: '冻结',
  pending: '待审核',
  cancelled: '已注销',
}

// 兜底角色名映射（doc104 §3.3，F22）：原表里 agent/member/finance/operator/guest
// 在 roles 表里根本不存在，而真实存在的 ops_admin/finance_admin/sales_manager/
// support_lead/tech 反而没有 —— 这些角色的用户列表里显示的是原始 code。
// 现在按 roles 表实际取值补齐；运行时优先用接口返回的 roles.name，
// 这张表只在接口未加载或角色已被删除时兜底。
const roleLabelMap: Record<string, string> = {
  admin: '管理员',
  super_admin: '超级管理员',
  ops_admin: '运维管理员',
  finance_admin: '财务管理员',
  user: '普通用户',
  sales: '销售',
  sales_manager: '销售主管',
  support: '客服',
  support_lead: '客服主管',
  tech: '技术',
  unassigned: '未分配',
}

// 只列系统真正实现的三家（微信 / QQ / 支付宝），与「系统配置 → 第三方登录」
// 的渠道列表和后端 pkg/oauth/provider 一一对应。此前还挂着 GitHub / Apple /
// Android 三个静态图标：它们既没有后端 provider，也没有绑定数据的可能，恒定
// 渲染成灰色，纯属误导；同时漏了已实现的支付宝。
const oauthProviders = [
  { key: 'wechat', label: '微信', icon: LogoWechatStrokeIcon },
  { key: 'qq', label: 'QQ', icon: LogoQqIcon },
  { key: 'alipay', label: '支付宝', icon: LogoAlipayFilledIcon },
] as const

const levelOptions = computed(() => [
  { label: '全部等级', value: undefined },
  ...userLevelOptions.value
    .filter((item) => item.status !== 'disabled')
    .map((item) => ({ label: item.name, value: item.id })),
])

// 代理身份筛选：三态。is_agent 与 agent_level_id 同时给时以后端为准（AND）。
const agentFilterOptions = [
  { label: '全部（含非代理）', value: '' },
  { label: '仅代理', value: 'true' },
  { label: '仅非代理', value: 'false' },
]

const agentLevelFilterOptions = computed(() => [
  { label: '全部代理等级', value: undefined },
  ...agentLevelOptions.value.map((item) => ({
    label: item.status === 'disabled' ? `${item.name}（已停用）` : item.name,
    value: item.id,
  })),
])

// 账号类型筛选（P4-10）：主账号 / 子账号
const accountTypeOptions = [
  { label: '主账号', value: 'false' },
  { label: '子账号', value: 'true' },
]

const roleSelectOptions = computed(() =>
  roleOptions.value
    .filter((item) => item.status !== 'disabled')
    .map((item) => ({ label: item.name || formatRoleLabel(item.code), value: item.id })),
)

const userGroupSelectOptions = computed(() =>
  userGroupOptions.value
    .filter((item) => item.status !== 'disabled')
    .map((item) => ({ label: item.name, value: item.id })),
)

// 建号时的等级下拉：只列启用中的等级，留空则由后端给起始等级。
const userLevelSelectOptions = computed(() =>
  userLevelOptions.value
    .filter((item) => item.status === 'active')
    .map((item) => ({
      label: item.upgrade_threshold > 0 ? `${item.name}（消费满 ${item.upgrade_threshold}）` : item.name,
      value: item.id,
    })),
)

// 建号时的代理等级下拉：只列启用中的等级（停用等级后端会拒绝指派）。
const agentLevelSelectOptions = computed(() =>
  agentLevelOptions.value
    .filter((item) => item.status === 'active')
    .map((item) => ({ label: item.name, value: item.id })),
)

// 用户组筛选：包含已禁用组（便于排查历史归属），并标注默认组
const userGroupFilterOptions = computed(() => [
  { label: '全部用户组', value: undefined },
  ...userGroupOptions.value.map((item) => {
    const suffix: string[] = []
    if (item.is_default) suffix.push('默认')
    if (item.status === 'disabled') suffix.push('已禁用')
    const label = suffix.length > 0 ? `${item.name}（${suffix.join('·')}）` : item.name
    return { label, value: item.id }
  }),
])

// 归属销售筛选（doc86 §4.1.10）：数据源为在职且开启销售能力的员工
const salesFilterOptions = computed(() => [
  { label: '全部销售', value: undefined },
  ...salesOptions.value.map((item) => ({
    label: `${item.real_name || item.username}${item.department_name ? `（${item.department_name}）` : ''}`,
    value: item.admin_id,
  })),
  { label: '未归属', value: 0 },
])

const activeFilterLabel = computed(() => {
  if (isRecycleView.value) return '回收站'
  if (filters.filter === 'today') return '今日新增'
  if (filters.filter === 'pending_real_name') return '待实名认证'
  if (filters.filter === 'purchased') return '已购用户'
  if (filters.filter === 'unassigned_sales') return '未归属销售'
  if (filters.status) return statusLabelMap[filters.status] || filters.status
  if (filters.user_group_id) {
    const group = userGroupOptions.value.find((item) => item.id === filters.user_group_id)
    return `用户组: ${group?.name || filters.user_group_id}`
  }
  if (filters.is_agent === 'true') return '仅代理'
  if (filters.is_agent === 'false') return '仅非代理'
  if (filters.agent_level_id) {
    const level = agentLevelOptions.value.find((item) => item.id === filters.agent_level_id)
    return `代理等级: ${level?.name || filters.agent_level_id}`
  }
  if (filters.sales_admin_id) {
    const sales = salesOptions.value.find((item) => item.admin_id === filters.sales_admin_id)
    return `归属销售: ${sales?.real_name || sales?.username || filters.sales_admin_id}`
  }
  if (filters.keyword) return `搜索: ${filters.keyword}`
  return ''
})

const sortedTableData = computed(() => tableData.value)

// 是否具备批量操作权限（回收站=恢复，列表=冻结/注销）。移动端据此决定
// 「批量操作」开关是否出现。
const canBatchSelect = computed(() => (isRecycleView.value ? canRestore.value : canUpdate.value || canDelete.value))

// 移动端批量模式是否真的生效。三个条件缺一不可，且必须由这一个 computed 派生：
// 列配置里若各写各的判断，一旦 batchMode 残留（例如在批量模式下切到回收站、
// 而该角色没有恢复权限），勾选列和操作列会同时被判定为「不该出现」，
// 表格会丢掉整列操作入口 —— 用同一个来源就不会出现这种自相矛盾的组合。
const mobileBatchActive = computed(() => isMobile.value && batchMode.value && canBatchSelect.value)

const columns = computed<PrimaryTableCol<UserInfo>[]>(() => {
  const base: PrimaryTableCol<UserInfo>[] = []
  // 移动端批量模式 = 勾选列顶替操作列的位置（同一格、贴右固定），
  // 默认态则是每行一个「⋮」按钮。两者不在同一帧出现，所以窄屏始终只有一个
  // 右侧固定列，信息列能留住宽度。移动端与桌面端的取值方向不同，所以分开判断。
  const desktopSelect = !isMobile.value && canBatchSelect.value
  if (mobileBatchActive.value) {
    // 窄屏：勾选列放在右侧，与默认态的操作列同位置，选中后不再遮挡账号/金额。
    base.push({ colKey: 'row-select', type: 'multiple', width: 70, fixed: 'right' as const, align: 'center' as const })
  } else if (desktopSelect) {
    // 宽屏放得下：勾选列沿用左固定，批量操作随时可用，不需要额外开关。
    base.push({ colKey: 'row-select', type: 'multiple', width: 46, fixed: 'left' as const })
  }
  base.push(
    {
      colKey: 'id',
      // 表头排序符号：点一下在倒序/正序间切换。
      // 用 title 渲染函数而不是模板插槽 —— 原来写的 #title-id 不是 TDesign 的
      // 表头插槽名，插槽内容从未渲染（实测 DOM 里 .id-sort-button 数量为 0），
      // 表头一直只是一个纯文本「ID」。
      title: () => {
        const order = sortOrder.value
        return h(
          'button',
          {
            type: 'button',
            class: 'id-sort-button',
            'aria-label': order === 'asc' ? '按 ID 正序，点击切换为倒序' : '按 ID 倒序，点击切换为正序',
            title: order === 'asc' ? '当前正序，点击切换为倒序' : '当前倒序，点击切换为正序',
            onClick: () => void toggleIdSort(),
          },
          [
            h('span', { class: 'id-sort-button__label' }, 'ID'),
            h(ArrowUpIcon, { size: '14', class: ['id-sort-button__icon', { 'is-asc': order === 'asc' }] }),
          ],
        )
      },
      width: 92,
    },
    { colKey: 'username', title: '账号信息', minWidth: 260 },
    { colKey: 'real_name', title: '实名信息', minWidth: 220 },
    { colKey: 'balance', title: '账户余额', width: 130, align: 'right' as const },
    { colKey: 'total_consume_amount', title: '总消费金额', width: 150, align: 'right' as const },
    { colKey: 'role', title: '角色', minWidth: 180 },
    { colKey: 'user_level_name', title: '用户等级', width: 130 },
    { colKey: 'user_group_name', title: '用户组', minWidth: 180 },
    { colKey: 'agent_level_name', title: '代理等级', width: 130 },
    { colKey: 'sales_admin_name', title: '归属销售', minWidth: 150 },
    { colKey: 'last_login_ip', title: '登录 IP', minWidth: 220 },
    { colKey: 'oauth_provider', title: '第三方登录', minWidth: 180 },
    { colKey: 'status', title: '状态', width: 110 },
  )
  if (isRecycleView.value) {
    // 回收站独有：注销时间 + 原因 + 操作人。常规列表不展示（全是「—」没意义）。
    base.push({ colKey: 'deleted_at', title: '注销信息', minWidth: 260 })
  }
  base.push(
    { colKey: 'last_login_at', title: '最近登录', width: 180 },
  )
  // 操作列在移动端批量模式下不渲染：该位置已由上面的右固定勾选列占用。
  // 用同一个 mobileBatchActive 判断，保证「勾选列」与「操作列」恰好二选一。
  if (!mobileBatchActive.value) {
    base.push({
      colKey: 'action',
      title: '操作',
      width: isMobile.value ? 70 : 260,
      fixed: 'right' as const,
      align: 'center' as const,
    })
  }
  return base
})
function syncFiltersFromRoute() {
  const query = route.query as Record<string, string | undefined>
  filters.page = toPositiveInt(query.page, 1)
  filters.page_size = toPositiveInt(query.page_size, 10)
  filters.status = query.status || ''
  filters.filter = query.filter || ''
  filters.keyword = query.keyword || ''
  // 排序方向从 URL 还原，只认 asc；其余（含缺失、非法值）一律 desc。
  sortOrder.value = query.sort === 'asc' ? 'asc' : 'desc'
  filters.user_level_id = query.user_level_id ? Number(query.user_level_id) : undefined
  filters.user_group_id = query.user_group_id ? Number(query.user_group_id) : undefined
  filters.agent_level_id = query.agent_level_id ? Number(query.agent_level_id) : undefined
  filters.is_agent = query.is_agent === 'true' || query.is_agent === 'false' ? query.is_agent : ''
  filters.is_sub_account =
    query.is_sub_account === 'true' || query.is_sub_account === 'false' ? query.is_sub_account : ''
  filters.sales_admin_id = query.sales_admin_id ? Number(query.sales_admin_id) : undefined
  filters.unassigned_sales = query.unassigned_sales === 'true' ? 'true' : ''
  filters.include_deleted = query.include_deleted === 'true'
  currentView.value = filters.filter === 'deleted' ? 'recycle' : 'active'
  // 换视图后旧选择已不适用（行对象都换了），必须清空。
  selectedIds.value = []
  pagination.current = filters.page
  pagination.pageSize = filters.page_size
}

function toPositiveInt(value: string | undefined, fallback: number) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : fallback
}

function hasOauthProvider(row: UserInfo, provider: string) {
  if (Array.isArray(row.oauth_providers) && row.oauth_providers.length > 0) {
    return row.oauth_providers.includes(provider)
  }
  return row.oauth_provider === provider
}

function buildQuery() {
  const query: Record<string, string> = {}
  if (filters.page && filters.page !== 1) query.page = String(filters.page)
  if (filters.page_size && filters.page_size !== 10) query.page_size = String(filters.page_size)
  if (filters.status) query.status = filters.status
  // 回收站视图固定写 filter=deleted（而不是把 '' 也写进去），保证 URL 可分享可还原。
  if (isRecycleView.value) {
    query.filter = 'deleted'
  } else if (filters.filter) {
    query.filter = filters.filter
  }
  if (filters.include_deleted) query.include_deleted = 'true'
  if (filters.keyword) query.keyword = filters.keyword
  // 只写非默认值，URL 保持干净；desc 是默认，不写。
  if (sortOrder.value === 'asc') query.sort = 'asc'
  if (filters.user_level_id) query.user_level_id = String(filters.user_level_id)
  if (filters.user_group_id) query.user_group_id = String(filters.user_group_id)
  if (filters.agent_level_id) query.agent_level_id = String(filters.agent_level_id)
  if (filters.is_agent) query.is_agent = filters.is_agent
  if (filters.is_sub_account) query.is_sub_account = filters.is_sub_account
  // 「归属销售=未归属(0)」与快捷筛选「未归属销售」统一收敛为 unassigned_sales=true
  if (filters.filter === 'unassigned_sales' || filters.sales_admin_id === 0) {
    query.unassigned_sales = 'true'
  } else if (filters.sales_admin_id) {
    query.sales_admin_id = String(filters.sales_admin_id)
  }
  return query
}

// 视图切换：清掉与另一视图冲突的筛选（回收站里的 status=cancelled 与快捷筛选
// 都没有意义），再走统一的路由驱动重载。
async function handleViewChange(value: string | number | boolean) {
  const next = value === 'recycle' ? 'recycle' : 'active'
  currentView.value = next
  filters.page = 1
  filters.status = ''
  filters.filter = next === 'recycle' ? 'deleted' : ''
  filters.include_deleted = false
  selectedIds.value = []
  // 两个视图的批量权限不同（列表=冻结/注销，回收站=恢复），切过去后
  // 原选择已无意义，批量模式也一并退出，避免勾选列在无权限时被留下。
  batchMode.value = false
  pagination.current = 1
  await replaceRouteQuery()
}

// 切换 ID 升降序：改的是后端 ORDER BY 的方向，不只是重排当前页 ——
// 否则翻页后顺序又乱回去。排序列固定在 ID 上，不接受列名参数。
// 顺序变化后当前选择不再对应同一批可见行，清空，避免误操作到已翻走的用户。
async function toggleIdSort() {
  sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  filters.page = 1
  pagination.current = 1
  selectedIds.value = []
  await replaceRouteQuery()
}

// 移动端进入/退出批量模式。退出时清空已选：勾选列会从 DOM 消失，
// 残留的 selectedIds 会让顶部「已选 N 个」和批量按钮继续显示在一个
// 已经看不见勾选框的界面上，用户没法取消选择。
function toggleBatchMode() {
  batchMode.value = !batchMode.value
  if (!batchMode.value) selectedIds.value = []
}

function syncViewportState() {
  if (typeof window === 'undefined') return
  isMobile.value = window.innerWidth < 768
}

async function replaceRouteQuery() {
  await router.replace({ query: buildQuery() })
}

async function loadRoleOptions() {
  try {
    // 客户账号只能挂客户角色（roles.scope=user）。改造前这里拿的是后台角色列表，
    // 「新建用户」的角色下拉会列出 超级管理员/运维 等员工角色，建出来的客户号
    // 挂着员工角色（实测 users.id=12 即如此）。后端现在也会拒绝跨域绑定。
    roleOptions.value = await getRoleList('user')
  } catch {
    roleOptions.value = []
  }
}

async function loadUserGroupOptions() {
  try {
    // 不限定 status：筛选项需要覆盖已禁用组，创建弹窗再按 status 过滤
    const data = await getUserGroupList({ page: 1, page_size: 200 })
    userGroupOptions.value = data.items || []
  } catch {
    userGroupOptions.value = []
  }
}

async function loadUserLevelOptions() {
  try {
    const data = await getUserLevelList({ page: 1, page_size: 200, status: 'active' })
    userLevelOptions.value = data.items || []
  } catch {
    userLevelOptions.value = []
  }
}

// 代理等级筛选/建号下拉：要覆盖已停用等级，否则历史代理的等级名会显示成裸 ID。
async function loadAgentLevelOptions() {
  try {
    const data = await getAgentLevelList({ page: 1, page_size: 200 })
    agentLevelOptions.value = data.items || []
  } catch {
    agentLevelOptions.value = []
  }
}

// buildListParams 把筛选状态翻译成列表接口的查询参数。
function buildListParams(): UserListQuery {
  // 「未归属销售」既可由快捷筛选触发，也可由「归属销售=未归属(0)」触发。
  const onlyUnassigned = filters.filter === 'unassigned_sales' || filters.sales_admin_id === 0
  return {
    page: filters.page,
    page_size: filters.page_size,
    status: filters.status || undefined,
    filter: filters.filter && filters.filter !== 'unassigned_sales' ? filters.filter : undefined,
    keyword: filters.keyword || undefined,
    user_level_id: filters.user_level_id || undefined,
    user_group_id: filters.user_group_id || undefined,
    agent_level_id: filters.agent_level_id || undefined,
    is_agent: filters.is_agent || undefined,
    is_sub_account: filters.is_sub_account || undefined,
    sales_admin_id: !onlyUnassigned && filters.sales_admin_id ? filters.sales_admin_id : undefined,
    unassigned_sales: onlyUnassigned ? 'true' : undefined,
    include_deleted: filters.include_deleted || undefined,
    // 排序由后端执行：只排当前页等于「这一页内看起来是排好的」，
    // 翻到下一页顺序又乱回去，是比不排更坏的误导。
    sort: sortOrder.value,
  }
}

async function loadUsers() {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await getUserList(buildListParams())
    tableData.value = data.items || []
    pagination.current = data.meta.page
    pagination.pageSize = data.meta.page_size
    pagination.total = data.meta.total
    mobilePage.total = data.meta.total
    mobilePage.current = data.meta.page
    mobilePage.pageSize = data.meta.page_size
    mobilePage.total = data.meta.total
    filters.page = data.meta.page
    filters.page_size = data.meta.page_size
  } catch (error) {
    tableData.value = []
    pagination.total = 0
    mobilePage.total = 0
    mobilePage.total = 0
    errorMessage.value = (error as Error)?.message || '加载用户列表失败'
  } finally {
    loading.value = false
  }
}

async function loadSalesOptions() {
  try {
    const data = await getSalesCandidates({})
    salesOptions.value = data.items || []
  } catch {
    salesOptions.value = []
  }
}

async function loadAll() {
  await Promise.all([
    loadRoleOptions(),
    loadUserGroupOptions(),
    loadUserLevelOptions(),
    loadAgentLevelOptions(),
    loadSalesOptions(),
    loadUsers(),
  ])
}

async function handleSearch() {
  filters.page = 1
  pagination.current = 1
  await replaceRouteQuery()
}

async function handleReset() {
  filters.page = 1
  filters.page_size = 10
  filters.status = ''
  // 重置保留当前视图：在回收站点「重置」应回到「回收站第一页无筛选」，
  // 而不是把用户弹回常规列表。
  filters.filter = isRecycleView.value ? 'deleted' : ''
  filters.keyword = ''
  filters.user_level_id = undefined
  filters.user_group_id = undefined
  filters.agent_level_id = undefined
  filters.is_agent = ''
  filters.is_sub_account = ''
  filters.sales_admin_id = undefined
  filters.unassigned_sales = ''
  filters.include_deleted = false
  selectedIds.value = []
  sortOrder.value = 'desc'
  pagination.current = 1
  pagination.pageSize = 10
  await replaceRouteQuery()
}

async function reload() {
  await loadAll()
}

async function handlePageChange(pageInfo: PageInfo) {
  filters.page = pageInfo.current
  filters.page_size = pageInfo.pageSize
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  await replaceRouteQuery()
}

// —— 移动端分页交互 ——
function goMobilePage(target: number) {
  const clamped = Math.min(Math.max(target, 1), Math.max(1, Math.ceil(mobilePage.total / mobilePage.pageSize)))
  if (clamped === mobilePage.current) return
  void applyMobilePage(clamped, mobilePage.pageSize)
}

async function applyMobilePage(current: number, pageSize: number) {
  filters.page = current
  filters.page_size = pageSize
  pagination.current = current
  pagination.pageSize = pageSize
  await replaceRouteQuery()
}

function handleMobilePageSizeChange(pageSize: number) {
  mobilePage.pageSize = pageSize
  void applyMobilePage(1, pageSize)
}

function resolveRoles(row: UserInfo) {
  if (Array.isArray(row.roles) && row.roles.length) return row.roles
  if (row.role) return [row.role]
  return ['unassigned']
}

// 角色标签按语义区分颜色：超管红、管理类橙、普通绿、其它灰
function roleTagTheme(role: string): 'danger' | 'warning' | 'primary' | 'default' {
  const code = String(role || '')
    .trim()
    .toLowerCase()
  if (code.includes('super') || code.includes('超管')) return 'danger'
  if (code.includes('admin') || code.includes('管理员') || code.includes('运营') || code.includes(' ops'))
    return 'warning'
  if (code === 'unassigned' || !code) return 'default'
  return 'primary'
}

function formatRoleLabel(role: string) {
  const normalizedRole = String(role || '')
    .trim()
    .toLowerCase()
  if (!normalizedRole) return '未分配'
  // 接口返回的角色中文名是权威来源；静态表只兜底未加载 / 角色已删除的情况。
  const fromApi = roleOptions.value.find((item) => item.code?.toLowerCase() === normalizedRole)
  if (fromApi?.name) return fromApi.name
  return roleLabelMap[normalizedRole] || role || '未分配'
}

function formatMoney(value: number) {
  return new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(value || 0))
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    hour12: false,
  })
}

function goUserDetail(row: UserInfo) {
  router.push({
    path: '/users/accounts/detail',
    query: { id: String(row.id) },
  })
}

function handleRecharge(row: UserInfo) {
  rechargeForm.user_id = row.id
  rechargeForm.username = row.username
  rechargeForm.amount = 0
  rechargeForm.remark = ''
  rechargeVisible.value = true
}

// ===== 添加订单 =====
// 选购细节（商品/规格/周期/配置项/自定义价）在独立的代下单页面完成，
// 列表页只负责把「给谁下单」带过去。原弹窗内容量过大，已改为整页（见 order-create 页）。
function handleAddOrder(row: UserInfo) {
  router.push({
    path: '/users/accounts/order-create',
    query: {
      id: String(row.id),
      name: row.username,
      from: '/users/accounts/list',
    },
  })
}

function buildMobileActionOptions(row: UserInfo) {
  if (isRecycleView.value) {
    return [{ content: '详情', value: 'detail' }, ...(canRestore.value ? [{ content: '恢复', value: 'restore' }] : [])]
  }
  // 与桌面端操作列同口径：常规动作用默认色，状态变更用 warning，破坏性动作用 error。
  return [
    { content: '详情', value: 'detail' },
    { content: '充值', value: 'recharge' },
    { content: '订单', value: 'order' },
    { content: '登录', value: 'impersonate', disabled: row.status !== 'active' },
    { content: row.status === 'active' ? '冻结' : '解冻', value: 'toggle-status', theme: 'warning' },
    ...(canDelete.value ? [{ content: '注销', value: 'delete', theme: 'error' }] : []),
  ]
}

function handleMobileActionClick(
  data: string | number | Record<string, unknown> | { value?: string | number },
  row: UserInfo,
) {
  const value =
    typeof data === 'string' || typeof data === 'number'
      ? String(data)
      : String((data as { value?: string | number })?.value ?? '')
  if (value === 'detail') {
    goUserDetail(row)
    return
  }
  if (value === 'recharge') {
    handleRecharge(row)
    return
  }
  if (value === 'order') {
    handleAddOrder(row)
    return
  }
  if (value === 'impersonate') {
    void handleImpersonate(row)
    return
  }
  if (value === 'toggle-status') {
    void toggleStatus(row)
    return
  }
  if (value === 'restore') {
    void restoreUserRow(row)
    return
  }
  if (value === 'delete') {
    void openDeleteDialog(row)
  }
}

async function handleImpersonate(row: UserInfo) {
  if (row.status !== 'active') {
    MessagePlugin.warning('仅可代登录正常状态用户')
    return
  }
  try {
    const res = await impersonateUser({ user_id: row.id })
    // 代登录：新窗口打开用户端并携带 user token，绝不在管理端写入用户登录态。
    // 用户端地址统一由 utils/config 解析（VITE_USER_BASE_URL），不在页面里写兜底端口。
    const url = `${USER_CONSOLE_URL}/?token=${encodeURIComponent(res.token)}`
    window.open(url, '_blank')
    MessagePlugin.success(`已在用户端窗口代为登录 ${row.username}`)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '代登录失败')
  }
}

function resolveTableScrollContainer() {
  const root = tableDragRef.value
  if (!root) return null

  const candidates = root.querySelectorAll<HTMLElement>(
    '.t-table__content, .t-table__body, .t-table, .t-table__inner, .t-table__header',
  )
  for (const candidate of candidates) {
    if (candidate.scrollWidth > candidate.clientWidth) {
      return candidate
    }
  }

  return root.scrollWidth > root.clientWidth ? root : null
}

function handleTableDragStart(event: MouseEvent) {
  if (event.button !== 0) return
  const target = event.target as HTMLElement | null
  if (target?.closest('a,button,input,textarea,[role="button"],.t-link,.t-button,.t-input,.t-tag,.t-popup')) return
  const container = resolveTableScrollContainer()
  if (!container) return
  activeTableScrollRef.value = container
  isTableDragging.value = true
  dragState.startX = event.clientX
  dragState.startScrollLeft = container.scrollLeft
  document.body.classList.add('user-table-dragging')
  window.addEventListener('mousemove', handleTableDragging)
  window.addEventListener('mouseup', handleTableDragEnd)
  window.addEventListener('mouseleave', handleTableDragEnd)
  event.preventDefault()
}

function handleTableDragging(event: MouseEvent) {
  if (!isTableDragging.value || !activeTableScrollRef.value) return
  const deltaX = event.clientX - dragState.startX
  activeTableScrollRef.value.scrollLeft = dragState.startScrollLeft - deltaX
  event.preventDefault()
}

function handleTableDragEnd() {
  activeTableScrollRef.value = null
  if (!isTableDragging.value) return
  isTableDragging.value = false
  document.body.classList.remove('user-table-dragging')
  window.removeEventListener('mousemove', handleTableDragging)
  window.removeEventListener('mouseup', handleTableDragEnd)
  window.removeEventListener('mouseleave', handleTableDragEnd)
}

function initFormData(): UserCreateRequest {
  return {
    id: undefined,
    username: '',
    email: '',
    phone: '',
    password: '',
    status: 'active',
    role_ids: [],
    user_group_id: undefined,
    user_level_id: undefined,
    agent_level_id: undefined,
  }
}

// ===== 行选择与批量操作（doc104 §3.3 F19）=====

function handleSelectChange(keys: Array<string | number>) {
  selectedIds.value = keys.map((key) => Number(key)).filter((id) => Number.isFinite(id) && id > 0)
}

function clearSelection() {
  selectedIds.value = []
}

// 批量冻结/解冻：后端没有批量状态接口，逐个调 PATCH /users/:id/status。
// 用 Promise.allSettled 而不是 all —— 一条失败不该让其余回滚（它们已经生效了）。
async function handleBatchStatus(status: 'active' | 'disabled') {
  if (!selectedIds.value.length) return
  const label = status === 'active' ? '解冻' : '冻结'
  batchSubmitting.value = true
  try {
    const results = await Promise.allSettled(selectedIds.value.map((id) => updateUserStatus(id, { status })))
    const ok = results.filter((item) => item.status === 'fulfilled').length
    const failed = results.length - ok
    if (failed === 0) {
      MessagePlugin.success(`已${label} ${ok} 个用户`)
    } else {
      MessagePlugin.warning(`已${label} ${ok} 个，${failed} 个失败`)
    }
    selectedIds.value = []
    await loadUsers()
  } finally {
    batchSubmitting.value = false
  }
}

// ===== 注销（软删除）=====

// 打开注销确认框：先拉前置校验，再决定能否提交。
// 不用 t-popconfirm 是因为校验结果是异步的，且需要在弹窗里渲染明细。
async function openDeleteDialog(row: UserInfo) {
  deleteTarget.value = row
  deleteCheck.value = null
  deleteForce.value = false
  deleteForm.reason = ''
  deleteVisible.value = true
  deleteCheckLoading.value = true
  try {
    deleteCheck.value = await getUserDeletionCheck(row.id)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '注销前置校验失败')
    deleteVisible.value = false
  } finally {
    deleteCheckLoading.value = false
  }
}

async function handleDeleteConfirm() {
  const target = deleteTarget.value
  if (!target) return
  if (!deleteForm.reason.trim()) {
    MessagePlugin.warning('请填写注销原因')
    return
  }
  // 有警告项时必须显式勾选强制，否则后端会回 40903（这里提前拦住少一次往返）。
  if (deleteCheck.value?.warnings.length && !deleteForce.value) {
    MessagePlugin.warning('存在未结清事项，请勾选强制注销')
    return
  }
  deleteSubmitting.value = true
  try {
    await deleteUser(target.id, { reason: deleteForm.reason.trim(), force: deleteForce.value })
    MessagePlugin.success(`已注销 ${target.username}`)
    deleteVisible.value = false
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '注销失败')
  } finally {
    deleteSubmitting.value = false
  }
}

function openBatchDeleteDialog() {
  if (!selectedIds.value.length) return
  deleteForm.reason = ''
  batchForce.value = false
  batchDeleteVisible.value = true
}

async function handleBatchDeleteConfirm() {
  if (!deleteForm.reason.trim()) {
    MessagePlugin.warning('请填写注销原因')
    return
  }
  batchSubmitting.value = true
  try {
    const resp = await batchDeleteUsers({
      ids: selectedIds.value,
      reason: deleteForm.reason.trim(),
      force: batchForce.value,
    })
    const skipped = resp.skipped || []
    if (skipped.length) {
      // 逐条展示跳过原因：批量里最常见的失败是「在管实例」，运营需要知道是哪几个。
      const detail = skipped.map((item) => `#${item.id} ${item.reason}`).join('；')
      MessagePlugin.warning(`已注销 ${resp.affected} 个，跳过 ${skipped.length} 个：${detail}`)
    } else {
      MessagePlugin.success(`已注销 ${resp.affected} 个用户`)
    }
    batchDeleteVisible.value = false
    selectedIds.value = []
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '批量注销失败')
  } finally {
    batchSubmitting.value = false
  }
}

// ===== 恢复 =====

async function restoreUserRow(row: UserInfo) {
  try {
    await restoreUser(row.id)
    MessagePlugin.success(`已恢复 ${row.username}`)
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '恢复失败')
  }
}

async function handleBatchRestore() {
  if (!selectedIds.value.length) return
  batchSubmitting.value = true
  try {
    const resp = await batchRestoreUsers({ ids: selectedIds.value })
    const skipped = resp.skipped || []
    if (skipped.length) {
      const detail = skipped.map((item) => `#${item.id} ${item.reason}`).join('；')
      MessagePlugin.warning(`已恢复 ${resp.affected} 个，跳过 ${skipped.length} 个：${detail}`)
    } else {
      MessagePlugin.success(`已恢复 ${resp.affected} 个用户`)
    }
    selectedIds.value = []
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '批量恢复失败')
  } finally {
    batchSubmitting.value = false
  }
}

// ===== 留存期清理（硬删除，仅超管）=====

async function openPurgeDialog() {
  purgeResult.value = null
  purgeVisible.value = true
  // 一律先 dry_run：运营必须先看清这一轮会删掉谁。
  await runPurge(true)
}

async function runPurge(dryRun: boolean) {
  purgeLoading.value = true
  try {
    purgeResult.value = await purgeUsers({ dry_run: dryRun })
    if (!dryRun) {
      const skipped = purgeResult.value.skipped?.length || 0
      MessagePlugin.success(`已彻底删除 ${purgeResult.value.purged} 个用户${skipped ? `，跳过 ${skipped} 个` : ''}`)
      await loadUsers()
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '清理失败')
  } finally {
    purgeLoading.value = false
  }
}

// 执行清理前再确认一次：这是全系统唯一的不可逆硬删除入口。
function handlePurgeExecute() {
  const count = purgeResult.value?.candidates.length || 0
  if (!count) return
  const dialog = DialogPlugin.confirm({
    header: '确认彻底删除',
    body: `将彻底删除 ${count} 个用户及其全部个人数据，此操作不可恢复。确认继续？`,
    confirmBtn: { content: '确认删除', theme: 'danger' },
    cancelBtn: { content: '取消' },
    onConfirm: async () => {
      dialog.destroy()
      await runPurge(false)
    },
    onClose: () => dialog.destroy(),
  })
}

function openCreate() {
  Object.assign(formData, initFormData())
  formRef.value?.clearValidate?.()
  dialogVisible.value = true
}

const rechargeVisible = ref(false)
const rechargeSubmitting = ref(false)
const rechargeForm = reactive<{ user_id: number; username: string; amount: number; remark: string }>({
  user_id: 0,
  username: '',
  amount: 0,
  remark: '',
})

async function handleRechargeConfirm() {
  if (!rechargeForm.amount || rechargeForm.amount <= 0) {
    MessagePlugin.warning('请输入有效的充值金额')
    return
  }
  rechargeSubmitting.value = true
  try {
    const result = await rechargeUser(rechargeForm.user_id, {
      amount: rechargeForm.amount,
      remark: rechargeForm.remark || undefined,
    })
    // 带上充值单号：运营充完钱需要凭单号去「充值管理」核对，只提示「充值成功」
    // 等于让人凭时间戳去列表里翻。
    MessagePlugin.success(result?.recharge_no ? `充值成功，充值单号 ${result.recharge_no}` : '充值成功')
    rechargeVisible.value = false
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '充值失败')
  } finally {
    rechargeSubmitting.value = false
  }
}

async function handleCreateUser() {
  const validate = await formRef.value?.validate?.()
  if (validate !== true) return (submitting.value = true)
  try {
    await createUser({
      id: formData.id ? Number(formData.id) : undefined,
      username: formData.username,
      email: formData.email,
      phone: formData.phone,
      password: formData.password,
      status: formData.status,
      // 留空则后端给起始等级（白银会员）；显式选中的等级才上报。
      user_level_id: formData.user_level_id || undefined,
      // 留空 = 非代理（无折扣）。建号默认不授予代理身份，与后端语义一致。
      agent_level_id: formData.agent_level_id || undefined,
    })
    MessagePlugin.success('用户创建成功')
    dialogVisible.value = false
    filters.page = 1
    pagination.current = 1
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '创建用户失败')
  } finally {
    submitting.value = false
  }
}

function handleDialogClose() {
  formRef.value?.clearValidate?.()
}

async function toggleStatus(row: UserInfo) {
  const newStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await updateUserStatus(row.id, { status: newStatus })
    MessagePlugin.success(newStatus === 'active' ? '用户已解冻' : '用户已冻结')
    await loadUsers()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '操作失败')
  }
}

watch(
  () => route.query,
  async () => {
    syncFiltersFromRoute()
    await loadUsers()
  },
)

onMounted(async () => {
  syncFiltersFromRoute()
  syncViewportState()
  window.addEventListener('resize', syncViewportState)
  await loadAll()
})

onBeforeUnmount(() => {
  handleTableDragEnd()
  window.removeEventListener('resize', syncViewportState)
})
</script>

<style>
/*
 * 用户模块共享样式（.users-module .filter-card 外壳内边距、.surface-card 卡底、
 * TDesign 细节对齐）。路由懒加载下 detail 页里的引入不会覆盖到本页，
 * 与 accounts/detail/index.vue 同款方式各引一次，Vite 会去重。
 */
@import '../../shared.css';
</style>

<style scoped lang="css">
.user-list-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.list-header,
.table-panel {
  border-radius: var(--hs-radius-lg);
}

.list-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 18px 20px;
  border-color: var(--td-brand-color-2);
}

.list-header__icon {
  width: 44px;
  height: 44px;
  border-radius: var(--hs-radius-xl);
  background: linear-gradient(135deg, var(--td-brand-color-6), var(--color-primary));
  color: #ffffff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.list-header__main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
}

.list-header__title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.list-header__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  color: var(--color-foreground);
}

.list-header__actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  align-items: center;
}

.view-switch {
  flex-shrink: 0;
}

.selection-hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.table-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.cell-sub {
  display: block;
  font-size: 12px;
  color: var(--color-muted-foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.danger-note {
  padding: 10px 12px;
  border-radius: var(--hs-radius-md);
  background: var(--td-error-color-1);
  color: var(--td-error-color-7);
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 12px;
}

.detail-row {
  display: flex;
  gap: 8px;
  font-size: 13px;
  margin-bottom: 12px;
}

.detail-row__label {
  color: var(--color-muted-foreground);
  flex-shrink: 0;
}

.detail-row__value {
  color: var(--color-foreground);
  font-weight: 600;
}

.check-block {
  padding: 10px 12px;
  border-radius: var(--hs-radius-md);
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 12px;
}

.check-block--danger {
  background: var(--td-error-color-1);
  color: var(--td-error-color-7);
}

.check-block--warning {
  background: var(--td-warning-color-1);
  color: var(--td-warning-color-7);
}

.check-block--ok {
  background: var(--td-success-color-1);
  color: var(--td-success-color-7);
}

.check-block__title {
  font-weight: 700;
  margin-bottom: 4px;
}

.check-list {
  margin: 0 0 6px;
  padding-left: 18px;
}

.delete-form {
  margin-top: 8px;
}

.batch-force {
  margin-bottom: 12px;
}

.purge-alert {
  margin-bottom: 12px;
}

.purge-meta {
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--color-muted-foreground);
  margin-bottom: 12px;
}

.purge-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}

/* 筛选卡外壳（surface-card 底、圆角、内边距）由 users/shared.css 的
   .users-module .filter-card / .surface-card 规则提供，本页不再自写。 */

.table-panel__title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
}

/* 字段写在页面 scoped 里：.field 元素是本页插槽内容，带本页 scope 属性。
   users/shared.css 目前没有 .users-module .field 规则，先在本页兜底。 */
.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.field__label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-muted-foreground);
}

/* 已生效的筛选项加一道强调色左边条 + 标签变色：一屏九个下拉框，
   不标出来就得逐个点开看自己到底筛了什么。 */
.field--active {
  position: relative;
  padding-left: 8px;
}

.field--active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 2px;
  bottom: 2px;
  width: 2px;
  border-radius: 1px;
  background: var(--color-accent, #f97316);
}

.field--active .field__label {
  color: var(--color-foreground);
}

/* 表头的 ID 排序按钮。必须用 :deep()：该节点由列配置里的 title 渲染函数生成，
   渲染上下文在 TDesign 的表头组件内，拿不到本组件的 scoped 属性，
   普通 scoped 规则匹配不到它。 */
:deep(.id-sort-button) {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
}

:deep(.id-sort-button:hover) {
  color: var(--td-brand-color);
}

:deep(.id-sort-button:focus-visible) {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
  border-radius: 2px;
}

/* 箭头方向表达排序方向：正序朝上，倒序翻转为朝下。
   始终用同一个图标 + CSS 翻转，避免按状态换图标导致节点重建、丢失过渡。 */
:deep(.id-sort-button__icon) {
  flex-shrink: 0;
  transition: transform var(--hs-duration-fast) var(--hs-ease-out);
}

:deep(.id-sort-button__icon:not(.is-asc)) {
  transform: rotate(180deg);
}

.table-panel {
  padding: 16px;
  background: var(--hs-surface-1);
  border-color: var(--td-brand-color-2);
}

.table-drag-scroll {
  overflow-x: auto;
  cursor: grab;
}

.table-drag-scroll--dragging {
  cursor: grabbing;
}

.table-drag-scroll--dragging :deep(*) {
  user-select: none;
}

.table-panel__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

/* 移动端批量开关：与标题同行，靠右。批量条（已选 N 个 + 动作按钮）在选中后
   会出现并与它争同一行空间，所以窄屏允许换行，而不是把按钮压成图标。 */
.batch-toggle {
  flex-shrink: 0;
}

@media (max-width: 767px) {
  .table-panel__head {
    flex-wrap: wrap;
  }

  .batch-toggle {
    order: 2;
  }
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(239, 68, 68, 0.18);
  border-radius: var(--hs-radius-md);
  background: rgba(239, 68, 68, 0.06);
  color: var(--color-destructive);
}

.id-cell,
.user-cell,
.user-cell--primary {
  gap: 8px;
}

.user-cell__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.account-type-tag {
  flex-shrink: 0;
}

.account-type-tag--sub {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.id-cell__value {
  color: var(--hs-text-mono);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

/* 用户名是「点进去」的入口，保留主题色链接感；实名/销售归属是身份信息，
   用前景色加粗即可 —— 原来两者都吃主题色，一列列看过去全是同一种灰。 */
.user-cell__name {
  color: var(--hs-text-identity);
  font-weight: 700;
}

.user-cell__name--secondary {
  color: var(--hs-text-identity);
  font-weight: 600;
}

.copy-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.copy-row__value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}

/* 邮箱/手机号属技术字段：统一灰阶 + 等宽，和「人」有关的字段（姓名）拉开层次。 */
.copy-row__value--email,
.copy-row__value--phone {
  color: var(--hs-text-tech);
  font-variant-numeric: tabular-nums;
}

.role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.oauth-cell {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.oauth-provider-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--color-muted-foreground);
  transition:
    color 0.2s ease,
    opacity 0.2s ease;
}

/* 未绑定的渠道降到「几乎只是占位」：深色下 #cbd5e1 反而比绑定态更亮，
   改用透明度表达「未启用」，明暗两套主题都成立。 */
.oauth-provider-icon--inactive {
  color: var(--color-muted-foreground);
  opacity: 0.35;
}

.oauth-provider-icon--wechat:not(.oauth-provider-icon--inactive) {
  color: #07c160;
}

.oauth-provider-icon--qq:not(.oauth-provider-icon--inactive) {
  color: #12b7f5;
}

.oauth-provider-icon--alipay:not(.oauth-provider-icon--inactive) {
  color: #1677ff;
}

.action-cell {
  display: flex;
  width: 100%;
  gap: 6px;
  align-items: center;
  justify-content: center;
}

/* 操作列按「动作的危险程度」分色：查看=中性灰、资金/下单=主题色、
   登录=技术色、解冻/恢复=绿、冻结=橙、注销=红。
   原先六个动作全是主题色链接，一整列分不出哪个是破坏性操作。
   悬停一律用 opacity 而不是换色：语义色本身随主题翻转，
   再写一套 hover 色号等于把明暗两套值在各页重复一遍。 */
:deep(.action-link) {
  font-weight: 500;
  font-size: 13px;
  transition:
    color var(--hs-duration-fast),
    opacity var(--hs-duration-fast);
}

:deep(.action-link:hover) {
  color: inherit;
  opacity: 0.72;
}

:deep(.action-link--view) {
  color: var(--color-muted-foreground);
}

/* 中性/次要动作的悬停要「更醒目」而不是更淡，单独反转一次透明度方向。 */
:deep(.action-link--view:hover),
:deep(.action-link--login:hover) {
  opacity: 1;
  color: var(--color-foreground);
}

:deep(.action-link--recharge),
:deep(.action-link--order) {
  color: var(--color-primary);
}

:deep(.action-link--login) {
  color: var(--hs-text-tech);
}

:deep(.action-link--restore) {
  color: var(--hs-status-ok-text);
}

:deep(.action-link--freeze) {
  color: var(--hs-status-warn-text);
}

:deep(.action-link--delete) {
  color: var(--hs-status-danger-text);
}

.action-menu-button {
  min-width: 0;
  width: 36px;
  height: 32px;
  padding: 0;
}

.mobile-action-button {
  width: 32px;
  height: 32px;
  min-width: 32px;
  padding: 0;
  color: var(--color-muted-foreground);
  background: var(--hs-surface-1);
  border-color: transparent;
}

/* 模板里用到但此前全站没有定义过：没有这条规则，「未分组/未归属/非代理」
   这些占位文案会继承单元格前景色，和真实数据一样黑，看起来像真有值。 */
.text-muted {
  color: var(--color-muted-foreground);
}

/* TDesign 原生 MoreIcon 是描边式三竖点（1em svg），在固定列 flex 压缩下会被
   挤成 2px 宽而不可见；这里解除压缩并锁定渲染宽度，保证点阵完整。 */
.mobile-action-button :deep(.t-icon) {
  width: 16px;
  height: 16px;
  min-width: 16px;
  max-width: none;
  flex: 0 0 auto;
  overflow: visible;
}

/* 固定操作列悬浮于表格内容之上，但必须低于 sticky 顶栏（z-index:5），
   否则窗口下滑时操作列会浮到导航栏之上；表格 sticky 单元默认 z-index:31，
   这里统一压到 3。

   注意类名是双横线：TDesign 实际渲染 .t-table__cell--fixed-left /
   --fixed-right，旧的单横线写法（.t-table__cell-fixed-left）匹配不到任何元素，
   左侧勾选列就一直保持默认 z-index:30 —— 移动端下滑时复选框浮到顶部导航之上，
   正是这条规则没生效导致的。两种写法都留着，兼容依赖单横线类名的旧版本。 */
:deep(.user-table .t-table__cell--fixed-left),
:deep(.user-table .t-table__cell--fixed-right),
:deep(.user-table .t-table__cell-fixed-left),
:deep(.user-table .t-table__cell-fixed-right),
:deep(.user-table .t-table__fixed-left),
:deep(.user-table .t-table__fixed-right),
:deep(.user-table th.t-table__cell--fixed-left),
:deep(.user-table td.t-table__cell--fixed-left),
:deep(.user-table th.t-table__cell--fixed-right),
:deep(.user-table td.t-table__cell--fixed-right) {
  z-index: 3;
}

.ip-cell__value {
  color: var(--hs-text-tech);
  font-weight: 500;
  font-family: var(--hs-font-mono);
  font-size: 12px;
}

/* 金额按含义分色：余额=可动用资产（绿）、总消费=已支出（橙红）。
   两者共用 .money 的数字排版（等宽数字、右对齐对齐小数点），
   只是色相不同 —— 原来两列都是主题色，扫一眼分不出哪个是哪个。 */
.money {
  display: inline-flex;
  align-items: baseline;
  gap: 1px;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.money__symbol {
  font-size: 0.85em;
  font-weight: 600;
  opacity: 0.75;
}

.money-cell--balance .money {
  color: var(--hs-text-money-in);
}

.money-cell--consume .money {
  color: var(--hs-text-money-out);
}

/* 0 值不抢注意力：金额为 0 时退成灰阶，避免整列「¥0.00」都刷成彩色。 */
.money-cell .money--zero {
  color: var(--color-muted-foreground);
  font-weight: 600;
}

.time-text {
  color: var(--hs-text-mono);
  font-size: 12px;
  line-height: 1.6;
  font-variant-numeric: tabular-nums;
}

.time-text--muted {
  color: var(--color-muted-foreground);
}

:deep(.page-btn.t-button--theme-primary) {
  background-color: var(--color-primary);
  border-color: var(--color-primary);
}

:deep(.page-btn.t-button--theme-primary:hover),
:deep(.page-btn.t-button--theme-primary:focus-visible) {
  background-color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-8);
}

:deep(.page-btn--ghost) {
  color: var(--color-primary);
  border-color: var(--td-brand-color-3);
  background: var(--td-brand-color-1);
}

:deep(.page-btn--ghost:hover),
:deep(.page-btn--ghost:focus-visible) {
  color: var(--td-brand-color-8);
  border-color: var(--td-brand-color-4);
  background: var(--td-brand-color-2);
}

:deep(.page-chip.t-tag--primary.t-tag--variant-light),
:deep(.role-tag.t-tag--primary.t-tag--variant-light),
:deep(.copy-tag.t-tag--primary.t-tag--variant-light) {
  color: var(--td-brand-color-8);
  background: var(--td-brand-color-1);
  border-color: var(--td-brand-color-3);
}

:deep(.copy-tag) {
  cursor: pointer;
  opacity: 0;
  transform: translateX(-2px);
  pointer-events: none;
  transition:
    opacity var(--hs-duration-fast),
    transform var(--hs-duration-fast),
    background-color var(--hs-duration-fast),
    border-color var(--hs-duration-fast),
    color var(--hs-duration-fast);
}

.copy-row:hover :deep(.copy-tag),
.copy-row:focus-within :deep(.copy-tag) {
  opacity: 1;
  transform: translateX(0);
  pointer-events: auto;
}

/* 触屏无 hover：移动端下复制标签常驻可见可点 */
@media (hover: none), (max-width: 767px) {
  :deep(.copy-tag) {
    opacity: 1;
    transform: none;
    pointer-events: auto;
  }
}

:deep(.copy-tag:hover) {
  color: var(--td-brand-color-9);
  background: var(--td-brand-color-2);
  border-color: var(--td-brand-color-4);
}

:deep(.page-link),
:deep(.page-link.t-link),
:deep(.user-link),
:deep(.user-link.t-link) {
  color: var(--color-primary);
}

:deep(.page-link:hover),
:deep(.page-link.t-link:hover),
:deep(.user-link:hover),
:deep(.user-link.t-link:hover) {
  color: var(--td-brand-color-8);
}

:deep(.user-link) {
  font-weight: 700;
  font-size: 15px;
}

:deep(.unified-control .t-input),
:deep(.unified-control .t-input__wrap),
:deep(.unified-control .t-input-adornment),
:deep(.unified-control .t-input__suffix),
:deep(.unified-control .t-select__wrap) {
  background: var(--hs-surface-2);
}

:deep(.unified-control .t-input),
:deep(.unified-control .t-select__wrap) {
  border-color: var(--color-border);
  border-radius: var(--hs-radius-md);
}

:deep(.unified-control.t-is-focused .t-input),
:deep(.unified-control.t-is-focused .t-select__wrap),
:deep(.unified-control .t-input:focus-within),
:deep(.unified-control .t-select__wrap:focus-within) {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px rgba(var(--color-primary-rgb), 0.1);
}

:deep(.user-table .t-table) {
  min-width: 1450px;
  border-color: var(--td-brand-color-2);
}

/* 表头样式由全局 styles/table-head.css 统一提供，本页即为该规格的基准页
   （白底 + 品牌色 3% 叠加层、微灰字、600 字重、品牌色下边框）。此前这段规则
   与全局规格逐字相同地重复了一份，后来者改一处漏一处就会重新分叉，故删除：
   全局文件里已逐条记录这组取值的理由（含固定列必须不透明的原因）。 */

:deep(.user-table .t-table__body td) {
  color: var(--color-foreground);
  border-bottom-color: var(--td-brand-color-1);
  vertical-align: middle;
}

/* 行悬停由全局 styles/table-head.css 统一（同为本页的 3% 品牌色叠加层），
   此处不再重复定义，否则两层叠加会变成约 6%。 */

:deep(.status-tag) {
  border-radius: var(--hs-radius-xl);
  font-weight: 600;
  border: 1px solid transparent;
}

/* 状态点：颜色跟状态走，脱色环境下也能靠位置分辨 */
:deep(.status-tag__dot) {
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 4px;
  border-radius: 50%;
  background: currentColor;
  vertical-align: middle;
}

:deep(.status-tag--active) {
  color: var(--hs-status-ok-text);
  background: var(--hs-status-ok-bg);
  border-color: var(--hs-status-ok-border);
}

:deep(.status-tag--disabled) {
  color: var(--hs-status-danger-text);
  background: var(--hs-status-danger-bg);
  border-color: var(--hs-status-danger-border);
}

:deep(.status-tag--pending) {
  color: var(--hs-status-warn-text);
  background: var(--hs-status-warn-bg);
  border-color: var(--hs-status-warn-border);
}

:deep(.status-tag--cancelled),
:deep(.status-tag--default) {
  color: var(--hs-status-neutral-text);
  background: var(--hs-status-neutral-bg);
  border-color: var(--hs-status-neutral-border);
}

@media (max-width: 1200px) {
  .table-panel__head {
    flex-direction: column;
    align-items: stretch;
  }
}

@media (max-width: 768px) {
  .list-header {
    flex-direction: column;
    align-items: stretch;
  }

  .list-header__actions {
    justify-content: flex-start;
  }
}

@media (max-width: 767px) {
  .table-panel {
    padding: 8px;
  }

  .action-cell {
    justify-content: center;
  }

  .mobile-action-button {
    width: 30px;
    height: 30px;
    min-width: 30px;
  }

  :deep(.user-table .t-table__header th),
  :deep(.user-table .t-table__body td) {
    padding-inline: 4px;
  }
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 16px;
}

.status-radio-group {
  width: 220px;
}

.status-radio-group :deep(.t-radio-button) {
  min-width: 96px;
  text-align: center;
}

.form-hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}
</style>
