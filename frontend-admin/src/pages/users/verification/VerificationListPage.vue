<template>
  <SecurityListPage
    :title="title"
    :icon="icon"
    :table-title="tableTitle"
    :total="pagination.total"
    :data="tableData"
    :columns="columns"
    :loading="loading"
    :error-message="errorMessage"
    :empty-text="emptyText"
    :pagination="pagination"
    @search="handleSearch"
    @reset="handleReset"
    @reload="loadData"
    @page-change="handlePageChange"
  >
    <template #filters>
      <div class="filter-grid">
        <div class="field">
          <span class="field__label">用户名</span>
          <t-input v-model="filters.username" clearable placeholder="用户名" />
        </div>
        <div class="field">
          <span class="field__label">认证类型</span>
          <t-select v-model="filters.verification_type" clearable :options="typeOptions" placeholder="认证类型" />
        </div>
        <div class="field">
          <span class="field__label">审核人</span>
          <t-input v-model="filters.reviewer_name" clearable placeholder="审核人" />
        </div>
        <div class="field">
          <span class="field__label">关键词</span>
          <t-input v-model="filters.keyword" clearable placeholder="关键词 / 主体 / 姓名" />
        </div>
      </div>
    </template>

    <template #verification_type="{ row }">
      <t-tag variant="light-outline" theme="primary">{{ typeLabel[row.verification_type] || row.verification_type || '—' }}</t-tag>
    </template>

    <template #status="{ row }">
      <t-tag :theme="statusTheme[row.status] || 'default'" variant="light-outline">{{ statusLabel[row.status] || row.status || '—' }}</t-tag>
    </template>

    <template #submitted_at="{ row }">{{ formatTime(row.submitted_at) }}</template>
    <template #reviewed_at="{ row }">{{ formatTime(row.reviewed_at) }}</template>
    <template #reviewer_name="{ row }">{{ row.reviewer_name || '—' }}</template>
    <template #reject_reason="{ row }">{{ row.reject_reason || '—' }}</template>

    <template #action="{ row }">
      <t-link theme="primary" hover="color" @click="openDetail(row)">查看审核</t-link>
    </template>
  </SecurityListPage>

  <!-- 审核抽屉：整单粒度（doc104 §5.1）——通过/驳回作用于整份申请，不逐条审附件。 -->
  <t-drawer
    v-model:visible="detailVisible"
    header="实名认证审核"
    size="640px"
    :footer="false"
    @close="detailVisible = false"
  >
    <div v-if="detailLoading" class="drawer-loading"><t-loading text="加载中…" /></div>
    <div v-else-if="!detail" class="drawer-loading"><t-empty description="申请不存在或已删除" /></div>
    <div v-else class="detail-body">
      <div class="detail-head">
        <div class="detail-head__main">
          <span class="cell-strong">{{ detail.subject_name || detail.real_name }}</span>
          <span class="price-sub">{{ detail.username }} · 第 {{ detail.review_round }} 次提交</span>
        </div>
        <t-tag :theme="statusTheme[detail.status] || 'default'" variant="light-outline">
          {{ statusLabel[detail.status] || detail.status }}
        </t-tag>
      </div>

      <t-alert
        v-if="isOrphan(detail)"
        theme="warning"
        class="detail-alert"
        message="该申请未关联到现存用户"
        description="申请单的 user_id 为空或对应用户已不存在（历史演示数据）。审核动作仍会写审计轨迹，但不会影响任何账号的实名状态。"
      />

      <section class="detail-section">
        <h4 class="detail-section__title">主体信息</h4>
        <div class="detail-grid">
          <div class="detail-item"><span class="detail-item__label">认证类型</span><span>{{ typeLabel[detail.verification_type] || detail.verification_type || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">证件类型</span><span>{{ idTypeLabel(detail.id_type) }}</span></div>
          <div class="detail-item"><span class="detail-item__label">证件号</span><span>{{ detail.id_number_masked || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">手机号</span><span>{{ detail.mobile_masked || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">提交时间</span><span>{{ formatTime(detail.submitted_at) }}</span></div>
          <div class="detail-item"><span class="detail-item__label">审核时间</span><span>{{ formatTime(detail.reviewed_at) }}</span></div>
          <div class="detail-item"><span class="detail-item__label">审核人</span><span>{{ detail.reviewer_name || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">风险标记</span><span>{{ detail.risk_flags || '—' }}</span></div>
        </div>
      </section>

      <section v-if="detail.enterprise" class="detail-section">
        <h4 class="detail-section__title">企业信息</h4>
        <div class="detail-grid">
          <div class="detail-item"><span class="detail-item__label">企业名称</span><span>{{ detail.enterprise.company_name || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">统一社会信用代码</span><span>{{ detail.enterprise.credit_code_masked || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">法定代表人</span><span>{{ detail.enterprise.legal_person_name || '—' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">联系人</span><span>{{ detail.enterprise.contact_name || '—' }}</span></div>
        </div>
      </section>

      <section class="detail-section">
        <h4 class="detail-section__title">核验服务商</h4>
        <div class="detail-grid">
          <div class="detail-item"><span class="detail-item__label">当前服务商</span><span>{{ detail.provider || 'manual' }}</span></div>
          <div class="detail-item"><span class="detail-item__label">核验结果</span><span>{{ providerResultLabel(detail.provider_result) }}</span></div>
          <div class="detail-item"><span class="detail-item__label">核验时间</span><span>{{ formatTime(detail.provider_checked_at) }}</span></div>
          <div class="detail-item detail-item--wide"><span class="detail-item__label">核验信息</span><span>{{ detail.provider_message || '—' }}</span></div>
        </div>
        <t-button variant="outline" size="small" :loading="checking" :disabled="!canAudit" @click="handleProviderCheck">
          调用服务商核验
        </t-button>
        <p class="field-help">
          跳转式服务商（支付宝）不支持无跳转核验，点击后会提示用户改用「授权核验」流程，不会误报失败。
        </p>
      </section>

      <section v-if="detail.documents?.length" class="detail-section">
        <h4 class="detail-section__title">证件附件</h4>
        <div class="doc-list">
          <a v-for="doc in detail.documents" :key="doc.id" class="doc-item" :href="doc.file_url" target="_blank" rel="noopener">
            <FileIcon size="16" aria-hidden="true" />
            <span>{{ documentTypeLabel(doc.document_type) }}</span>
          </a>
        </div>
      </section>

      <section v-if="detail.logs?.length" class="detail-section">
        <h4 class="detail-section__title">审核轨迹</h4>
        <t-timeline>
          <t-timeline-item v-for="log in detail.logs" :key="log.id" :time="formatTime(log.created_at)">
            <div class="log-item">
              <span class="cell-strong">{{ actionLabel(log.action) }}</span>
              <span class="price-sub">{{ log.operator_name || '系统' }} · {{ log.from_status || '—' }} → {{ log.to_status || '—' }}</span>
              <span v-if="log.reject_reason || log.note" class="price-sub">{{ log.reject_reason || log.note }}</span>
            </div>
          </t-timeline-item>
        </t-timeline>
      </section>

      <div v-if="canAudit" class="detail-actions">
        <template v-if="detail.status === 'pending'">
          <t-button theme="primary" :loading="acting" @click="handleApprove">通过</t-button>
          <t-button theme="danger" variant="outline" :loading="acting" @click="openReject">驳回</t-button>
        </template>
        <t-button v-else-if="detail.status === 'approved'" theme="warning" variant="outline" :loading="acting" @click="openRevoke">
          撤销认证
        </t-button>
        <span v-else class="price-sub">已驳回的申请不可再审核，用户可在冷却期后重新提交。</span>
      </div>
      <div v-else class="detail-actions">
        <span class="price-sub">当前账号没有审核权限（需要 verification:audit）。</span>
      </div>
    </div>
  </t-drawer>

  <!-- 驳回：理由必填，理由码可选，两者都会进审核轨迹供用户查看 -->
  <t-dialog
    v-model:visible="rejectVisible"
    header="驳回实名申请"
    :confirm-btn="{ content: '确认驳回', theme: 'danger', loading: acting }"
    @confirm="handleReject"
  >
    <t-form label-align="top" @submit.prevent>
      <t-form-item label="驳回理由码">
        <t-select v-model="rejectForm.reject_reason_code" :options="rejectCodeOptions" clearable placeholder="选填，用于统计归类" />
      </t-form-item>
      <t-form-item label="驳回理由（用户可见）">
        <t-textarea v-model="rejectForm.reject_reason" :autosize="{ minRows: 3, maxRows: 5 }" placeholder="例如：证件照片不清晰，请重新上传" />
      </t-form-item>
      <t-form-item label="内部备注（用户不可见）">
        <t-textarea v-model="rejectForm.note" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
      </t-form-item>
    </t-form>
  </t-dialog>

  <!-- 撤销：已通过的实名被撤回会同步清空 users.real_name_verified_at -->
  <t-dialog
    v-model:visible="revokeVisible"
    header="撤销实名认证"
    theme="warning"
    :confirm-btn="{ content: '确认撤销', theme: 'warning', loading: acting }"
    @confirm="handleRevoke"
  >
    <p>撤销后该账号的实名状态立即失效（<code>real_name_verified_at</code> 被清空），用户需重新提交。</p>
    <t-form label-align="top" @submit.prevent>
      <t-form-item label="撤销原因">
        <t-textarea v-model="revokeForm.reject_reason" :autosize="{ minRows: 3, maxRows: 5 }" placeholder="例如：监管要求复核，原审核有误" />
      </t-form-item>
      <t-form-item label="内部备注">
        <t-textarea v-model="revokeForm.note" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="选填" />
      </t-form-item>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { type Component, computed, onMounted, reactive, ref } from 'vue'
import type { PageInfo, PrimaryTableCol } from 'tdesign-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import { FileIcon } from 'tdesign-icons-vue-next'

import {
  approveVerification,
  getVerificationDetail,
  providerCheckVerification,
  rejectVerification,
  revokeVerification,
  type VerificationDetail,
  type VerificationInfo,
  type VerificationListQuery,
  type VerificationReviewRequest,
} from '@/api/verification'
import SecurityListPage from '../security/SecurityListPage.vue'
import { useUserStore } from '@/store'

const props = defineProps<{
  title: string
  tableTitle: string
  emptyText: string
  fetcher: (params: VerificationListQuery) => Promise<{ items: VerificationInfo[]; meta: { total: number } }>
  icon?: Component
}>()

defineOptions({ name: 'VerificationListPage' })

const userStore = useUserStore()
// 审核类动作统一要求 verification:audit（与后端路由的权限码一致），
// 只读查看仍沿用 verification:list，因此只读角色看到的抽屉里没有操作按钮。
const canAudit = computed(() => userStore.permissions?.includes('verification:audit') || userStore.isSuperAdmin)

const loading = ref(false)
const errorMessage = ref('')
const tableData = ref<VerificationInfo[]>([])
const filters = reactive<VerificationListQuery>({ page: 1, page_size: 10, username: '', verification_type: '', reviewer_name: '', keyword: '' })
const pagination = reactive({ current: 1, pageSize: 10, total: 0, showJumper: true, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<VerificationDetail | null>(null)
const acting = ref(false)
const checking = ref(false)

const rejectVisible = ref(false)
const revokeVisible = ref(false)
const rejectForm = reactive<VerificationReviewRequest>({ reject_reason_code: '', reject_reason: '', note: '' })
const revokeForm = reactive<VerificationReviewRequest>({ reject_reason: '', note: '' })

const typeOptions = [
  { label: '个人认证', value: 'personal' },
  { label: '企业认证', value: 'enterprise' },
]

const rejectCodeOptions = [
  { label: '证件照片不清晰', value: 'document_blur' },
  { label: '信息与证件不一致', value: 'info_mismatch' },
  { label: '证件已过期', value: 'document_expired' },
  { label: '企业资料不完整', value: 'enterprise_incomplete' },
  { label: '三方核验未通过', value: 'provider_failed' },
  { label: '其他', value: 'other' },
]

const typeLabel: Record<string, string> = {
  personal: '个人认证',
  enterprise: '企业认证',
}

const idTypeLabelMap: Record<string, string> = {
  id_card: '身份证',
  passport: '护照',
  business_license: '营业执照',
}

const statusLabel: Record<string, string> = {
  pending: '待审核',
  approved: '审核通过',
  rejected: '审核拒绝',
}

const statusTheme: Record<string, string> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'danger',
}

const actionLabelMap: Record<string, string> = {
  submit: '提交申请',
  approve: '审核通过',
  reject: '审核驳回',
  revoke: '撤销认证',
  provider_pass: '三方核验通过',
  provider_fail: '三方核验失败',
  auto_approve: '自动通过',
  auto_reject: '自动驳回',
}

const columns: PrimaryTableCol<VerificationInfo>[] = [
  { colKey: 'username', title: '用户名', width: 140 },
  { colKey: 'real_name', title: '姓名', width: 120 },
  { colKey: 'subject_name', title: '认证主体', minWidth: 220, ellipsis: true },
  { colKey: 'verification_type', title: '认证类型', width: 120 },
  { colKey: 'id_number_masked', title: '证件号', width: 160 },
  { colKey: 'mobile_masked', title: '手机号', width: 140 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'reviewer_name', title: '审核人', width: 120 },
  { colKey: 'submitted_at', title: '提交时间', width: 180 },
  { colKey: 'reviewed_at', title: '审核时间', width: 180 },
  { colKey: 'reject_reason', title: '拒绝原因', minWidth: 180, ellipsis: true },
  { colKey: 'action', title: '操作', width: 110, fixed: 'right' },
]

function formatTime(value?: string) {
  if (!value) return '—'
  return value.replace('T', ' ').slice(0, 19)
}

function idTypeLabel(value: string) {
  return idTypeLabelMap[value] || value || '—'
}

function documentTypeLabel(value: string) {
  const map: Record<string, string> = {
    id_front: '证件正面',
    id_back: '证件反面',
    handheld: '手持证件照',
    business_license: '营业执照',
    authorization: '授权书',
  }
  return map[value] || value || '附件'
}

function providerResultLabel(value: string) {
  const map: Record<string, string> = { passed: '通过', failed: '未通过', pending: '待核验' }
  return map[value] || '未核验'
}

function actionLabel(value: string) {
  return actionLabelMap[value] || value || '操作'
}

// 孤儿申请（user_id=0 或用户已不存在）在列表里也要能点开：历史演示数据的
// user_id 曾是 0，若直接隐藏这些行，运营会看到「总数对不上」。
function isOrphan(row: VerificationInfo) {
  return !row.user_id
}

async function loadData() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await props.fetcher({ ...filters, page: pagination.current, page_size: pagination.pageSize })
    tableData.value = response.items || []
    pagination.total = response.meta.total || 0
  } catch (error) {
    errorMessage.value = (error as Error)?.message || '加载实名认证列表失败'
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.current = 1
  void loadData()
}

function handleReset() {
  filters.username = ''
  filters.verification_type = ''
  filters.reviewer_name = ''
  filters.keyword = ''
  pagination.current = 1
  pagination.pageSize = 10
  void loadData()
}

function handlePageChange(pageInfo: PageInfo) {
  pagination.current = pageInfo.current
  pagination.pageSize = pageInfo.pageSize
  void loadData()
}

async function openDetail(row: VerificationInfo) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await getVerificationDetail(row.id)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载申请详情失败')
  } finally {
    detailLoading.value = false
  }
}

async function reloadDetail() {
  if (!detail.value) return
  try {
    detail.value = await getVerificationDetail(detail.value.id)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '刷新详情失败')
  }
}

async function handleApprove() {
  if (!detail.value) return
  acting.value = true
  try {
    await approveVerification(detail.value.id)
    MessagePlugin.success('已通过，用户实名状态即时生效')
    await reloadDetail()
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '审核失败')
  } finally {
    acting.value = false
  }
}

function openReject() {
  rejectForm.reject_reason_code = ''
  rejectForm.reject_reason = ''
  rejectForm.note = ''
  rejectVisible.value = true
}

async function handleReject() {
  if (!detail.value) return
  if (!rejectForm.reject_reason?.trim()) {
    MessagePlugin.warning('请填写驳回理由，用户需要看到具体原因才能改正')
    return
  }
  acting.value = true
  try {
    await rejectVerification(detail.value.id, { ...rejectForm })
    MessagePlugin.success('已驳回')
    rejectVisible.value = false
    await reloadDetail()
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '驳回失败')
  } finally {
    acting.value = false
  }
}

function openRevoke() {
  revokeForm.reject_reason = ''
  revokeForm.note = ''
  revokeVisible.value = true
}

async function handleRevoke() {
  if (!detail.value) return
  acting.value = true
  try {
    await revokeVerification(detail.value.id, { ...revokeForm })
    MessagePlugin.success('已撤销，用户实名状态已失效')
    revokeVisible.value = false
    await reloadDetail()
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '撤销失败')
  } finally {
    acting.value = false
  }
}

async function handleProviderCheck() {
  if (!detail.value) return
  checking.value = true
  try {
    const result = await providerCheckVerification(detail.value.id)
    if (result?.ok) {
      MessagePlugin.success(result.message || '核验完成')
    } else {
      // ok=false 是业务状态（跳转式服务商不支持无跳转核验），不是请求失败。
      MessagePlugin.warning(result?.message || '该服务商不支持无跳转核验')
    }
    await reloadDetail()
    await loadData()
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '核验请求失败')
  } finally {
    checking.value = false
  }
}

onMounted(() => {
  void loadData()
})
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.drawer-loading {
  padding: 48px 0;
  display: flex;
  justify-content: center;
}

.detail-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.detail-head__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.detail-alert {
  border-radius: var(--hs-radius-lg, 8px);
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--hs-border-color, #e5e7eb);
}

.detail-section:last-of-type {
  border-bottom: none;
}

.detail-section__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 16px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.detail-item--wide {
  grid-column: 1 / -1;
}

.detail-item__label {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.doc-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.doc-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border: 1px solid var(--hs-border-color, #e5e7eb);
  border-radius: var(--hs-radius-lg, 8px);
  font-size: 12px;
  color: var(--td-brand-color-7, #2b5cff);
  text-decoration: none;
}

.log-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.detail-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.price-sub {
  font-size: 12px;
  color: var(--color-muted-foreground);
}

.cell-strong {
  font-weight: 600;
}

@media (max-width: 1200px) {
  .filter-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .filter-grid,
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
