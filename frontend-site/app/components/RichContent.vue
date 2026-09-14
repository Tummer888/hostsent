<template>
  <!--
    富文本正文渲染。

    信任边界（doc100 §6.1）：`html` 一定是后端 internal/pkg/sanitize 净化过的结果，
    前端只负责排版，不做二次过滤 —— 再过滤一遍会把运营合法排版的表格/图注误伤成纯文本。

    `format === 'text'` 分支是给存量公告用的：早期公告正文是纯文本，里面可能出现
    `<`、`>`（例如「延迟 < 5ms」），按 HTML 渲染会把这段内容吃掉。所以按文本节点
    分段输出，不做任何解析。
  -->
  <div v-if="format === 'html'" class="rich-content" v-html="html" />
  <div v-else class="rich-content rich-content--plain">
    <p v-for="(paragraph, index) in plainParagraphs" :key="index">{{ paragraph }}</p>
  </div>
</template>

<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    /** 正文内容：HTML（已净化）或纯文本，由 format 决定如何解释。 */
    html: string
    format?: 'html' | 'text'
  }>(),
  { format: 'html' },
)

const plainParagraphs = computed(() =>
  props.html
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== ''),
)
</script>
