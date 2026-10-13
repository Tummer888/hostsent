<template>
  <!-- -->
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
