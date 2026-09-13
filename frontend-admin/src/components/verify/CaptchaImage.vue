<template>
  <div class="verify-captcha">
    <button
      type="button"
      class="verify-captcha__image"
      :title="tip"
      :aria-label="tip"
      :disabled="disabled"
      @click="refresh"
    >
      <img
        v-if="imageBase64"
        :src="imageBase64"
        alt="图形验证码"
        width="110"
        height="40"
        loading="eager"
      />
      <span v-else class="verify-captcha__placeholder">{{ loading ? '加载中…' : '换一张' }}</span>
    </button>
    <div class="verify-captcha__hint">
      <button type="button" class="verify-captcha__link" :disabled="disabled" @click="refresh">
        看不清？换一张
      </button>
    </div>
    <!-- 降级提示：接口失败时不再阻断用户，与后端「Redis 降级放行」策略一致（doc91 §3.5）。 -->
    <div v-if="failed" class="verify-captcha__degraded">
      验证码服务暂不可用，本次可跳过校验
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 图形验证码组件（doc91 §3.5）：admin / user 两侧各一份，逻辑完全一致。
 *
 * 安全前提：答案只在服务端（Redis）保存，本组件只持有 key 与图片。
 * 这里不做任何「前端判断答案」的逻辑——那样等于没验证。
 */
import { computed, onMounted, ref } from 'vue'

import { getImageChallenge } from '@/api/public'

defineOptions({ name: 'CaptchaImage' })

const props = withDefaults(
  defineProps<{
    /** 场景码，决定用哪个场景的策略与服务商 */
    scene: string
    /** 图形难度（策略里配置；不传用服务端默认 normal） */
    level?: string
    disabled?: boolean
  }>(),
  { level: '', disabled: false },
)

/**
 * captcha_key：随登录/注册请求提交给后端。
 * code：用户看到的验证码答案（6 位内随意字符，后端不区分大小写由服务端处理）。
 */
const captchaKey = defineModel<string>('key', { default: '' })
const captchaCode = defineModel<string>('code', { default: '' })

const imageBase64 = ref('')
const loading = ref(false)
const failed = ref(false)

const tip = computed(() => (failed.value ? '验证码服务暂不可用' : '点击刷新验证码'))

/** 拉取新挑战。旧 key 作废（服务端一次性），因此必须同时清空用户已输入的答案。 */
async function refresh() {
  if (props.disabled || loading.value) return
  loading.value = true
  try {
    const res = await getImageChallenge(props.scene, props.level || undefined)
    imageBase64.value = res.image_base64 || ''
    captchaKey.value = res.captcha_key || ''
    captchaCode.value = ''
    failed.value = !res.image_base64
  } catch {
    // 接口失败：清空 key，让后端按「无图形码」处理；同时给出可见提示。
    failed.value = true
    imageBase64.value = ''
    captchaKey.value = ''
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

defineExpose({ refresh })
</script>

<style scoped>
.verify-captcha {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.verify-captcha__image {
  width: 110px;
  height: 40px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--color-border, #dcdfe6);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8fafc;
  user-select: none;
  padding: 0;
  transition: border-color 0.2s, transform 0.2s;
}

.verify-captcha__image:hover:not(:disabled) {
  border-color: var(--td-brand-color-4, #0052d9);
  transform: translateY(-1px);
}

.verify-captcha__image:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.verify-captcha__image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.verify-captcha__placeholder {
  font-size: 11px;
  color: var(--color-muted-foreground, #94a3b8);
  letter-spacing: 0.08em;
}

.verify-captcha__hint {
  line-height: 1;
}

.verify-captcha__link {
  background: transparent;
  border: 0;
  padding: 0;
  font-size: 11px;
  color: var(--td-brand-color, #0052d9);
  cursor: pointer;
}

.verify-captcha__link:disabled {
  color: var(--color-muted-foreground, #94a3b8);
  cursor: not-allowed;
}

.verify-captcha__degraded {
  max-width: 110px;
  font-size: 11px;
  line-height: 1.4;
  text-align: center;
  color: #d97706;
}
</style>
