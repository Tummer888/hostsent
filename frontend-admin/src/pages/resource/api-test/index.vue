<template>
  <div class="page-body resource-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <AiToolIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">API 测试</h2>
          <p class="page-header__desc">对上游提供商执行连接测试，校验 API 端点与密钥是否可用。</p>
        </div>
      </div>
      <t-button variant="outline" :loading="loading" @click="loadProviders">
        <template #icon>
          <RefreshIcon aria-hidden="true" />
        </template>
        刷新
      </t-button>
    </header>

    <section class="table-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">提供商连接测试</h3>
        <span class="table-card__meta">共 {{ providers.length }} 个提供商</span>
      </div>
      <t-table row-key="id" :data="providers" :columns="columns" :loading="loading" size="small" hover bordered table-layout="fixed" cell-empty-content="—">
        <template #name="{ row }">
          <span class="cell-strong">{{ row.name }}</span>
        </template>
        <template #type="{ row }">
          <t-tag theme="primary" variant="light" size="small" shape="round">{{ row.provider_type }}</t-tag>
        </template>
        <template #endpoint="{ row }">
          <span class="cell-muted">{{ row.api_endpoint }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 1 ? 'success' : 'default'" variant="light" size="small" shape="round">{{ row.status === 1 ? '启用中' : '已停用' }}</t-tag>
        </template>
        <template #action="{ row }">
          <t-button size="small" theme="primary" variant="outline" :loading="testingId === row.id" @click="handleTest(row)">
            <template #icon>
              <LinkIcon aria-hidden="true" />
            </template>
            测试连接
          </t-button>
        </template>
        <template #empty>
          <t-empty description="暂无提供商，请先添加上游提供商" />
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { AiToolIcon, LinkIcon, RefreshIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin, type PrimaryTableCol } from 'tdesign-vue-next'

import { getProviderList, testConnection } from '@/api/admin'
import type { ProviderInfo } from '@/types/interface'

defineOptions({ name: 'ResourceApiTest' })

const providers = ref<ProviderInfo[]>([])
const loading = ref(false)
const testingId = ref<number | null>(null)

const columns: PrimaryTableCol<ProviderInfo>[] = [
  { colKey: 'name', title: '提供商', minWidth: 160 },
  { colKey: 'type', title: '类型', width: 120 },
  { colKey: 'endpoint', title: 'API 地址', minWidth: 220 },
  { colKey: 'status', title: '状态', width: 100 },
  { colKey: 'action', title: '操作', width: 150, fixed: 'right' as const, align: 'center' as const },
]

async function loadProviders() {
  loading.value = true
  try {
    const data = await getProviderList({ page: 1, page_size: 100 })
    providers.value = data.items
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载提供商失败')
  } finally {
    loading.value = false
  }
}

async function handleTest(row: ProviderInfo) {
  if (testingId.value === row.id) return
  testingId.value = row.id
  try {
    const result = await testConnection(row.id)
    if (result.success) {
      MessagePlugin.success(`「${row.name}」连接成功`)
    } else {
      MessagePlugin.error(result.message || `「${row.name}」连接失败`)
    }
  } catch (error) {
    MessagePlugin.error((error as Error).message || `「${row.name}」连接测试失败`)
  } finally {
    testingId.value = null
  }
}

onMounted(loadProviders)
</script>

<style lang="css">
@import '../shared.css';
</style>

<style scoped lang="css">
.resource-module {
  --chip-bg: linear-gradient(135deg, #0284c7, #0369a1);
  --chip-shadow: 0 4px 10px rgba(2, 132, 199, 0.25);
}
</style>
