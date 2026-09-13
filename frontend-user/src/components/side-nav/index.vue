<template>
  <!-- 桌面端：常驻侧边栏；移动端：抽屉遮罩 + 滑入面板 -->
  <div v-if="isMobile && open" class="side-nav__backdrop" @click="emit('update:open', false)"></div>

  <aside
    class="side-nav"
    :class="{
      'side-nav--collapsed': isCollapsed && !isMobile,
      'side-nav--mobile': isMobile,
      'side-nav--mobile-open': isMobile && open,
    }"
    aria-label="控制台导航"
  >
    <nav class="side-nav__list">
      <template v-for="item in visibleMenus" :key="item.id">
        <!-- 一级目录：可展开 -->
        <div v-if="hasChildren(item)" class="side-nav__group">
          <button
            class="side-nav__item side-nav__item--group"
            :class="{ 'is-expanded': isExpanded(item.id) }"
            type="button"
            :aria-expanded="isExpanded(item.id)"
            @click="toggleGroup(item.id)"
          >
            <span class="side-nav__icon"><component :is="item.icon" v-if="item.icon" /></span>
            <span class="side-nav__label">{{ item.name }}</span>
            <ChevronDownIcon class="side-nav__arrow" size="14" />
          </button>

          <transition name="side-nav-collapse">
            <div v-show="isExpanded(item.id)" class="side-nav__children">
              <button
                v-for="child in visibleChildren(item)"
                :key="child.id"
                class="side-nav__item side-nav__item--child"
                :class="{ 'is-active': isActive(child.path) }"
                type="button"
                @click="go(child)"
              >
                <span class="side-nav__label">{{ child.name }}</span>
              </button>
            </div>
          </transition>
        </div>

        <!-- 一级菜单：直接跳转 -->
        <button
          v-else
          class="side-nav__item"
          :class="{ 'is-active': isActive(item.path) }"
          type="button"
          :title="item.name"
          @click="go(item)"
        >
          <span class="side-nav__icon"><component :is="item.icon" v-if="item.icon" /></span>
          <span class="side-nav__label">{{ item.name }}</span>
        </button>
      </template>

      <p v-if="!visibleMenus.length" class="side-nav__empty">菜单加载中…</p>
    </nav>

    <!-- 折叠开关只在桌面端出现：移动端用抽屉，无需折叠 -->
    <button
      v-if="!isMobile"
      class="side-nav__collapse"
      type="button"
      :aria-label="isCollapsed ? '展开侧边栏' : '收起侧边栏'"
      @click="toggleCollapse"
    >
      <ChevronRightIcon v-if="isCollapsed" size="16" />
      <ChevronLeftIcon v-else size="16" />
    </button>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronDownIcon, ChevronLeftIcon, ChevronRightIcon } from 'tdesign-icons-vue-next'

import { OWNER_ONLY_MENU_PATHS, useMenuStore, type FlatMenu } from '@/store/modules/menu'
import { useMemberStore } from '@/store/modules/member'
import { useIsMobile } from '@/composables/useIsMobile'

/**
 * 控制台左侧导航。
 *
 * 数据源是后端菜单树（menuStore.sidebarMenus），不在前端维护导航清单 ——
 * 这样「加一个页面」只需后端 seed + 前端路由，不会出现两份导航对不上的情况。
 *
 * ownerOnly 过滤（成员管理等）在这里做而不是菜单 store 里：它需要登录态（memberStore），
 * 而菜单 store 是登录态的前置依赖，互相依赖会成环。
 */

const props = withDefaults(
  defineProps<{
    /** 移动端抽屉开关（桌面端忽略） */
    open?: boolean
  }>(),
  { open: false },
)

const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const router = useRouter()
const route = useRoute()
const menuStore = useMenuStore()
const memberStore = useMemberStore()
const { isMobile } = useIsMobile()

const COLLAPSE_KEY = 'console_sidebar_collapsed'
const isCollapsed = ref(localStorage.getItem(COLLAPSE_KEY) === '1')

// 一级目录默认展开（当前路由所在的目录一定展开）
const expanded = ref<Set<number>>(new Set())

function toggleCollapse() {
  isCollapsed.value = !isCollapsed.value
  localStorage.setItem(COLLAPSE_KEY, isCollapsed.value ? '1' : '0')
}

// 顶栏汉堡按钮在桌面端也控制折叠，这里把开关暴露给布局层，
// 避免布局层去猜侧边栏的折叠状态（localStorage 可能是脏值）。
defineExpose({ toggleCollapse, isCollapsed })

function isOwnerOnly(path?: string): boolean {
  return !!path && OWNER_ONLY_MENU_PATHS.has(path)
}

/** 子账号隐藏 ownerOnly 菜单（如成员管理）；主账号全部可见。 */
function filterVisible(items: FlatMenu[]): FlatMenu[] {
  if (memberStore.isOwner) return items
  return items.filter((item) => !isOwnerOnly(item.path))
}

const visibleMenus = computed<FlatMenu[]>(() => filterVisible(menuStore.sidebarMenus))

function visibleChildren(item: FlatMenu): FlatMenu[] {
  return filterVisible(item.children ?? [])
}

function hasChildren(item: FlatMenu): boolean {
  return visibleChildren(item).length > 0
}

function isExpanded(id: number): boolean {
  return expanded.value.has(id)
}

function toggleGroup(id: number) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

/** 当前路由命中判定：菜单路径是 /billing，子页 /billing/transactions 也应保持高亮。 */
function isActive(path?: string): boolean {
  if (!path) return false
  return route.path === path || route.path.startsWith(`${path}/`)
}

function go(item: FlatMenu) {
  if (!item.path) return
  router.push(item.path)
  if (isMobile.value) emit('update:open', false)
}

// 菜单加载完成或路由变化时，展开当前路由所在目录
watch(
  [() => menuStore.sidebarMenus, () => route.path],
  ([menus, path]) => {
    if (!menus.length) return
    const next = new Set(expanded.value)
    for (const item of menus) {
      const children = item.children ?? []
      if (children.some((child) => child.path && (path === child.path || path.startsWith(`${child.path}/`)))) {
        next.add(item.id)
      }
    }
    expanded.value = next
  },
  { immediate: true },
)

// 移动端抽屉打开时锁定页面滚动
watch(
  () => props.open,
  (open) => {
    if (isMobile.value) document.body.style.overflow = open ? 'hidden' : ''
  },
)
</script>

<style scoped>
.side-nav {
  display: flex;
  flex-direction: column;
  width: 216px;
  flex-shrink: 0;
  padding: 12px 10px;
  /* 顶栏是 sticky 的，侧边栏跟着固定在它下方，滚动内容时导航不跟着走。
     用 sticky 而不是固定高度 app shell：内容区仍是文档流的一部分，
     vue-router 的 scrollBehavior 才能继续生效。 */
  position: sticky;
  top: 64px;
  height: calc(100vh - 64px);
  background: var(--hs-surface-1);
  border-right: 1px solid var(--color-border);
  overflow-y: auto;
  transition: width var(--hs-duration-base) var(--hs-ease-out);
}

.side-nav--collapsed {
  width: 64px;
  padding: 12px 8px;
}

.side-nav__list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

/* ---------- 菜单项 ---------- */
.side-nav__item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 12px;
  border: none;
  border-radius: var(--hs-radius-md);
  background: transparent;
  color: var(--color-foreground);
  font-size: 13.5px;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
  transition: background-color var(--hs-duration-fast) var(--hs-ease-out),
    color var(--hs-duration-fast) var(--hs-ease-out);
}

.side-nav__item:hover {
  background: var(--hs-surface-3);
  color: var(--color-primary);
}

.side-nav__item.is-active {
  background: var(--color-primary-light);
  color: var(--color-primary);
  font-weight: 600;
}

.side-nav__item--child {
  padding-left: 40px;
  font-size: 13px;
  color: var(--color-muted-foreground);
}

.side-nav__item--child.is-active {
  color: var(--color-primary);
}

.side-nav__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  font-size: 16px;
}

.side-nav__label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.side-nav__arrow {
  flex-shrink: 0;
  color: var(--color-muted-foreground);
  transition: transform var(--hs-duration-base) var(--hs-ease-out);
}

.side-nav__item--group.is-expanded .side-nav__arrow {
  transform: rotate(180deg);
}

.side-nav__children {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0 4px;
}

.side-nav__empty {
  padding: 16px 12px;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

/* ---------- 折叠态：只留图标 ---------- */
.side-nav--collapsed .side-nav__label,
.side-nav--collapsed .side-nav__arrow,
.side-nav--collapsed .side-nav__children {
  display: none;
}

.side-nav--collapsed .side-nav__item {
  justify-content: center;
  padding: 9px 0;
}

/* ---------- 折叠按钮 ---------- */
.side-nav__collapse {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  align-self: flex-end;
  width: 28px;
  height: 28px;
  margin-top: 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md);
  background: var(--hs-surface-1);
  color: var(--color-muted-foreground);
  cursor: pointer;
  flex-shrink: 0;
  transition: color var(--hs-duration-fast) var(--hs-ease-out),
    border-color var(--hs-duration-fast) var(--hs-ease-out);
}

.side-nav__collapse:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
}

/* ---------- 移动端抽屉 ---------- */
.side-nav__backdrop {
  position: fixed;
  inset: 0;
  z-index: 39;
  background: rgba(15, 23, 42, 0.42);
}

.side-nav--mobile {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 40;
  width: 258px;
  padding: 16px 12px;
  transform: translateX(-100%);
  transition: transform var(--hs-duration-base) var(--hs-ease-out);
  box-shadow: var(--hs-shadow-lg);
}

.side-nav--mobile-open {
  transform: translateX(0);
}

/* ---------- 展开/收起动画 ---------- */
.side-nav-collapse-enter-active,
.side-nav-collapse-leave-active {
  overflow: hidden;
  transition: max-height var(--hs-duration-base) var(--hs-ease-out),
    opacity var(--hs-duration-fast) var(--hs-ease-out);
  max-height: 320px;
}

.side-nav-collapse-enter-from,
.side-nav-collapse-leave-to {
  max-height: 0;
  opacity: 0;
}

/* ---------- 深色适配 ---------- */
:root.dark .side-nav {
  background: var(--hs-surface-1);
  border-right-color: var(--color-border);
}

:root.dark .side-nav__item {
  color: #cbd5e1;
}

:root.dark .side-nav__item:hover {
  background: var(--hs-surface-3);
}

:root.dark .side-nav__item.is-active {
  background: var(--color-primary-light);
  color: var(--color-primary);
}
</style>
