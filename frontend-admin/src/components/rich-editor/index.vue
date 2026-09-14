<template>
  <div class="rich-editor" :class="{ 'rich-editor--disabled': disabled }">
    <div v-if="editor" class="rich-editor__toolbar">
      <button
        v-for="item in toolbarItems"
        :key="item.action"
        type="button"
        class="rich-editor__btn"
        :class="{ 'is-active': item.isActive() }"
        :title="item.title"
        :disabled="disabled"
        @click="item.run()"
      >
        <span v-html="item.icon" />
      </button>
      <button
        type="button"
        class="rich-editor__btn"
        title="插入链接"
        :disabled="disabled"
        @click="applyLink"
      >
        <span v-html="ICONS.link" />
      </button>
      <button
        type="button"
        class="rich-editor__btn"
        title="插入图片（外链地址）"
        :disabled="disabled"
        @click="applyImage"
      >
        <span v-html="ICONS.image" />
      </button>
    </div>
    <EditorContent class="rich-editor__content" :editor="editor" />
    <div class="rich-editor__footer">
      <span class="rich-editor__hint">{{ hint }}</span>
      <span class="rich-editor__count">{{ charCount }} 字</span>
    </div>
  </div>
</template>

<script setup lang="ts">
// 项目内封装的富文本编辑器（tiptap）。
//
// 为什么自己封装而不是各页直接用 tiptap：
//   1. 扩展集（标题/列表/表格/链接/图片）与工具栏是**一套产品决策**，散在各页会漂移；
//   2. 公告编辑页与内容文章编辑页共用同一份 HTML 契约（服务端 bluemonday 白名单），
//      两处各装一套会出现「一边能插表格、一边被过滤掉」的困惑；
//   3. 输出必须是 sanitize 白名单里的标签，这里限制了扩展集，就降低了「编辑所见，
//      入库后被过滤」的概率。
//
// v-model 契约：`modelValue` / `update:modelValue` 都是 HTML 字符串。
// 空文档在 tiptap 里是 `<p></p>`，这里归一化成空串，避免「看起来没写内容但校验不通过」。
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Link from '@tiptap/extension-link'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import Table from '@tiptap/extension-table'
import TableRow from '@tiptap/extension-table-row'
import TableCell from '@tiptap/extension-table-cell'
import TableHeader from '@tiptap/extension-table-header'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    placeholder?: string
    disabled?: boolean
    /** 字符数上限提示；0 表示不限制 */
    maxlength?: number
  }>(),
  { modelValue: '', placeholder: '请输入正文', disabled: false, maxlength: 0 },
)

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const ICONS: Record<string, string> = {
  bold: 'B',
  italic: 'I',
  strike: 'S',
  h2: 'H2',
  h3: 'H3',
  bulletList: '•',
  orderedList: '1.',
  blockquote: '❝',
  codeBlock: '</>',
  hr: '—',
  link: '🔗',
  image: '🖼',
  clear: '⌫',
}

const editor = useEditor({
  content: normalizeIn(props.modelValue),
  editable: !props.disabled,
  extensions: [
    // heading 只开 2–4 级：文档正文用 h1 会让页面出现两个主标题（页面标题已占 h1），
    // 对 SEO 与无障碍都是错误结构。
    StarterKit.configure({ heading: { levels: [2, 3, 4] } }),
    Link.configure({
      openOnClick: false,
      autolink: true,
      // 服务端净化会强制补 rel="nofollow noreferrer"，这里保持一致，
      // 让「编辑器里看到的」与「门户上渲染出来的」是同一个东西。
      HTMLAttributes: { rel: 'nofollow noreferrer', target: '_blank' },
    }),
    Image.configure({ inline: false, allowBase64: false }),
    Placeholder.configure({ placeholder: () => props.placeholder }),
    Table.configure({ resizable: false }),
    TableRow,
    TableHeader,
    TableCell,
  ],
  onUpdate: ({ editor: instance }) => {
    emit('update:modelValue', normalizeOut(instance.getHTML()))
  },
})

watch(
  () => props.modelValue,
  (value) => {
    if (!editor.value) return
    const current = normalizeOut(editor.value.getHTML())
    const next = normalizeIn(value)
    // 只在外部值与编辑器现值真的不同步时才 setContent：否则每次输入都会
    // 重置光标到文档开头（这是受控组件包富文本编辑器最经典的坑）。
    if (current !== next) {
      editor.value.commands.setContent(next, false)
    }
  },
)

watch(
  () => props.disabled,
  (value) => editor.value?.setEditable(!value),
)

onBeforeUnmount(() => editor.value?.destroy())

const charCount = computed(() => plainText(normalizeIn(props.modelValue)).length)

const hint = computed(() =>
  props.maxlength > 0 ? `支持标题、列表、表格、链接与图片，上限 ${props.maxlength} 字` : '支持标题、列表、表格、链接与图片',
)

function run(command: () => void) {
  if (props.disabled) return
  command()
}

interface ToolbarItem {
  action: string
  title: string
  icon: string
  isActive: () => boolean
  run: () => void
}

const toolbarItems = computed<ToolbarItem[]>(() => [
  {
    action: 'bold',
    title: '加粗',
    icon: ICONS.bold,
    isActive: () => !!editor.value?.isActive('bold'),
    run: () => run(() => editor.value?.chain().focus().toggleBold().run()),
  },
  {
    action: 'italic',
    title: '斜体',
    icon: ICONS.italic,
    isActive: () => !!editor.value?.isActive('italic'),
    run: () => run(() => editor.value?.chain().focus().toggleItalic().run()),
  },
  {
    action: 'strike',
    title: '删除线',
    icon: ICONS.strike,
    isActive: () => !!editor.value?.isActive('strike'),
    run: () => run(() => editor.value?.chain().focus().toggleStrike().run()),
  },
  {
    action: 'h2',
    title: '二级标题',
    icon: ICONS.h2,
    isActive: () => !!editor.value?.isActive('heading', { level: 2 }),
    run: () => run(() => editor.value?.chain().focus().toggleHeading({ level: 2 }).run()),
  },
  {
    action: 'h3',
    title: '三级标题',
    icon: ICONS.h3,
    isActive: () => !!editor.value?.isActive('heading', { level: 3 }),
    run: () => run(() => editor.value?.chain().focus().toggleHeading({ level: 3 }).run()),
  },
  {
    action: 'bulletList',
    title: '无序列表',
    icon: ICONS.bulletList,
    isActive: () => !!editor.value?.isActive('bulletList'),
    run: () => run(() => editor.value?.chain().focus().toggleBulletList().run()),
  },
  {
    action: 'orderedList',
    title: '有序列表',
    icon: ICONS.orderedList,
    isActive: () => !!editor.value?.isActive('orderedList'),
    run: () => run(() => editor.value?.chain().focus().toggleOrderedList().run()),
  },
  {
    action: 'blockquote',
    title: '引用',
    icon: ICONS.blockquote,
    isActive: () => !!editor.value?.isActive('blockquote'),
    run: () => run(() => editor.value?.chain().focus().toggleBlockquote().run()),
  },
  {
    action: 'codeBlock',
    title: '代码块',
    icon: ICONS.codeBlock,
    isActive: () => !!editor.value?.isActive('codeBlock'),
    run: () => run(() => editor.value?.chain().focus().toggleCodeBlock().run()),
  },
  {
    action: 'table',
    title: '插入 3×3 表格',
    icon: '⊞',
    isActive: () => !!editor.value?.isActive('table'),
    run: () =>
      run(() =>
        editor.value?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(),
      ),
  },
  {
    action: 'hr',
    title: '分割线',
    icon: ICONS.hr,
    isActive: () => false,
    run: () => run(() => editor.value?.chain().focus().setHorizontalRule().run()),
  },
  {
    action: 'clear',
    title: '清除格式',
    icon: ICONS.clear,
    isActive: () => false,
    run: () => run(() => editor.value?.chain().focus().unsetAllMarks().clearNodes().run()),
  },
])

// applyLink / applyImage 用输入弹窗而不是 prompt()：
// prompt 在部分浏览器/嵌入式 webview 里被禁用，且无法做「留空即删除链接」的语义。
function applyLink() {
  if (props.disabled || !editor.value) return
  const previous = (editor.value.getAttributes('link').href as string) || ''
  let input = previous

  const dialog = DialogPlugin.confirm({
    header: previous ? '修改链接' : '插入链接',
    body: '请输入链接地址（http/https）。留空并确认可移除已有链接。',
    confirmBtn: { content: '确定', theme: 'primary' },
    cancelBtn: { content: '取消' },
    onConfirm: () => {
      const url = input.trim()
      const chain = editor.value?.chain().focus().extendMarkRange('link')
      if (!url) {
        chain?.unsetLink().run()
      } else if (!/^https?:\/\//i.test(url)) {
        MessagePlugin.warning('链接必须以 http:// 或 https:// 开头')
        return // 不关闭弹窗，让用户改
      } else {
        chain?.setLink({ href: url }).run()
      }
      dialog.destroy()
    },
    onClose: () => dialog.destroy(),
  })

  // TDesign 的 confirm 弹窗没有内建输入位，这里把输入框插进 body 描述区。
  const body = (dialog as unknown as { body?: HTMLElement }).body
  if (body) {
    const wrapper = document.createElement('div')
    wrapper.className = 'rich-editor__dialog-input'
    const el = document.createElement('input')
    el.className = 't-input__inner'
    el.placeholder = 'https://example.com'
    el.value = previous
    el.style.cssText = 'width:100%;margin-top:8px;padding:6px 8px;border:1px solid var(--td-component-border);border-radius:4px;'
    el.addEventListener('input', (e) => {
      input = (e.target as HTMLInputElement).value
    })
    wrapper.appendChild(el)
    body.appendChild(wrapper)
    setTimeout(() => el.focus(), 0)
  }
}

function applyImage() {
  if (props.disabled || !editor.value) return
  let input = ''
  const dialog = DialogPlugin.confirm({
    header: '插入图片',
    body: '请输入图片地址（http/https 外链）。',
    confirmBtn: { content: '插入', theme: 'primary' },
    cancelBtn: { content: '取消' },
    onConfirm: () => {
      const url = input.trim()
      if (!/^https?:\/\//i.test(url)) {
        MessagePlugin.warning('图片地址必须以 http:// 或 https:// 开头')
        return
      }
      editor.value?.chain().focus().setImage({ src: url }).run()
      dialog.destroy()
    },
    onClose: () => dialog.destroy(),
  })

  const body = (dialog as unknown as { body?: HTMLElement }).body
  if (body) {
    const wrapper = document.createElement('div')
    wrapper.className = 'rich-editor__dialog-input'
    const el = document.createElement('input')
    el.className = 't-input__inner'
    el.placeholder = 'https://cdn.example.com/banner.png'
    el.style.cssText = 'width:100%;margin-top:8px;padding:6px 8px;border:1px solid var(--td-component-border);border-radius:4px;'
    el.addEventListener('input', (e) => {
      input = (e.target as HTMLInputElement).value
    })
    wrapper.appendChild(el)
    body.appendChild(wrapper)
    setTimeout(() => el.focus(), 0)
  }
}

/** 空文档（`<p></p>`）归一化为空串：让「必填校验」判断的是内容而不是标签。 */
function normalizeOut(html: string): string {
  const trimmed = html.trim()
  if (!trimmed || trimmed === '<p></p>' || trimmed === '<p></p>\n') return ''
  return trimmed
}

/**
 * 外部传入值的入口归一化。
 *
 * 存量公告正文是纯文本（body_format=text），直接塞给 tiptap 会因为存在裸 `<`
 * 被解析成标签而丢内容；这里只做最小转换：整体当一个段落。
 */
function normalizeIn(value: string | undefined): string {
  const raw = (value ?? '').trim()
  if (!raw) return ''
  if (/^\s*</.test(raw)) return raw
  return `<p>${escapeHtml(raw).replace(/\n/g, '<br />')}</p>`
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function plainText(html: string): string {
  return html.replace(/<[^>]*>/g, '')
}

defineExpose({ editor })
</script>

<style scoped>
.rich-editor {
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  background: var(--td-bg-color-container);
}

.rich-editor--disabled {
  opacity: 0.6;
}

.rich-editor__toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  padding: 6px;
  border-bottom: 1px solid var(--td-component-stroke);
}

.rich-editor__btn {
  min-width: 30px;
  height: 28px;
  padding: 0 6px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-primary);
  font-size: 13px;
  line-height: 1;
  cursor: pointer;
}

.rich-editor__btn:hover:not(:disabled) {
  background: var(--td-bg-color-container-hover);
}

.rich-editor__btn.is-active {
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
}

.rich-editor__btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.rich-editor__content :deep(.ProseMirror) {
  min-height: 240px;
  max-height: 520px;
  overflow-y: auto;
  padding: 12px;
  outline: none;
  line-height: 1.7;
}

.rich-editor__content :deep(.ProseMirror p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  float: left;
  height: 0;
  color: var(--td-text-color-placeholder);
  pointer-events: none;
}

.rich-editor__content :deep(table) {
  width: 100%;
  border-collapse: collapse;
}

.rich-editor__content :deep(th),
.rich-editor__content :deep(td) {
  border: 1px solid var(--td-component-border);
  padding: 6px 8px;
}

.rich-editor__content :deep(blockquote) {
  margin: 8px 0;
  padding-left: 12px;
  border-left: 3px solid var(--td-brand-color);
  color: var(--td-text-color-secondary);
}

.rich-editor__content :deep(img) {
  max-width: 100%;
  height: auto;
}

.rich-editor__content :deep(pre) {
  padding: 10px;
  border-radius: 4px;
  background: var(--td-bg-color-secondarycontainer);
  overflow-x: auto;
}

.rich-editor__footer {
  display: flex;
  justify-content: space-between;
  padding: 6px 10px;
  border-top: 1px solid var(--td-component-stroke);
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
</style>
