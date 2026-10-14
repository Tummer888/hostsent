<template>
  <div class="interval-field">
    <t-input-number v-model="amount" :min="1" theme="column" class="interval-field__num" />
    <t-select v-model="unit" :options="unitOptions" class="interval-field__unit" @change="emitChange" />
    <!-- 快捷档位：常用节奏一点即填，免去"秒 → 分钟"的心算 -->
    <t-space :size="4" class="interval-field__presets">
      <t-link
        v-for="preset in presets"
        :key="preset.seconds"
        :theme="seconds === preset.seconds ? 'primary' : 'default'"
        hover="color"
        @click="applyPreset(preset.seconds)"
      >
        {{ preset.label }}
      </t-link>
    </t-space>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

/**
 * 间隔输入（数值 + 单位）。
 *
 * 为什么不用裸秒数：原先把「同步间隔」「全量对账周期」都做成秒输入框，
 * 想填「6 小时」得先算 21600——这类换算每次配置都要做一遍，
 * 既慢又容易填错一个数量级（3600 与 36000 只差一个 0，后果是同步频率差十倍）。
 * 这里改成「数值 + 单位」，并给几个常用档位一点即填。
 *
 * 对外仍以秒为单位（v-model:seconds）：后端与已有配置存的就是秒，
 * 单位只存在于输入体验这一层，不引入第二套时间口径。
 */
const props = withDefaults(
  defineProps<{
    /** 秒数（唯一口径）。 */
    seconds: number
    /** 最小秒数（校验下限：间隔 30 秒、全量对账 300 秒）。 */
    minSeconds?: number
  }>(),
  { minSeconds: 30 },
)

const emit = defineEmits<{ (e: 'update:seconds', value: number): void }>()

const unitOptions = [
  { label: '秒', value: 1 },
  { label: '分钟', value: 60 },
  { label: '小时', value: 3600 },
  { label: '天', value: 86400 },
]

const presets = [
  { label: '5 分钟', seconds: 300 },
  { label: '15 分钟', seconds: 900 },
  { label: '1 小时', seconds: 3600 },
  { label: '6 小时', seconds: 21600 },
  { label: '1 天', seconds: 86400 },
]

// 拆分：把秒数拆成「最合适的单位 + 数值」，避免 21600 显示成「21600 秒」。
// 从大到小试，能整除就用该单位，保证 3600 显示为「1 小时」而不是「60 分钟」。
function splitSeconds(total: number): { amount: number; unit: number } {
  const value = Math.max(props.minSeconds, Math.floor(total || props.minSeconds))
  for (const u of [86400, 3600, 60]) {
    if (value % u === 0) return { amount: value / u, unit: u }
  }
  return { amount: value, unit: 1 }
}

const initial = splitSeconds(props.seconds)
const amount = ref(initial.amount)
const unit = ref(initial.unit)

/** 当前输入换算出的秒数。 */
const totalSeconds = computed(() => amount.value * unit.value)

function emitChange() {
  // 下限保护：单位切到「秒」而数值填 1 时，30 秒下限仍然成立。
  const value = Math.max(props.minSeconds, totalSeconds.value || props.minSeconds)
  emit('update:seconds', value)
}

function applyPreset(value: number) {
  const s = splitSeconds(value)
  amount.value = s.amount
  unit.value = s.unit
  emit('update:seconds', value)
}

// 外部值变化（如打开不同渠道的配置弹窗）时同步回拆解结果。
watch(
  () => props.seconds,
  (next) => {
    if (next === totalSeconds.value) return
    const s = splitSeconds(next)
    amount.value = s.amount
    unit.value = s.unit
  },
)

// 数值改动也要上抛（t-input-number 变更不走 select 的 change）。
watch(amount, () => emitChange())
</script>

<style scoped lang="css">
.interval-field {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.interval-field__num {
  width: 110px;
  flex-shrink: 0;
}

.interval-field__unit {
  width: 90px;
  flex-shrink: 0;
}

.interval-field__presets {
  margin-left: 4px;
  font-size: 12px;
}
</style>
