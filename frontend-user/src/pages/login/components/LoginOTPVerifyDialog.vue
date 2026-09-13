<template>
  <t-dialog
    :visible="visible"
    :header="dialogTitle"
    :close-on-overlay-click="false"
    :confirm-btn="{ content: '验证并登录', loading: submitting }"
    :cancel-btn="{ content: '返回', disabled: submitting }"
    width="400px"
    @update:visible="onVisibleChange"
    @confirm="onSubmit"
    @cancel="onCancel"
  >
    <div class="otp-dialog">
      <p class="otp-dialog__tip">
        为保护账号安全，请输入发送至
        <strong>{{ otpTargetMasked || '你绑定的账号' }}</strong>
        的{{ channelLabel }}验证码。
      </p>

      <div class="otp-dialog__cells" @paste="onPaste">
        <input
          v-for="(_, i) in cells"
          :key="i"
          :ref="(el) => setCellRef(el, i)"
          v-model="cells[i]"
          class="otp-cell"
          type="text"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="1"
          :disabled="submitting"
          :aria-label="`第 ${i + 1} 位验证码`"
          @input="onCellInput(i, $event)"
          @keydown="onCellKeydown(i, $event)"
        />
      </div>

      <div class="otp-dialog__foot">
        <span v-if="expireHint" class="otp-dialog__expire">{{ expireHint }}</span>
        <button
          type="button"
          class="otp-dialog__resend"
          :disabled="countdown > 0 || submitting"
          @click="onResend"
        >
          {{ countdown > 0 ? `重新发送（${countdown}s）` : '重新发送验证码' }}
        </button>
      </div>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
/**
 * 登录二次验证弹窗（doc91 §4.6/§5.3）。
 *
 * 6 格分隔输入：自动前进、退格回退、粘贴整串自动拆分。
 * 「重新发送」不是另开端点：后端重发就是再走一次密码登录（重新下发 OTP 并
 * 换发新的 otp_token），因此发事件交给父组件重放密码步骤。
 */
import { computed, nextTick, ref, watch } from 'vue'

import { MessagePlugin } from 'tdesign-vue-next'

defineOptions({ name: 'LoginOTPVerifyDialog' })

const props = withDefaults(
  defineProps<{
    visible: boolean
    otpToken: string
    otpChannel?: string
    otpTargetMasked?: string
    otpExpireIn?: number
    submitting?: boolean
  }>(),
  { otpChannel: '', otpTargetMasked: '', otpExpireIn: 0, submitting: false },
)

const emit = defineEmits<{
  'update:visible': [value: boolean]
  verify: [code: string]
  resend: []
}>()

const cells = ref<string[]>(['', '', '', '', '', ''])
const cellRefs = ref<HTMLInputElement[]>([])
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const channelLabel = computed(() => (props.otpChannel === 'sms' ? '短信' : '邮箱'))
const dialogTitle = computed(() => `二次验证 · ${channelLabel.value}`)
const expireHint = computed(() =>
  props.otpExpireIn > 0 ? `验证码 ${Math.round(props.otpExpireIn / 60)} 分钟内有效` : '',
)
const code = computed(() => cells.value.join(''))

function setCellRef(el: unknown, index: number) {
  if (el) cellRefs.value[index] = el as HTMLInputElement
}

function startCountdown() {
  stopCountdown()
  countdown.value = 60
  timer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) stopCountdown()
  }, 1000)
}

function stopCountdown() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function resetCells() {
  cells.value = ['', '', '', '', '', '']
  nextTick(() => cellRefs.value[0]?.focus())
}

function sanitize(raw: string): string {
  return (raw || '').replace(/\D/g, '').slice(0, 6)
}

function fillFrom(start: number, digits: string) {
  const chars = digits.split('')
  for (let i = 0; i < chars.length && start + i < 6; i += 1) {
    cells.value[start + i] = chars[i]
  }
  cellRefs.value[Math.min(start + chars.length, 5)]?.focus()
}

function onCellInput(index: number, event: Event) {
  const el = event.target as HTMLInputElement
  const digits = sanitize(el.value)
  if (digits.length > 1) {
    fillFrom(index, digits)
    return
  }
  cells.value[index] = digits
  if (digits && index < 5) cellRefs.value[index + 1]?.focus()
}

function onCellKeydown(index: number, event: KeyboardEvent) {
  const el = event.target as HTMLInputElement
  if (event.key === 'Backspace' && !el.value && index > 0) {
    // 空格回退：清掉前一格并聚焦，符合一次性验证码的输入习惯。
    cells.value[index - 1] = ''
    cellRefs.value[index - 1]?.focus()
  }
  if (event.key === 'Enter') onSubmit()
}

function onPaste(event: ClipboardEvent) {
  const text = sanitize(event.clipboardData?.getData('text') || '')
  if (!text) return
  event.preventDefault()
  fillFrom(0, text)
}

function onSubmit() {
  if (props.submitting) return
  if (code.value.length !== 6) {
    MessagePlugin.warning('请输入 6 位验证码')
    return
  }
  emit('verify', code.value)
}

function onResend() {
  if (countdown.value > 0) return
  emit('resend')
  resetCells()
  startCountdown()
}

function onCancel() {
  emit('update:visible', false)
}

function onVisibleChange(value: boolean) {
  emit('update:visible', value)
}

// 弹窗打开：清空输入、聚焦首格、起 60 秒重发倒计时（首次发送刚刚完成）。
watch(
  () => props.visible,
  (open) => {
    if (open) {
      resetCells()
      startCountdown()
    } else {
      stopCountdown()
    }
  },
)

// 令牌被换发（重发成功）时清空已输入内容并提示。
watch(
  () => props.otpToken,
  (next, prev) => {
    if (next && prev && next !== prev) {
      resetCells()
      MessagePlugin.success('验证码已重新发送')
    }
  },
)
</script>

<style scoped>
.otp-dialog {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.otp-dialog__tip {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: #475569;
}

.otp-dialog__tip strong {
  color: #0f172a;
}

.otp-dialog__cells {
  display: flex;
  justify-content: center;
  gap: 10px;
}

/* 6 格分隔输入：固定正方形、居中大字号，便于一次性读入 6 位数字。 */
.otp-cell {
  width: 44px;
  height: 50px;
  text-align: center;
  font-size: 22px;
  font-weight: 600;
  font-family: 'Courier New', Consolas, monospace;
  color: #0f172a;
  border: 1px solid rgba(0, 82, 217, 0.2);
  border-radius: 10px;
  background: #f8fafc;
  outline: none;
  transition: border-color 0.2s, box-shadow 0.2s, background-color 0.2s;
}

.otp-cell:focus {
  border-color: #0052d9;
  background: #fff;
  box-shadow: 0 0 0 3px rgba(0, 82, 217, 0.12);
}

.otp-cell:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.otp-dialog__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: #94a3b8;
}

.otp-dialog__resend {
  background: transparent;
  border: 0;
  padding: 0;
  font-size: 12px;
  color: #0052d9;
  cursor: pointer;
}

.otp-dialog__resend:disabled {
  color: #94a3b8;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .otp-cell {
    width: 38px;
    height: 44px;
    font-size: 20px;
  }
}
</style>
