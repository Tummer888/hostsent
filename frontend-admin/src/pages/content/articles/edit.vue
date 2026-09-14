<template>
  <t-drawer
    v-model:visible="visible"
    :header="isEdit ? '编辑内容' : '新建内容'"
    size="720px"
    :confirm-btn="{ content: '保存', theme: 'primary', loading: saving }"
    :cancel-btn="{ content: '取消' }"
    @confirm="handleSave"
    @close="handleClose"
  >
    <t-loading :loading="loading">
      <t-form label-align="top" :data="form" @submit.prevent>
        <div class="form-grid">
          <t-form-item label="内容类型" name="kind" :class="{ 'form-item--full': true }">
            <t-select
              v-model="form.kind"
              :options="kindOptions"
              :disabled="isEdit && kindIsSingleton(form.kind)"
              @change="handleKindChange"
            />
            <p v-if="isEdit && kindIsSingleton(form.kind)" class="field-help">
              条款与隐私政策保存即发布，且每类型只保留一篇，不能改类型或留空正文。
            </p>
            <p v-else class="field-help">
              门户落点：{{ kindPortalPath(form.kind) || '—' }}；条款/隐私政策保存即发布，不需要走草稿。
            </p>
          </t-form-item>

          <t-form-item label="标题" name="title" class="form-item--full" :rules="[{ required: true, message: '请输入标题' }]">
            <t-input v-model="form.title" placeholder="请输入标题" :maxlength="120" />
          </t-form-item>

          <t-form-item v-if="needsCategory" label="所属分类" name="category_id">
            <t-select v-model="form.category_id" clearable placeholder="不选则为未分类" :options="categoryOptions" />
          </t-form-item>

          <t-form-item label="URL 标识（slug）" name="slug">
            <t-input v-model="form.slug" placeholder="留空自动生成" :maxlength="120" />
            <p class="field-help">门户详情页地址，如 /news/spring-launch；中文标题无法自动生成时请手填。</p>
          </t-form-item>

          <t-form-item v-if="isSingleton" label="版本号" name="version">
            <t-input v-model="form.version" placeholder="如 v1.2" :maxlength="20" />
            <p class="field-help">条款/隐私政策修改时建议升版本号，便于对外说明「何时起生效」。</p>
          </t-form-item>

          <t-form-item label="摘要" name="summary" class="form-item--full">
            <t-textarea
              v-model="form.summary"
              placeholder="留空则自动从正文提取"
              :autosize="{ minRows: 2, maxRows: 3 }"
              :maxlength="500"
            />
          </t-form-item>

          <t-form-item label="封面图地址" name="cover" class="form-item--full">
            <t-input v-model="form.cover" placeholder="https://... （留空则不显示封面）" />
          </t-form-item>

          <t-form-item label="标签" name="tags" class="form-item--full">
            <t-input v-model="form.tags" placeholder="多个标签用英文逗号分隔，如：发布,新品" :maxlength="255" />
          </t-form-item>

          <t-form-item label="正文" name="body" class="form-item--full">
            <RichEditor v-model="form.body" :placeholder="bodyPlaceholder" />
            <p class="field-help">
              正文会经过服务端白名单净化后入库：&lt;script&gt;、内联事件（onclick 等）与
              javascript: 链接会被剔除，插入的表格/图片/链接会保留。
            </p>
          </t-form-item>

          <t-form-item label="置顶" name="pinned">
            <t-switch v-model="form.pinned" />
            <p class="field-help">置顶内容在列表最前。</p>
          </t-form-item>

          <t-form-item label="排序" name="sort_order">
            <t-input-number v-model="form.sort_order" :min="0" placeholder="数值越小越靠前" />
          </t-form-item>

          <t-form-item v-if="!isSingleton" label="定时发布时间" name="publish_at" class="form-item--full">
            <t-date-picker
              v-model="form.publish_at"
              placeholder="不填则立即发布"
              enable-time-picker
              clearable
              style="width: 100%"
            />
            <p class="field-help">
              不填 = 保存即发布；填未来时间 = 存为草稿，到点后由后端可见性判定自动对外（无需人工再点发布）。
            </p>
          </t-form-item>
        </div>
      </t-form>
    </t-loading>
  </t-drawer>
</template>

<script setup lang="ts">
// 文章编辑抽屉：列表页（新标签页/嵌入表单）与新建入口共用同一份表单，
// 避免「新增」与「编辑」两套字段列表各自漂移（漏字段是最常见的一类 bug）。
import { computed, reactive, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'

import {
  createArticle,
  getArticle,
  getContentCategories,
  updateArticle,
  type ArticleKind,
  type ArticleSaveRequest,
  type CategoryItem,
} from '@/api/content'
import RichEditor from '@/components/rich-editor/index.vue'

import { KIND_OPTIONS, kindIsSingleton, kindNeedsCategory, kindPortalPath } from '../constants'

const props = defineProps<{
  visible: boolean
  /** 有值 = 编辑，null = 新建 */
  articleId: number | null
  /** 新建时的默认类型（跟随列表页当前分栏） */
  defaultKind?: ArticleKind
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'saved'): void
}>()

const visible = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})

const kindOptions = KIND_OPTIONS
const isEdit = computed(() => props.articleId !== null)
const loading = ref(false)
const saving = ref(false)
const categories = ref<CategoryItem[]>([])

const form = reactive<{
  kind: ArticleKind
  category_id?: number
  slug: string
  title: string
  summary: string
  body: string
  cover: string
  tags: string
  pinned: boolean
  sort_order: number
  version: string
  publish_at: string
}>({
  kind: 'news',
  category_id: undefined,
  slug: '',
  title: '',
  summary: '',
  body: '',
  cover: '',
  tags: '',
  pinned: false,
  sort_order: 0,
  version: '',
  publish_at: '',
})

const needsCategory = computed(() => kindNeedsCategory(form.kind))
const isSingleton = computed(() => kindIsSingleton(form.kind))

const bodyPlaceholder = computed(() =>
  isSingleton.value ? '请输入条款/隐私政策正文' : '请输入正文内容',
)

const categoryOptions = computed(() => {
  const out: Array<{ label: string; value: number }> = []
  const walk = (nodes: CategoryItem[], depth: number) => {
    for (const node of nodes) {
      out.push({ label: `${'　'.repeat(depth)}${node.name}`, value: node.id })
      if (node.children?.length) walk(node.children, depth + 1)
    }
  }
  walk(categories.value, 0)
  return out
})

watch(
  () => props.visible,
  (open) => {
    if (!open) return
    void initialize()
  },
)

async function initialize() {
  resetForm()
  if (props.articleId !== null) {
    loading.value = true
    try {
      const detail = await getArticle(props.articleId)
      form.kind = detail.kind
      form.category_id = detail.category_id > 0 ? detail.category_id : undefined
      form.slug = detail.slug
      form.title = detail.title
      form.summary = detail.summary
      form.body = detail.body
      form.cover = detail.cover
      form.tags = detail.tags
      form.pinned = !!detail.pinned
      form.sort_order = detail.sort_order
      form.version = detail.version
      form.publish_at = detail.publish_at ? detail.publish_at.replace('T', ' ').slice(0, 19) : ''
    } catch (e) {
      MessagePlugin.error((e as Error).message || '加载内容详情失败')
      visible.value = false
      return
    } finally {
      loading.value = false
    }
  } else if (props.defaultKind) {
    form.kind = props.defaultKind
  }
  await loadCategories()
}

function resetForm() {
  form.kind = props.defaultKind ?? 'news'
  form.category_id = undefined
  form.slug = ''
  form.title = ''
  form.summary = ''
  form.body = ''
  form.cover = ''
  form.tags = ''
  form.pinned = false
  form.sort_order = 0
  form.version = ''
  form.publish_at = ''
}

async function loadCategories() {
  if (!needsCategory.value) {
    categories.value = []
    return
  }
  try {
    const resp = await getContentCategories({ kind: form.kind })
    categories.value = resp.items || []
  } catch {
    categories.value = []
  }
}

function handleKindChange() {
  form.category_id = undefined
  void loadCategories()
}

function handleClose() {
  visible.value = false
}

async function handleSave() {
  if (!form.title.trim()) {
    MessagePlugin.warning('请输入标题')
    return
  }
  if (isSingleton.value && !stripHtml(form.body).trim()) {
    MessagePlugin.warning('条款/隐私政策的正文不能为空')
    return
  }
  saving.value = true
  try {
    const payload: ArticleSaveRequest = {
      kind: form.kind,
      category_id: form.category_id ?? 0,
      slug: form.slug.trim() || undefined,
      title: form.title.trim(),
      summary: form.summary.trim(),
      body: form.body,
      body_format: 'html',
      cover: form.cover.trim(),
      tags: form.tags.trim(),
      pinned: form.pinned,
      sort_order: form.sort_order,
      version: form.version.trim(),
      publish_at: form.publish_at || undefined,
    }
    if (props.articleId !== null) {
      await updateArticle(props.articleId, payload)
      MessagePlugin.success('内容已更新')
    } else {
      await createArticle(payload)
      MessagePlugin.success('内容已创建')
    }
    emit('saved')
  } catch (e) {
    MessagePlugin.error((e as Error).message || '保存内容失败')
  } finally {
    saving.value = false
  }
}

function stripHtml(html: string): string {
  return html.replace(/<[^>]*>/g, '')
}
</script>
