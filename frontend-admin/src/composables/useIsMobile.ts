import { onBeforeUnmount, onMounted, ref } from 'vue'

/**
 * 视口断点侦测：窄屏（<768px）视为移动端。
 * 与 users/accounts/list 的 isMobile 语义保持一致。
 */
export function useIsMobile(breakpoint = 768) {
  const isMobile = ref(false)

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
