import { onBeforeUnmount, onMounted, ref } from 'vue'

/**
 * 视口断点侦测：窄屏（<768px）视为移动端。
 * 与 users/accounts/list 的 isMobile 语义保持一致。
 */
export function useIsMobile(breakpoint = 768) {
  // 同步初始化：调用方（setup 阶段）可能在静态列定义里读取 isMobile.value，
  // 必须在挂载前就拿到正确视口，否则列宽恒为桌面值。
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
