<template>
  <div class="ticket-create-page">
    <!-- 页头 -->
    <section class="create-hero">
      <t-button variant="text" shape="square" aria-label="返回" class="back-btn" @click="goBack">
        <template #icon><ChevronLeftIcon /></template>
      </t-button>
      <div class="hero-info">
        <span class="hero-title">提交工单</span>
        <span class="hero-desc">请描述您遇到的问题，我们的工程师会尽快跟进处理。</span>
      </div>
    </section>

    <!-- 表单 -->
    <section class="create-panel">
      <t-form
        ref="formRef"
        :data="formData"
        :rules="rules"
        label-width="90px"
        label-align="top"
        @submit="onSubmit"
      >
        <t-form-item label="工单标题" name="title">
          <t-input v-model="formData.title" placeholder="简要概括问题，例如：实例无法远程连接" :maxlength="100" show-limit-number />
        </t-form-item>

        <t-form-item label="问题分类" name="category">
          <t-select v-model="formData.category" placeholder="请选择问题分类" :loading="categoryLoading" @change="handleCategoryChange">
            <t-option
              v-for="item in categories"
              :key="item.id"
              :value="item.code"
              :label="item.name"
              :disabled="isCategoryBlocked(item)"
            >
              <div class="category-option">
                <span>{{ item.name }}</span>
                <span v-if="isCategoryBlocked(item)" class="category-option__lock">需先完成实名认证</span>
              </div>
            </t-option>
          </t-select>
          <div v-if="selectedCategory" class="category-help">
            <span v-if="selectedCategory.description">{{ selectedCategory.description }}</span>
            <span v-if="selectedCategory.need_review" class="category-help__tip">
              该分类的客服回复需内部复核通过后才会送达，可能稍有延迟。
            </span>
          </div>
        </t-form-item>

        <!-- 前置条件：关联订单/实例（分类要求时必填） -->
        <template v-if="selectedCategory?.require_binding">
          <t-alert theme="warning" class="binding-alert">
            <template #message>
              该分类需关联您本人的订单或实例，请至少选择一项，否则无法提交。
            </template>
          </t-alert>
          <t-form-item label="关联订单" name="order_id">
            <t-select
              v-model="formData.order_id"
              clearable
              filterable
              placeholder="选择订单（可选）"
              :options="orderOptions"
              :loading="orderLoading"
            />
          </t-form-item>
          <t-form-item label="关联实例" name="instance_id">
            <t-select
              v-model="formData.instance_id"
              clearable
              filterable
              placeholder="选择实例（可选）"
              :options="instanceOptions"
              :loading="instanceLoading"
            />
          </t-form-item>
        </template>

        <t-form-item label="优先级" name="priority">
          <t-radio-group v-model="formData.priority">
            <t-radio-button v-for="item in ticketPriorityOptions" :key="item.value" :value="item.value">
              {{ item.label }}
            </t-radio-button>
          </t-radio-group>
        </t-form-item>

        <t-form-item label="问题描述" name="description">
          <t-textarea
            v-model="formData.description"
            placeholder="请详细描述问题现象、发生时间、影响范围等，便于工程师快速定位"
            :maxlength="2000"
            :autosize="{ minRows: 6, maxRows: 12 }"
            show-limit-number
          />
        </t-form-item>

        <div class="form-actions">
          <t-button theme="default" variant="outline" @click="goBack">取消</t-button>
          <t-button theme="primary" type="submit" :loading="submitting">提交工单</t-button>
        </div>
      </t-form>
      <p class="attachment-tip">如需附带截图或日志，可在提交成功后于工单详情页上传附件。</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ChevronLeftIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormProps } from 'tdesign-vue-next'

import { createMyTicket, getTicketCategories, type TicketCategoryInfo } from '@/api/support'
import { getMyOrders } from '@/api/shop'
import { listInstances } from '@/api/cloud'
import { ticketPriorityOptions } from '@/pages/support/constants'

defineOptions({ name: 'UserTicketCreate' })

const router = useRouter()

const formRef = ref<FormInstanceFunctions>()

// 表单数据（priority 默认 medium，与后端约定一致）
const formData = reactive<{
  title: string
  category: string
  priority: string
  description: string
  order_id: number | undefined
  instance_id: number | undefined
}>({
  title: '',
  category: '',
  priority: 'medium',
  description: '',
  order_id: undefined,
  instance_id: undefined,
})

// 表单校验规则（与后端 binding 约束对齐）
const rules: FormProps['rules'] = {
  title: [{ required: true, message: '请输入工单标题', trigger: 'blur' }],
  category: [{ required: true, message: '请选择问题分类', trigger: 'change' }],
  description: [{ required: true, message: '请描述您的问题', trigger: 'blur' }],
}

const categories = ref<TicketCategoryInfo[]>([])
const categoryLoading = ref(false)
const realnameOK = ref(true)
const submitting = ref(false)

// 分类要求关联订单/实例时才拉取可选列表，避免无谓请求
const orderOptions = ref<{ label: string; value: number }[]>([])
const orderLoading = ref(false)
const instanceOptions = ref<{ label: string; value: number }[]>([])
const instanceLoading = ref(false)

const selectedCategory = computed(() => categories.value.find((item) => item.code === formData.category))

// 实名未完成时，要求实名的分类直接置灰（后端仍会再校验一次）
function isCategoryBlocked(item: TicketCategoryInfo): boolean {
  return item.require_realname && !realnameOK.value
}

// 加载可用工单分类（含前置条件与实名状态）
async function loadCategories() {
  categoryLoading.value = true
  try {
    const { data } = await getTicketCategories()
    categories.value = data?.items ?? []
    realnameOK.value = data?.realname_ok !== false
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载分类失败')
  } finally {
    categoryLoading.value = false
  }
}

async function loadBindingOptions() {
  if (orderOptions.value.length === 0 && !orderLoading.value) {
    orderLoading.value = true
    try {
      const response = await getMyOrders({ page: 1, page_size: 50 })
      const items = response?.data?.items ?? []
      orderOptions.value = items.map((item) => ({
        label: `${item.order_no} · ${item.product_name}`,
        value: item.id,
      }))
    } catch {
      // 订单列表拉取失败不阻断提交，用户可仅填实例
    } finally {
      orderLoading.value = false
    }
  }
  if (instanceOptions.value.length === 0 && !instanceLoading.value) {
    instanceLoading.value = true
    try {
      const response = await listInstances()
      const items = response?.data?.items ?? []
      instanceOptions.value = items.map((item) => ({
        label: `${item.name}（${item.instance_id || item.id}）`,
        value: item.id,
      }))
    } catch {
      // 实例列表拉取失败同上
    } finally {
      instanceLoading.value = false
    }
  }
}

function handleCategoryChange() {
  formData.order_id = undefined
  formData.instance_id = undefined
  if (selectedCategory.value?.require_binding) {
    void loadBindingOptions()
  }
}

// 提交工单
async function onSubmit({ validateResult }: { validateResult: boolean }) {
  if (validateResult !== true) return
  if (selectedCategory.value?.require_binding && !formData.order_id && !formData.instance_id) {
    MessagePlugin.warning('该分类需关联您本人的订单或实例，请至少选择一项')
    return
  }
  submitting.value = true
  try {
    const { data } = await createMyTicket({
      title: formData.title.trim(),
      description: formData.description.trim(),
      category: formData.category,
      priority: formData.priority,
      order_id: formData.order_id,
      instance_id: formData.instance_id,
    })
    MessagePlugin.success('工单提交成功，我们会尽快处理')
    if (data?.id) {
      router.replace(`/support/tickets/${data.id}`)
    } else {
      router.replace('/support/tickets')
    }
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '提交失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

function goBack() {
  router.back()
}

onMounted(loadCategories)
</script>

<style scoped>
.ticket-create-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 860px;
}

/* 页头 */
.create-hero {
  background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
  border-radius: 16px;
  padding: 20px 28px;
  color: #fff;
  display: flex;
  align-items: center;
  gap: 12px;
}

.back-btn {
  color: #fff;
}

.hero-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-title {
  font-size: 18px;
  font-weight: 700;
}

.hero-desc {
  font-size: 13px;
  opacity: 0.85;
}

/* 表单面板 */
.create-panel {
  background: #fff;
  border-radius: 12px;
  padding: 28px 32px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 8px;
}

.category-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.category-option__lock {
  font-size: 12px;
  color: #d97706;
}

.category-help {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
  color: #94a3b8;
  line-height: 1.6;
}

.category-help__tip {
  color: #d97706;
}

.binding-alert {
  margin-bottom: 16px;
}

.attachment-tip {
  margin: 12px 0 0;
  font-size: 12px;
  color: #94a3b8;
}
</style>
