<template>
  <div class="page-body finance-module">
    <header class="page-header surface-card">
      <div class="page-header__main">
        <span class="page-header__chip">
          <SettingIcon size="22" aria-hidden="true" />
        </span>
        <div class="page-header__text">
          <h2 class="page-header__title">财务配置</h2>
          <p class="page-header__desc">
            只列运行时真正读取的参数；每项标注生效点，保存后对相应接口立即生效
          </p>
        </div>
      </div>
      <t-space size="small">
        <t-button variant="outline" :loading="loading" @click="load">
          <template #icon><RefreshIcon aria-hidden="true" /></template>
          刷新
        </t-button>
      </t-space>
    </header>

    <section v-for="group in groups" :key="group.key" class="form-card surface-card">
      <div class="table-card__head">
        <h3 class="card-title">{{ group.label }}</h3>
        <span class="table-card__meta">config_group = {{ group.key }}</span>
      </div>
      <p class="group-hint">{{ group.hint }}</p>

      <div class="setting-list">
        <div v-for="item in group.items" :key="item.key" class="setting-item">
          <div class="setting-item__main">
            <div class="setting-item__title">
              <span>{{ item.label }}</span>
              <t-tag variant="light" size="small" shape="round" theme="default">{{ item.key }}</t-tag>
            </div>
            <p class="setting-item__desc">{{ item.description }}</p>
            <p class="setting-item__usage">{{ item.usage }}</p>
          </div>
          <div class="setting-item__control">
            <t-switch v-if="item.value_type === 'bool'" v-model="formValues[item.key]" />
            <t-input-number
              v-else
              v-model="formValues[item.key]"
              theme="column"
              :min="item.min"
              :max="item.max || undefined"
              :precision="2"
              style="width: 170px"
            />
          </div>
        </div>
      </div>
    </section>

    <section class="form-card surface-card">
      <div class="form-footer">
        <span class="form-footer__hint">
          比率类参数为 0-1 小数（如 0.10 = 10%）；保存只写白名单键，未列出的系统配置不受影响。
        </span>
        <t-space size="small">
          <t-button variant="outline" @click="load">重置</t-button>
          <t-button theme="primary" :loading="saving" @click="save">保存参数</t-button>
        </t-space>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { RefreshIcon, SettingIcon } from 'tdesign-icons-vue-next'
import { MessagePlugin } from 'tdesign-vue-next'

import { getFinanceSettings, saveFinanceSettings } from '@/api/finance'
import type { FinanceSettingGroup } from '@/types/interface'

defineOptions({ name: 'FinanceConfig' })

const loading = ref(false)
const saving = ref(false)
const groups = ref<FinanceSettingGroup[]>([])
const formValues = reactive<Record<string, string | number | boolean>>({})

function applyGroups(list: FinanceSettingGroup[]) {
  groups.value = list
  for (const group of list) {
    for (const item of group.items) {
      formValues[item.key] = item.value_type === 'bool' ? item.value === 'true' : Number(item.value)
    }
  }
}

async function load() {
  loading.value = true
  try {
    const data = await getFinanceSettings()
    applyGroups(data.groups)
  } catch (error) {
    MessagePlugin.error((error as Error).message || '加载财务参数失败')
    groups.value = []
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const payload: Record<string, string> = {}
    for (const group of groups.value) {
      for (const item of group.items) {
        const value = formValues[item.key]
        payload[item.key] = item.value_type === 'bool' ? String(Boolean(value)) : String(value ?? item.default_value)
      }
    }
    const data = await saveFinanceSettings(payload)
    applyGroups(data.groups)
    MessagePlugin.success('财务参数已保存，相关接口即时生效')
  } catch (error) {
    MessagePlugin.error((error as Error).message || '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style lang="css">
@import '../shared.css';

.finance-module .form-card {
  padding: var(--space-lg) var(--space-xl) var(--space-xl);
}

.finance-module .group-hint {
  margin: 0;
  padding: 4px 0 var(--space-sm);
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--color-muted-foreground);
}

.finance-module .setting-list {
  display: grid;
  gap: var(--space-md);
  padding-top: var(--space-sm);
}

.finance-module .setting-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-xl);
  padding: var(--space-md) var(--space-lg);
  border: 1px solid var(--color-border);
  border-radius: var(--hs-radius-md, 8px);
  background: var(--hs-surface-2);
}

.finance-module .setting-item__main {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.finance-module .setting-item__title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.finance-module .setting-item__desc {
  margin: 0;
  font-size: 12.5px;
  color: var(--color-muted-foreground);
}

.finance-module .setting-item__usage {
  margin: 0;
  font-size: 12px;
  color: var(--finance-green);
}

.finance-module .setting-item__control {
  flex-shrink: 0;
}

.finance-module .form-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-lg);
  flex-wrap: wrap;
}

.finance-module .form-footer__hint {
  font-size: 12px;
  color: var(--color-muted-foreground);
}
</style>
