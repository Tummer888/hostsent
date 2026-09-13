<template>
  <div class="captcha-image">
    <button
      type="button"
      class="captcha-image__box"
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
        height="46"
        loading="eager"
      />
      <span v-else class="captcha-image__placeholder">{{ loading ? '加载中…' : '换一张' }}</span>
    </button>
    <!-- 降级提示：接口失败时不再阻断用户，与后端「Redis 降级放行」策略一致（doc91 §3.5）。 -->
    <span v-if="failed" class="captcha-image__degraded">验证码服务暂不可用，本次可跳过</span>
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

/** captcha_key：随登录/注册请求提交给后端。 */
const captchaKey = defineModel<string>('key', { default: '' })
/** 用户看到的验证码答案。 */
const captchaCode = defineModel<string>('code', { default: '' })

const imageBase64 = ref('')
const loading = ref(false)
const failed = ref(false)

const tip = computed(() => (failed.value ? '验证码服务暂不可用' : '点击刷新验证码'))

/** 拉取新挑战。旧 key 作废（服务端一次性），因此必须同时清空已输入的答案。 */
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
.captcha-image {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex-shrink: 0;
}

.captcha-image__box {
  width: 110px;
  height: 46px;
  border-radius: 12px;
  background: linear-gradient(135deg, #f0f5ff 0%, #e0ecff 50%, #e8f0ff 100%);
  border: 1px solid rgba(0, 82, 217, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  user-select: none;
  padding: 0;
  overflow: hidden;
  transition: border-color 0.25s ease, transform 0.25s ease;
}

.captcha-image__box:hover:not(:disabled) {
  border-color: #0052d9;
  transform: scale(1.02);
}

.captcha-image__box:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.captcha-image__box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.captcha-image__placeholder {
  font-size: 12px;
  color: #64748b;
  letter-spacing: 0.08em;
}

.captcha-image__degraded {
  max-width: 110px;
  font-size: 11px;
  line-height: 1.4;
  text-align: center;
  color: #d97706;
}

@media (max-width: 480px) {
  .captcha-image,
  .captcha-image__box {
    width: 100%;
  }
}
</style>
