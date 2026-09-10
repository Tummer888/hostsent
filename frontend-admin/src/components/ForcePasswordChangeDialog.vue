<template>
  <!-- 首次登录/被重置密码后强制改密：不可关闭、不可点遮罩/ESC 关闭 -->
  <t-dialog
    v-model:visible="visible"
    header="请修改初始密码"
    width="440px"
    :close-btn="false"
    :close-on-overlay-click="false"
    :close-on-esc-keydown="false"
    :cancel-btn="null"
    :confirm-btn="null"
    :destroy-on-close="false"
  >
    <div class="force-pwd">
      <p class="force-pwd__tip">
        当前账号使用的是初始密码，为保障安全请先修改密码后再继续使用。
      </p>
      <t-form
        ref="formRef"
        :data="formData"
        :rules="rules"
        label-align="top"
        colonless
        @submit="onSubmit"
      >
        <t-form-item label="当前密码" name="oldPassword">
          <t-input v-model="formData.oldPassword" type="password" size="large" placeholder="请输入当前密码" />
        </t-form-item>
        <t-form-item label="新密码" name="newPassword">
          <t-input v-model="formData.newPassword" type="password" size="large" placeholder="8-64 位，建议含字母与数字" />
        </t-form-item>
        <t-form-item label="确认新密码" name="confirmPassword">
          <t-input
            v-model="formData.confirmPassword"
            type="password"
            size="large"
            placeholder="请再次输入新密码"
            @enter="onSubmitClick"
          />
        </t-form-item>
      </t-form>
    </div>
    <template #footer>
      <t-button theme="primary" :loading="submitting" @click="onSubmitClick">确认修改</t-button>
    </template>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import type { FormInstanceFunctions, FormRule } from 'tdesign-vue-next'

import { useRouter } from 'vue-router'

import { useUserStore } from '@/store'

defineOptions({ name: 'ForcePasswordChangeDialog' })

const userStore = useUserStore()
const router = useRouter()
const formRef = ref<FormInstanceFunctions | null>(null)
const submitting = ref(false)

const formData = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const visible = computed({
  get: () => userStore.mustChangePassword,
  set: () => {
    /* 由 store 状态控制，禁止外部直接关闭 */
  },
})

const rules: Record<string, FormRule[]> = {
  oldPassword: [{ required: true, message: '请输入当前密码', type: 'error' }],
  newPassword: [
    { required: true, message: '请输入新密码', type: 'error' },
    { min: 8, max: 64, message: '密码长度为 8-64 个字符', type: 'error' },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', type: 'error' },
    {
      validator: (val: string) => val === formData.newPassword,
      message: '两次输入的密码不一致',
      type: 'error',
    },
  ],
}

watch(
  () => userStore.mustChangePassword,
  (need) => {
    if (!need) {
      formData.oldPassword = ''
      formData.newPassword = ''
      formData.confirmPassword = ''
    }
  },
)

async function onSubmit() {
  if (formData.newPassword === formData.oldPassword) {
    MessagePlugin.warning('新密码不能与当前密码相同')
    return
  }
  submitting.value = true
  try {
    await userStore.changePassword(formData.oldPassword, formData.newPassword)
    MessagePlugin.success('密码修改成功')
    // 改密后原 token 可能失效，回到登录页重新登录更稳妥
    await userStore.logout()
    await router.replace('/login')
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '密码修改失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

async function onSubmitClick() {
  await formRef.value?.submit?.()
}
</script>

<style scoped>
.force-pwd__tip {
  margin: 0 0 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}
</style>
