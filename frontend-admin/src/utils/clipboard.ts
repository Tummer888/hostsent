import { MessagePlugin } from 'tdesign-vue-next'

/**
 * 复制文本到剪贴板并给出统一提示。
 *
 * 先走异步 Clipboard API；移动端浏览器常未授权 `clipboard-write`，会抛
 * "Write permission denied"，此时回退到 `execCommand('copy')`（隐藏 textarea +
 * 手动选区）。回退路径会临时抢走选区，因此复制后把用户原来的选区还回去 ——
 * 在正文里选中关键字再点复制时，选区不该被清掉。
 *
 * 此前这段逻辑在实例详情、资金流水、第三方登录、用户列表四处各写一遍，
 * 行为已经出现分叉（有的没做降级、有的降级后不还原选区）。
 */
export async function copyText(value: string, label: string): Promise<void> {
  if (!value) return

  try {
    await navigator.clipboard.writeText(value)
    MessagePlugin.success(`${label}已复制`)
    return
  } catch {
    // 继续走降级方案
  }

  try {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.setAttribute('readonly', '')
    textarea.style.position = 'fixed'
    textarea.style.top = '-9999px'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)

    const selection = document.getSelection()
    const previousRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null

    textarea.select()
    textarea.setSelectionRange(0, value.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)

    if (previousRange && selection) {
      selection.removeAllRanges()
      selection.addRange(previousRange)
    }

    if (ok) {
      MessagePlugin.success(`${label}已复制`)
    } else {
      MessagePlugin.error(`${label}复制失败`)
    }
  } catch {
    MessagePlugin.error(`${label}复制失败`)
  }
}
