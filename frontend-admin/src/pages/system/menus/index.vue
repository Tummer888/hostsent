<template>
  <div class="menu-page system-page">
    <div class="menu-page__header">
      <div class="page-header__main">
        <span class="page-header__chip">
          <MenuIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="menu-page__title">菜单管理</h2>
          <p class="menu-page__desc">
            菜单由后端 seed 定义（<code>backend/internal/pkg/db/db.go</code> 的
            <code>SeedMenus()</code>），本页只读。改菜单请改代码并跑
            <code>go test ./internal/pkg/db/ -run Menu</code>。
          </p>
        </div>
      </div>
      <div class="menu-page__actions">
        <t-radio-group v-model="platform" variant="default-filled" size="small" @change="loadTree">
          <t-radio-button value="admin">管理员后台</t-radio-button>
          <t-radio-button value="user">用户中心</t-radio-button>
        </t-radio-group>
        <t-button variant="outline" size="small" :loading="loading" @click="loadTree">
          <template #icon>
            <RefreshIcon />
          </template>
          刷新
        </t-button>
      </div>
    </div>

    <t-card :bordered="false" class="table-card">
      <t-table
        row-key="id"
        :data="treeData"
        :columns="columns"
        :loading="loading"
        :tree="{ childrenKey: 'children', treeNodeColumnIndex: 0 }"
        size="small"
        hover
        vertical-align="middle"
      >
        <template #icon="{ row }">
          <span class="menu-icon">{{ row.icon || '—' }}</span>
        </template>
        <template #type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">
            {{ typeLabel(row.type) }}
          </t-tag>
        </template>
        <template #platform="{ row }">
          <t-tag :theme="row.platform === 'admin' ? 'success' : 'warning'" variant="light" size="small" shape="round">
            {{ row.platform === 'admin' ? '管理员' : '用户中心' }}
          </t-tag>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'active' ? 'success' : 'danger'" variant="light" size="small" shape="round">
            {{ row.status === 'active' ? '启用' : '禁用' }}
          </t-tag>
        </template>
      </t-table>
    </t-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { MenuIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'
import type { PrimaryTableCol } from 'tdesign-vue-next'

import { getMenuTree, type MenuNode } from '@/api/menu'

defineOptions({ name: 'SystemMenus' })

const loading = ref(false)
const platform = ref('admin')
const treeData = ref<MenuNode[]>([])

const columns = computed<PrimaryTableCol<MenuNode>[]>(() => [
  { colKey: 'name', title: '菜单名称', minWidth: 200, ellipsis: true },
  { colKey: 'icon', title: '图标', width: 120 },
  { colKey: 'type', title: '类型', width: 100 },
  { colKey: 'path', title: '路由路径', minWidth: 180, ellipsis: true },
  { colKey: 'component', title: '前端组件', minWidth: 200, ellipsis: true },
  { colKey: 'platform', title: '平台', width: 110 },
  { colKey: 'sort_order', title: '排序', width: 80 },
  { colKey: 'status', title: '状态', width: 90 },
])

function typeLabel(type: string) {
  return type === 'directory' ? '目录' : '菜单'
}

async function loadTree() {
  loading.value = true
  try {
    treeData.value = await getMenuTree(platform.value)
  } catch (error) {
    MessagePlugin.error((error as Error)?.message || '加载菜单树失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadTree)
</script>

<style scoped lang="css">
@import '../shared.css';

.menu-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px;
}

.menu-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.menu-page__title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-foreground);
}

.menu-page__desc {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-muted-foreground);
}

.menu-page__desc code {
  font-family: var(--hs-font-mono);
  font-size: 12px;
  padding: 1px 5px;
  border-radius: 4px;
  background: var(--td-bg-color-container-hover);
}

.table-card {
  background: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
}

.menu-page__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.menu-icon {
  font-family: var(--hs-font-mono);
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
