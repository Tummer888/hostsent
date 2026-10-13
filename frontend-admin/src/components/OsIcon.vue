<template>
  <span class="os-icon" :class="[`os-icon--${info.key}`, { 'os-icon--lg': size === 'lg' }]" :title="info.label">
    <!--
      有官方图形的一律用图形（tdesign 图标库自带 Windows / Apple / Android）；
      没有的发行版用「品牌色 + 首字母」而不是自绘商标 —— 凭记忆手绘的厂商图形
      一旦比例走样，比没有图标更难看，且各发行版商标使用条款不一。首字母章在
      14~20px 尺寸下辨识度足够（配合 hover 的完整名与旁边的文字）。
    -->
    <WindowLogoIcon v-if="info.key === 'windows'" class="os-icon__svg" />
    <AppleLogoIcon v-else-if="info.key === 'macos'" class="os-icon__svg" />
    <AndroidLogoIcon v-else-if="info.key === 'android'" class="os-icon__svg" />
    <span v-else class="os-icon__mark">{{ info.mark }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { LogoAndroidIcon as AndroidLogoIcon, LogoAppleIcon as AppleLogoIcon, LogoWindowsIcon as WindowLogoIcon } from 'tdesign-icons-vue-next'

import { osIconInfo } from '@/pages/instances/os-icons'

defineOptions({ name: 'OsIcon' })

// prop 名 size 与 TDesign 一致：sm 为默认行内尺寸，lg 用于详情页主信息区。
const props = withDefaults(defineProps<{ os?: string; size?: 'sm' | 'lg' }>(), {
  os: '',
  size: 'sm',
})

const info = computed(() => osIconInfo(props.os))
</script>

<style scoped>
.os-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 6px;
  flex-shrink: 0;
  /* 兜底色：未识别的系统用中性灰，不用某一家的品牌色去冒充。 */
  background: #e2e8f0;
  color: #475569;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  user-select: none;
}

.os-icon--lg {
  width: 26px;
  height: 26px;
  border-radius: 8px;
  font-size: 13px;
}

.os-icon__svg {
  font-size: 14px;
}

.os-icon--lg .os-icon__svg {
  font-size: 18px;
}

/* 品牌色（仅用于首字母章底色，不代表官方商标配色）。 */
.os-icon--windows { background: #e6f0fd; color: #0f6cbd; }
.os-icon--macos { background: #eef1f5; color: #334155; }
.os-icon--android { background: #e6f7ec; color: #3ddc84; }
.os-icon--centos { background: #f0e9fb; color: #6b21a8; }
.os-icon--debian { background: #fde9f3; color: #c2156b; }
.os-icon--ubuntu { background: #fdece0; color: #d4530c; }
.os-icon--almalinux { background: #fde9e9; color: #b91c1c; }
.os-icon--rockylinux { background: #e8f5ec; color: #10693b; }
.os-icon--fedora { background: #e7f1fd; color: #1057a8; }
.os-icon--opensuse { background: #eaf7e6; color: #38761d; }
.os-icon--arch { background: #e8f4fb; color: #0e7490; }
.os-icon--alpine { background: #e6f2fb; color: #0b5394; }
.os-icon--freebsd { background: #fdeaea; color: #a31919; }
.os-icon--openwrt { background: #e8f6f8; color: #0e7490; }
.os-icon--linux { background: #fdf3d7; color: #92690b; }
</style>
