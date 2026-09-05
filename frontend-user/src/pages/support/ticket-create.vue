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
          <t-select v-model="formData.category" placeholder="请选择问题分类" :loading="categoryLoading">
            <t-option
              v-for="item in categories"
              :key="item.id"
              :value="item.code"
              :label="item.name"
            />
          </t-select>
        </t-form-item>

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
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ChevronLeftIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type FormInstanceFunctions, type FormProps } from 'tdesign-vue-next'

import { createMyTicket, getTicketCategories, type TicketCategoryInfo } from '@/api/support'
import { ticketPriorityOptions } from '@/pages/support/constants'

defineOptions({ name: 'UserTicketCreate' })

const router = useRouter()

const formRef = ref<FormInstanceFunctions>()

// 表单数据（priority 默认 medium，与后端约定一致）
const formData = reactive({
  title: '',
  category: '',
  priority: 'medium',
  description: '',
})

// 表单校验规则（与后端 binding 约束对齐）
const rules: FormProps['rules'] = {
  title: [{ required: true, message: '请输入工单标题', trigger: 'blur' }],
  category: [{ required: true, message: '请选择问题分类', trigger: 'change' }],
  description: [{ required: true, message: '请描述您的问题', trigger: 'blur' }],
}

const categories = ref<TicketCategoryInfo[]>([])
const categoryLoading = ref(false)
const submitting = ref(false)

// 加载可用工单分类
async function loadCategories() {
  categoryLoading.value = true
  try {
    const { data } = await getTicketCategories()
    if (data) categories.value = data
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载分类失败')
  } finally {
    categoryLoading.value = false
  }
}

// 提交工单
async function onSubmit({ validateResult }: { validateResult: boolean }) {
  if (validateResult !== true) return
  submitting.value = true
  try {
    const { data } = await createMyTicket({
      title: formData.title.trim(),
      description: formData.description.trim(),
      category: formData.category,
      priority: formData.priority,
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
</style>
