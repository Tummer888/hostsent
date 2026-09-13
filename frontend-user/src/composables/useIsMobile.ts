import { onBeforeUnmount, onMounted, ref } from 'vue'

/**
 * 视口断点侦测：窄屏（<768px）视为移动端。
 * 与 frontend-admin/src/composables/useIsMobile.ts 行为一致，便于两端列表页写法统一。
 */
export function useIsMobile(breakpoint = 768) {
  // 同步初始化：列定义等静态结构可能在 setup 阶段就读取 isMobile.value，
  // 必须在挂载前拿到正确视口，否则列宽恒为桌面值。
  const isMobile = ref(typeof window !== 'undefined' && window.innerWidth < breakpoint)

  function sync() {
    if (typeof window === 'undefined') return
    isMobile.value = window.innerWidth < breakpoint
  }

  onMounted(() => {
    sync()
    window.addEventListener('resize', sync)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('resize', sync)
  })

  return { isMobile, syncViewportState: sync }
}
