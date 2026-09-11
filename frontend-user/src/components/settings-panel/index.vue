<template>
  <t-drawer
    :visible="visible"
    placement="right"
    :size="drawerSize"
    :footer="false"
    destroy-on-close
    @close="emit('update:visible', false)"
  >
    <template #header>
      <div class="settings-header">
        <span class="settings-header__title">主题设置</span>
        <t-button variant="text" size="small" theme="primary" @click="settings.reset()">
          <template #icon><RotateIcon size="14" /></template>
          恢复默认
        </t-button>
      </div>
    </template>

    <div class="settings-body">
      <!-- 主题色 -->
      <section class="settings-section">
        <h4 class="settings-section__title">主题色</h4>
        <div class="swatch-grid">
          <button
            v-for="c in PRESET_THEME_COLORS"
            :key="c.value"
            class="swatch"
            :class="{ 'is-active': settings.themeColor === c.value }"
            :style="{ background: c.value }"
            :title="c.name"
            :aria-label="`主题色 ${c.name}`"
            @click="setColor(c.value)"
          >
            <CheckIcon v-if="settings.themeColor === c.value" size="14" />
          </button>
          <label class="swatch swatch--custom" title="自定义颜色">
            <AddIcon size="16" />
            <input
              type="color"
              class="swatch__picker"
              :value="settings.themeColor"
              @input="setColor(($event.target as HTMLInputElement).value)"
            />
          </label>
        </div>
      </section>

      <!-- 色彩方案 -->
      <section class="settings-section">
        <h4 class="settings-section__title">色彩方案</h4>
        <t-radio-group
          :value="settings.colorScheme"
          variant="default-filled"
          @change="(v: unknown) => settings.update({ colorScheme: v as ColorScheme })"
        >
          <t-radio-button value="light">浅色</t-radio-button>
          <t-radio-button value="dark">深色</t-radio-button>
          <t-radio-button value="auto">跟随系统</t-radio-button>
        </t-radio-group>
      </section>

      <!-- 圆角 -->
      <section class="settings-section">
        <h4 class="settings-section__title">
          圆角
          <span class="settings-section__hint">{{ settings.radius }}%</span>
        </h4>
        <t-slider
          :value="settings.radius"
          :min="50"
          :max="200"
          :step="10"
          @change="(v: unknown) => settings.update({ radius: Number(v) })"
        />
      </section>

      <!-- 色弱模式 -->
      <section class="settings-section">
        <div class="settings-row">
          <span>色弱模式</span>
          <t-switch
            :value="settings.colorWeak"
            @change="(v: unknown) => settings.update({ colorWeak: Boolean(v) })"
          />
        </div>
      </section>
    </div>
  </t-drawer>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { AddIcon, CheckIcon, RotateIcon } from 'tdesign-icons-vue-next'

import { useSettingsStore } from '@/store/modules/settings'
import type { ColorScheme } from '@/store/modules/settings'
import { PRESET_THEME_COLORS, normalizeHex } from '@/utils/theme'

defineOptions({ name: 'SettingsPanel' })

defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const settings = useSettingsStore()

// 抽屉宽度自适应：小屏用百分比避免溢出遮挡，桌面固定 380px
const windowWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1280)
function onWindowResize() {
  windowWidth.value = window.innerWidth
}
onMounted(() => window.addEventListener('resize', onWindowResize))
onUnmounted(() => window.removeEventListener('resize', onWindowResize))
const drawerSize = computed(() => (windowWidth.value <= 640 ? '92%' : '380px'))

function setColor(hex: string) {
  settings.update({ themeColor: normalizeHex(hex, settings.themeColor) })
}
</script>

<style scoped>
.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.settings-header__title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.settings-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.settings-section {
  padding: 12px 0;
  border-bottom: 1px solid #f1f5f9;
}

.settings-section:last-child {
  border-bottom: none;
}

.settings-section__title {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings-section__hint {
  font-weight: 400;
  font-size: 12px;
  color: #94a3b8;
}

.settings-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 36px;
  font-size: 13px;
  color: #475569;
}

/* 主题色板 */
.swatch-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
}

.swatch {
  position: relative;
  width: 44px;
  height: 44px;
  border-radius: 10px;
  border: 2px solid transparent;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  padding: 0;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.swatch:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.18);
}

.swatch.is-active {
  border-color: #0f172a;
  box-shadow: 0 0 0 2px #ffffff inset;
}

.swatch--custom {
  background: conic-gradient(#ef4444, #f59e0b, #22c55e, #06b6d4, #6366f1, #db2777, #ef4444);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  overflow: hidden;
}

.swatch__picker {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  width: 100%;
  height: 100%;
}

/* 深色适配 */
.dark .settings-header__title {
  color: #e5e7eb;
}

.dark .settings-section {
  border-bottom-color: #262626;
}

.dark .settings-section__title {
  color: #cbd5e1;
}

.dark .settings-row {
  color: #94a3b8;
}
</style>
