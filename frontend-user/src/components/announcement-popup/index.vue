<template>
  <!--
    公告弹窗（doc100 Q12 决策①）。

    数据来源是同一份「我的公告」接口（`GET /uc/announcements`），只在本地按
    `popup` 标记 + 已关闭记录筛出需要弹的条目 —— 后端没有必要为「谁看过哪个弹窗」
    建表（见 utils/announcementPopup.ts 的说明）。

    多条待弹时按队列逐条展示：TDesign 的 dialog 是单实例，一次弹多个会互相覆盖，
    排在后面的公告就永远看不到了。
  -->
  <t-dialog
    v-model:visible="visible"
    :header="current?.title || '平台公告'"
    width="560px"
    :footer="false"
    :close-on-overlay-click="false"
    @close="onClose"
  >
    <div v-if="current" class="ann-popup">
      <div class="ann-popup__meta">
        <t-tag :theme="levelTheme(current.level)" variant="light" size="small" shape="round">
          {{ levelLabel(current.level) }}
        </t-tag>
        <span v-if="remaining > 0" class="ann-popup__more">还有 {{ remaining }} 条公告</span>
        <span v-else-if="current.publish_at" class="ann-popup__time">
          {{ formatTime(current.publish_at) }}
        </span>
      </div>

      <!--
        与消息中心同一套渲染规则：body_format=html 才是服务端已净化的富文本，
        其余按纯文本输出（老公告里出现的 `<` 按 HTML 渲染会被吃掉）。
      -->
      <p v-if="current.body_format !== 'html'" class="ann-popup__text">{{ current.content }}</p>
      <div v-else class="ann-popup__text ann-popup__text--html" v-html="current.content" />

      <div class="ann-popup__actions">
        <t-button variant="text" @click="onViewAll">到公告列表</t-button>
        <t-button theme="primary" @click="onClose">
          {{ remaining > 0 ? '下一条' : '我知道了' }}
        </t-button>
      </div>
    </div>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { getMyAnnouncements, type AnnouncementInfo } from '@/api/notification'
import { hasSeenAnnouncement, markAnnouncementsSeen } from '@/utils/announcementPopup'

defineOptions({ name: 'AnnouncementPopup' })

const router = useRouter()

/** 待弹队列：只包含 popup=true 且本地没有关闭记录的公告。 */
const queue = ref<AnnouncementInfo[]>([])
const visible = ref(false)

const current = computed<AnnouncementInfo | null>(() => queue.value[0] || null)
/** 当前这条之后还剩几条（用于按钮文案与计数提示）。 */
const remaining = computed(() => Math.max(0, queue.value.length - 1))

async function load() {
  try {
    const { data } = await getMyAnnouncements()
    const items = data?.list ?? []
    queue.value = items.filter((item) => item.popup && !hasSeenAnnouncement(item.id))
    if (queue.value.length) visible.value = true
  } catch {
    // 弹窗是锦上添花：拉不到公告不该在控制台里报错打断用户（消息中心仍可查看公告）
  }
}

function onClose() {
  const item = current.value
  if (item) {
    // 关闭即视为「已提醒过」：本次队列里剩下的稍后再弹，不能一起标记
    markAnnouncementsSeen([item.id])
    queue.value = queue.value.slice(1)
  }
  visible.value = queue.value.length > 0
}

function onViewAll() {
  const item = current.value
  if (item) {
    markAnnouncementsSeen([item.id])
    queue.value = []
  }
  visible.value = false
  router.push('/profile/messages')
}

function formatTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value.replace(/-/g, '/'))
  if (Number.isNaN(date.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function levelLabel(level: string): string {
  if (level === 'critical') return '重要'
  if (level === 'warning') return '预警'
  return '公告'
}

function levelTheme(level: string): 'danger' | 'warning' | 'primary' {
  if (level === 'critical') return 'danger'
  if (level === 'warning') return 'warning'
  return 'primary'
}

onMounted(() => {
  void load()
})
</script>

<style scoped>
.ann-popup__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.ann-popup__more,
.ann-popup__time {
  font-size: 12.5px;
  color: #94a3b8;
}

.ann-popup__text {
  margin: 0;
  max-height: 46vh;
  overflow-y: auto;
  font-size: 14px;
  line-height: 1.75;
  color: #475569;
  white-space: pre-wrap;
  word-break: break-word;
}

.ann-popup__text--html {
  white-space: normal;
}

.ann-popup__text--html :deep(img) {
  max-width: 100%;
  height: auto;
}

.ann-popup__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 20px;
}
</style>
