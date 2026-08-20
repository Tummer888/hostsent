<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h2>权限分配</h2>
        <p>为角色分配菜单、页面和按钮级别的权限，保存操作将覆盖旧的权限集合。</p>
      </div>
      <t-button theme="primary" :loading="saving" :disabled="!selectedRoleId" @click="save">
        <template #icon><t-icon name="save" /></template>
        保存权限
      </t-button>
    </div>

    <div class="permission-layout">
      <!-- 左侧角色列表 -->
      <t-card title="选择角色" :bordered="false" class="role-panel">
        <t-list :split="true">
          <t-list-item
            v-for="role in roles"
            :key="role.id"
            :class="{ 'role-item-active': selectedRoleId === role.id }"
            class="role-item"
            @click="selectRole(role.id)"
          >
            <div class="role-item-content">
              <span class="role-name">{{ role.name }}</span>
              <t-tag size="small" variant="light-outline">{{ role.code }}</t-tag>
            </div>
          </t-list-item>
        </t-list>
      </t-card>

      <!-- 右侧权限树 -->
      <t-card
        :title="selectedRole ? `权限树 · ${selectedRole.name}` : '请先从左侧选择角色'"
        :bordered="false"
        class="tree-panel"
      >
        <div class="tree-toolbar">
          <t-space size="small">
            <t-button variant="outline" size="small" @click="expandAll">全部展开</t-button>
            <t-button variant="outline" size="small" @click="collapseAll">全部收起</t-button>
            <t-button variant="outline" size="small" @click="clearSelection">清空已选</t-button>
          </t-space>
        </div>

        <t-loading :loading="loading">
          <div class="tree-wrapper">
            <t-tree
              v-model:checked="checkedKeys"
              v-model:expanded="expandedKeys"
              :data="treeData"
              checkable
              line
              row-key="id"
              :keys="{ label: 'name', value: 'id', children: 'children' }"
            >
              <template #label="{ node }">
                <t-space size="small">
                  <t-icon v-if="node.data.type === 'directory'" name="folder" />
                  <t-icon v-else-if="node.data.type === 'menu'" name="view-module" />
                  <t-icon v-else name="gesture-click" />
                  <span>{{ node.label }}</span>
                  <span class="permission-code">{{ node.data.code }}</span>
                </t-space>
              </template>
            </t-tree>
          </div>
        </t-loading>
      </t-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next';
import { useRoute } from 'vue-router';
import {
  assignRolePermissions,
  getPermissionTree,
  getRoleList,
  getRolePermissionIds,
  type PermissionNode,
  type RoleInfo,
} from '@/api/user';

/**
 * 权限分配页面
 * 管理角色与权限树的绑定关系
 */
defineOptions({ name: 'SystemPermissions' });

const route = useRoute();
const roles = ref<RoleInfo[]>([]);
const treeData = ref<PermissionNode[]>([]);
const checkedKeys = ref<Array<string | number>>([]);
const expandedKeys = ref<Array<string | number>>([]);
const selectedRoleId = ref<number>(Number(route.query.role_id) || 0);

const loading = ref(false);
const saving = ref(false);

// 当前选中的角色对象
const selectedRole = computed(() => roles.value.find((role) => role.id === selectedRoleId.value));

/**
 * 初始化加载数据
 */
async function load() {
  loading.value = true;
  try {
    const [rolesList, permissionTree] = await Promise.all([getRoleList(), getPermissionTree()]);
    roles.value = rolesList;
    treeData.value = permissionTree;

    // 如果 URL 没带 role_id，默认选中第一个
    if (!selectedRoleId.value && roles.value[0]) {
      selectedRoleId.value = roles.value[0].id;
    }

    if (selectedRoleId.value) {
      await loadRolePermissions();
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载权限数据失败');
  } finally {
    loading.value = false;
  }
}

/**
 * 切换角色
 */
async function selectRole(id: number) {
  selectedRoleId.value = id;
  await loadRolePermissions();
}

/**
 * 加载当前角色的权限 ID 列表
 */
async function loadRolePermissions() {
  try {
    const ids = await getRolePermissionIds(selectedRoleId.value);
    checkedKeys.value = ids;
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载角色权限失败');
  }
}

/**
 * 获取所有权限 ID（用于全部展开）
 */
function getAllNodeIds(nodes: PermissionNode[]): Array<string | number> {
  const ids: Array<string | number> = [];
  nodes.forEach((node) => {
    ids.push(node.id);
    if (node.children && node.children.length > 0) {
      ids.push(...getAllNodeIds(node.children));
    }
  });
  return ids;
}

function expandAll() {
  expandedKeys.value = getAllNodeIds(treeData.value);
}

function collapseAll() {
  expandedKeys.value = [];
}

function clearSelection() {
  checkedKeys.value = [];
}

/**
 * 保存权限配置
 */
function save() {
  if (!selectedRoleId.value) return;

  const dialog = DialogPlugin.confirm({
    header: '确认保存权限',
    body: `确认要为角色【${selectedRole.value?.name}】更新权限配置吗？此操作将覆盖原有的权限设置。`,
    onConfirm: async () => {
      saving.value = true;
      try {
        await assignRolePermissions(selectedRoleId.value, {
          permission_ids: checkedKeys.value.map(Number),
        });
        MessagePlugin.success('权限分配成功');
        dialog.hide();
      } catch (error) {
        MessagePlugin.error((error as Error).message || '保存权限失败');
      } finally {
        saving.value = false;
      }
    },
  });
}

onMounted(load);
</script>

<style scoped>
.page-container {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.page-header h2 {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 500;
}

.page-header p {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 14px;
}

.permission-layout {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.role-panel {
  background: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
}

.role-item {
  cursor: pointer;
  transition: all 0.2s;
}

.role-item:hover {
  background: var(--td-bg-color-container-hover);
}

.role-item-active {
  background: var(--td-brand-color-light) !important;
  color: var(--td-brand-color);
}

.role-item-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.role-name {
  font-weight: 500;
}

.tree-panel {
  background: var(--td-bg-color-container);
  border-radius: var(--td-radius-medium);
  min-height: 600px;
}

.tree-toolbar {
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--td-component-border);
}

.tree-wrapper {
  padding: 8px 0;
}

.permission-code {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  margin-left: 8px;
}

@media (max-width: 960px) {
  .permission-layout {
    grid-template-columns: 1fr;
  }
}
</style>
