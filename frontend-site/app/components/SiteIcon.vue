<template>
  <svg
    class="site-icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    :stroke-width="strokeWidth"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <!-- 产品价值 -->
    <template v-if="name === 'server'">
      <rect x="3" y="4" width="18" height="7" rx="2" />
      <rect x="3" y="13" width="18" height="7" rx="2" />
      <path d="M7 7.5h.01M7 16.5h.01" />
    </template>
    <template v-else-if="name === 'shield'">
      <path d="M12 3l7 3v5.5c0 4.2-2.9 7.9-7 9.5-4.1-1.6-7-5.3-7-9.5V6l7-3z" />
      <path d="M9.2 12.2l2 2 3.6-3.8" />
    </template>
    <template v-else-if="name === 'bolt'">
      <path d="M13.5 3L5.5 13.5h5L10 21l8-10.5h-5L13.5 3z" />
    </template>
    <template v-else-if="name === 'support'">
      <path d="M4.5 13v-1a7.5 7.5 0 0115 0v1" />
      <rect x="2.8" y="12.6" width="4" height="6" rx="1.6" />
      <rect x="17.2" y="12.6" width="4" height="6" rx="1.6" />
      <path d="M19.2 18.6c0 1.5-1.6 2.4-3.7 2.4" />
    </template>

    <!-- 导航与操作 -->
    <template v-else-if="name === 'search'">
      <circle cx="11" cy="11" r="6.5" />
      <path d="M16 16l4.5 4.5" />
    </template>
    <template v-else-if="name === 'menu'">
      <path d="M4 7h16M4 12h16M4 17h16" />
    </template>
    <template v-else-if="name === 'close'">
      <path d="M6 6l12 12M18 6L6 18" />
    </template>
    <template v-else-if="name === 'arrow-right'">
      <path d="M5 12h14M13 6l6 6-6 6" />
    </template>
    <template v-else-if="name === 'arrow-up-right'">
      <path d="M7 17L17 7M9 7h8v8" />
    </template>
    <template v-else-if="name === 'chevron-right'">
      <path d="M9.5 6l6 6-6 6" />
    </template>
    <template v-else-if="name === 'external'">
      <path d="M14 4h6v6" />
      <path d="M20 4l-8.5 8.5" />
      <path d="M18 14.5V18a2 2 0 01-2 2H6a2 2 0 01-2-2V8a2 2 0 012-2h3.5" />
    </template>

    <!-- 功能区 -->
    <template v-else-if="name === 'book'">
      <path d="M4 5.5A2.5 2.5 0 016.5 3H19v18H6.5A2.5 2.5 0 014 18.5v-13z" />
      <path d="M4 17.5h15" />
      <path d="M8.5 7.5h6" />
    </template>
    <template v-else-if="name === 'code'">
      <path d="M9 8l-4 4 4 4M15 8l4 4-4 4" />
    </template>
    <template v-else-if="name === 'pulse'">
      <path d="M3 12.5h4l2-5 3.5 10 2.5-5h6" />
    </template>
    <template v-else-if="name === 'sparkle'">
      <path d="M12 3.5l1.8 4.7L18.5 10l-4.7 1.8L12 16.5l-1.8-4.7L5.5 10l4.7-1.8L12 3.5z" />
      <path d="M18.5 16.5l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8.8-2z" />
    </template>
    <template v-else-if="name === 'cart'">
      <path d="M3 4.5h2.2l2.3 10.6h10.2l2.1-7.6H6.2" />
      <circle cx="9" cy="19.5" r="1.4" />
      <circle cx="17" cy="19.5" r="1.4" />
    </template>
    <template v-else-if="name === 'ticket'">
      <path d="M4 8.5V7a1.5 1.5 0 011.5-1.5h13A1.5 1.5 0 0120 7v1.5a2 2 0 000 4V16a1.5 1.5 0 01-1.5 1.5h-13A1.5 1.5 0 014 16v-3.5a2 2 0 000-4z" />
      <path d="M13 6v12" stroke-dasharray="2 2.4" />
    </template>
    <template v-else-if="name === 'tag'">
      <path d="M11.4 3.6H20v8.6l-8.8 8.8a1.6 1.6 0 01-2.3 0l-6.3-6.3a1.6 1.6 0 010-2.3l8.8-8.8z" />
      <path d="M16.2 7.8h.01" />
    </template>
    <template v-else-if="name === 'refresh'">
      <path d="M20 11.5A8 8 0 006.3 6.3L4 8.5" />
      <path d="M4 4.5v4h4" />
      <path d="M4 12.5A8 8 0 0017.7 17.7L20 15.5" />
      <path d="M20 19.5v-4h-4" />
    </template>
    <template v-else-if="name === 'users'">
      <circle cx="9" cy="9" r="3.2" />
      <path d="M3.5 20c0-3 2.5-5.2 5.5-5.2s5.5 2.2 5.5 5.2" />
      <path d="M16 6.2A3.2 3.2 0 0116 12" />
      <path d="M17.5 14.9c1.9.6 3.2 2.4 3.2 5.1" />
    </template>
    <template v-else-if="name === 'layers'">
      <path d="M12 3.5l8.5 4.3L12 12 3.5 7.8 12 3.5z" />
      <path d="M3.5 12.4L12 16.6l8.5-4.2" />
      <path d="M3.5 16.6L12 20.8l8.5-4.2" />
    </template>

    <!-- 页脚图标（服务保障/社交/合规） -->
    <template v-else-if="name === 'time'">
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7.5V12l3 2" />
    </template>
    <template v-else-if="name === 'secured'">
      <path d="M12 3l7 3v5.5c0 4.2-2.9 7.9-7 9.5-4.1-1.6-7-5.3-7-9.5V6l7-3z" />
      <path d="M9.2 12.2l2 2 3.6-3.8" />
    </template>
    <template v-else-if="name === 'service'">
      <path d="M4.5 13v-1a7.5 7.5 0 0115 0v1" />
      <rect x="2.8" y="12.6" width="4" height="6" rx="1.6" />
      <rect x="17.2" y="12.6" width="4" height="6" rx="1.6" />
      <path d="M19.2 18.6c0 1.5-1.6 2.4-3.7 2.4" />
    </template>
    <template v-else-if="name === 'rollback'">
      <path d="M9.5 7L5 11.5 9.5 16" />
      <path d="M5 11.5h9a5.5 5.5 0 015.5 5.5" />
    </template>
    <template v-else-if="name === 'edit'">
      <path d="M4 20h4L19.2 8.8a1.6 1.6 0 000-2.3l-1.7-1.7a1.6 1.6 0 00-2.3 0L4 15.9V20z" />
      <path d="M13.5 6.5l4 4" />
    </template>
    <template v-else-if="name === 'wechat'">
      <path d="M9 5.2c3.6 0 6.5 2.4 6.5 5.4s-2.9 5.4-6.5 5.4c-.7 0-1.4-.1-2-.3l-2.3 1.2.6-2.1A4.9 4.9 0 012.5 10.6C2.5 7.6 5.4 5.2 9 5.2z" />
      <circle cx="6.8" cy="10" r=".8" />
      <circle cx="11" cy="10" r=".8" />
    </template>
    <template v-else-if="name === 'qq'">
      <path d="M12 3.5c2.2 0 3.8 1.6 3.8 4v1.2c0 1.3.4 2 1.5 3-.8.4-1.6.6-2.5.6 0 .8.3 1.6 1 2.3-2.1.7-4.8.7-7.6 0 .7-.7 1-1.5 1-2.3-.9 0-1.7-.2-2.5-.6 1.1-1 1.5-1.7 1.5-3V7.5C8.2 5.1 9.8 3.5 12 3.5z" />
      <path d="M8.2 16.5c.5 1 1.5 2 3.8 2s3.3-1 3.8-2" />
    </template>
    <template v-else-if="name === 'github'">
      <path d="M9 19.5c-3.5 1-3.5-2.1-5-2.6M15 21v-3.2c0-1 .3-1.7.7-2.4 2.3-.3 4.8-1.2 4.8-5.4a4.3 4.3 0 00-1.2-3A4 4 0 0019 4.4s-.8-.2-2.7.9a9 9 0 00-4.9 0C9.5 4.2 8.7 4.4 8.7 4.4a4 4 0 00-1.3 2.6 4.3 4.3 0 00-1.2 3c0 4.2 2.4 5.1 4.7 5.4.4.7.6 1.4.7 2.4V21" />
    </template>
    <template v-else-if="name === 'video'">
      <rect x="3" y="6" width="14" height="12" rx="2.5" />
      <path d="M17 10.5l4-2.5v8l-4-2.5" />
    </template>
    <template v-else-if="name === 'certificate'">
      <circle cx="12" cy="10" r="5" />
      <path d="M10.8 9.2l1 1 2-2.1" />
      <path d="M12 14.8V17l-1.5 1 1.5-1 1.5 1-1.5-1v-2.2" />
    </template>
    <template v-else-if="name === 'mobile'">
      <rect x="7.5" y="3" width="9" height="18" rx="2" />
      <path d="M11 17.5h2" />
    </template>

    <!-- 兜底 -->
    <template v-else>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 8.2v4.2l2.6 1.6" />
    </template>
  </svg>
</template>

<script setup lang="ts">
/**
 * 站点内置图标集（原生内联 SVG，规范 R10：不引入第三方图标库）。
 *
 * 全部图标统一 24x24 viewBox、`stroke="currentColor"`，随文字颜色与字号继承，
 * 因此在深色通栏与浅色卡片里都无需额外改色。
 */
withDefaults(defineProps<{ name: string; strokeWidth?: number }>(), {
  strokeWidth: 1.7,
})
</script>

<style scoped>
.site-icon {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
}
</style>
